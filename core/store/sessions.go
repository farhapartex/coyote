package store

import (
	"errors"
	"sync"
	"time"

	"github.com/farhapartex/coyote/core/session"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sessionStore struct {
	resolve  Resolver
	sweep    time.Duration
	stop     chan struct{}
	stopOnce sync.Once
	swept    time.Time
	mu       sync.Mutex
}

func Sessions(handle *gorm.DB, sweep time.Duration) session.ManageableStore {
	return LazySessions(func() (*gorm.DB, error) { return handle, nil }, sweep)
}

func LazySessions(resolve Resolver, sweep time.Duration) session.ManageableStore {
	return &sessionStore{resolve: resolve, sweep: sweep, stop: make(chan struct{}), swept: time.Now()}
}

func (s *sessionStore) handle() (*gorm.DB, error) {
	if s.resolve == nil {
		return nil, errors.New("coyote/store: no database resolver configured")
	}
	handle, err := s.resolve()
	if err != nil {
		return nil, err
	}
	if handle == nil {
		return nil, errors.New("coyote/store: no database connection")
	}
	s.maybeSweep(handle)
	return handle, nil
}

func (s *sessionStore) Load(id string) (*session.Session, bool) {
	handle, err := s.handle()
	if err != nil {
		return nil, false
	}
	rows := []session.Record{}
	if err := handle.Where("id = ?", id).Limit(1).Find(&rows).Error; err != nil || len(rows) == 0 {
		return nil, false
	}
	if time.Now().After(rows[0].ExpiresAt) {
		_ = s.Delete(id)
		return nil, false
	}
	restored, err := rows[0].Session()
	if err != nil {
		_ = s.Delete(id)
		return nil, false
	}
	return restored, true
}

func (s *sessionStore) Save(target *session.Session) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	record, err := session.RecordOf(target)
	if err != nil {
		return err
	}
	return handle.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "data", "expires_at"}),
	}).Create(&record).Error
}

func (s *sessionStore) Delete(id string) error {
	handle, err := s.handle()
	if err != nil {
		return err
	}
	return handle.Where("id = ?", id).Delete(&session.Record{}).Error
}

func (s *sessionStore) Count() int {
	handle, err := s.handle()
	if err != nil {
		return 0
	}
	var total int64
	if err := handle.Model(&session.Record{}).Where("expires_at > ?", time.Now()).Count(&total).Error; err != nil {
		return 0
	}
	return int(total)
}

func (s *sessionStore) All() []*session.Session {
	handle, err := s.handle()
	if err != nil {
		return nil
	}
	rows := []session.Record{}
	if err := handle.Where("expires_at > ?", time.Now()).Order("created_at desc").Find(&rows).Error; err != nil {
		return nil
	}
	out := make([]*session.Session, 0, len(rows))
	for _, row := range rows {
		restored, err := row.Session()
		if err != nil {
			continue
		}
		out = append(out, restored)
	}
	return out
}

func (s *sessionStore) DeleteByUserID(userID string) int {
	handle, err := s.handle()
	if err != nil {
		return 0
	}
	result := handle.Where("user_id = ?", userID).Delete(&session.Record{})
	if result.Error != nil {
		return 0
	}
	return int(result.RowsAffected)
}

func (s *sessionStore) Close() {
	s.stopOnce.Do(func() { close(s.stop) })
}

func (s *sessionStore) maybeSweep(handle *gorm.DB) {
	if s.sweep <= 0 {
		return
	}
	s.mu.Lock()
	due := time.Since(s.swept) >= s.sweep
	if due {
		s.swept = time.Now()
	}
	s.mu.Unlock()
	if !due {
		return
	}
	handle.Where("expires_at < ?", time.Now()).Delete(&session.Record{})
}

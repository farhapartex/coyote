package migrate

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const LedgerTable = "coyote_migrations"

type Applied struct {
	ID         string
	Checksum   string
	AppliedAt  time.Time
	DurationMS int64
}

type Ledger struct {
	handle *gorm.DB
}

func NewLedger(handle *gorm.DB) *Ledger { return &Ledger{handle: handle} }

func (l *Ledger) Ensure(ctx context.Context) error {
	statement := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (
  id VARCHAR(191) NOT NULL PRIMARY KEY,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMP NOT NULL,
  duration_ms INTEGER NOT NULL
)`, LedgerTable)
	if err := l.handle.WithContext(ctx).Exec(statement).Error; err != nil {
		return fmt.Errorf("coyote/migrate: creating %s: %w", LedgerTable, err)
	}
	return nil
}

func (l *Ledger) Exists(ctx context.Context) bool {
	if l.handle == nil {
		return false
	}
	return l.handle.WithContext(ctx).Migrator().HasTable(LedgerTable)
}

func (l *Ledger) Applied(ctx context.Context) ([]Applied, error) {
	rows := []map[string]any{}
	err := l.handle.WithContext(ctx).
		Table(LedgerTable).
		Order("id").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("coyote/migrate: reading %s: %w", LedgerTable, err)
	}
	out := make([]Applied, 0, len(rows))
	for _, row := range rows {
		record := Applied{}
		if value, ok := row["id"].(string); ok {
			record.ID = value
		}
		if value, ok := row["checksum"].(string); ok {
			record.Checksum = value
		}
		if value, ok := row["applied_at"].(time.Time); ok {
			record.AppliedAt = value
		}
		out = append(out, record)
	}
	return out, nil
}

func (l *Ledger) Record(ctx context.Context, tx *gorm.DB, m Migration, took time.Duration) error {
	err := tx.WithContext(ctx).Table(LedgerTable).Create(map[string]any{
		"id":          m.ID,
		"checksum":    m.Checksum(),
		"applied_at":  time.Now().UTC(),
		"duration_ms": took.Milliseconds(),
	}).Error
	if err != nil {
		return fmt.Errorf("coyote/migrate: recording %s: %w", m.ID, err)
	}
	return nil
}

func (l *Ledger) Forget(ctx context.Context, tx *gorm.DB, id string) error {
	err := tx.WithContext(ctx).Table(LedgerTable).Where("id = ?", id).Delete(nil).Error
	if err != nil {
		return fmt.Errorf("coyote/migrate: forgetting %s: %w", id, err)
	}
	return nil
}

package session

import "time"

type Record struct {
	ID        string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"index;size:64"`
	Data      []byte
	CreatedAt time.Time
	ExpiresAt time.Time `gorm:"index"`
}

func (Record) TableName() string { return "sessions" }

func RecordOf(s *Session) (Record, error) {
	data, err := EncodeValues(s.Values())
	if err != nil {
		return Record{}, err
	}
	return Record{
		ID:        s.ID(),
		UserID:    s.UserID(),
		Data:      data,
		CreatedAt: s.CreatedAt(),
		ExpiresAt: s.ExpiresAt(),
	}, nil
}

func (r Record) Session() (*Session, error) {
	values, err := DecodeValues(r.Data)
	if err != nil {
		return nil, err
	}
	return Restore(r.ID, values, r.CreatedAt, r.ExpiresAt), nil
}

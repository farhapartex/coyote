package jobs

import "time"

type Record struct {
	ID          string    `gorm:"primaryKey;size:64"`
	Queue       string    `gorm:"size:64;not null;index:idx_jobs_claim,priority:1"`
	Kind        string    `gorm:"size:100;not null;index"`
	State       State     `gorm:"size:20;not null;index:idx_jobs_claim,priority:2"`
	RunAt       time.Time `gorm:"not null;index:idx_jobs_claim,priority:3"`
	Priority    int       `gorm:"not null;index:idx_jobs_claim,priority:4"`
	Payload     []byte
	Attempts    int     `gorm:"not null"`
	MaxAttempts int     `gorm:"not null"`
	Fingerprint *string `gorm:"uniqueIndex;size:200" coyote:"public"`
	LockedBy    string  `gorm:"size:64"`
	LockedAt    *time.Time
	LastError   string `gorm:"size:2000"`
	FinishedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Record) TableName() string { return "jobs" }

func (r Record) Exhausted() bool {
	if r.MaxAttempts == Forever {
		return false
	}
	return r.Attempts >= r.MaxAttempts
}

func (r Record) Due(at time.Time) bool {
	return !r.RunAt.After(at)
}

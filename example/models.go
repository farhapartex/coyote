package main

import (
	"time"

	"github.com/farhapartex/coyote/core/upload"
)

type Product struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:200;not null"`
	SKU         string `gorm:"uniqueIndex;size:64;not null"`
	Price       float64
	Stock       int
	Description string     `gorm:"size:2000"`
	Photo       upload.Ref `gorm:"size:200" coyote:"path=products/photos,accept=image/*"`
	IsPublished bool       `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Checkout struct {
	ID        string `gorm:"primaryKey;size:64"`
	Reference string `gorm:"uniqueIndex;size:64;not null"`
	Total     float64
	PaidAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

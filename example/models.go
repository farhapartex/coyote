package main

import "time"

type Product struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:200;not null"`
	SKU         string `gorm:"uniqueIndex;size:64;not null"`
	Price       float64
	Stock       int
	Description string `gorm:"size:2000"`
	IsPublished bool   `gorm:"index"`
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

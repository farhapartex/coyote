package main

import "time"

type Category struct {
	ID       string `gorm:"primaryKey;size:36"`
	Name     string `gorm:"size:120;not null"`
	Slug     string `gorm:"uniqueIndex;size:120;not null"`
	Blurb    string `gorm:"size:400"`
	Position int    `gorm:"index"`
}

type Product struct {
	ID          string `gorm:"primaryKey;size:36"`
	CategoryID  string `gorm:"size:36;index"`
	Category    *Category
	Name        string `gorm:"size:200;not null"`
	Slug        string `gorm:"uniqueIndex;size:200;not null"`
	Summary     string `gorm:"size:400"`
	Description string `gorm:"size:4000"`
	PriceCents  int64  `gorm:"not null"`
	Stock       int    `gorm:"not null;index"`
	Featured    bool   `gorm:"index"`
	IsActive    bool   `gorm:"index;default:true"`
	Image       string `gorm:"size:300" coyote:"path=products,accept=image/*"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

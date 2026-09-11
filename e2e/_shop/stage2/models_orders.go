package main

import "time"

type Address struct {
	ID       string `gorm:"primaryKey;size:36"`
	UserID   string `gorm:"size:64;index;not null"`
	Label    string `gorm:"size:80"`
	Line1    string `gorm:"size:200;not null"`
	Line2    string `gorm:"size:200"`
	City     string `gorm:"size:120;not null"`
	Postcode string `gorm:"size:20;not null"`
	Country  string `gorm:"size:2;not null"`
}

type Coupon struct {
	ID         string `gorm:"primaryKey;size:36"`
	Code       string `gorm:"uniqueIndex;size:40;not null"`
	PercentOff int    `gorm:"not null"`
	IsActive   bool   `gorm:"index;default:true"`
}

type Order struct {
	ID            string `gorm:"primaryKey;size:36"`
	Reference     string `gorm:"uniqueIndex;size:20;not null"`
	UserID        string `gorm:"size:64;index;not null"`
	State         string `gorm:"size:20;not null;index"`
	Email         string `gorm:"size:320;not null"`
	ShipTo        string `gorm:"size:400;not null"`
	CouponCode    string `gorm:"size:40"`
	GoodsCents    int64  `gorm:"not null"`
	DiscountCents int64  `gorm:"not null"`
	ShippingCents int64  `gorm:"not null"`
	TotalCents    int64  `gorm:"not null"`
	PlacedAt      time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OrderLine struct {
	ID        string `gorm:"primaryKey;size:36"`
	OrderID   string `gorm:"size:36;index;not null"`
	Order     *Order
	ProductID string `gorm:"size:36;index;not null"`
	Product   *Product
	Title     string `gorm:"size:200;not null"`
	Quantity  int    `gorm:"not null"`
	UnitCents int64  `gorm:"not null"`
	LineCents int64  `gorm:"not null"`
}

type ShipmentEvent struct {
	ID         string `gorm:"primaryKey;size:36"`
	OrderID    string `gorm:"size:36;index;not null"`
	Order      *Order
	State      string    `gorm:"size:20;not null"`
	Note       string    `gorm:"size:300"`
	HappenedAt time.Time `gorm:"index"`
}

type StockMovement struct {
	ID        string `gorm:"primaryKey;size:36"`
	ProductID string `gorm:"size:36;index;not null"`
	Product   *Product
	Delta     int    `gorm:"not null"`
	Reason    string `gorm:"size:80;not null"`
	CreatedAt time.Time
}

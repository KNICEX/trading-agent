package entity

import (
	"time"
)

type Symbol struct {
	Id          int64  `gorm:"primaryKey"`
	Base        string `gorm:"index"`
	Quote       string `gorm:"index"`
	About       string
	MarketValue int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

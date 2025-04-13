package entity

import (
	"time"
)

type Symbol struct {
	Id          int64  `gorm:"primaryKey"`
	Base        string `gorm:"uniqueIndex:symbol_idx"`
	Quote       string `gorm:"uniqueIndex:symbol_idx"`
	About       string
	MarketValue int64 // 市值, 单位: 美元
	Mark        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const (
	MarkIgnore   = "ignore"
	MarkFavorite = "favorite"
)

package entity

import (
	"time"
)

// Abnormal 交易对异动
type Abnormal struct {
	Id           int64  `gorm:"primaryKey;autoIncrement"`
	BaseSymbol   string `gorm:"index"`
	QuoteSymbol  string `gorm:"index"`
	Price        string
	AbnormalType string `gorm:"index"`
	Confidence   float64
	Reason       string
	Status       int       `gorm:"index"` // 预测情况， 0:运行中， 1:成功，2:失败， 以30min后的运行情况为标准
	CreatedAt    time.Time `gorm:"index"`
	UpdatedAt    time.Time `gorm:"index"`
}

const (
	AbnormalStatusRunning = 0
	AbnormalStatusSuccess = 1
	AbnormalStatusFailed  = 2
)

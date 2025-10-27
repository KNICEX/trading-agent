package exchange

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type AccountBalance struct {
	AccountAlias     string
	Asset            string
	Balance          decimal.Decimal
	UnrealizedPnl    decimal.Decimal
	AvailableBalance decimal.Decimal
	Margin           decimal.Decimal
}

type Cursor[T any] interface {
	Next() ([]T, error)
}

type TransferHistoryType string

type Direction string

const (
	DirectionIn  Direction = "IN"
	DirectionOut Direction = "OUT"
)

type TransferStatus string

const (
	TransferStatusPending TransferStatus = "PENDING"
	TransferStatusSuccess TransferStatus = "SUCCESS"
	TransferStatusFailed  TransferStatus = "FAILED"
)

type TransferHistory struct {
	TimeStamp time.Time
	Type      TransferHistoryType
	Amount    decimal.Decimal
	Direction Direction
	Status    TransferStatus
}

type AccountService interface {
	UpdateLeverage(ctx context.Context, TradingPair TradingPair, leverage int) error
	Balances(ctx context.Context) ([]AccountBalance, error)
	TransferHistody(ctx context.Context) (Cursor[TransferHistory], error)
}

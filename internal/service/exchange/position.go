package exchange

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// https://developers.binance.com/docs/zh-CN/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V3

type MarginType string

const (
	MarginTypeIsolated MarginType = "ISOLATED"
	MarginTypeCross    MarginType = "CROSS"
)

type Position struct {
	Symbol           Symbol
	Side             OrderSide
	EntryPrice       decimal.Decimal
	BreakEvenPrice   decimal.Decimal
	MarginType       MarginType
	Leverage         int
	LiquidationPrice decimal.Decimal
	MarkPrice        decimal.Decimal
	PositionAmount   decimal.Decimal
	// 保证金
	MarginAmount     decimal.Decimal
	UnrealizedProfit decimal.Decimal
	UpdatedAt        time.Time
}

type PositionService interface {
	GetPositions(ctx context.Context) ([]Position, error)
	// 平仓api在订单部分
	ClosePositon(ctx context.Context, symbol Symbol, side OrderSide) error
}

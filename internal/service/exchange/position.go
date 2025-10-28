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
	TradingPair      TradingPair
	PositionSide     PositionSide
	EntryPrice       decimal.Decimal
	BreakEvenPrice   decimal.Decimal
	MarginType       MarginType
	Leverage         int
	LiquidationPrice decimal.Decimal
	MarkPrice        decimal.Decimal
	Quantity         decimal.Decimal
	// 保证金
	MarginAmount  decimal.Decimal
	UnrealizedPnl decimal.Decimal

	CreatedAt time.Time
	UpdatedAt time.Time
}

type PositionHistory struct {
	TradingPair  TradingPair
	PositionSide PositionSide
	EntryPrice   decimal.Decimal
	ClosePrice   decimal.Decimal
	MaxQuantity  decimal.Decimal
	OpenedAt     time.Time
	ClosedAt     time.Time

	Events []PositionEvent
}

type PositionEventType string

const (
	// 创建仓位
	PositionEventTypeCreate PositionEventType = "CREATE"
	// 增加仓位
	PositionEventTypeIncrease PositionEventType = "INCREASE"
	// 减少仓位
	PositionEventTypeDecrease PositionEventType = "DECREASE"
	// 完全平仓
	PositionEventTypeClose PositionEventType = "CLOSE"
)

type PositionEvent struct {
	OrderId        OrderId
	EventType      PositionEventType
	Quantity       decimal.Decimal
	BeforeQuantity decimal.Decimal
	AfterQuantity  decimal.Decimal
	Price          decimal.Decimal
	RealizedPnl    decimal.Decimal
	Fee            decimal.Decimal // U本位

	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt time.Time
}

type PositionService interface {
	GetActivePositions(ctx context.Context, pairs []TradingPair) ([]Position, error)

	GetHistoryPositions(ctx context.Context, req GetHistoryPositionsReq) ([]PositionHistory, error)

	SetLeverage(ctx context.Context, req SetLeverageReq) error
}

type SetLeverageReq struct {
	TradingPair TradingPair
	Leverage    int
}

type GetHistoryPositionsReq struct {
	TradingPairs []TradingPair
	StartTime    time.Time
	EndTime      time.Time
}

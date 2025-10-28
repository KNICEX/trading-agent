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
	PositionAmount   decimal.Decimal
	// 保证金
	MarginAmount     decimal.Decimal
	UnrealizedProfit decimal.Decimal
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PositionService interface {
	// GetPositionRisk 获取账户的持仓风险信息（仓位方向、仓位大小、未实现盈亏、标记价格、强平价等）
	GetActivePosition(ctx context.Context, pair TradingPair) ([]Position, error)

	GetActivePositions(ctx context.Context) ([]Position, error)

	GetHistoryPositions(ctx context.Context, req GetHistoryPositionsReq) ([]Position, error)

	ChangeLeverage(ctx context.Context, req ChangeLeverageReq) error
}

type ChangeLeverageReq struct {
	TradingPair TradingPair
	Leverage    int
}

type GetHistoryPositionsReq struct {
	TradingPair TradingPair
	Limit       int
	StartTime   time.Time
	EndTime     time.Time
}

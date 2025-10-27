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
	Symbol           TradingPair
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
	UpdatedAt        time.Time
}

type PositionService interface {
	// GetPositionRisk 获取账户的持仓风险信息（仓位方向、仓位大小、未实现盈亏、标记价格、强平价等）
	GetPositionRisk(ctx context.Context, req GetPositionRiskReq) ([]*Position, error)

	ChangeLeverage(ctx context.Context, req ChangeLeverageReq) error
}
type ChangeLeverageReq struct {
	Symbol   TradingPair
	Leverage int
}
type GetPositionRiskReq struct {
	Symbol     TradingPair
	RecvWindow int64
}
type GetPositionMarginHistoryReq struct {
	Symbol TradingPair
	Type   int // 1-加保证金 2-减保证金

}
type UpdatePositionMarginReq struct {
	Symbol TradingPair
	Amount decimal.Decimal
}
type ChangePositionModeReq struct {
	Symbol TradingPair
}
type GetPositionModeReq struct {
	Symbol TradingPair
}
type GetTopLongShortPositionRatioReq struct {
	Symbol TradingPair
}

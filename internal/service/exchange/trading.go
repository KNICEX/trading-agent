package exchange

import (
	"context"

	"github.com/shopspring/decimal"
)

// OpenPositionReq 开仓/加仓请求
type OpenPositionReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide    // LONG / SHORT
	Price        decimal.Decimal // 限价：有值则为限价单，为空则为市价单
	Quantity     decimal.Decimal // 开仓数量（具体值）

	// 使用账户余额的百分比开仓（与 Quantity 互斥）
	// 例如：BalancePercent = 50 表示使用 50% 的可用余额开仓
	BalancePercent decimal.Decimal

	// 止盈止损（可选）
	TakeProfit StopOrder // 止盈单
	StopLoss   StopOrder // 止损单
}

// ClosePositionReq 平仓请求
type ClosePositionReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide    // LONG / SHORT（平哪个方向的仓位）
	Price        decimal.Decimal // 限价：有值则为限价单，为空则为市价单

	// 平仓数量（具体值）
	Quantity decimal.Decimal

	// 平仓百分比（与 Quantity 互斥）
	// 例如：Percent = 50 表示平掉当前持仓的 50%
	Percent decimal.Decimal

	// 是否平掉该方向的全部仓位
	CloseAll bool
}

type StopOrder struct {
	Price decimal.Decimal // 触发价格
	// 内部会自动判断：有触发价格 = 市价止盈止损（TAKE_PROFIT_MARKET / STOP_MARKET）
}

func (s StopOrder) IsValid() bool {
	return !s.Price.IsZero()
}

type TradingService interface {
	// OpenPosition 开仓/加仓
	// 返回开仓订单 ID，如果设置了止盈止损，还会返回对应的订单 ID
	OpenPosition(ctx context.Context, req OpenPositionReq) (*OpenPositionResp, error)

	ClosePosition(ctx context.Context, req ClosePositionReq) (OrderId, error)

	SetStopOrders(ctx context.Context, req SetStopOrdersReq) (*SetStopOrdersResp, error)
}

type OpenPositionResp struct {
	OrderId        OrderId         // 开仓订单 ID
	TakeProfitId   OrderId         // 止盈单 ID（如果设置了）
	StopLossId     OrderId         // 止损单 ID（如果设置了）
	EstimatedCost  decimal.Decimal // 预估占用保证金
	EstimatedPrice decimal.Decimal // 预估成交价格（市价单为当前市价）
}

type SetStopOrdersReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide // 针对哪个方向的仓位
	TakeProfit   StopOrder    // 止盈单（可选）
	StopLoss     StopOrder    // 止损单（可选）
}

type SetStopOrdersResp struct {
	TakeProfitId OrderId // 止盈单 ID
	StopLossId   OrderId // 止损单 ID
}

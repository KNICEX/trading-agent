package backtest

import (
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// OrderInfo 扩展订单信息（用于回测）
type OrderInfo struct {
	exchange.OrderInfo

	// 订单类型
	OrderType    exchange.OrderType
	PositionSide exchange.PositionSide
}

// StopOrderInfo 止盈止损订单信息
type StopOrderInfo struct {
	Id           exchange.OrderId
	TradingPair  exchange.TradingPair
	PositionSide exchange.PositionSide // 关联的持仓方向

	// 止盈止损类型
	StopType  StopOrderType      // TAKE_PROFIT / STOP_LOSS
	OrderSide exchange.OrderSide // BUY / SELL (平仓方向)

	// 触发价格
	TriggerPrice decimal.Decimal

	// 成交数量（为空则平全部）
	Quantity decimal.Decimal

	// 关联的持仓
	PositionKey string
}

// StopOrderType 区分止盈止损订单类型
type StopOrderType string

const (
	StopOrderTypeTakeProfit StopOrderType = "TAKE_PROFIT" // 止盈
	StopOrderTypeStopLoss   StopOrderType = "STOP_LOSS"   // 止损
)

// PendingStopOrders 待设置的止盈止损订单（等待开仓订单成交后设置）
type PendingStopOrders struct {
	TradingPair  exchange.TradingPair
	PositionSide exchange.PositionSide
	TakeProfit   exchange.StopOrder
	StopLoss     exchange.StopOrder
	TakeProfitId exchange.OrderId // 预分配的止盈订单ID
	StopLossId   exchange.OrderId // 预分配的止损订单ID
}

package exchange

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// https://developers.binance.com/docs/zh-CN/derivatives/usds-margined-futures/trade/rest-api

type OrderId string

func (id OrderId) ToString() string {
	return string(id)
}

type OrderService interface {
	// 下单 止盈/止损
	CreateOrder(ctx context.Context, req CreateOrderReq) (OrderId, error)

	CancelOrder(ctx context.Context, id OrderId)
	CancelAllOrders(ctx context.Context)
	GetOrder(ctx context.Context, id OrderId)
	GetAllOrders(ctx context.Context)
}

type OrderSide string

const (
	OrderSideLong  OrderSide = "LONG"
	OrderSideShort OrderSide = "SHORT"
)

type OrderStatus string

// 创建一个订单，是不是就一个id，订单成交了是不是就变成一个仓位了，仓位应该有一个单独的id
// 这个时候是不是可以用之前订单的id去撤销未完全成交的订单
const (
	OrderStatusPending         = "pending"
	OrderStatusFilled          = "filled"
	OrderStatusPartiallyFilled = "partially_filled"
)

type OrderType string

const (
	OrderTypeLimit      OrderType = "LIMIT"
	OrderTypeMarket     OrderType = "MARKET"
	OrderTypeTakeProfit OrderType = "TAKE_PROFIT"
	OrderTypeStopLoss   OrderType = "STOP_LOSS"
)

type CreateOrderReq struct {
	Symbol    Symbol
	Side      OrderSide
	OrderType OrderType
	Price     decimal.Decimal // 限价单时有效
	Amount    decimal.Decimal // U本位
	Leverage  int             // 杠杆倍数， 实际仓位= U本位 * 杠杆倍数
}

type OrderInfo struct {
	Id        string
	Symbol    Symbol
	Side      OrderSide
	Price     decimal.Decimal
	Amount    decimal.Decimal
	Status    OrderStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

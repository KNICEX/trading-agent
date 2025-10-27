package exchange

import (
	"context"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// https://developers.binance.com/docs/zh-CN/derivatives/usds-margined-futures/trade/rest-api

type OrderId string

func (id OrderId) IsZero() bool {
	if id == "" {
		return true
	}
	return false
}
func (id OrderId) ToString() string {
	return string(id)
}
func (id OrderId) ToInt64() int64 {
	orderId, err := strconv.Atoi(id.ToString())
	if err != nil {
		return int64(0)
	}
	return int64(orderId)
}

// OrderService (Includes only unfulfilled orders ,except GetOrder)
type OrderService interface {
	// create
	CreateOrder(ctx context.Context, req CreateOrderReq) (OrderId, error)
	CreateBatchOrders(ctx context.Context, req []CreateOrderReq) ([]OrderId, error)

	// modify
	ModifyOrder(ctx context.Context, req ModifyOrderReq) error
	ModifyBatchOrders(ctx context.Context, req []ModifyOrderReq) error

	// get
	GetOrder(ctx context.Context, req GetOrderReq) (*OrderInfo, error)
	GetOpenOrder(ctx context.Context, req GetOpenOrderReq) (*OrderInfo, error)

	// list
	ListOrders(ctx context.Context, req ListOrdersReq) ([]OrderInfo, error)
	ListOpenOrders(ctx context.Context, req ListOpenOrdersReq) ([]OrderInfo, error)

	// cancel order
	CancelOrder(ctx context.Context, req CancelOrderReq) error                   // cancel the order with a specified id for a certain trading pair
	CancelAllOpenOrders(ctx context.Context, req CancelAllOpenOrdersReq) error   // cancel all unfulfilled orders
	CancelMultipleOrders(ctx context.Context, req CancelMultipleOrdersReq) error //batch cancel orders
}

// create req
type CreateOrderReq struct {
	Symbol      TradingPair
	Side        OrderSide
	OrderType   OrderType
	PositonSide PositionSide
	Price       decimal.Decimal // 限价单时有效
	Quantity    decimal.Decimal //  多少个交易对
	Leverage    int             // 杠杆倍数， 实际仓位= amount * 交易对price || 需要保证金= 实际仓位 /  leverage
}

// modify req
type ModifyOrderReq struct {
	Id       OrderId
	Symbol   TradingPair
	Side     OrderSide
	Price    decimal.Decimal // 限价单时有效
	Quantity decimal.Decimal //  多少个交易对
	Leverage int             // 杠杆倍数，
}

// get req
type GetOrderReq struct {
	Id     OrderId
	Symbol TradingPair
}
type GetOpenOrderReq struct {
	Id          OrderId
	TradingPair TradingPair
}

// list req
type ListOrdersReq struct {
	TradingPair TradingPair
	Limit       int
	StartTime   time.Time
	EndTime     time.Time
}
type ListOpenOrdersReq struct {
	TradingPair TradingPair
	Limit       int
	StartTime   time.Time
	EndTime     time.Time
}

// cancel req
type CancelOrderReq struct {
	TradingPair TradingPair
	Id          OrderId
}
type CancelAllOpenOrdersReq struct {
	TradingPair TradingPair
}
type CancelMultipleOrdersReq struct {
	TradingPair TradingPair
	Ids         []OrderId
}

type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
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

type OrderInfo struct {
	Id          string
	TradingPair TradingPair
	Side        OrderSide
	Price       decimal.Decimal
	Quantity    decimal.Decimal
	Status      OrderStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

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
	return id == ""
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
	CreateOrders(ctx context.Context, req []CreateOrderReq) ([]OrderId, error)

	// modify unfulfilled orders
	ModifyOrder(ctx context.Context, req ModifyOrderReq) error
	ModifyOrders(ctx context.Context, req []ModifyOrderReq) error

	// get unfulfilled orders
	GetOrder(ctx context.Context, req GetOrderReq) (OrderInfo, error)
	GetOrders(ctx context.Context, req GetOrdersReq) ([]OrderInfo, error)

	CancelOrder(ctx context.Context, req CancelOrderReq) error
	CancelOrders(ctx context.Context, req CancelOrdersReq) error
}

// create req
type CreateOrderReq struct {
	TradingPair TradingPair
	OrderType   OrderType       // OPEN / CLOSE
	PositonSide PositionSide    // LONG / SHORT
	Price       decimal.Decimal // 限价单时有效
	Quantity    decimal.Decimal
	// Side 会根据 OrderType 和 PositionSide 自动计算：
	// - OPEN + LONG = BUY
	// - OPEN + SHORT = SELL
	// - CLOSE + LONG = SELL
	// - CLOSE + SHORT = BUY
	Timestamp time.Time
}

// modify req
type ModifyOrderReq struct {
	Id          OrderId
	TradingPair TradingPair
	Side        OrderSide
	Price       decimal.Decimal // 限价单时有效
	Quantity    decimal.Decimal
}

type GetOrderReq struct {
	Id          OrderId
	TradingPair TradingPair
}

// get req
type GetOrdersReq struct {
	TradingPair TradingPair
}

type CancelOrdersReq struct {
	TradingPair TradingPair // if trading pair is empty, cancel all orders
	Ids         []OrderId   // if ids is empty, cancel all orders of the trading pair
}

type CancelOrderReq struct {
	Id          OrderId
	TradingPair TradingPair
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

// GetCloseOrderSide 根据持仓方向获取平仓订单方向
func (ps PositionSide) GetCloseOrderSide() OrderSide {
	switch ps {
	case PositionSideLong:
		return OrderSideSell // 多头平仓用卖单
	case PositionSideShort:
		return OrderSideBuy // 空头平仓用买单
	default:
		return OrderSideSell
	}
}

type OrderStatus string

// 创建一个订单，是不是就一个id，订单成交了是不是就变成一个仓位了，仓位应该有一个单独的id
// 这个时候是不是可以用之前订单的id去撤销未完全成交的订单
const (
	OrderStatusPending         OrderStatus = "pending"
	OrderStatusFilled          OrderStatus = "filled"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
)

// IsFilled 判断订单是否已完全成交
func (s OrderStatus) IsFilled() bool {
	return s == OrderStatusFilled
}

type OrderType string

const (
	OrderTypeOpen  OrderType = "OPEN"
	OrderTypeClose OrderType = "CLOSE"
)

type OrderInfo struct {
	Id               string
	TradingPair      TradingPair
	Side             OrderSide
	Price            decimal.Decimal // 限价单价格
	StopPrice        decimal.Decimal // 止盈止损触发价格（STOP/TAKE_PROFIT 类型订单使用）
	Quantity         decimal.Decimal
	ExecutedQuantity decimal.Decimal // 已成交数量
	Status           OrderStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      time.Time
}

// IsActive 判断订单是否处于活跃状态（未完全成交）
func (o *OrderInfo) IsActive() bool {
	return !o.Status.IsFilled()
}

// GetFilledPercentage 获取订单成交百分比
func (o *OrderInfo) GetFilledPercentage() decimal.Decimal {
	if o.Quantity.IsZero() {
		return decimal.Zero
	}
	return o.ExecutedQuantity.Div(o.Quantity).Mul(decimal.NewFromInt(100))
}

// ============ TradingService 相关类型定义 ============

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

	Timestamp time.Time
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

	Timestamp time.Time
}

// StopOrder 止盈止损订单
type StopOrder struct {
	Price decimal.Decimal // 触发价格
	// 内部会自动判断：有触发价格 = 市价止盈止损（TAKE_PROFIT_MARKET / STOP_MARKET）
}

func (s StopOrder) IsValid() bool {
	return !s.Price.IsZero()
}

// TradingService 交易服务接口
type TradingService interface {
	// OpenPosition 开仓/加仓
	// 返回开仓订单 ID，如果设置了止盈止损，还会返回对应的订单 ID
	OpenPosition(ctx context.Context, req OpenPositionReq) (*OpenPositionResp, error)

	// ClosePosition 平仓
	ClosePosition(ctx context.Context, req ClosePositionReq) (OrderId, error)

	// SetStopOrders 为已有仓位设置止盈止损
	SetStopOrders(ctx context.Context, req SetStopOrdersReq) (*SetStopOrdersResp, error)
}

// OpenPositionResp 开仓响应
type OpenPositionResp struct {
	OrderId        OrderId         // 开仓订单 ID
	TakeProfitId   OrderId         // 止盈单 ID（如果设置了）
	StopLossId     OrderId         // 止损单 ID（如果设置了）
	EstimatedCost  decimal.Decimal // 预估占用保证金
	EstimatedPrice decimal.Decimal // 预估成交价格（市价单为当前市价）
}

// SetStopOrdersReq 设置止盈止损请求
type SetStopOrdersReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide // 针对哪个方向的仓位
	TakeProfit   StopOrder    // 止盈单（可选）
	StopLoss     StopOrder    // 止损单（可选）
	Timestamp    time.Time
}

// SetStopOrdersResp 设置止盈止损响应
type SetStopOrdersResp struct {
	TakeProfitId OrderId // 止盈单 ID
	StopLossId   OrderId // 止损单 ID
}

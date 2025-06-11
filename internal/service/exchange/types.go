package exchange

import (
	"context"
	"fmt"
	"time"
)

// Symbol 交易对
type Symbol struct {
	Base  string
	Quote string
	Price string
}

func (s *Symbol) ToString() string {
	return fmt.Sprintf("%s%s", s.Base, s.Quote)
}

func (s *Symbol) ToSlashString() string {
	return fmt.Sprintf("%s/%s", s.Base, s.Quote)
}

type Interval string

func (i Interval) ToString() string {
	return string(i)
}

const (
	Interval1m  Interval = "1m"
	Interval3m  Interval = "3m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval30m Interval = "30m"
	Interval1h  Interval = "1h"
	Interval2h  Interval = "2h"
	Interval4h  Interval = "4h"
	Interval6h  Interval = "6h"
	Interval8h  Interval = "8h"
	Interval12h Interval = "12h"
	Interval1d  Interval = "1d"
	Interval3d  Interval = "3d"
	Interval1w  Interval = "1w"
	Interval1M  Interval = "1M"
)

type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

type PositionSide string

const (
	Long  PositionSide = "long"
	Short PositionSide = "short"
)

type Service interface {
}

type OrderService interface {
	CreateOrder(ctx context.Context, orders []Order) ([]Order, error)
	CancelOrder(ctx context.Context, orderIds []string) error
	GetOrder(ctx context.Context, orderId string) (Order, error)
	GetOpenOrders(ctx context.Context, symbol Symbol) ([]Order, error)
}

type Kline struct {
	OpenTime         time.Time
	CloseTime        time.Time
	Open             string
	Close            string
	High             string
	Low              string
	Volume           string // 成交量
	QuoteAssetVolume string // 成交额
	TradeNum         int64  // 成交笔数
}

type MarketService interface {
	GetKlines(ctx context.Context, symbol Symbol, interval Interval, startTime, endTime time.Time) ([]Kline, error)
}

type SymbolService interface {
	GetAllSymbols(ctx context.Context) ([]Symbol, error)
	GetSymbolPrice(ctx context.Context, symbol Symbol) (Symbol, error)
}

type Position struct {
	Symbol    Symbol
	Amount    string    // 持仓量
	Price     string    // 持仓均价
	Side      Side      // 持仓方向 buy/sell
	OpenTime  time.Time // 持仓开仓时间
	CloseTime time.Time // 持仓平仓时间
	Profit    string    // 持仓盈亏, quote asset
}

type OrderStatus string

const (
	OrderStatusCreated       OrderStatus = "created"        // 订单已创建
	OrderStatusPartialFilled OrderStatus = "partial_filled" // 部分成交
	OrderStatusFilled        OrderStatus = "filled"         // 订单已成交
	OrderStatusCancelled     OrderStatus = "cancelled"      // 订单已取消
)

type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"
)

type Order struct {
	Id           string
	Side         Side         // 挂单方向 buy/sell
	PositionSide PositionSide // 持仓方向 long/short

	Type   OrderType
	Symbol Symbol
	Price  string      // 挂单价格
	Amount string      // 挂单数量
	Status OrderStatus // 挂单状态

	StopLossPrice   string // 止损价格
	TakeProfitPrice string // 止盈价格
}

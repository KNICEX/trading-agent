package exchange

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"time"
)

// Symbol 交易对
type Symbol struct {
	Base  string
	Quote string
	Price decimal.Decimal
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

type Service interface {
}

type OrderService interface {
	CreateLimitBuy(ctx context.Context, symbol Symbol, amount, price float64) (string, error)
	CreateLimitSell(ctx context.Context, symbol Symbol, amount, price string)

	CreateMarketBuy()
	CreateMarketSell()

	CancelOrder()
	CancelAllOrders()
	GetOrder()
	GetOpenOrders()
	GetAllOrders()
	GetOrderBook()
}

type Kline struct {
	OpenTime         time.Time
	CloseTime        time.Time
	Open             decimal.Decimal
	Close            decimal.Decimal
	High             decimal.Decimal
	Low              decimal.Decimal
	Volume           decimal.Decimal // 成交量
	QuoteAssetVolume decimal.Decimal // 成交额
	TradeNum         int64           // 成交笔数
}

type MarketService interface {
	GetKlines(ctx context.Context, symbol Symbol, interval Interval, startTime, endTime time.Time) ([]Kline, error)
}

type SymbolService interface {
	GetAllSymbols(ctx context.Context) ([]Symbol, error)
	GetSymbolPrice(ctx context.Context, symbol Symbol) (Symbol, error)
}

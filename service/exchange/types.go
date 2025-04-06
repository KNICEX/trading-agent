package exchange

import (
	"context"
	"time"
)

// Symbol 交易对
type Symbol struct {
	Base  string
	Quote string
}

type Service interface {
}

type OrderService interface {
	CreateLimitBuy(ctx context.Context, symbol Symbol, amount, price float64) (string, error)
	CreateLimitSell()

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
	Open             float64
	Close            float64
	High             float64
	Low              float64
	Volume           float64 // 成交量
	QuoteAssetVolume float64 // 成交额
	TradeNum         int64   // 成交笔数
}

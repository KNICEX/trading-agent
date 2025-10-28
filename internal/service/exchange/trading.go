package exchange

import (
	"context"

	"github.com/shopspring/decimal"
)

type OpenTradingReq struct {
	TradingPair TradingPair
	OrderType   OrderType
	Price       decimal.Decimal // 限价单时有效
	OpenAll     bool
	Quantity    decimal.Decimal
}

type CloseTradingReq struct {
	TradingPair TradingPair
	OrderType   OrderType
	Price       decimal.Decimal // 限价单时有效
	CloseAll    bool
	Quantity    decimal.Decimal
}

type TradingService interface {
	ChangeLeverage(ctx context.Context, pair TradingPair, leverage int) error
	OpenLong(ctx context.Context, req OpenTradingReq) error
	OpenShort(ctx context.Context, req OpenTradingReq) error
	LimitClose(ctx context.Context, req CloseTradingReq) error
	MarketClose(ctx context.Context, req CloseTradingReq) error
}

package exchange

import (
	"context"
	"github.com/shopspring/decimal"
)

type TradingReq struct {
	Amount decimal.Decimal
	Price  decimal.Decimal
	Type   OrderType
	Symbol TradingPair
}
type TradingService interface {
	OpenLong(ctx context.Context, req TradingReq) (OrderId, error)
	OpenShort(ctx context.Context, req TradingReq) (OrderId, error)
	CloseLong(ctx context.Context, req TradingReq) error
	CloseShort(ctx context.Context, req TradingReq) error
}

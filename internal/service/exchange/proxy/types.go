package proxy

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

type Service interface {
	GetSymbolPrice(ctx context.Context, symbol exchange.Symbol) (string, error)
	GetKline(ctx context.Context, symbol exchange.Symbol, interval exchange.Interval, limit int) ([]exchange.Kline, error)

	MarketBuy(ctx context.Context, symbol exchange.Symbol, amount string) (bool, error)
	MarketSell(ctx context.Context, symbol exchange.Symbol, amount string) (bool, error)

	LimitBuy(ctx context.Context, symbol exchange.Symbol, price, amount string) error
	LimitSell(ctx context.Context, symbol exchange.Symbol, price, amount string) error
}

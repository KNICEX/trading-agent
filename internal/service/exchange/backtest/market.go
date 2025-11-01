package backtest

import (
	"context"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

type BinanceExchangeService struct {
	cli       *futures.Client
	startTime time.Time
	endTime   time.Time

	clock time.Time
}

func (svc *BinanceExchangeService) Ticker(ctx context.Context, tradingPair exchange.TradingPair) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (svc *BinanceExchangeService) SubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) (chan exchange.Kline, error) {
	ch := make(chan exchange.Kline, 10)

	go func() {
		defer close(ch)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				ch <- exchange.Kline{
					OpenTime:  time.Now(),
					CloseTime: time.Now(),
					Open:      decimal.Zero,
					Close:     decimal.Zero,
					High:      decimal.Zero,
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()
	return ch, nil
}

func (svc *BinanceExchangeService) UnsubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) error {
	return nil
}

func (svc *BinanceExchangeService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	return nil, nil
}

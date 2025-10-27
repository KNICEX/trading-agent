package binance

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
	"time"
)

type MarketService struct {
	cli *futures.Client
}

func (m *MarketService) convertKlines(klines []*futures.Kline) []*exchange.Kline {
	kls := make([]*exchange.Kline, len(klines))
	for i, k := range klines {
		klineOpen, err := decimal.NewFromString(k.Open)
		if err != nil {
			panic(err)
		}
		klineClose, err := decimal.NewFromString(k.Close)
		if err != nil {
			panic(err)
		}
		klineHigh, err := decimal.NewFromString(k.High)
		if err != nil {
			panic(err)
		}
		klineLow, err := decimal.NewFromString(k.Low)
		if err != nil {
			panic(err)
		}
		klineVolume, err := decimal.NewFromString(k.Volume)
		if err != nil {
			panic(err)
		}
		klineQuoteAssetVolume, err := decimal.NewFromString(k.QuoteAssetVolume)
		if err != nil {
			panic(err)
		}
		kls[i] = &exchange.Kline{
			OpenTime:         time.UnixMilli(k.OpenTime),
			CloseTime:        time.UnixMilli(k.CloseTime),
			Open:             klineOpen,
			Close:            klineClose,
			High:             klineHigh,
			Low:              klineLow,
			TradeNum:         k.TradeNum,
			Volume:           klineVolume,
			QuoteAssetVolume: klineQuoteAssetVolume,
		}
	}
	return kls
}
func (m *MarketService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]*exchange.Kline, error) {
	svc := m.cli.NewKlinesService().Symbol(req.Symbol.ToSlashString())
	if req.Interval != "" {
		svc.Interval(req.Interval.ToString())
	}
	if req.Limit != 0 {
		svc.Limit(req.Limit)
	}
	if !req.StartTime.IsZero() {
		svc.StartTime(req.StartTime.UnixMilli())
	}
	if !req.EndTime.IsZero() {
		svc.EndTime(req.EndTime.UnixMilli())
	}
	res, err := svc.Do(ctx)
	return m.convertKlines(res), err
}

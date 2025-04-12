package binance

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2"
	"github.com/samber/lo"
	"time"
)

type MarketService struct {
	cli *binance.Client
}

func NewMarketService(cli *binance.Client) *MarketService {
	return &MarketService{cli: cli}
}

func (svc *MarketService) GetKlines(ctx context.Context, symbol exchange.Symbol, interval exchange.Interval, startTime, endTime time.Time) ([]exchange.Kline, error) {
	lines, err := svc.cli.NewKlinesService().Symbol(symbol.ToString()).Interval(interval.ToString()).
		StartTime(startTime.UnixMilli()).EndTime(endTime.UnixMilli()).Do(ctx)
	if err != nil {
		return nil, err
	}
	return lo.Map(lines, func(item *binance.Kline, index int) exchange.Kline {
		return exchange.Kline{
			OpenTime:         time.UnixMilli(item.OpenTime),
			CloseTime:        time.UnixMilli(item.CloseTime),
			Open:             item.Open,
			Close:            item.Close,
			High:             item.High,
			Low:              item.Low,
			Volume:           item.Volume,
			QuoteAssetVolume: item.QuoteAssetVolume,
			TradeNum:         item.TradeNum,
		}
	}), nil
}

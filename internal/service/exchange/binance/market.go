package binance

import (
	"context"
	"log"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

var _ exchange.MarketService = (*MarketService)(nil)

type MarketService struct {
	cli *futures.Client
}

// NewMarketService 创建市场数据服务
func NewMarketService(cli *futures.Client) *MarketService {
	return &MarketService{cli: cli}
}

func (m *MarketService) convertKlines(klines []*futures.Kline) []exchange.Kline {
	kls := make([]exchange.Kline, len(klines))
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
		kls[i] = exchange.Kline{
			OpenTime:         time.UnixMilli(k.OpenTime),
			CloseTime:        time.UnixMilli(k.CloseTime),
			Open:             klineOpen,
			Close:            klineClose,
			High:             klineHigh,
			Low:              klineLow,
			Volume:           klineVolume,
			QuoteAssetVolume: klineQuoteAssetVolume,
		}
	}
	return kls
}
func (m *MarketService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	svc := m.cli.NewKlinesService().Symbol(req.TradingPair.ToString()) // 币安合约API使用 BTCUSDT 格式，不是 BTC/USDT
	if req.Interval.ToString() != "" {
		svc.Interval(req.Interval.ToString())
	}
	if !req.StartTime.IsZero() {
		svc.StartTime(req.StartTime.UnixMilli())
	}
	if !req.EndTime.IsZero() {
		svc.EndTime(req.EndTime.UnixMilli())
	}
	res, err := svc.Do(ctx)
	if err != nil {
		return nil, err
	}
	return m.convertKlines(res), nil
}

func (m *MarketService) SubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) (chan exchange.Kline, error) {
	ch := make(chan exchange.Kline, 10)

	// 启动WebSocket订阅
	doneC, stopC, err := futures.WsKlineServe(
		tradingPair.ToString(),
		interval.ToString(),
		func(event *futures.WsKlineEvent) {
			// 只处理已关闭的K线
			if !event.Kline.IsFinal {
				return
			}

			// 转换K线数据
			klineOpen, err := decimal.NewFromString(event.Kline.Open)
			if err != nil {
				return
			}
			klineClose, err := decimal.NewFromString(event.Kline.Close)
			if err != nil {
				return
			}
			klineHigh, err := decimal.NewFromString(event.Kline.High)
			if err != nil {
				return
			}
			klineLow, err := decimal.NewFromString(event.Kline.Low)
			if err != nil {
				return
			}
			klineVolume, err := decimal.NewFromString(event.Kline.Volume)
			if err != nil {
				return
			}
			klineQuoteAssetVolume, err := decimal.NewFromString(event.Kline.QuoteVolume)
			if err != nil {
				return
			}

			kline := exchange.Kline{
				OpenTime:         time.UnixMilli(event.Kline.StartTime),
				CloseTime:        time.UnixMilli(event.Kline.EndTime),
				Open:             klineOpen,
				Close:            klineClose,
				High:             klineHigh,
				Low:              klineLow,
				Volume:           klineVolume,
				QuoteAssetVolume: klineQuoteAssetVolume,
			}

			// 发送K线数据到channel
			select {
			case ch <- kline:
			case <-ctx.Done():
				return
			}
		},
		func(err error) {
			log.Fatalf("ws kline error: %v", err)
		},
	)

	if err != nil {
		close(ch)
		return nil, err
	}

	// 启动协程处理context取消和清理
	go func() {
		select {
		case <-ctx.Done():
			// context被取消，关闭WebSocket
			close(stopC)
			close(ch)
		case <-doneC:
			// WebSocket连接断开
			close(ch)
		}
	}()

	return ch, nil
}

func (m *MarketService) Ticker(ctx context.Context, tradingPair exchange.TradingPair) (decimal.Decimal, error) {
	prices, err := m.cli.NewListPricesService().Symbol(tradingPair.ToString()).Do(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromString(prices[0].Price)
}

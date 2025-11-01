package backtest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

type BinanceExchangeService struct {
	cli       *futures.Client
	startTime time.Time
	endTime   time.Time

	clockMu sync.RWMutex
	clock   time.Time

	clockUpdateCallbacks []func(t time.Time)
	timeMultiplier       int
}

func NewBinanceExchangeService(cli *futures.Client, startTime, endTime time.Time, timeMultiplier int) *BinanceExchangeService {
	svc := &BinanceExchangeService{
		cli:                  cli,
		startTime:            startTime,
		endTime:              endTime,
		timeMultiplier:       timeMultiplier,
		clock:                startTime,
		clockUpdateCallbacks: []func(t time.Time){},
	}
	svc.clockLoop()
	return svc
}

func (svc *BinanceExchangeService) now() time.Time {
	svc.clockMu.RLock()
	defer svc.clockMu.RUnlock()
	return svc.clock
}

func (svc *BinanceExchangeService) updateClock(t time.Time) {
	svc.clockMu.Lock()
	defer svc.clockMu.Unlock()
	svc.clock = t
	go func() {
		for _, callback := range svc.clockUpdateCallbacks {
			callback(t)
		}
	}()
}

func (svc *BinanceExchangeService) onClockUpdate(callback func(t time.Time)) {
	svc.clockUpdateCallbacks = append(svc.clockUpdateCallbacks, callback)
}

// clockLoop 定时更新clock
func (svc *BinanceExchangeService) clockLoop() {
	startTime := svc.startTime
	go func() {
		baseInterval := time.Millisecond * 100
		for range time.Tick(baseInterval) {
			svc.updateClock(startTime.Add(baseInterval * time.Duration(svc.timeMultiplier)))
		}
	}()
}

func (svc *BinanceExchangeService) Ticker(ctx context.Context, tradingPair exchange.TradingPair) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (svc *BinanceExchangeService) SubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) (chan exchange.Kline, error) {
	ch := make(chan exchange.Kline, 10)

	svc.onClockUpdate(func(t time.Time) {

		if t.Unix()%int64(interval.Duration().Seconds()) > 10 {
			// 误差大于10秒，跳过
			return
		}

		closeTime := t.Truncate(interval.Duration())
		openTime := closeTime.Add(-interval.Duration())

		klines, err := svc.GetKlines(ctx, exchange.GetKlinesReq{
			TradingPair: tradingPair,
			Interval:    interval,
			StartTime:   openTime,
			EndTime:     closeTime,
		})
		if err != nil {
			fmt.Println("get klines error", err)
			return
		}
		if len(klines) == 0 {
			fmt.Println("no klines found for ", openTime, " to ", closeTime)
			return
		}
		ch <- klines[0]
	})
	return ch, nil
}

func (svc *BinanceExchangeService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	return nil, nil
}

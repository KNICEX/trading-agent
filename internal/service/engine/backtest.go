package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/analytics"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/exchange/backtest"
	"github.com/KNICEX/trading-agent/internal/service/portfolio"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
)

var _ strategy.Context = (*BacktestContext)(nil)

type BacktestContext struct {
	tradingPair exchange.TradingPair
	marketSvc   exchange.MarketService
	positionSvc exchange.PositionService

	clockMu sync.RWMutex
	clock   time.Time
}

func NewBacktestContext(marketSvc exchange.MarketService, positionSvc exchange.PositionService) *BacktestContext {
	return &BacktestContext{
		marketSvc:   marketSvc,
		positionSvc: positionSvc,
	}
}

func (c *BacktestContext) GetKlines(ctx context.Context, req strategy.GetKlinesReq) ([]exchange.Kline, error) {
	return c.marketSvc.GetKlines(ctx, exchange.GetKlinesReq{
		TradingPair: c.tradingPair,
		Interval:    req.Interval,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
	})
}

func (c *BacktestContext) GetPositions(ctx context.Context) ([]exchange.Position, error) {
	return c.positionSvc.GetActivePositions(ctx, []exchange.TradingPair{})
}

func (c *BacktestContext) Now() time.Time {
	c.clockMu.RLock()
	defer c.clockMu.RUnlock()
	return c.clock
}

func (c *BacktestContext) setTime(t time.Time) {
	c.clockMu.Lock()
	defer c.clockMu.Unlock()
	c.clock = t
}

func (c *BacktestContext) TradingPair() exchange.TradingPair {
	return c.tradingPair
}

type BacktestEngine struct {
	exchangeSvc exchange.Service

	strategies    []strategy.Strategy
	positionSizer portfolio.PositionSizer

	executor *Executor

	startTime time.Time
	endTime   time.Time
}

func NewBacktestEngine(startTime, endTime time.Time, exchangeSvc exchange.Service) *BacktestEngine {
	return &BacktestEngine{
		exchangeSvc: exchangeSvc,
		startTime:   startTime,
		endTime:     endTime,
		executor: &Executor{
			tradingSvc:  exchange.NewTradingService(exchangeSvc, &backtest.PercisionProvider{}),
			orderSvc:    exchangeSvc.OrderService(),
			positionSvc: exchangeSvc.PositionService(),
		},
	}
}

func (e *BacktestEngine) Run(ctx context.Context) error {
	analyzer := analytics.NewAnalyzer(e.exchangeSvc)
	err := analyzer.Initialize(ctx)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for _, sg := range e.strategies {

		wg.Add(1)
		go func() {
			defer wg.Done()
			sgCtx := &BacktestContext{
				tradingPair: sg.TradingPair(),
				marketSvc:   e.exchangeSvc.MarketService(),
				positionSvc: e.exchangeSvc.PositionService(),
				clock:       e.startTime,
			}

			err := sg.Initialize(ctx, sgCtx)
			if err != nil {
				return
			}
			klineChan, err := e.exchangeSvc.MarketService().
				SubscribeKline(ctx, sg.TradingPair(), sg.Interval())
			if err != nil {
				return
			}

			for {
				select {
				case <-ctx.Done():
					return
				case kline, ok := <-klineChan:
					if !ok {
						return
					}
					// 更新ctx时钟
					sgCtx.setTime(kline.CloseTime)

					if kline.CloseTime.After(e.endTime) {
						// 结束回测，生成报告
						return
					}
					signal, err := sg.OnKline(context.Background(), kline)
					if err != nil {
						continue
					}

					if signal.Action == strategy.SignalActionHold {
						// do nothing
						continue
					}

					fmt.Println("signal", signal)

					enhancedSignal, err := e.positionSizer.HandleSignal(context.Background(), signal)
					if err != nil {
						continue
					}

					if !enhancedSignal.Validated {
						fmt.Println("enhancedSignal not validated", enhancedSignal.Reason)
						continue
					}

					err = e.executor.Execute(context.Background(), enhancedSignal.EnhancedSignal)
					if err != nil {
						fmt.Println("execute error", err)
						continue
					}

				}
			}
		}()
	}

	wg.Wait()

	report, err := analyzer.Analyze(ctx)
	if err != nil {
		return err
	}
	fmt.Println(report.String())

	return nil
}

func (e *BacktestEngine) Stop(ctx context.Context) error {
	return nil
}

func (e *BacktestEngine) AddStrategy(ctx context.Context, strategy strategy.Strategy) error {
	e.strategies = append(e.strategies, strategy)
	return nil
}

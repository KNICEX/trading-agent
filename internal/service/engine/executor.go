package engine

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/portfolio"
)

type Executor struct {
	tradingSvc  exchange.TradingService
	orderSvc    exchange.OrderService
	positionSvc exchange.PositionService
}

func (e *Executor) Execute(ctx context.Context, signal portfolio.EnhancedSignal) error {
	_, err := e.tradingSvc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  signal.TradingPair,
		PositionSide: signal.PositionSide,
		Quantity:     signal.Quantity,
		TakeProfit: exchange.StopOrder{
			Price: signal.TakeProfit,
		},
		StopLoss: exchange.StopOrder{
			Price: signal.StopLoss,
		},
	})
	return err
}

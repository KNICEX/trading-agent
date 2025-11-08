package engine

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/portfolio"
)

type Executor struct {
	tradingSvc  exchange.TradingService
	orderSvc    exchange.OrderService
	positionSvc exchange.PositionService
}

func (e *Executor) Execute(ctx context.Context, signal portfolio.EnhancedSignal) error {
	// 1. 获取当前持仓
	positions, err := e.positionSvc.GetActivePositions(ctx, []exchange.TradingPair{signal.TradingPair})
	if err != nil {
		return fmt.Errorf("failed to get active positions: %w", err)
	}

	// 2. 查找多单和空单
	var longPosition, shortPosition *exchange.Position
	for i := range positions {
		pos := &positions[i]
		if !pos.Quantity.IsZero() {
			if pos.PositionSide == exchange.PositionSideLong {
				longPosition = pos
			} else if pos.PositionSide == exchange.PositionSideShort {
				shortPosition = pos
			}
		}
	}

	// 3. 根据信号类型和当前持仓执行不同的操作
	if signal.PositionSide == exchange.PositionSideLong {
		// 信号是做多
		if shortPosition != nil {
			// 有空单，先平空
			fmt.Println("检测到空单，先平空单再开多单")
			err := e.closePosition(ctx, signal.TradingPair, exchange.PositionSideShort, shortPosition.Quantity.Abs())
			if err != nil {
				return fmt.Errorf("failed to close short position: %w", err)
			}

			// 撤掉所有订单
			err = e.orderSvc.CancelOrders(ctx, exchange.CancelOrdersReq{
				TradingPair: signal.TradingPair,
			})
			if err != nil {
				return fmt.Errorf("failed to cancel short position order: %w", err)
			}
		}

		// 开多单或加多仓
		if longPosition != nil {
			fmt.Println("加多仓")
		} else {
			fmt.Println("开多单")
		}
		return e.openPosition(ctx, signal)

	} else if signal.PositionSide == exchange.PositionSideShort {
		// 信号是做空
		if longPosition != nil {
			// 有多单，先平多
			fmt.Println("检测到多单，先平多单再开空单")
			err := e.closePosition(ctx, signal.TradingPair, exchange.PositionSideLong, longPosition.Quantity.Abs())
			if err != nil {
				return fmt.Errorf("failed to close long position: %w", err)
			}

			err = e.orderSvc.CancelOrders(ctx, exchange.CancelOrdersReq{
				TradingPair: signal.TradingPair,
			})
			if err != nil {
				return fmt.Errorf("failed to cancel long position order: %w", err)
			}
		}

		// 开空单或加空仓
		if shortPosition != nil {
			fmt.Println("加空仓")
		} else {
			fmt.Println("开空单")
		}
		return e.openPosition(ctx, signal)
	}

	return fmt.Errorf("unsupported position side: %s", signal.PositionSide)
}

// openPosition 开仓或加仓
func (e *Executor) openPosition(ctx context.Context, signal portfolio.EnhancedSignal) error {
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
		Timestamp: signal.Timestamp,
	})
	return err
}

// closePosition 平仓
func (e *Executor) closePosition(ctx context.Context, tradingPair exchange.TradingPair, positionSide exchange.PositionSide, quantity interface{}) error {
	_, err := e.tradingSvc.ClosePosition(ctx, exchange.ClosePositionReq{
		TradingPair:  tradingPair,
		PositionSide: positionSide,
		CloseAll:     true, // 全部平仓
	})
	return err
}

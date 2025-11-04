package backtest

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

// ============ PositionService 实现 ============

// GetActivePositions 获取活跃持仓
func (svc *BinanceExchangeService) GetActivePositions(ctx context.Context, pairs []exchange.TradingPair) ([]exchange.Position, error) {
	svc.positionMu.RLock()
	defer svc.positionMu.RUnlock()

	var positions []exchange.Position

	if len(pairs) == 0 {
		// 返回所有持仓
		for _, pos := range svc.positions {
			positions = append(positions, *pos)
		}
	} else {
		// 只返回指定交易对的持仓
		for _, pair := range pairs {
			for _, pos := range svc.positions {
				if pos.TradingPair == pair {
					positions = append(positions, *pos)
				}
			}
		}
	}

	return positions, nil
}

// GetHistoryPositions 获取历史持仓
func (svc *BinanceExchangeService) GetHistoryPositions(ctx context.Context, req exchange.GetHistoryPositionsReq) ([]exchange.PositionHistory, error) {
	// 回测模式：返回所有历史持仓
	return svc.positionHistories, nil
}

// SetLeverage 设置杠杆（回测模式：固定杠杆为1）
func (svc *BinanceExchangeService) SetLeverage(ctx context.Context, req exchange.SetLeverageReq) error {
	// 回测模式不支持修改杠杆
	return nil
}

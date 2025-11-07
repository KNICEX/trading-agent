package backtest

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

// ============ PositionService 实现 ============

// GetActivePositions 获取活跃持仓
func (svc *ExchangeService) GetActivePositions(ctx context.Context, pairs []exchange.TradingPair) ([]exchange.Position, error) {
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
func (svc *ExchangeService) GetHistoryPositions(ctx context.Context, req exchange.GetHistoryPositionsReq) ([]exchange.PositionHistory, error) {
	// 回测模式：返回所有历史持仓
	return svc.positionHistories, nil
}

// SetLeverage 设置杠杆
func (svc *ExchangeService) SetLeverage(ctx context.Context, req exchange.SetLeverageReq) error {
	if req.Leverage < 1 || req.Leverage > 125 {
		return fmt.Errorf("invalid leverage: %d, must be between 1 and 125", req.Leverage)
	}

	svc.leverageMu.Lock()
	svc.leverages[req.TradingPair.ToString()] = req.Leverage
	svc.leverageMu.Unlock()

	// 如果已有持仓，更新持仓的杠杆（但不改变保证金，因为保证金已锁定）
	svc.positionMu.Lock()
	for key, position := range svc.positions {
		if position.TradingPair == req.TradingPair {
			position.Leverage = req.Leverage
			svc.positions[key] = position
		}
	}
	svc.positionMu.Unlock()

	return nil
}

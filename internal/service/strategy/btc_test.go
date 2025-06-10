package strategy

import (
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/exchange/proxy"
	"github.com/shopspring/decimal"
)

type StrategyStatus int

const (
	StrategyStatusCreated StrategyStatus = 1
	// 分批建仓中
	StrategyStatusBuilding StrategyStatus = 2
	// 持仓中
	StrategyStatusHolding StrategyStatus = 3
	// 分批卖出中
	StrategyStatusSelling StrategyStatus = 4
	// 止盈
	StrategyStatusTakeProfit StrategyStatus = 5
	// 止损
	StrategyStatusStopLoss StrategyStatus = 6
	// 策略不符合预期
	StrategyStatusInvalid StrategyStatus = 7
)

type Strategy struct {
	Side exchange.Side
	// 交易对
	Symbol    exchange.Symbol
	CreatedAt time.Time

	// 持仓
	Position exchange.Position
	// 买入挂单
	BuyOrders []exchange.Order
	// 卖出挂单
	SellOrders []exchange.Order

	Status StrategyStatus // 状态

	IsValid bool // 是否有效
}

type PriceZone struct {
	High decimal.Decimal
	Low  decimal.Decimal
}

type BtcLongTestStrategy struct {
	service proxy.Service
	symbol  exchange.Symbol

	strategy Strategy
}

/**

1. 检查价格生成买卖区间， 作为一个strategy
2. strategy追踪, 挂单，
3. strategy定时检查，如果判定strategy不再有效，取消挂单，平仓
4. strategy有

*/

func (s *BtcLongTestStrategy) Init(service proxy.Service) error {
	s.service = service
	s.symbol = exchange.Symbol{
		Base:  "BTC",
		Quote: "USDT",
	}

	return nil
}

func (s *BtcLongTestStrategy) Exec() error {
	if !s.strategy.IsValid {
		// 没有运行中的策略，尝试生成一个新的策略
		strategy, ok, err := s.generateStrategy()
		if err != nil {
			return err
		}

		if ok {
			s.strategy = strategy
		} else {
			time.Sleep(time.Minute)
			return nil
		}
	}

	if err := s.adjustStrategy(); err != nil {
		return err
	}

	switch s.strategy.Status {
	case StrategyStatusCreated, StrategyStatusBuilding, StrategyStatusHolding:
		return nil
	case StrategyStatusTakeProfit:
		// 止盈，记录报告，
		s.strategy.IsValid = false
	case StrategyStatusStopLoss:
		// 止损，记录报告，
		s.strategy.IsValid = false
	case StrategyStatusInvalid:
		// 策略不符合预期，取消挂单，平仓
		s.strategy.IsValid = false
	default:
		// 未知状态，记录日志
	}
	return nil
}

func (s *BtcLongTestStrategy) generateStrategy() (Strategy, bool, error) {
	// todo: 检查市场，是否可以生成买入或卖出策略

	// 创建买入卖出挂单（包含止损）
	return Strategy{}, false, nil
}

func (s *BtcLongTestStrategy) adjustStrategy() error {
	// 检查策略时候还有效
	// 检查是否需要调整止盈止损
	return nil
}

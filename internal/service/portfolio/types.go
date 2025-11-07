package portfolio

import (
	"context"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"github.com/shopspring/decimal"
)

type RiskConfig struct {
	// 最大止损全仓资金比例
	MaxStopLossRatio float64

	// 最大止损金额
	// MaxStopLossAmount float64

	// 全仓最大杠杆
	MaxLeverage int

	// 单仓位最大总资金比例(杠杆前)
	// MaxPositionRatio float64

	// 最小盈亏比(仅限 止盈止损订单有效， 跟踪止盈无效)
	MinProfitLossRatio float64

	// 置信度阈值 > 50
	ConfidenceThreshold float64
}

type PositionSizer interface {
	Initialize(ctx context.Context, riskConfig RiskConfig) error
	// 增强信号 计算止损以及需要成交的quantity
	HandleSignal(ctx context.Context, signal strategy.Signal) (HandleSignalResult, error)
}

type HandleSignalResult struct {
	EnhancedSignal EnhancedSignal
	Validated      bool   // 是否通过风控
	Reason         string // 风控理由
}

type EnhancedSignal struct {
	TradingPair  exchange.TradingPair
	PositionSide exchange.PositionSide
	Quantity     decimal.Decimal
	TakeProfit   decimal.Decimal
	StopLoss     decimal.Decimal // 必须不为0
	Timestamp    time.Time
}

package strategy

import (
	"context"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// Strategy 策略接口
type Strategy interface {
	// Name 策略名称（唯一标识）
	Name() string

	TradingPair() exchange.TradingPair

	// Interval 策略运行的K线周期
	// 外部应该根据这个周期订阅K线数据来驱动策略
	Interval() exchange.Interval

	// Initialize 初始化策略
	// 在策略启动时调用，可以加载历史数据、初始化指标等
	Initialize(ctx context.Context, strategyCtx Context) error

	OnKline(ctx context.Context, kline exchange.Kline) (Signal, error)

	Shutdown(ctx context.Context) error
}

// SignalAction 信号动作（简化版：只有6种操作）
type SignalAction string

const (
	// 无持仓时的操作
	SignalActionLong  SignalAction = "LONG"  // 做多（开多仓）
	SignalActionShort SignalAction = "SHORT" // 做空（开空仓）
	SignalActionHold  SignalAction = "HOLD"  // 观望（不操作）

	// 有持仓时的操作
	// SignalActionAdd    SignalAction = "ADD"    // 加仓
	// SignalActionReduce SignalAction = "REDUCE" // 减仓
	// SignalActionClose  SignalAction = "CLOSE"  // 平仓
)

// Signal 交易信号（简化版）
type Signal struct {
	TradingPair exchange.TradingPair
	Action      SignalAction
	Timestamp   time.Time

	// 置信度
	Confidence float64

	// 止盈止损（可选）
	TakeProfit decimal.Decimal
	StopLoss   decimal.Decimal

	Reason string // 信号原因（用于日志和分析）
	// 元数据
	Metadata map[string]any
}

// Context 策略上下文
type Context interface {
	Now() time.Time

	TradingPair() exchange.TradingPair

	// GetKlines 获取历史K线数据
	GetKlines(ctx context.Context, req GetKlinesReq) ([]exchange.Kline, error)

	// GetPositions 获取当前持仓
	GetPositions(ctx context.Context) ([]exchange.Position, error)
}

type GetKlinesReq struct {
	Interval  exchange.Interval
	StartTime time.Time
	EndTime   time.Time
}

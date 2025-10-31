package strategy

import (
	"context"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// Strategy 策略接口
// 策略只负责产生信号，不直接执行交易
// 这样可以统一做风控、回测、日志记录
type Strategy interface {
	// Name 策略名称（唯一标识）
	Name() string

	// Initialize 初始化策略
	// 在策略启动时调用，可以加载历史数据、初始化指标等
	Initialize(ctx context.Context, strategyCtx Context) error

	// OnOrder 订单状态变化时调用（用于策略感知订单执行情况）
	OnOrder(ctx context.Context, order *OrderUpdate) error

	// OnPosition 持仓状态变化时调用（用于策略感知持仓变化）
	OnPosition(ctx context.Context, position *PositionUpdate) error

	// Shutdown 策略关闭时调用
	// 用于清理资源、保存状态等
	Shutdown(ctx context.Context) error
}

// ================ 策略输入数据 ================

// Bar K线数据（策略的主要输入）
type Bar struct {
	TradingPair exchange.TradingPair
	Interval    exchange.Interval
	Kline       *exchange.Kline
}

// OrderUpdate 订单状态更新
type OrderUpdate struct {
	OrderId   exchange.OrderId
	Order     exchange.OrderInfo
	Timestamp time.Time
}

// PositionUpdate 持仓状态更新
type PositionUpdate struct {
	Position  exchange.Position
	Timestamp time.Time
}

// ================ 策略输出信号 ================

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

	// 元数据
	Reason   string // 信号原因（用于日志和分析）
	Metadata map[string]any
}

// IsValid 检查信号是否有效
func (s *Signal) IsValid() bool {
	if s.TradingPair.IsZero() {
		return false
	}
	if s.Action == "" {
		return false
	}
	return true
}

// ================ 策略上下文 ================

// Context 策略上下文
// 为策略提供所有需要的资源访问接口
// 在回测和实盘时有不同的实现
type Context interface {
	// ========== 市场数据访问 ==========

	// GetKlines 获取历史K线数据
	GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error)

	// GetPositions 获取当前持仓
	GetPositions(ctx context.Context, pair exchange.TradingPair) (exchange.Position, error)
}

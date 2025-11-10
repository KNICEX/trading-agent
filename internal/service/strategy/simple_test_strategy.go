package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// SimpleTestStrategy 简单的测试策略
// 使用双均线交叉策略：短期均线上穿长期均线做多，下穿做空
type SimpleTestStrategy struct {
	name        string
	tradingPair exchange.TradingPair

	// 策略参数
	shortPeriod int // 短期均线周期
	longPeriod  int // 长期均线周期
	interval    exchange.Interval

	// 缓存的K线数据
	klines []exchange.Kline

	// 上一次的信号，避免重复信号
	lastSignal SignalAction
}

// NewSimpleTestStrategy 创建一个简单测试策略
func NewSimpleTestStrategy(tradingPair exchange.TradingPair) *SimpleTestStrategy {
	return &SimpleTestStrategy{
		name:        "simple_test_strategy",
		tradingPair: tradingPair,
		shortPeriod: 5,  // 5周期短期均线
		longPeriod:  20, // 20周期长期均线
		interval:    exchange.Interval1h,
		klines:      make([]exchange.Kline, 0, 100),
		lastSignal:  SignalActionHold,
	}
}

// Name 策略名称
func (s *SimpleTestStrategy) Name() string {
	return s.name
}

// TradingPair 交易对
func (s *SimpleTestStrategy) TradingPair() exchange.TradingPair {
	return s.tradingPair
}

// Interval 策略运行的K线周期
func (s *SimpleTestStrategy) Interval() exchange.Interval {
	return s.interval
}

// Initialize 初始化策略
func (s *SimpleTestStrategy) Initialize(ctx context.Context, strategyCtx Context) error {
	// 获取足够的历史数据来计算长期均线
	endTime := strategyCtx.Now()
	startTime := endTime.Add(-time.Duration(s.longPeriod*2) * s.interval.Duration())

	klines, err := strategyCtx.GetKlines(ctx, GetKlinesReq{
		Interval:  s.interval,
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		return fmt.Errorf("failed to get initial klines: %w", err)
	}

	s.klines = klines
	return nil
}

// OnKline 处理新的K线数据
func (s *SimpleTestStrategy) OnKline(ctx context.Context, kline exchange.Kline) (Signal, error) {
	// 添加新K线到缓存
	s.klines = append(s.klines, kline)

	// 只保留需要的数量
	if len(s.klines) > s.longPeriod*2 {
		s.klines = s.klines[1:]
	}

	// 数据不足，无法计算
	if len(s.klines) < s.longPeriod {
		return Signal{
			TradingPair: s.tradingPair,
			Action:      SignalActionHold,
			Timestamp:   kline.OpenTime,
			Confidence:  0,
			Reason:      "insufficient data for calculation",
		}, nil
	}

	// 计算当前的短期和长期均线
	shortMA := s.calculateSMA(s.shortPeriod)
	longMA := s.calculateSMA(s.longPeriod)

	// 计算前一根K线的均线（用于判断交叉）
	prevShortMA := s.calculateSMAAt(s.shortPeriod, len(s.klines)-2)
	prevLongMA := s.calculateSMAAt(s.longPeriod, len(s.klines)-2)

	var action SignalAction
	var reason string
	confidence := 0.7 // 固定置信度

	// 判断交叉信号
	if prevShortMA.LessThanOrEqual(prevLongMA) && shortMA.GreaterThan(longMA) {
		// 金叉：做多
		action = SignalActionLong
		reason = fmt.Sprintf("golden cross: short MA(%.2f) crosses above long MA(%.2f)",
			shortMA.InexactFloat64(), longMA.InexactFloat64())
	} else if prevShortMA.GreaterThanOrEqual(prevLongMA) && shortMA.LessThan(longMA) {
		// 死叉：做空
		action = SignalActionShort
		reason = fmt.Sprintf("death cross: short MA(%.2f) crosses below long MA(%.2f)",
			shortMA.InexactFloat64(), longMA.InexactFloat64())
	} else {
		// 无交叉：观望
		action = SignalActionHold
		reason = fmt.Sprintf("no cross: short MA(%.2f), long MA(%.2f)",
			shortMA.InexactFloat64(), longMA.InexactFloat64())
	}

	// 避免重复信号
	if action == s.lastSignal && action != SignalActionHold {
		action = SignalActionHold
		reason = "duplicate signal, hold"
		confidence = 0
	}

	s.lastSignal = action

	// 设置简单的止盈止损（基于当前价格的百分比）
	currentPrice := kline.Close
	takeProfit := decimal.Zero
	stopLoss := decimal.Zero

	if action == SignalActionLong {
		takeProfit = currentPrice.Mul(decimal.NewFromFloat(1.02)) // 2% 止盈
		stopLoss = currentPrice.Mul(decimal.NewFromFloat(0.99))   // 1% 止损
	} else if action == SignalActionShort {
		takeProfit = currentPrice.Mul(decimal.NewFromFloat(0.98)) // 2% 止盈
		stopLoss = currentPrice.Mul(decimal.NewFromFloat(1.01))   // 1% 止损
	}

	return Signal{
		TradingPair: s.tradingPair,
		Action:      action,
		Timestamp:   kline.OpenTime,
		Confidence:  confidence,
		TakeProfit:  takeProfit,
		StopLoss:    stopLoss,
		Reason:      reason,
		Metadata: map[string]any{
			"short_ma":    shortMA.String(),
			"long_ma":     longMA.String(),
			"close_price": currentPrice.String(),
		},
	}, nil
}

// Shutdown 关闭策略
func (s *SimpleTestStrategy) Shutdown(ctx context.Context) error {
	// 清理资源
	s.klines = nil
	return nil
}

// calculateSMA 计算简单移动平均线（使用最新的n个K线）
func (s *SimpleTestStrategy) calculateSMA(period int) decimal.Decimal {
	return s.calculateSMAAt(period, len(s.klines)-1)
}

// calculateSMAAt 计算指定位置的简单移动平均线
func (s *SimpleTestStrategy) calculateSMAAt(period int, endIndex int) decimal.Decimal {
	if endIndex < period-1 {
		return decimal.Zero
	}

	sum := decimal.Zero
	for i := endIndex - period + 1; i <= endIndex; i++ {
		sum = sum.Add(s.klines[i].Close)
	}

	return sum.Div(decimal.NewFromInt(int64(period)))
}

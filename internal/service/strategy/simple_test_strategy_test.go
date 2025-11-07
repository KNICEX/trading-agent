package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStrategyContext 模拟策略上下文
type MockStrategyContext struct {
	mock.Mock
}

func (m *MockStrategyContext) Now() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockStrategyContext) TradingPair() exchange.TradingPair {
	args := m.Called()
	return args.Get(0).(exchange.TradingPair)
}

func (m *MockStrategyContext) GetKlines(ctx context.Context, req GetKlinesReq) ([]exchange.Kline, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]exchange.Kline), args.Error(1)
}

func (m *MockStrategyContext) GetPositions(ctx context.Context) ([]exchange.Position, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]exchange.Position), args.Error(1)
}

// generateTestKlines 生成测试用的K线数据
func generateTestKlines(basePrice float64, count int, trend string) []exchange.Kline {
	klines := make([]exchange.Kline, count)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < count; i++ {
		var price float64
		switch trend {
		case "up":
			price = basePrice + float64(i)*0.5 // 上升趋势
		case "down":
			price = basePrice - float64(i)*0.5 // 下降趋势
		default:
			price = basePrice // 横盘
		}

		klines[i] = exchange.Kline{
			OpenTime:  baseTime.Add(time.Duration(i) * time.Minute),
			Open:      decimal.NewFromFloat(price),
			High:      decimal.NewFromFloat(price + 1),
			Low:       decimal.NewFromFloat(price - 1),
			Close:     decimal.NewFromFloat(price),
			Volume:    decimal.NewFromFloat(1000),
			CloseTime: baseTime.Add(time.Duration(i+1) * time.Minute),
		}
	}

	return klines
}

func TestSimpleTestStrategy_Initialize(t *testing.T) {
	tradingPair := exchange.TradingPair{
		Base:  "BTC",
		Quote: "USDT",
	}

	strategy := NewSimpleTestStrategy(tradingPair)

	// 创建模拟上下文
	mockCtx := new(MockStrategyContext)
	now := time.Now()

	mockCtx.On("Now").Return(now)
	mockCtx.On("GetKlines", mock.Anything, mock.Anything).Return(
		generateTestKlines(50000, 50, "flat"),
		nil,
	)

	// 初始化策略
	err := strategy.Initialize(context.Background(), mockCtx)

	assert.NoError(t, err)
	assert.Equal(t, "simple_test_strategy", strategy.Name())
	assert.Equal(t, tradingPair, strategy.TradingPair())
	assert.Equal(t, exchange.Interval5m, strategy.Interval())
	mockCtx.AssertExpectations(t)
}

func TestSimpleTestStrategy_OnKline_GoldenCross(t *testing.T) {
	tradingPair := exchange.TradingPair{
		Base:  "BTC",
		Quote: "USDT",
	}

	strategy := NewSimpleTestStrategy(tradingPair)

	// 初始化上下文
	mockCtx := new(MockStrategyContext)
	now := time.Now()
	mockCtx.On("Now").Return(now)

	// 先是下跌趋势的数据
	initialKlines := generateTestKlines(50000, 30, "down")
	mockCtx.On("GetKlines", mock.Anything, mock.Anything).Return(initialKlines, nil)

	err := strategy.Initialize(context.Background(), mockCtx)
	assert.NoError(t, err)

	// 然后添加上升趋势的K线，制造金叉
	upKlines := generateTestKlines(49985, 10, "up")

	var lastSignal Signal
	for _, kline := range upKlines {
		signal, err := strategy.OnKline(context.Background(), kline)
		assert.NoError(t, err)
		if signal.Action != SignalActionHold {
			lastSignal = signal
		}
	}

	// 应该产生做多信号
	assert.Equal(t, SignalActionLong, lastSignal.Action)
	assert.Greater(t, lastSignal.Confidence, 0.0)
	assert.True(t, lastSignal.TakeProfit.GreaterThan(decimal.Zero))
	assert.True(t, lastSignal.StopLoss.GreaterThan(decimal.Zero))
}

func TestSimpleTestStrategy_OnKline_DeathCross(t *testing.T) {
	tradingPair := exchange.TradingPair{
		Base:  "BTC",
		Quote: "USDT",
	}

	strategy := NewSimpleTestStrategy(tradingPair)

	// 初始化上下文
	mockCtx := new(MockStrategyContext)
	now := time.Now()
	mockCtx.On("Now").Return(now)

	// 先是上升趋势的数据
	initialKlines := generateTestKlines(50000, 30, "up")
	mockCtx.On("GetKlines", mock.Anything, mock.Anything).Return(initialKlines, nil)

	err := strategy.Initialize(context.Background(), mockCtx)
	assert.NoError(t, err)

	// 然后添加下降趋势的K线，制造死叉
	downKlines := generateTestKlines(50015, 10, "down")

	var lastSignal Signal
	for _, kline := range downKlines {
		signal, err := strategy.OnKline(context.Background(), kline)
		assert.NoError(t, err)
		if signal.Action != SignalActionHold {
			lastSignal = signal
		}
	}

	// 应该产生做空信号
	assert.Equal(t, SignalActionShort, lastSignal.Action)
	assert.Greater(t, lastSignal.Confidence, 0.0)
}

func TestSimpleTestStrategy_OnKline_InsufficientData(t *testing.T) {
	tradingPair := exchange.TradingPair{
		Base:  "BTC",
		Quote: "USDT",
	}

	strategy := NewSimpleTestStrategy(tradingPair)

	// 初始化上下文，只有很少的数据
	mockCtx := new(MockStrategyContext)
	now := time.Now()
	mockCtx.On("Now").Return(now)
	mockCtx.On("GetKlines", mock.Anything, mock.Anything).Return(
		generateTestKlines(50000, 5, "flat"), // 只有5根K线，不足以计算
		nil,
	)

	err := strategy.Initialize(context.Background(), mockCtx)
	assert.NoError(t, err)

	// 添加一根新K线
	kline := exchange.Kline{
		OpenTime:  time.Now(),
		Open:      decimal.NewFromFloat(50000),
		High:      decimal.NewFromFloat(50100),
		Low:       decimal.NewFromFloat(49900),
		Close:     decimal.NewFromFloat(50050),
		Volume:    decimal.NewFromFloat(1000),
		CloseTime: time.Now(),
	}

	signal, err := strategy.OnKline(context.Background(), kline)

	assert.NoError(t, err)
	assert.Equal(t, SignalActionHold, signal.Action)
	assert.Contains(t, signal.Reason, "insufficient data")
}

func TestSimpleTestStrategy_Shutdown(t *testing.T) {
	tradingPair := exchange.TradingPair{
		Base:  "BTC",
		Quote: "USDT",
	}

	strategy := NewSimpleTestStrategy(tradingPair)

	// 初始化
	mockCtx := new(MockStrategyContext)
	now := time.Now()
	mockCtx.On("Now").Return(now)
	mockCtx.On("GetKlines", mock.Anything, mock.Anything).Return(
		generateTestKlines(50000, 30, "flat"),
		nil,
	)

	err := strategy.Initialize(context.Background(), mockCtx)
	assert.NoError(t, err)
	assert.NotNil(t, strategy.klines)

	// 关闭策略
	err = strategy.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, strategy.klines)
}

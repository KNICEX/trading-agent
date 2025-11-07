package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ============ Mock 定义 ============

type MockExchangeService struct {
	mock.Mock
}

func (m *MockExchangeService) MarketService() exchange.MarketService {
	args := m.Called()
	return args.Get(0).(exchange.MarketService)
}

func (m *MockExchangeService) PositionService() exchange.PositionService {
	args := m.Called()
	return args.Get(0).(exchange.PositionService)
}

func (m *MockExchangeService) AccountService() exchange.AccountService {
	args := m.Called()
	return args.Get(0).(exchange.AccountService)
}

func (m *MockExchangeService) OrderService() exchange.OrderService {
	args := m.Called()
	return args.Get(0).(exchange.OrderService)
}

func (m *MockExchangeService) TradingService() exchange.TradingService {
	args := m.Called()
	return args.Get(0).(exchange.TradingService)
}

type MockMarketService struct {
	mock.Mock
}

func (m *MockMarketService) Ticker(ctx context.Context, tradingPair exchange.TradingPair) (decimal.Decimal, error) {
	args := m.Called(ctx, tradingPair)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

func (m *MockMarketService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]exchange.Kline), args.Error(1)
}

func (m *MockMarketService) SubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) (chan exchange.Kline, error) {
	args := m.Called(ctx, tradingPair, interval)
	return args.Get(0).(chan exchange.Kline), args.Error(1)
}

type MockAccountService struct {
	mock.Mock
}

func (m *MockAccountService) GetAccountInfo(ctx context.Context) (exchange.AccountInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(exchange.AccountInfo), args.Error(1)
}

func (m *MockAccountService) GetTransferHistory(ctx context.Context, req exchange.GetTransferHistoryReq) ([]exchange.TransferHistory, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]exchange.TransferHistory), args.Error(1)
}

type MockPositionService struct {
	mock.Mock
}

func (m *MockPositionService) GetActivePositions(ctx context.Context, pairs []exchange.TradingPair) ([]exchange.Position, error) {
	args := m.Called(ctx, pairs)
	return args.Get(0).([]exchange.Position), args.Error(1)
}

func (m *MockPositionService) GetHistoryPositions(ctx context.Context, req exchange.GetHistoryPositionsReq) ([]exchange.PositionHistory, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]exchange.PositionHistory), args.Error(1)
}

func (m *MockPositionService) SetLeverage(ctx context.Context, req exchange.SetLeverageReq) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

// ============ 测试用例 ============

func TestSimplePositionSizer_Initialize(t *testing.T) {
	tests := []struct {
		name        string
		riskConfig  RiskConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "有效配置",
			riskConfig: RiskConfig{
				MaxStopLossRatio:    0.05,
				MaxLeverage:         10,
				MinProfitLossRatio:  1.5,
				ConfidenceThreshold: 60,
			},
			expectError: false,
		},
		{
			name: "MaxStopLossRatio 无效 - 太小",
			riskConfig: RiskConfig{
				MaxStopLossRatio:    0,
				MaxLeverage:         10,
				MinProfitLossRatio:  1.5,
				ConfidenceThreshold: 60,
			},
			expectError: true,
			errorMsg:    "MaxStopLossRatio 必须在 (0, 1) 之间",
		},
		{
			name: "MaxStopLossRatio 无效 - 太大",
			riskConfig: RiskConfig{
				MaxStopLossRatio:    1.5,
				MaxLeverage:         10,
				MinProfitLossRatio:  1.5,
				ConfidenceThreshold: 60,
			},
			expectError: true,
			errorMsg:    "MaxStopLossRatio 必须在 (0, 1) 之间",
		},
		{
			name: "MaxLeverage 无效",
			riskConfig: RiskConfig{
				MaxStopLossRatio:    0.05,
				MaxLeverage:         0,
				MinProfitLossRatio:  1.5,
				ConfidenceThreshold: 60,
			},
			expectError: true,
			errorMsg:    "MaxLeverage 必须大于 0",
		},
		{
			name: "ConfidenceThreshold 无效 - 太小",
			riskConfig: RiskConfig{
				MaxStopLossRatio:    0.05,
				MaxLeverage:         10,
				MinProfitLossRatio:  1.5,
				ConfidenceThreshold: 50,
			},
			expectError: true,
			errorMsg:    "ConfidenceThreshold 必须在 (50, 100] 之间",
		},
		{
			name: "ConfidenceThreshold 无效 - 太大",
			riskConfig: RiskConfig{
				MaxStopLossRatio:    0.05,
				MaxLeverage:         10,
				MinProfitLossRatio:  1.5,
				ConfidenceThreshold: 101,
			},
			expectError: true,
			errorMsg:    "ConfidenceThreshold 必须在 (50, 100] 之间",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExchange := new(MockExchangeService)
			sizer := NewSimplePositionSizer(mockExchange)

			err := sizer.Initialize(context.Background(), tt.riskConfig)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSimplePositionSizer_HandleSignal_ConfidenceCheck(t *testing.T) {
	mockExchange := new(MockExchangeService)
	sizer := NewSimplePositionSizer(mockExchange)

	riskConfig := RiskConfig{
		MaxStopLossRatio:    0.05,
		MaxLeverage:         10,
		MinProfitLossRatio:  1.5,
		ConfidenceThreshold: 60,
	}
	err := sizer.Initialize(context.Background(), riskConfig)
	assert.NoError(t, err)

	// 置信度低于阈值的信号
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionLong,
		Confidence:  55, // 低于 60
		StopLoss:    decimal.NewFromInt(45000),
		Timestamp:   time.Now(),
	}

	result, err := sizer.HandleSignal(context.Background(), signal)
	assert.NoError(t, err)
	assert.False(t, result.Validated)
	assert.Contains(t, result.Reason, "置信度")
	assert.Contains(t, result.Reason, "低于阈值")
}

func TestSimplePositionSizer_HandleSignal_StopLossCheck(t *testing.T) {
	mockExchange := new(MockExchangeService)
	sizer := NewSimplePositionSizer(mockExchange)

	riskConfig := RiskConfig{
		MaxStopLossRatio:    0.05,
		MaxLeverage:         10,
		MinProfitLossRatio:  1.5,
		ConfidenceThreshold: 60,
	}
	err := sizer.Initialize(context.Background(), riskConfig)
	assert.NoError(t, err)

	// 没有设置止损的信号
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionLong,
		Confidence:  70,
		StopLoss:    decimal.Zero, // 未设置止损
		Timestamp:   time.Now(),
	}

	result, err := sizer.HandleSignal(context.Background(), signal)
	assert.NoError(t, err)
	assert.False(t, result.Validated)
	assert.Contains(t, result.Reason, "止损价格未设置")
}

func TestSimplePositionSizer_HandleSignal_ProfitLossRatioCheck(t *testing.T) {
	mockExchange := new(MockExchangeService)
	mockMarket := new(MockMarketService)

	mockExchange.On("MarketService").Return(mockMarket)

	// 当前价格 50000
	mockMarket.On("Ticker", mock.Anything, mock.Anything).Return(decimal.NewFromInt(50000), nil)

	sizer := NewSimplePositionSizer(mockExchange)
	riskConfig := RiskConfig{
		MaxStopLossRatio:    0.05,
		MaxLeverage:         10,
		MinProfitLossRatio:  2.0, // 要求至少 2:1 的盈亏比
		ConfidenceThreshold: 60,
	}
	err := sizer.Initialize(context.Background(), riskConfig)
	assert.NoError(t, err)

	// 盈亏比不足的信号
	// 当前价 50000，止盈 51000（盈利1000），止损 49000（亏损1000）
	// 盈亏比 = 1:1，低于要求的 2:1
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionLong,
		Confidence:  70,
		TakeProfit:  decimal.NewFromInt(51000),
		StopLoss:    decimal.NewFromInt(49000),
		Timestamp:   time.Now(),
	}

	result, err := sizer.HandleSignal(context.Background(), signal)
	assert.NoError(t, err)
	assert.False(t, result.Validated)
	assert.Contains(t, result.Reason, "盈亏比")
	assert.Contains(t, result.Reason, "低于最小值")

	mockExchange.AssertExpectations(t)
	mockMarket.AssertExpectations(t)
}

func TestSimplePositionSizer_HandleSignal_SuccessfulLong(t *testing.T) {
	mockExchange := new(MockExchangeService)
	mockMarket := new(MockMarketService)
	mockAccount := new(MockAccountService)
	mockPosition := new(MockPositionService)

	mockExchange.On("MarketService").Return(mockMarket)
	mockExchange.On("AccountService").Return(mockAccount)
	mockExchange.On("PositionService").Return(mockPosition)

	// 当前价格 50000
	currentPrice := decimal.NewFromInt(50000)
	mockMarket.On("Ticker", mock.Anything, mock.Anything).Return(currentPrice, nil)

	// 账户余额 10000 USDT
	mockAccount.On("GetAccountInfo", mock.Anything).Return(exchange.AccountInfo{
		TotalBalance:     decimal.NewFromInt(10000),
		AvailableBalance: decimal.NewFromInt(10000),
	}, nil)

	// 无持仓
	mockPosition.On("GetActivePositions", mock.Anything, mock.Anything).Return([]exchange.Position{}, nil)

	sizer := NewSimplePositionSizer(mockExchange)
	riskConfig := RiskConfig{
		MaxStopLossRatio:    0.05, // 最大止损 5%
		MaxLeverage:         10,   // 最大杠杆 10x
		MinProfitLossRatio:  1.5,  // 最小盈亏比 1.5:1
		ConfidenceThreshold: 60,   // 最小置信度 60%
	}
	err := sizer.Initialize(context.Background(), riskConfig)
	assert.NoError(t, err)

	// 做多信号
	// 当前价 50000，止损 49500（止损距离 1%），止盈 51500（盈亏比 3:1）
	// 置信度 80%
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionLong,
		Confidence:  80,
		TakeProfit:  decimal.NewFromInt(51500),
		StopLoss:    decimal.NewFromInt(49500),
		Timestamp:   time.Now(),
	}

	result, err := sizer.HandleSignal(context.Background(), signal)
	assert.NoError(t, err)
	assert.True(t, result.Validated)
	assert.Equal(t, exchange.PositionSideLong, result.EnhancedSignal.PositionSide)
	assert.Equal(t, signal.TradingPair, result.EnhancedSignal.TradingPair)
	assert.True(t, result.EnhancedSignal.Quantity.GreaterThan(decimal.Zero))

	// 验证计算逻辑：
	// 止损距离比例 = (50000 - 49500) / 50000 = 1%
	// 理论最大杠杆 = 5% / 1% = 5x
	// 置信度调整：80% 在 [60%, 100%] 范围内，调整因子 = (80-60)/(100-60) = 0.5
	// 杠杆倍数 = 0.5 + 0.5 * 0.5 = 0.75
	// 实际杠杆 = 5 * 0.75 = 3.75x
	// 开仓价值 = 10000 * 3.75 = 37500
	// 开仓数量 = 37500 / 50000 = 0.75 BTC
	expectedQuantity := decimal.NewFromFloat(0.75)
	assert.True(t, result.EnhancedSignal.Quantity.Sub(expectedQuantity).Abs().LessThan(decimal.NewFromFloat(0.01)))

	mockExchange.AssertExpectations(t)
	mockMarket.AssertExpectations(t)
	mockAccount.AssertExpectations(t)
	mockPosition.AssertExpectations(t)
}

func TestSimplePositionSizer_HandleSignal_SuccessfulShort(t *testing.T) {
	mockExchange := new(MockExchangeService)
	mockMarket := new(MockMarketService)
	mockAccount := new(MockAccountService)
	mockPosition := new(MockPositionService)

	mockExchange.On("MarketService").Return(mockMarket)
	mockExchange.On("AccountService").Return(mockAccount)
	mockExchange.On("PositionService").Return(mockPosition)

	// 当前价格 50000
	currentPrice := decimal.NewFromInt(50000)
	mockMarket.On("Ticker", mock.Anything, mock.Anything).Return(currentPrice, nil)

	// 账户余额 10000 USDT
	mockAccount.On("GetAccountInfo", mock.Anything).Return(exchange.AccountInfo{
		TotalBalance:     decimal.NewFromInt(10000),
		AvailableBalance: decimal.NewFromInt(10000),
	}, nil)

	// 无持仓
	mockPosition.On("GetActivePositions", mock.Anything, mock.Anything).Return([]exchange.Position{}, nil)

	sizer := NewSimplePositionSizer(mockExchange)
	riskConfig := RiskConfig{
		MaxStopLossRatio:    0.05,
		MaxLeverage:         10,
		MinProfitLossRatio:  1.5,
		ConfidenceThreshold: 60,
	}
	err := sizer.Initialize(context.Background(), riskConfig)
	assert.NoError(t, err)

	// 做空信号
	// 当前价 50000，止损 50500（止损距离 1%），止盈 48500（盈亏比 3:1）
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionShort,
		Confidence:  80,
		TakeProfit:  decimal.NewFromInt(48500),
		StopLoss:    decimal.NewFromInt(50500),
		Timestamp:   time.Now(),
	}

	result, err := sizer.HandleSignal(context.Background(), signal)
	assert.NoError(t, err)
	assert.True(t, result.Validated)
	assert.Equal(t, exchange.PositionSideShort, result.EnhancedSignal.PositionSide)
	assert.True(t, result.EnhancedSignal.Quantity.GreaterThan(decimal.Zero))

	mockExchange.AssertExpectations(t)
	mockMarket.AssertExpectations(t)
	mockAccount.AssertExpectations(t)
	mockPosition.AssertExpectations(t)
}

func TestSimplePositionSizer_HandleSignal_MaxLeverageCheck(t *testing.T) {
	mockExchange := new(MockExchangeService)
	mockMarket := new(MockMarketService)
	mockAccount := new(MockAccountService)
	mockPosition := new(MockPositionService)

	mockExchange.On("MarketService").Return(mockMarket)
	mockExchange.On("AccountService").Return(mockAccount)
	mockExchange.On("PositionService").Return(mockPosition)

	currentPrice := decimal.NewFromInt(50000)
	mockMarket.On("Ticker", mock.Anything, mock.Anything).Return(currentPrice, nil)

	// 账户余额 10000 USDT
	mockAccount.On("GetAccountInfo", mock.Anything).Return(exchange.AccountInfo{
		TotalBalance:     decimal.NewFromInt(10000),
		AvailableBalance: decimal.NewFromInt(10000),
	}, nil)

	// 已有持仓，总价值 90000（杠杆 9x）
	existingPosition := exchange.Position{
		TradingPair:  exchange.TradingPair{Base: "ETH", Quote: "USDT"},
		PositionSide: exchange.PositionSideLong,
		Quantity:     decimal.NewFromInt(30),
		MarkPrice:    decimal.NewFromInt(3000),
	}
	mockPosition.On("GetActivePositions", mock.Anything, mock.Anything).Return([]exchange.Position{existingPosition}, nil)

	sizer := NewSimplePositionSizer(mockExchange)
	riskConfig := RiskConfig{
		MaxStopLossRatio:    0.05,
		MaxLeverage:         10, // 最大杠杆 10x，已使用 9x
		MinProfitLossRatio:  1.5,
		ConfidenceThreshold: 60,
	}
	err := sizer.Initialize(context.Background(), riskConfig)
	assert.NoError(t, err)

	// 信号要求的杠杆会超过可用杠杆
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionLong,
		Confidence:  100,                       // 最高置信度
		StopLoss:    decimal.NewFromInt(49500), // 1% 止损
		Timestamp:   time.Now(),
	}

	result, err := sizer.HandleSignal(context.Background(), signal)
	assert.NoError(t, err)
	// 应该通过，但杠杆被限制为可用杠杆（1x）
	if result.Validated {
		// 可用杠杆只有 1x，所以仓位应该很小
		expectedMaxQuantity := decimal.NewFromFloat(0.2) // 10000 * 1 / 50000
		assert.True(t, result.EnhancedSignal.Quantity.LessThanOrEqual(expectedMaxQuantity))
	}

	mockExchange.AssertExpectations(t)
	mockMarket.AssertExpectations(t)
	mockAccount.AssertExpectations(t)
	mockPosition.AssertExpectations(t)
}

func TestSimplePositionSizer_CalculateStopLossRatio(t *testing.T) {
	mockExchange := new(MockExchangeService)
	sizer := NewSimplePositionSizer(mockExchange)

	tests := []struct {
		name          string
		action        strategy.SignalAction
		currentPrice  decimal.Decimal
		stopLoss      decimal.Decimal
		expectError   bool
		expectedRatio float64
	}{
		{
			name:          "做多 - 正常止损",
			action:        strategy.SignalActionLong,
			currentPrice:  decimal.NewFromInt(50000),
			stopLoss:      decimal.NewFromInt(49000),
			expectError:   false,
			expectedRatio: 0.02, // 2%
		},
		{
			name:         "做多 - 止损价高于当前价",
			action:       strategy.SignalActionLong,
			currentPrice: decimal.NewFromInt(50000),
			stopLoss:     decimal.NewFromInt(51000),
			expectError:  true,
		},
		{
			name:          "做空 - 正常止损",
			action:        strategy.SignalActionShort,
			currentPrice:  decimal.NewFromInt(50000),
			stopLoss:      decimal.NewFromInt(51000),
			expectError:   false,
			expectedRatio: 0.02, // 2%
		},
		{
			name:         "做空 - 止损价低于当前价",
			action:       strategy.SignalActionShort,
			currentPrice: decimal.NewFromInt(50000),
			stopLoss:     decimal.NewFromInt(49000),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, err := sizer.calculateStopLossRatio(tt.action, tt.currentPrice, tt.stopLoss)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expectedRatio, ratio.InexactFloat64(), 0.0001)
			}
		})
	}
}

func TestSimplePositionSizer_CalculateProfitLossRatio(t *testing.T) {
	mockExchange := new(MockExchangeService)
	sizer := NewSimplePositionSizer(mockExchange)

	tests := []struct {
		name          string
		action        strategy.SignalAction
		currentPrice  decimal.Decimal
		takeProfit    decimal.Decimal
		stopLoss      decimal.Decimal
		expectError   bool
		expectedRatio float64
	}{
		{
			name:          "做多 - 盈亏比 2:1",
			action:        strategy.SignalActionLong,
			currentPrice:  decimal.NewFromInt(50000),
			takeProfit:    decimal.NewFromInt(52000),
			stopLoss:      decimal.NewFromInt(49000),
			expectError:   false,
			expectedRatio: 2.0,
		},
		{
			name:         "做多 - 止盈价低于当前价",
			action:       strategy.SignalActionLong,
			currentPrice: decimal.NewFromInt(50000),
			takeProfit:   decimal.NewFromInt(49000),
			stopLoss:     decimal.NewFromInt(48000),
			expectError:  true,
		},
		{
			name:          "做空 - 盈亏比 3:1",
			action:        strategy.SignalActionShort,
			currentPrice:  decimal.NewFromInt(50000),
			takeProfit:    decimal.NewFromInt(47000),
			stopLoss:      decimal.NewFromInt(51000),
			expectError:   false,
			expectedRatio: 3.0,
		},
		{
			name:         "做空 - 止盈价高于当前价",
			action:       strategy.SignalActionShort,
			currentPrice: decimal.NewFromInt(50000),
			takeProfit:   decimal.NewFromInt(51000),
			stopLoss:     decimal.NewFromInt(52000),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, err := sizer.calculateProfitLossRatio(tt.action, tt.currentPrice, tt.takeProfit, tt.stopLoss)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.expectedRatio, ratio.InexactFloat64(), 0.0001)
			}
		})
	}
}

package backtest

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPositionService_GetActivePositions 测试获取活跃持仓
func TestPositionService_GetActivePositions(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 50000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair1 := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	pair2 := exchange.TradingPair{Base: "ETH", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair1, interval, startTime, 50000.0, 5, "up")
	provider.GenerateKlines(pair2, interval, startTime, 3000.0, 5, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan1, _ := svc.SubscribeKline(ctx, pair1, interval)
	klineChan2, _ := svc.SubscribeKline(ctx, pair2, interval)

	<-klineChan1
	<-klineChan2

	// 开两个仓位
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair2,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideShort,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(1.0),
		Timestamp:   time.Now(),
	})

	<-klineChan1
	<-klineChan2

	// 测试获取所有持仓
	allPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{})
	require.NoError(t, err)
	assert.Len(t, allPositions, 2)

	// 测试获取指定交易对的持仓
	btcPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair1})
	require.NoError(t, err)
	assert.Len(t, btcPositions, 1)
	assert.Equal(t, pair1, btcPositions[0].TradingPair)

	ethPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair2})
	require.NoError(t, err)
	assert.Len(t, ethPositions, 1)
	assert.Equal(t, pair2, ethPositions[0].TradingPair)

	// 测试获取多个交易对的持仓
	bothPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair1, pair2})
	require.NoError(t, err)
	assert.Len(t, bothPositions, 2)
}

// TestPositionService_GetHistoryPositions 测试获取历史持仓
func TestPositionService_GetHistoryPositions(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 开仓
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	<-klineChan

	// 平仓
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	<-klineChan
	<-klineChan

	// 获取历史持仓
	histories, err := svc.GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{})
	require.NoError(t, err)
	assert.Len(t, histories, 1)

	history := histories[0]
	assert.Equal(t, pair, history.TradingPair)
	assert.Equal(t, exchange.PositionSideLong, history.PositionSide)
	assert.False(t, history.EntryPrice.IsZero())
	assert.False(t, history.ClosePrice.IsZero())
	assert.NotZero(t, history.OpenedAt)
	assert.NotZero(t, history.ClosedAt)
	assert.NotEmpty(t, history.Events)

	// 检查持仓事件
	hasCreateEvent := false
	hasCloseEvent := false
	for _, event := range history.Events {
		if event.EventType == exchange.PositionEventTypeCreate {
			hasCreateEvent = true
		}
		if event.EventType == exchange.PositionEventTypeClose {
			hasCloseEvent = true
		}
	}
	assert.True(t, hasCreateEvent, "应该有创建事件")
	assert.True(t, hasCloseEvent, "应该有平仓事件")

	t.Logf("持仓历史: %+v", history)
}

// TestPositionService_PositionEvents 测试持仓事件记录
func TestPositionService_PositionEvents(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 15, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 开仓 0.1
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	<-klineChan

	// 加仓 0.05
	<-klineChan
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.05),
		Timestamp:   time.Now(),
	})
	<-klineChan

	// 减仓 0.03
	<-klineChan
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.03),
		Timestamp:   time.Now(),
	})
	<-klineChan

	// 全平
	<-klineChan
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.12),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan

	// 获取历史持仓
	histories, err := svc.GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{})
	require.NoError(t, err)
	assert.Len(t, histories, 1)

	history := histories[0]

	// 检查事件数量和类型
	assert.Len(t, history.Events, 4, "应该有4个事件：创建、加仓、减仓、平仓")

	// 验证事件顺序和内容
	assert.Equal(t, exchange.PositionEventTypeCreate, history.Events[0].EventType)
	assert.Equal(t, decimal.NewFromFloat(0.1), history.Events[0].Quantity)
	assert.True(t, history.Events[0].BeforeQuantity.IsZero())
	assert.Equal(t, decimal.NewFromFloat(0.1), history.Events[0].AfterQuantity)

	assert.Equal(t, exchange.PositionEventTypeIncrease, history.Events[1].EventType)
	assert.Equal(t, decimal.NewFromFloat(0.05), history.Events[1].Quantity)
	assert.Equal(t, decimal.NewFromFloat(0.1), history.Events[1].BeforeQuantity)
	assert.Equal(t, decimal.NewFromFloat(0.15), history.Events[1].AfterQuantity)

	assert.Equal(t, exchange.PositionEventTypeDecrease, history.Events[2].EventType)
	assert.Equal(t, decimal.NewFromFloat(0.03), history.Events[2].Quantity)
	assert.Equal(t, decimal.NewFromFloat(0.15), history.Events[2].BeforeQuantity)
	assert.Equal(t, decimal.NewFromFloat(0.12), history.Events[2].AfterQuantity)

	assert.Equal(t, exchange.PositionEventTypeClose, history.Events[3].EventType)
	assert.Equal(t, decimal.NewFromFloat(0.12), history.Events[3].Quantity)
	assert.Equal(t, decimal.NewFromFloat(0.12), history.Events[3].BeforeQuantity)
	assert.True(t, history.Events[3].AfterQuantity.IsZero())

	// 验证最大持仓数量
	assert.Equal(t, decimal.NewFromFloat(0.15), history.MaxQuantity)

	t.Logf("持仓历史事件: %+v", history.Events)
}

// TestPositionService_SetLeverage 测试设置杠杆
func TestPositionService_SetLeverage(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 5, "up")

	ctx := context.Background()

	// 测试设置有效杠杆
	err := svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    20,
	})
	require.NoError(t, err)

	// 测试设置无效杠杆（< 1）
	err = svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    0,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid leverage")

	// 测试设置无效杠杆（> 125）
	err = svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    126,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid leverage")
}

// TestPositionService_LeverageAfterPositionOpen 测试持仓后修改杠杆
func TestPositionService_LeverageAfterPositionOpen(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	// 设置5倍杠杆
	svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    5,
	})

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 开仓
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan

	// 检查持仓杠杆
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)
	assert.Equal(t, 5, positions[0].Leverage)

	originalMargin := positions[0].MarginAmount

	// 修改杠杆为10倍
	err := svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    10,
	})
	require.NoError(t, err)

	// 检查持仓杠杆已更新，但保证金不变
	positions, _ = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)
	assert.Equal(t, 10, positions[0].Leverage, "杠杆应该更新")
	assert.True(t, positions[0].MarginAmount.Equal(originalMargin),
		"已有持仓的保证金不应该改变")

	t.Logf("原始杠杆: 5x, 原始保证金: %s", originalMargin)
	t.Logf("修改后杠杆: 10x, 保证金: %s", positions[0].MarginAmount)
}

// TestPositionService_UnrealizedPnLUpdate 测试未实现盈亏的实时更新
func TestPositionService_UnrealizedPnLUpdate(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成先涨后跌的K线
	klines := make([]exchange.Kline, 20)
	basePrice := 50000.0
	for i := 0; i < 20; i++ {
		var price float64
		if i < 10 {
			price = basePrice + float64(i)*100 // 先涨
		} else {
			price = basePrice + 1000 - float64(i-10)*150 // 后跌
		}

		openTime := startTime.Add(time.Duration(i) * interval.Duration())
		closeTime := openTime.Add(interval.Duration())

		klines[i] = exchange.Kline{
			OpenTime:         openTime,
			CloseTime:        closeTime,
			Open:             decimal.NewFromFloat(price - 10),
			Close:            decimal.NewFromFloat(price),
			High:             decimal.NewFromFloat(price + 20),
			Low:              decimal.NewFromFloat(price - 20),
			Volume:           decimal.NewFromFloat(1000),
			QuoteAssetVolume: decimal.NewFromFloat(price * 1000),
		}
	}
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 开多仓
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan

	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)
	entryPrice := positions[0].EntryPrice

	// 记录未实现盈亏变化
	var pnlHistory []decimal.Decimal

	for i := 0; i < 15; i++ {
		<-klineChan

		positions, _ = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
		if len(positions) > 0 {
			pnlHistory = append(pnlHistory, positions[0].UnrealizedPnl)
		}
	}

	// 验证盈亏先增后减
	assert.True(t, len(pnlHistory) > 10, "应该有足够的盈亏记录")

	// 找到最大盈亏点
	maxPnL := pnlHistory[0]
	maxIdx := 0
	for i, pnl := range pnlHistory {
		if pnl.GreaterThan(maxPnL) {
			maxPnL = pnl
			maxIdx = i
		}
	}

	// 验证有上涨阶段（盈亏为正）
	assert.True(t, maxPnL.GreaterThan(decimal.Zero), "应该有盈利阶段")

	// 验证后续有下跌（盈亏减少）
	if maxIdx < len(pnlHistory)-3 {
		laterPnL := pnlHistory[maxIdx+3]
		assert.True(t, laterPnL.LessThan(maxPnL), "价格下跌后盈亏应该减少")
	}

	t.Logf("入场价: %s", entryPrice)
	t.Logf("最大盈亏: %s (第%d根K线)", maxPnL, maxIdx)
	t.Logf("盈亏历史: %v", pnlHistory)
}

// TestPositionService_LongAndShortUnrealizedPnL 测试多空持仓的未实现盈亏计算
func TestPositionService_LongAndShortUnrealizedPnL(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 50000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 同时开多仓和空仓（同一交易对的不同方向）
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideShort,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	<-klineChan
	<-klineChan

	// 获取两个持仓
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 2)

	var longPosition, shortPosition *exchange.Position
	for i := range positions {
		if positions[i].PositionSide == exchange.PositionSideLong {
			longPosition = &positions[i]
		} else {
			shortPosition = &positions[i]
		}
	}

	require.NotNil(t, longPosition)
	require.NotNil(t, shortPosition)

	// 等待价格上涨
	for i := 0; i < 5; i++ {
		<-klineChan
	}

	// 再次获取持仓
	positions, _ = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})

	for i := range positions {
		if positions[i].PositionSide == exchange.PositionSideLong {
			longPosition = &positions[i]
		} else {
			shortPosition = &positions[i]
		}
	}

	// 价格上涨，多头盈利，空头亏损
	assert.True(t, longPosition.UnrealizedPnl.GreaterThan(decimal.Zero),
		"价格上涨，多头应该盈利")
	assert.True(t, shortPosition.UnrealizedPnl.LessThan(decimal.Zero),
		"价格上涨，空头应该亏损")

	// 两者盈亏绝对值应该相近（因为数量相同）
	diff := longPosition.UnrealizedPnl.Add(shortPosition.UnrealizedPnl).Abs()
	assert.True(t, diff.LessThan(decimal.NewFromFloat(1)),
		"相同数量的多空对冲，总盈亏应该接近0")

	t.Logf("多头盈亏: %s", longPosition.UnrealizedPnl)
	t.Logf("空头盈亏: %s", shortPosition.UnrealizedPnl)
	t.Logf("总盈亏: %s", longPosition.UnrealizedPnl.Add(shortPosition.UnrealizedPnl))
}

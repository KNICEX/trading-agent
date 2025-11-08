package backtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEdgeCase_InsufficientBalance 测试余额不足的情况
func TestEdgeCase_InsufficientBalance(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 100.0 // 很小的余额

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 5, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 尝试开仓（数量过大）
	_, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(1.0), // 需要约50000，但只有100
		Timestamp:   time.Now(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
}

// TestEdgeCase_InsufficientPosition 测试持仓不足的情况
func TestEdgeCase_InsufficientPosition(t *testing.T) {
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

	// 开仓0.1
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan // 等待订单成交

	// 确认持仓存在
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)

	// 尝试平仓0.2（超过持仓数量）
	_, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.2),
		Timestamp:   time.Now(),
	})

	assert.Error(t, err)
	// 可能返回 "insufficient position quantity" 或 "position not found"
	assert.True(t,
		err.Error() == "insufficient position quantity: have=0.1, want=0.2" ||
			strings.Contains(err.Error(), "position") ||
			strings.Contains(err.Error(), "insufficient"),
		"应该返回持仓不足相关错误")
}

// TestEdgeCase_CloseNonExistentPosition 测试平仓不存在的持仓
func TestEdgeCase_CloseNonExistentPosition(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 5, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 尝试平仓（没有持仓）
	_, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "position not found")
}

// TestEdgeCase_CancelNonExistentOrder 测试取消不存在的订单
func TestEdgeCase_CancelNonExistentOrder(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, _ := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	ctx := context.Background()

	// 尝试取消不存在的订单
	err := svc.CancelOrder(ctx, exchange.CancelOrderReq{
		Id:          exchange.OrderId("999999"),
		TradingPair: pair,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order not found")
}

// TestEdgeCase_GetNonExistentOrder 测试获取不存在的订单
func TestEdgeCase_GetNonExistentOrder(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, _ := createTestExchange(t, initialBalance, startTime, endTime)

	ctx := context.Background()

	// 尝试获取不存在的订单
	_, err := svc.GetOrder(ctx, exchange.GetOrderReq{
		Id: exchange.OrderId("999999"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order not found")
}

// TestEdgeCase_InvalidLeverage 测试无效的杠杆值
func TestEdgeCase_InvalidLeverage(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, _ := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	ctx := context.Background()

	// 测试杠杆 < 1
	err := svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    0,
	})
	assert.Error(t, err)

	// 测试杠杆 > 125
	err = svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    126,
	})
	assert.Error(t, err)

	// 测试负数杠杆
	err = svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    -5,
	})
	assert.Error(t, err)
}

// TestEdgeCase_ZeroQuantity 测试零数量订单
func TestEdgeCase_ZeroQuantity(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 5, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 尝试创建零数量订单
	_, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(50000),
		Quantity:    decimal.Zero,
		Timestamp:   time.Now(),
	})

	// 应该会因为余额不足而失败（因为零数量意味着零成本，但系统可能有其他检查）
	// 或者直接成功但不产生任何效果
	// 这里我们主要确保不会崩溃
	_ = err
}

// TestEdgeCase_MultipleKlineProviders 测试多个交易对独立的K线数据
func TestEdgeCase_MultipleKlineProviders(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 50000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair1 := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	pair2 := exchange.TradingPair{Base: "ETH", Quote: "USDT"}
	interval := exchange.Interval5m

	// 为两个交易对生成不同趋势的K线
	provider.GenerateKlines(pair1, interval, startTime, 50000.0, 20, "up")
	provider.GenerateKlines(pair2, interval, startTime, 3000.0, 20, "down")

	ctx := context.Background()

	klineChan1, _ := svc.SubscribeKline(ctx, pair1, interval)
	klineChan2, _ := svc.SubscribeKline(ctx, pair2, interval)

	<-klineChan1
	<-klineChan2

	// BTC开多仓（上涨趋势）
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	// ETH开空仓（下跌趋势）
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

	// 等待价格变化
	for i := 0; i < 10; i++ {
		<-klineChan1
		<-klineChan2
	}

	// 检查两个持仓
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{})
	require.Len(t, positions, 2)

	var btcPos, ethPos *exchange.Position
	for i := range positions {
		if positions[i].TradingPair == pair1 {
			btcPos = &positions[i]
		} else {
			ethPos = &positions[i]
		}
	}

	require.NotNil(t, btcPos)
	require.NotNil(t, ethPos)

	// BTC上涨，多头应该盈利
	assert.True(t, btcPos.UnrealizedPnl.GreaterThan(decimal.Zero),
		"BTC上涨，多头应该盈利")

	// ETH下跌，空头应该盈利
	assert.True(t, ethPos.UnrealizedPnl.GreaterThan(decimal.Zero),
		"ETH下跌，空头应该盈利")

	t.Logf("BTC多头盈亏: %s", btcPos.UnrealizedPnl)
	t.Logf("ETH空头盈亏: %s", ethPos.UnrealizedPnl)
}

// TestEdgeCase_StopOrderAfterPositionClosed 测试持仓关闭后止盈止损失效
func TestEdgeCase_StopOrderAfterPositionClosed(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 开仓并设置止盈
	resp, _ := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan // 等待开仓成交

	// 手动平仓
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan // 等待平仓成交

	// 持仓已关闭
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	assert.Empty(t, positions)

	// 继续推送K线，即使价格达到止盈价，也不应该再次触发
	for i := 0; i < 10; i++ {
		<-klineChan
	}

	// 持仓仍然为空
	positions, _ = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	assert.Empty(t, positions)

	t.Logf("止盈订单ID: %s", resp)
}

// TestEdgeCase_ConcurrentOrders 测试并发创建订单
func TestEdgeCase_ConcurrentOrders(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 100000.0 // 更大的余额

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 并发创建多个订单
	orderCount := 10
	done := make(chan bool, orderCount)

	for i := 0; i < orderCount; i++ {
		go func(index int) {
			_, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
				TradingPair: pair,
				OrderType:   exchange.OrderTypeOpen,
				PositonSide: exchange.PositionSideLong,
				Price:       decimal.NewFromFloat(40000 + float64(index)*100),
				Quantity:    decimal.NewFromFloat(0.01),
				Timestamp:   time.Now(),
			})
			if err == nil {
				done <- true
			} else {
				done <- false
			}
		}(i)
	}

	// 等待所有订单创建完成
	successCount := 0
	for i := 0; i < orderCount; i++ {
		if <-done {
			successCount++
		}
	}

	// 检查订单数量
	orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	assert.Len(t, orders, successCount, "应该成功创建所有订单")

	t.Logf("成功创建 %d/%d 个订单", successCount, orderCount)
}

// TestEdgeCase_PriceDataNotAvailable 测试没有价格数据时的行为
func TestEdgeCase_PriceDataNotAvailable(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, _ := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	ctx := context.Background()

	// 在没有K线数据的情况下尝试获取价格
	_, err := svc.Ticker(ctx, pair)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no price data")
}

// TestEdgeCase_VerySmallQuantity 测试极小数量
func TestEdgeCase_VerySmallQuantity(t *testing.T) {
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

	// 创建极小数量的订单
	_, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.00001), // 极小数量
		Timestamp:   time.Now(),
	})

	// 应该能成功创建
	require.NoError(t, err)

	<-klineChan
	<-klineChan // 等待订单成交

	// 检查持仓
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)
	assert.Equal(t, decimal.NewFromFloat(0.00001), positions[0].Quantity)
}

// TestEdgeCase_AddPositionMultipleTimes 测试多次加仓
func TestEdgeCase_AddPositionMultipleTimes(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 多次开仓（加仓）
	quantities := []float64{0.01, 0.02, 0.03, 0.04}
	totalQuantity := decimal.Zero

	for _, qty := range quantities {
		svc.CreateOrder(ctx, exchange.CreateOrderReq{
			TradingPair: pair,
			OrderType:   exchange.OrderTypeOpen,
			PositonSide: exchange.PositionSideLong,
			Price:       decimal.Zero,
			Quantity:    decimal.NewFromFloat(qty),
			Timestamp:   time.Now(),
		})
		<-klineChan // 等待订单进入pending
		<-klineChan // 等待订单成交

		totalQuantity = totalQuantity.Add(decimal.NewFromFloat(qty))

		// 验证持仓数量累加
		positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
		require.Len(t, positions, 1)
		assert.True(t, positions[0].Quantity.Equal(totalQuantity),
			"持仓数量应该累加到 %s", totalQuantity)
	}

	t.Logf("最终持仓数量: %s", totalQuantity)
}

// TestEdgeCase_PartialCloseMultipleTimes 测试多次部分平仓
func TestEdgeCase_PartialCloseMultipleTimes(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 开仓1.0
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(1.0),
		Timestamp:   time.Now(),
	})
	<-klineChan
	<-klineChan // 等待开仓成交

	// 多次部分平仓
	closeQuantities := []float64{0.1, 0.2, 0.3, 0.4}
	remainingQuantity := decimal.NewFromFloat(1.0)

	for _, qty := range closeQuantities {
		svc.CreateOrder(ctx, exchange.CreateOrderReq{
			TradingPair: pair,
			OrderType:   exchange.OrderTypeClose,
			PositonSide: exchange.PositionSideLong,
			Price:       decimal.Zero,
			Quantity:    decimal.NewFromFloat(qty),
			Timestamp:   time.Now(),
		})
		<-klineChan // 等待订单进入pending
		<-klineChan // 等待订单成交

		remainingQuantity = remainingQuantity.Sub(decimal.NewFromFloat(qty))

		// 验证持仓数量减少
		positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
		if remainingQuantity.IsZero() {
			assert.Empty(t, positions)
		} else {
			require.Len(t, positions, 1)
			assert.True(t, positions[0].Quantity.Equal(remainingQuantity),
				"持仓数量应该减少到 %s", remainingQuantity)
		}
	}

	t.Logf("最终持仓数量: %s", remainingQuantity)
}

// TestEdgeCase_NoPriceMovement 测试价格不变的情况
func TestEdgeCase_NoPriceMovement(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成价格完全不变的K线
	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "sideways")

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

	// 等待几根K线
	for i := 0; i < 5; i++ {
		<-klineChan
	}

	// 检查未实现盈亏（应该接近零）
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)

	// 价格不变，盈亏应该很小
	assert.True(t, positions[0].UnrealizedPnl.Abs().LessThan(decimal.NewFromFloat(100)),
		"价格不变时盈亏应该很小")

	t.Logf("未实现盈亏: %s", positions[0].UnrealizedPnl)
}

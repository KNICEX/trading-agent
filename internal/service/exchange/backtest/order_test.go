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

// TestOrderService_CreateMarketOrder 测试创建市价单
func TestOrderService_CreateMarketOrder(t *testing.T) {
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

	// 创建市价单
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero, // 市价单
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, orderId)

	// 市价单应该在下一根K线立即成交
	<-klineChan

	orderInfo, err := svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusFilled, orderInfo.Status)
	assert.Equal(t, decimal.NewFromFloat(0.1), orderInfo.ExecutedQuantity)
}

// TestOrderService_CreateLimitOrder 测试创建限价单
func TestOrderService_CreateLimitOrder(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	klines := createTestKlines(startTime, 10, 50000.0, interval, TrendUp, 0.01)
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 创建限价买单（价格低于市价）
	limitPrice := decimal.NewFromFloat(49980.0)
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       limitPrice,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 订单应该处于挂单状态
	orderInfo, err := svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusPending, orderInfo.Status)

	// 等待K线触发成交
	for i := 0; i < 5; i++ {
		<-klineChan
	}

	// 检查订单是否成交
	orderInfo, err = svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusFilled, orderInfo.Status)
}

// TestOrderService_GetOrders 测试获取订单列表
func TestOrderService_GetOrders(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair1 := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	pair2 := exchange.TradingPair{Base: "ETH", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair1, interval, startTime, 50000.0, 5, "up")
	provider.GenerateKlines(pair2, interval, startTime, 3000.0, 5, "up")

	ctx := context.Background()

	klineChan1, _ := svc.SubscribeKline(ctx, pair1, interval)
	klineChan2, _ := svc.SubscribeKline(ctx, pair2, interval)
	<-klineChan1
	<-klineChan2

	// 创建多个限价单（不会立即成交）
	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(40000),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(41000),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair2,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(2500),
		Quantity:    decimal.NewFromFloat(1.0),
		Timestamp:   time.Now(),
	})

	// 获取所有订单
	allOrders, err := svc.GetOrders(ctx, exchange.GetOrdersReq{})
	require.NoError(t, err)
	assert.Len(t, allOrders, 3)

	// 获取指定交易对的订单
	btcOrders, err := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair1})
	require.NoError(t, err)
	assert.Len(t, btcOrders, 2)

	ethOrders, err := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair2})
	require.NoError(t, err)
	assert.Len(t, ethOrders, 1)
}

// TestOrderService_CancelOrder 测试取消订单
func TestOrderService_CancelOrder(t *testing.T) {
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

	// 创建限价单
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(40000),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 验证订单存在
	orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	assert.Len(t, orders, 1)

	// 取消订单
	err = svc.CancelOrder(ctx, exchange.CancelOrderReq{
		Id:          orderId,
		TradingPair: pair,
	})
	require.NoError(t, err)

	// 验证订单已移除
	orders, _ = svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	assert.Empty(t, orders)
}

// TestOrderService_CancelOrders 测试批量取消订单
func TestOrderService_CancelOrders(t *testing.T) {
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

	// 创建多个限价单
	for i := 0; i < 3; i++ {
		svc.CreateOrder(ctx, exchange.CreateOrderReq{
			TradingPair: pair,
			OrderType:   exchange.OrderTypeOpen,
			PositonSide: exchange.PositionSideLong,
			Price:       decimal.NewFromFloat(40000 + float64(i)*100),
			Quantity:    decimal.NewFromFloat(0.1),
			Timestamp:   time.Now(),
		})
	}

	// 验证订单存在
	orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	assert.Len(t, orders, 3)

	// 批量取消所有订单
	err := svc.CancelOrders(ctx, exchange.CancelOrdersReq{
		TradingPair: pair,
	})
	require.NoError(t, err)

	// 验证所有订单已移除
	orders, _ = svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	assert.Empty(t, orders)
}

// TestOrderService_OrderFillPrice 测试订单成交价格
func TestOrderService_OrderFillPrice(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	klines := createTestKlines(startTime, 10, 50000.0, interval, TrendUp, 0.01)
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan

	// 创建限价买单
	limitPrice := decimal.NewFromFloat(49990.0)
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       limitPrice,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待成交
	for i := 0; i < 5; i++ {
		<-klineChan
	}

	// 检查持仓入场价
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)

	// 限价单成交，入场价应该等于限价
	assert.True(t, positions[0].EntryPrice.Equal(limitPrice),
		"限价单成交价应该等于限价")

	t.Logf("订单ID: %s, 限价: %s, 入场价: %s",
		orderId, limitPrice, positions[0].EntryPrice)
}

// TestOrderService_OrderScanMechanism 测试K线驱动的订单扫描机制
func TestOrderService_OrderScanMechanism(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 创建精确的K线数据来测试订单成交逻辑
	klines := []exchange.Kline{
		{
			OpenTime:  startTime,
			CloseTime: startTime.Add(interval.Duration()),
			Open:      decimal.NewFromFloat(50000),
			Close:     decimal.NewFromFloat(50100),
			High:      decimal.NewFromFloat(50200),
			Low:       decimal.NewFromFloat(49900),
			Volume:    decimal.NewFromFloat(1000),
		},
		{
			OpenTime:  startTime.Add(interval.Duration()),
			CloseTime: startTime.Add(2 * interval.Duration()),
			Open:      decimal.NewFromFloat(50100),
			Close:     decimal.NewFromFloat(50000),
			High:      decimal.NewFromFloat(50150),
			Low:       decimal.NewFromFloat(49950),
			Volume:    decimal.NewFromFloat(1000),
		},
		{
			OpenTime:  startTime.Add(2 * interval.Duration()),
			CloseTime: startTime.Add(3 * interval.Duration()),
			Open:      decimal.NewFromFloat(50000),
			Close:     decimal.NewFromFloat(49800),
			High:      decimal.NewFromFloat(50050),
			Low:       decimal.NewFromFloat(49750),
			Volume:    decimal.NewFromFloat(1000),
		},
	}
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan // 第一根K线

	// 创建买单（限价49950），应该在第二根K线成交（最低价49950）
	buyOrderId, _ := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(49950),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	// 创建卖单（限价50150），应该在第二根K线成交（最高价50150）
	sellOrderId, _ := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideShort,
		Price:       decimal.NewFromFloat(50150),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	// 第一根K线后，订单应该还在挂单
	orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	assert.Len(t, orders, 2)

	<-klineChan // 第二根K线，两个订单都应该成交

	// 检查订单状态
	buyOrder, _ := svc.GetOrder(ctx, exchange.GetOrderReq{Id: buyOrderId})
	sellOrder, _ := svc.GetOrder(ctx, exchange.GetOrderReq{Id: sellOrderId})

	assert.Equal(t, exchange.OrderStatusFilled, buyOrder.Status, "买单应该成交")
	assert.Equal(t, exchange.OrderStatusFilled, sellOrder.Status, "卖单应该成交")

	// 检查持仓
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	assert.Len(t, positions, 2)

	t.Logf("买单成交价: %s", buyOrder.Price)
	t.Logf("卖单成交价: %s", sellOrder.Price)
}

// TestOrderService_FrozenFunds 测试冻结资金机制
func TestOrderService_FrozenFunds(t *testing.T) {
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

	// 记录初始余额
	accountBefore, _ := svc.GetAccountInfo(ctx)
	initialAvailable := accountBefore.AvailableBalance

	// 创建开仓限价单
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(50000),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 创建订单后，资金应该被冻结
	accountAfterCreate, _ := svc.GetAccountInfo(ctx)
	assert.True(t, accountAfterCreate.AvailableBalance.LessThan(initialAvailable),
		"创建开仓订单后应该冻结资金")

	frozenAmount := initialAvailable.Sub(accountAfterCreate.AvailableBalance)
	expectedFrozen := decimal.NewFromFloat(50000 * 0.1) // 价格 * 数量

	assert.True(t, frozenAmount.Sub(expectedFrozen).Abs().LessThan(decimal.NewFromFloat(1)),
		"冻结金额应该约等于订单价值")

	// 等待成交
	<-klineChan

	// 成交后，冻结资金应该转为保证金
	accountAfterFill, _ := svc.GetAccountInfo(ctx)
	assert.True(t, accountAfterFill.UsedMargin.GreaterThan(decimal.Zero),
		"成交后应该有保证金")

	t.Logf("初始可用: %s", initialAvailable)
	t.Logf("创建订单后可用: %s, 冻结: %s", accountAfterCreate.AvailableBalance, frozenAmount)
	t.Logf("成交后可用: %s, 保证金: %s", accountAfterFill.AvailableBalance, accountAfterFill.UsedMargin)

	_ = orderId
}

// TestOrderService_FrozenPosition 测试冻结持仓机制
func TestOrderService_FrozenPosition(t *testing.T) {
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
		Quantity:    decimal.NewFromFloat(0.2),
		Timestamp:   time.Now(),
	})
	<-klineChan

	// 创建平仓限价单（不会立即成交）
	closeOrderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(60000), // 远高于市价
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 尝试创建另一个平仓订单（数量过大，超过可用持仓）
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(60000),
		Quantity:    decimal.NewFromFloat(0.15), // 0.1已冻结，只剩0.1可用
		Timestamp:   time.Now(),
	})
	assert.Error(t, err, "应该因为持仓数量不足而失败")
	assert.Contains(t, err.Error(), "insufficient position quantity")

	// 取消第一个平仓订单，释放冻结的持仓
	err = svc.CancelOrder(ctx, exchange.CancelOrderReq{
		Id:          closeOrderId,
		TradingPair: pair,
	})
	require.NoError(t, err)

	// 现在应该可以创建新的平仓订单
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(60000),
		Quantity:    decimal.NewFromFloat(0.15),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err, "取消订单后应该可以创建新订单")
}

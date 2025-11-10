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
	initialBalance := 20000.0 // 增加初始余额以支持多个订单

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
	initialBalance := 20000.0 // 增加初始余额以支持多个订单

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
	initialBalance := 20000.0 // 增加初始余额以支持两个订单

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
	buyOrderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(49950),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err, "创建买单应该成功")

	// 创建卖单（限价50150），应该在第二根K线成交（最高价50150）
	sellOrderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideShort,
		Price:       decimal.NewFromFloat(50150),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err, "创建卖单应该成功")

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

// TestOrderService_MultiplePendingCloseOrders 测试多个平仓挂单
// 币安交易所允许创建多个平仓挂单，即使总数量超过持仓
// 只要单个订单的数量不超过当前持仓数量即可
func TestOrderService_MultiplePendingCloseOrders(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 15000.0 // 增加初始余额以应对市价单价格波动

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

	// 创建第一个平仓限价单（不会立即成交）
	closeOrderId1, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(60000), // 远高于市价
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 创建第二个平仓订单（允许创建，因为单个订单数量不超过持仓）
	closeOrderId2, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(61000),
		Quantity:    decimal.NewFromFloat(0.15),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err, "应该可以创建第二个平仓订单")

	// 尝试创建数量超过持仓的订单（应该失败）
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(62000),
		Quantity:    decimal.NewFromFloat(0.3), // 超过持仓数量0.2
		Timestamp:   time.Now(),
	})
	assert.Error(t, err, "应该因为持仓数量不足而失败")
	assert.Contains(t, err.Error(), "insufficient position quantity")

	// 取消两个订单
	err = svc.CancelOrder(ctx, exchange.CancelOrderReq{
		Id:          closeOrderId1,
		TradingPair: pair,
	})
	require.NoError(t, err)

	err = svc.CancelOrder(ctx, exchange.CancelOrderReq{
		Id:          closeOrderId2,
		TradingPair: pair,
	})
	require.NoError(t, err)
}

// TestOrderService_PartialFill 测试部分成交机制
// 当成交价格比冻结时更差，且余额不足时，应该部分成交
func TestOrderService_PartialFill(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0 // 初始余额10000

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 创建K线数据：第一根50000，第二根价格大幅上涨到52000
	klines := []exchange.Kline{
		{
			OpenTime:  startTime,
			CloseTime: startTime.Add(interval.Duration()),
			Open:      decimal.NewFromFloat(50000),
			Close:     decimal.NewFromFloat(50000),
			High:      decimal.NewFromFloat(50100),
			Low:       decimal.NewFromFloat(49900),
			Volume:    decimal.NewFromFloat(1000),
		},
		{
			OpenTime:  startTime.Add(interval.Duration()),
			CloseTime: startTime.Add(2 * interval.Duration()),
			Open:      decimal.NewFromFloat(52000), // 开盘价大幅上涨
			Close:     decimal.NewFromFloat(52000),
			High:      decimal.NewFromFloat(52100),
			Low:       decimal.NewFromFloat(51900),
			Volume:    decimal.NewFromFloat(1000),
		},
	}
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	klineChan, _ := svc.SubscribeKline(ctx, pair, interval)
	<-klineChan // 第一根K线

	// 创建市价单：买入0.2个BTC
	// 冻结金额 = 50000 × 0.2 = 10000 USDT（刚好用完全部余额）
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero, // 市价单
		Quantity:    decimal.NewFromFloat(0.2),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 检查余额被冻结
	accountAfterCreate, _ := svc.GetAccountInfo(ctx)
	assert.True(t, accountAfterCreate.AvailableBalance.IsZero(),
		"创建订单后可用余额应该为0（全部冻结）")

	<-klineChan // 第二根K线，价格上涨到52000，订单成交

	// 检查订单状态：应该部分成交
	order, _ := svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	t.Logf("订单状态: %s", order.Status)
	t.Logf("订单数量: %s", order.Quantity)
	t.Logf("成交数量: %s", order.ExecutedQuantity)

	// 断言：应该部分成交
	assert.Equal(t, exchange.OrderStatusPartiallyFilled, order.Status,
		"由于价格上涨且余额不足，应该部分成交")
	assert.True(t, order.ExecutedQuantity.LessThan(order.Quantity),
		"成交数量应该小于订单数量")
	assert.True(t, order.ExecutedQuantity.GreaterThan(decimal.Zero),
		"成交数量应该大于0")

	// 检查持仓
	positions, _ := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.Len(t, positions, 1)
	assert.Equal(t, order.ExecutedQuantity, positions[0].Quantity,
		"持仓数量应该等于实际成交数量")

	// 检查账户：可用余额应该为0（全部用完），保证金应该等于冻结的10000
	accountAfterFill, _ := svc.GetAccountInfo(ctx)
	assert.True(t, accountAfterFill.AvailableBalance.IsZero(),
		"成交后可用余额应该为0（全部用作保证金）")
	assert.True(t, accountAfterFill.UsedMargin.Equal(decimal.NewFromFloat(10000)),
		"保证金应该等于冻结的全部资金10000")

	t.Logf("预期成交数量: 0.2 BTC")
	t.Logf("实际成交数量: %s BTC", order.ExecutedQuantity)
	t.Logf("成交价格: 52000 (vs 冻结时估算: 50000)")
	t.Logf("理论最大数量: 10000 / 52000 = %s BTC",
		decimal.NewFromFloat(10000).Div(decimal.NewFromFloat(52000)))
}

// TestOrderService_CancelOrderLogic 测试取消订单的逻辑
func TestOrderService_CancelOrderLogic(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 50000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair1 := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	pair2 := exchange.TradingPair{Base: "ETH", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair1, interval, startTime, 50000.0, 5, "flat")
	provider.GenerateKlines(pair2, interval, startTime, 3000.0, 5, "flat")

	ctx := context.Background()

	klineChan1, _ := svc.SubscribeKline(ctx, pair1, interval)
	klineChan2, _ := svc.SubscribeKline(ctx, pair2, interval)
	<-klineChan1
	<-klineChan2

	// 创建多个订单
	order1, _ := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(49000),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	order2, _ := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(49500),
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})

	_, _ = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair2,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.NewFromFloat(2900),
		Quantity:    decimal.NewFromFloat(1.0),
		Timestamp:   time.Now(),
	})

	// 验证所有订单都在挂单列表中
	orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{})
	assert.Len(t, orders, 3, "应该有3个挂单")

	t.Run("取消特定订单（有ID和TradingPair）", func(t *testing.T) {
		err := svc.CancelOrder(ctx, exchange.CancelOrderReq{
			Id:          order1,
			TradingPair: pair1,
		})
		require.NoError(t, err)

		// 检查订单状态
		canceledOrder, _ := svc.GetOrder(ctx, exchange.GetOrderReq{Id: order1})
		assert.Equal(t, exchange.OrderStatus("cancelled"), canceledOrder.Status)

		// 检查挂单列表
		orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{})
		assert.Len(t, orders, 2, "应该剩余2个挂单")
	})

	t.Run("取消特定订单但TradingPair不匹配", func(t *testing.T) {
		err := svc.CancelOrder(ctx, exchange.CancelOrderReq{
			Id:          order2,
			TradingPair: pair2, // 订单2属于pair1，但这里用pair2
		})
		assert.Error(t, err, "TradingPair不匹配应该返回错误")
		assert.Contains(t, err.Error(), "does not belong to trading pair")

		// 订单应该还在
		orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{})
		assert.Len(t, orders, 2, "订单不应该被取消")
	})

	t.Run("取消交易对的所有订单（ID为空）", func(t *testing.T) {
		// pair1还有order2
		// pair2有order3
		err := svc.CancelOrder(ctx, exchange.CancelOrderReq{
			Id:          "", // 空ID
			TradingPair: pair1,
		})
		require.NoError(t, err)

		// pair1的订单应该被取消
		orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair1})
		assert.Len(t, orders, 0, "pair1的订单应该全部被取消")

		// pair2的订单应该还在
		orders, _ = svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair2})
		assert.Len(t, orders, 1, "pair2的订单应该还在")
	})

	t.Run("取消所有订单（ID和TradingPair都为空）", func(t *testing.T) {
		// 创建新订单
		svc.CreateOrder(ctx, exchange.CreateOrderReq{
			TradingPair: pair1,
			OrderType:   exchange.OrderTypeOpen,
			PositonSide: exchange.PositionSideLong,
			Price:       decimal.NewFromFloat(49000),
			Quantity:    decimal.NewFromFloat(0.1),
			Timestamp:   time.Now(),
		})

		orders, _ := svc.GetOrders(ctx, exchange.GetOrdersReq{})
		assert.Len(t, orders, 2, "应该有2个挂单")

		err := svc.CancelOrder(ctx, exchange.CancelOrderReq{
			Id:          "",                     // 空ID
			TradingPair: exchange.TradingPair{}, // 空TradingPair
		})
		require.NoError(t, err)

		// 所有订单应该被取消
		orders, _ = svc.GetOrders(ctx, exchange.GetOrdersReq{})
		assert.Len(t, orders, 0, "所有订单应该被取消")
	})
}

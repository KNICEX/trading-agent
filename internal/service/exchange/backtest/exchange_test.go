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

// 测试辅助函数

// createTestExchange 创建测试用的交易所实例
func createTestExchange(t *testing.T, initialBalance float64, startTime, endTime time.Time) (*ExchangeService, *MockKlineProvider) {
	provider := NewMockKlineProvider()
	svc := NewExchangeService(
		startTime,
		endTime,
		decimal.NewFromFloat(initialBalance),
		provider,
	)
	return svc, provider
}

// KlineTrendType K线趋势类型
type KlineTrendType string

const (
	TrendUp         KlineTrendType = "up"           // 单边上涨
	TrendDown       KlineTrendType = "down"         // 单边下跌
	TrendUpThenDown KlineTrendType = "up_then_down" // 先涨后跌
	TrendDownThenUp KlineTrendType = "down_then_up" // 先跌后涨
)

// createTestKlines 创建测试用的K线数据
// startTime: 起始时间
// count: K线数量
// basePrice: 基础价格
// interval: K线周期
// trendType: 趋势类型
// changePercent: 每根K线的价格变化百分比 (例如: 0.01 表示 1%)
func createTestKlines(startTime time.Time, count int, basePrice float64, interval exchange.Interval, trendType KlineTrendType, changePercent float64) []exchange.Kline {
	klines := make([]exchange.Kline, count)
	currentPrice := basePrice

	for i := 0; i < count; i++ {
		openTime := startTime.Add(time.Duration(i) * interval.Duration())
		closeTime := openTime.Add(interval.Duration())

		// 根据趋势类型计算价格变化方向
		priceChange := 1.0
		switch trendType {
		case TrendUp:
			// 单边上涨
			priceChange = 1.0 + changePercent
		case TrendDown:
			// 单边下跌
			priceChange = 1.0 - changePercent
		case TrendUpThenDown:
			// 先涨后跌：前半段上涨，后半段下跌
			if i < count/2 {
				priceChange = 1.0 + changePercent
			} else {
				priceChange = 1.0 - changePercent
			}
		case TrendDownThenUp:
			// 先跌后涨：前半段下跌，后半段上涨
			if i < count/2 {
				priceChange = 1.0 - changePercent
			} else {
				priceChange = 1.0 + changePercent
			}
		}

		// 计算当前K线的价格
		openPrice := currentPrice
		closePrice := currentPrice * priceChange

		// 根据开盘和收盘价计算最高最低价
		var high, low float64
		if closePrice > openPrice {
			// 上涨K线
			high = closePrice * (1.0 + changePercent*0.3) // 最高价略高于收盘价
			low = openPrice * (1.0 - changePercent*0.2)   // 最低价略低于开盘价
		} else {
			// 下跌K线
			high = openPrice * (1.0 + changePercent*0.2) // 最高价略高于开盘价
			low = closePrice * (1.0 - changePercent*0.3) // 最低价略低于收盘价
		}

		klines[i] = exchange.Kline{
			OpenTime:         openTime,
			CloseTime:        closeTime,
			Open:             decimal.NewFromFloat(openPrice),
			Close:            decimal.NewFromFloat(closePrice),
			High:             decimal.NewFromFloat(high),
			Low:              decimal.NewFromFloat(low),
			Volume:           decimal.NewFromFloat(1000),
			QuoteAssetVolume: decimal.NewFromFloat((openPrice + closePrice) / 2 * 1000),
		}

		// 更新当前价格为收盘价，作为下一根K线的开盘价
		currentPrice = closePrice
	}
	return klines
}

// TestExchangeService_InitialState 测试交易所初始状态
func TestExchangeService_InitialState(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, _ := createTestExchange(t, initialBalance, startTime, endTime)

	ctx := context.Background()

	// 检查初始账户状态
	account, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	assert.Equal(t, decimal.NewFromFloat(initialBalance), account.TotalBalance)
	assert.Equal(t, decimal.NewFromFloat(initialBalance), account.AvailableBalance)
	assert.True(t, account.UnrealizedPnl.IsZero())
	assert.True(t, account.UsedMargin.IsZero())

	// 检查初始持仓状态
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{})
	require.NoError(t, err)
	assert.Empty(t, positions)

	// 检查初始订单状态
	orders, err := svc.GetOrders(ctx, exchange.GetOrdersReq{})
	require.NoError(t, err)
	assert.Empty(t, orders)
}

// TestExchangeService_MarketOrder_Buy 测试市价单开多仓
func TestExchangeService_MarketOrder_Buy(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成上涨趋势的K线数据
	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	kline1 := <-klineChan
	require.NotNil(t, kline1)

	// 创建市价开多单
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

	// 市价单需要等待下一根K线才会成交
	kline2 := <-klineChan
	require.NotNil(t, kline2)

	// 再等待一根K线确保订单已成交
	<-klineChan

	// 检查订单状态
	orderInfo, err := svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusFilled, orderInfo.Status)
	assert.Equal(t, decimal.NewFromFloat(0.1), orderInfo.ExecutedQuantity)

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Len(t, positions, 1)

	position := positions[0]
	assert.Equal(t, pair, position.TradingPair)
	assert.Equal(t, exchange.PositionSideLong, position.PositionSide)
	assert.Equal(t, decimal.NewFromFloat(0.1), position.Quantity)
	assert.False(t, position.EntryPrice.IsZero())

	// 检查账户余额（应该扣除了保证金）
	account, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)
	assert.True(t, account.UsedMargin.GreaterThan(decimal.Zero))
	assert.True(t, account.AvailableBalance.LessThan(decimal.NewFromFloat(initialBalance)))
}

// TestExchangeService_LimitOrder_Buy 测试限价单开多仓
func TestExchangeService_LimitOrder_Buy(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成价格在50000附近波动的K线
	klines := createTestKlines(startTime, 10, 50000.0, interval, TrendUp, 0.01)
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	kline1 := <-klineChan
	require.NotNil(t, kline1)
	t.Logf("第一根K线: Low=%s, High=%s, Close=%s", kline1.Low, kline1.High, kline1.Close)

	// 创建限价开多单（价格设置在当前价格上方，这样下一根K线的Low会触及到）
	// 因为K线价格在涨，我们设置一个高于当前Close的限价
	limitPrice := kline1.Close.Add(decimal.NewFromFloat(5))
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
	orders, err := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, exchange.OrderStatusPending, orders[0].Status)

	// 等待下一根K线，此时价格上涨，K线的Low应该能触及我们的限价
	kline2 := <-klineChan
	require.NotNil(t, kline2)
	t.Logf("第二根K线: Low=%s, High=%s, Close=%s, 限价=%s", kline2.Low, kline2.High, kline2.Close, limitPrice)

	// 再等待一根K线确保订单已处理
	<-klineChan

	// 检查订单是否成交
	orderInfo, err := svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusFilled, orderInfo.Status)

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Len(t, positions, 1)
}

// TestExchangeService_LimitOrder_Sell 测试限价单开空仓
func TestExchangeService_LimitOrder_Sell(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成价格在50000附近波动的K线
	klines := createTestKlines(startTime, 10, 50000.0, interval, TrendUp, 0.01)
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	kline1 := <-klineChan
	require.NotNil(t, kline1)

	// 创建限价开空单（价格设置在当前价格上方）
	limitPrice := decimal.NewFromFloat(50010.0)
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideShort,
		Price:       limitPrice,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待K线触发订单成交（K线最高价会高于限价）
	for i := 0; i < 5; i++ {
		kline := <-klineChan
		require.NotNil(t, kline)
	}

	// 检查订单是否成交
	orderInfo, err := svc.GetOrder(ctx, exchange.GetOrderReq{Id: orderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusFilled, orderInfo.Status)

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Len(t, positions, 1)
	assert.Equal(t, exchange.PositionSideShort, positions[0].PositionSide)
}

// TestExchangeService_ClosePosition 测试平仓
func TestExchangeService_ClosePosition(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成上涨趋势的K线数据（价格从50000涨到50100）
	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	kline1 := <-klineChan
	require.NotNil(t, kline1)

	// 开多仓
	openOrderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero, // 市价单
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待开仓成交
	kline2 := <-klineChan
	require.NotNil(t, kline2)

	// 确认持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	entryPrice := positions[0].EntryPrice

	// 记录开仓后的账户状态
	accountAfterOpen, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 等待几根K线让价格上涨
	for i := 0; i < 5; i++ {
		<-klineChan
	}

	// 平仓
	closeOrderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero, // 市价单
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待平仓成交
	kline3 := <-klineChan
	require.NotNil(t, kline3)

	// 检查订单状态
	closeOrder, err := svc.GetOrder(ctx, exchange.GetOrderReq{Id: closeOrderId})
	require.NoError(t, err)
	assert.Equal(t, exchange.OrderStatusFilled, closeOrder.Status)

	// 检查持仓已关闭
	positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Empty(t, positions)

	// 检查盈亏
	accountAfterClose, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 由于是上涨趋势，多头应该盈利
	assert.True(t, accountAfterClose.TotalBalance.GreaterThan(accountAfterOpen.TotalBalance),
		"平仓后总余额应该增加（盈利）")

	// 保证金应该已释放
	assert.True(t, accountAfterClose.UsedMargin.IsZero(), "平仓后保证金应该为0")

	// 检查持仓历史
	histories, err := svc.GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{})
	require.NoError(t, err)
	assert.Len(t, histories, 1)

	history := histories[0]
	assert.Equal(t, pair, history.TradingPair)
	assert.Equal(t, entryPrice, history.EntryPrice)
	assert.False(t, history.ClosePrice.IsZero())
	assert.NotEmpty(t, history.Events)

	t.Logf("开仓订单ID: %s", openOrderId)
	t.Logf("平仓订单ID: %s", closeOrderId)
	t.Logf("入场价: %s, 平仓价: %s", entryPrice, history.ClosePrice)
	t.Logf("开仓后余额: %s, 平仓后余额: %s",
		accountAfterOpen.TotalBalance, accountAfterClose.TotalBalance)
}

// TestExchangeService_Leverage 测试杠杆功能
func TestExchangeService_Leverage(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	// 设置10倍杠杆
	err := svc.SetLeverage(ctx, exchange.SetLeverageReq{
		TradingPair: pair,
		Leverage:    10,
	})
	require.NoError(t, err)

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 记录开仓前的可用余额
	accountBefore, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)
	availableBefore := accountBefore.AvailableBalance

	// 开仓
	quantity := decimal.NewFromFloat(0.1)
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    quantity,
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待成交
	<-klineChan

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	position := positions[0]
	assert.Equal(t, 10, position.Leverage)

	// 检查账户
	account, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 计算预期的保证金使用（价格 * 数量 / 杠杆）
	price := position.EntryPrice
	expectedMargin := price.Mul(quantity).Div(decimal.NewFromInt(10))

	// 使用的保证金应该约等于预期值
	assert.True(t, account.UsedMargin.Sub(expectedMargin).Abs().LessThan(decimal.NewFromFloat(1)),
		"使用的保证金应该约等于 价格*数量/杠杆")

	// 减少的可用余额应该约等于保证金
	reducedBalance := availableBefore.Sub(account.AvailableBalance)
	assert.True(t, reducedBalance.Sub(expectedMargin).Abs().LessThan(decimal.NewFromFloat(1)),
		"减少的余额应该约等于保证金")

	t.Logf("价格: %s, 数量: %s, 杠杆: %d", price, quantity, position.Leverage)
	t.Logf("预期保证金: %s, 实际使用保证金: %s", expectedMargin, account.UsedMargin)
	t.Logf("开仓前可用: %s, 开仓后可用: %s, 减少: %s",
		availableBefore, account.AvailableBalance, reducedBalance)
}

// TestExchangeService_StopLoss 测试止损功能
func TestExchangeService_StopLoss(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成先涨后跌的K线数据
	klines := make([]exchange.Kline, 20)
	basePrice := 50000.0
	for i := 0; i < 20; i++ {
		var price float64
		if i < 5 {
			price = basePrice + float64(i)*20 // 先涨
		} else {
			price = basePrice + 100 - float64(i-5)*30 // 后跌
		}

		openTime := startTime.Add(time.Duration(i) * interval.Duration())
		closeTime := openTime.Add(interval.Duration())

		klines[i] = exchange.Kline{
			OpenTime:         openTime,
			CloseTime:        closeTime,
			Open:             decimal.NewFromFloat(price - 5),
			Close:            decimal.NewFromFloat(price),
			High:             decimal.NewFromFloat(price + 20),
			Low:              decimal.NewFromFloat(price - 20),
			Volume:           decimal.NewFromFloat(1000),
			QuoteAssetVolume: decimal.NewFromFloat(price * 1000),
		}
	}
	provider.AddKlines(pair, interval, klines)

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 开多仓
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待开仓成交
	<-klineChan

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	entryPrice := positions[0].EntryPrice

	// 设置止损（价格下跌2%触发）
	stopLossPrice := entryPrice.Mul(decimal.NewFromFloat(0.98))
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       stopLossPrice,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 记录设置止损后的余额
	accountBefore, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 等待K线直到止损触发
	triggered := false
	for i := 0; i < 15; i++ {
		<-klineChan

		// 检查持仓是否已关闭
		positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
		require.NoError(t, err)

		if len(positions) == 0 {
			triggered = true
			t.Logf("止损在第 %d 根K线后触发", i+1)
			break
		}
	}

	assert.True(t, triggered, "止损应该被触发")

	// 检查持仓已关闭
	positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Empty(t, positions)

	// 检查账户余额（应该亏损）
	accountAfter, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	assert.True(t, accountAfter.TotalBalance.LessThan(accountBefore.TotalBalance),
		"触发止损后应该亏损")

	t.Logf("入场价: %s, 止损价: %s", entryPrice, stopLossPrice)
	t.Logf("止损前余额: %s, 止损后余额: %s, 亏损: %s",
		accountBefore.TotalBalance, accountAfter.TotalBalance,
		accountBefore.TotalBalance.Sub(accountAfter.TotalBalance))
}

// TestExchangeService_TakeProfit 测试止盈功能
func TestExchangeService_TakeProfit(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成上涨趋势的K线数据
	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 开多仓
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待开仓成交
	<-klineChan

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	entryPrice := positions[0].EntryPrice

	// 设置止盈（价格上涨2%触发）
	takeProfitPrice := entryPrice.Mul(decimal.NewFromFloat(1.02))
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: exchange.PositionSideLong,
		Price:       takeProfitPrice,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 记录设置止盈后的余额
	accountBefore, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 等待K线直到止盈触发
	triggered := false
	for i := 0; i < 15; i++ {
		<-klineChan

		// 检查持仓是否已关闭
		positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
		require.NoError(t, err)

		if len(positions) == 0 {
			triggered = true
			t.Logf("止盈在第 %d 根K线后触发", i+1)
			break
		}
	}

	assert.True(t, triggered, "止盈应该被触发")

	// 检查账户余额（应该盈利）
	accountAfter, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	assert.True(t, accountAfter.TotalBalance.GreaterThan(accountBefore.TotalBalance),
		"触发止盈后应该盈利")

	t.Logf("入场价: %s, 止盈价: %s", entryPrice, takeProfitPrice)
	t.Logf("止盈前余额: %s, 止盈后余额: %s, 盈利: %s",
		accountBefore.TotalBalance, accountAfter.TotalBalance,
		accountAfter.TotalBalance.Sub(accountBefore.TotalBalance))
}

// TestExchangeService_CancelOrder 测试取消订单
func TestExchangeService_CancelOrder(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 记录创建订单前的余额
	accountBefore, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 创建一个不会马上成交的限价单
	limitPrice := decimal.NewFromFloat(40000.0) // 远低于市价
	orderId, err := svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       limitPrice,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 检查订单状态（应该是挂单）
	orders, err := svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, exchange.OrderStatusPending, orders[0].Status)

	// 检查余额（应该已冻结资金）
	accountAfterCreate, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)
	assert.True(t, accountAfterCreate.AvailableBalance.LessThan(accountBefore.AvailableBalance),
		"创建订单后应该冻结资金")

	// 取消订单
	err = svc.CancelOrder(ctx, exchange.CancelOrderReq{
		Id:          orderId,
		TradingPair: pair,
	})
	require.NoError(t, err)

	// 检查订单已从待成交列表移除
	orders, err = svc.GetOrders(ctx, exchange.GetOrdersReq{TradingPair: pair})
	require.NoError(t, err)
	assert.Empty(t, orders)

	// 检查余额（应该已解冻资金）
	accountAfterCancel, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)
	assert.True(t, accountAfterCancel.AvailableBalance.Equal(accountBefore.AvailableBalance),
		"取消订单后应该返还冻结资金")

	t.Logf("创建订单前余额: %s", accountBefore.AvailableBalance)
	t.Logf("创建订单后余额: %s", accountAfterCreate.AvailableBalance)
	t.Logf("取消订单后余额: %s", accountAfterCancel.AvailableBalance)
}

// TestExchangeService_InsufficientBalance 测试余额不足
func TestExchangeService_InsufficientBalance(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 100.0 // 很少的余额

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 5, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 尝试开仓（数量过大，余额不足）
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(1.0), // 需要约50000，但只有100
		Timestamp:   time.Now(),
	})

	// 应该返回余额不足错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
}

// TestExchangeService_UnrealizedPnL 测试未实现盈亏更新
func TestExchangeService_UnrealizedPnL(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成上涨趋势的K线
	provider.GenerateKlines(pair, interval, startTime, 50000.0, 10, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 开多仓
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待开仓成交
	<-klineChan

	// 检查初始持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	entryPrice := positions[0].EntryPrice

	// 等待几根K线，价格上涨
	for i := 0; i < 5; i++ {
		<-klineChan
	}

	// 检查未实现盈亏
	positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	position := positions[0]

	// 由于价格上涨，多头的未实现盈亏应该为正
	assert.True(t, position.UnrealizedPnl.GreaterThan(decimal.Zero),
		"价格上涨，多头未实现盈亏应该为正")

	// 标记价格应该高于入场价
	assert.True(t, position.MarkPrice.GreaterThan(entryPrice),
		"标记价格应该高于入场价")

	// 账户的未实现盈亏应该与持仓一致
	account, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)
	assert.True(t, account.UnrealizedPnl.Equal(position.UnrealizedPnl),
		"账户未实现盈亏应该与持仓一致")

	t.Logf("入场价: %s, 当前价: %s", entryPrice, position.MarkPrice)
	t.Logf("未实现盈亏: %s", position.UnrealizedPnl)
}

// TestExchangeService_MultiplePositions 测试多个持仓
func TestExchangeService_MultiplePositions(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 50000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair1 := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	pair2 := exchange.TradingPair{Base: "ETH", Quote: "USDT"}
	interval := exchange.Interval5m

	// 为两个交易对生成K线
	provider.GenerateKlines(pair1, interval, startTime, 50000.0, 10, "up")
	provider.GenerateKlines(pair2, interval, startTime, 3000.0, 10, "up")

	ctx := context.Background()

	// 订阅BTC K线
	klineChan1, err := svc.SubscribeKline(ctx, pair1, interval)
	require.NoError(t, err)

	// 订阅ETH K线
	klineChan2, err := svc.SubscribeKline(ctx, pair2, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan1
	<-klineChan2

	// 开BTC多仓
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair1,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(0.1),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 开ETH空仓
	_, err = svc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: pair2,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideShort,
		Price:       decimal.Zero,
		Quantity:    decimal.NewFromFloat(1.0),
		Timestamp:   time.Now(),
	})
	require.NoError(t, err)

	// 等待成交
	<-klineChan1
	<-klineChan2

	// 检查持仓
	allPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{})
	require.NoError(t, err)
	assert.Len(t, allPositions, 2)

	// 检查BTC持仓
	btcPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair1})
	require.NoError(t, err)
	assert.Len(t, btcPositions, 1)
	assert.Equal(t, pair1, btcPositions[0].TradingPair)
	assert.Equal(t, exchange.PositionSideLong, btcPositions[0].PositionSide)

	// 检查ETH持仓
	ethPositions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair2})
	require.NoError(t, err)
	assert.Len(t, ethPositions, 1)
	assert.Equal(t, pair2, ethPositions[0].TradingPair)
	assert.Equal(t, exchange.PositionSideShort, ethPositions[0].PositionSide)

	t.Logf("BTC持仓: %+v", btcPositions[0])
	t.Logf("ETH持仓: %+v", ethPositions[0])
}

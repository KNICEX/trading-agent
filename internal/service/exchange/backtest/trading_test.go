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

// TestTradingService_OpenPosition 测试高层开仓接口
func TestTradingService_OpenPosition(t *testing.T) {
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

	// 使用TradingService开仓
	resp, err := svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero, // 市价单
		Quantity:     decimal.NewFromFloat(0.1),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.OrderId)
	assert.False(t, resp.EstimatedCost.IsZero())
	assert.False(t, resp.EstimatedPrice.IsZero())

	// 等待成交
	<-klineChan
	<-klineChan

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Len(t, positions, 1)

	t.Logf("订单ID: %s", resp.OrderId)
	t.Logf("预估成本: %s", resp.EstimatedCost)
	t.Logf("预估价格: %s", resp.EstimatedPrice)
}

// TestTradingService_OpenPositionWithBalancePercent 测试使用余额百分比开仓
func TestTradingService_OpenPositionWithBalancePercent(t *testing.T) {
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

	// 使用50%余额开仓
	_, err = svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:    pair,
		PositionSide:   exchange.PositionSideLong,
		Price:          decimal.Zero,
		BalancePercent: decimal.NewFromFloat(50), // 使用50%余额
	})
	require.NoError(t, err)

	// 等待成交
	<-klineChan
	<-klineChan

	// 检查账户余额
	account, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 使用的保证金应该约为初始余额的50%
	expectedMargin := decimal.NewFromFloat(initialBalance * 0.5)
	assert.True(t, account.UsedMargin.Sub(expectedMargin).Abs().LessThan(decimal.NewFromFloat(100)),
		"使用的保证金应该约为初始余额的50%")

	t.Logf("初始余额: %f", initialBalance)
	t.Logf("使用保证金: %s (预期约 %s)", account.UsedMargin, expectedMargin)
	t.Logf("可用余额: %s", account.AvailableBalance)
}

// TestTradingService_OpenPositionWithStopOrders 测试开仓时设置止盈止损
func TestTradingService_OpenPositionWithStopOrders(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 开仓并设置止盈止损
	resp, err := svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		Quantity:     decimal.NewFromFloat(0.1),
		TakeProfit: exchange.StopOrder{
			Price: decimal.NewFromFloat(51000), // 止盈价
		},
		StopLoss: exchange.StopOrder{
			Price: decimal.NewFromFloat(49000), // 止损价
		},
	})

	// 等待成交
	<-klineChan

	require.NoError(t, err)
	assert.NotEmpty(t, resp.OrderId)
	assert.NotEmpty(t, resp.TakeProfitId, "应该创建止盈订单")
	assert.NotEmpty(t, resp.StopLossId, "应该创建止损订单")

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	// 等待价格上涨触发止盈
	triggered := false
	for i := 0; i < 20; i++ {
		<-klineChan

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
	account, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)
	assert.True(t, account.TotalBalance.GreaterThan(decimal.NewFromFloat(initialBalance)),
		"触发止盈后应该盈利")

	t.Logf("止盈订单ID: %s", resp.TakeProfitId)
	t.Logf("止损订单ID: %s", resp.StopLossId)
	t.Logf("最终余额: %s (初始: %f)", account.TotalBalance, initialBalance)
}

// TestTradingService_ClosePosition 测试高层平仓接口
func TestTradingService_ClosePosition(t *testing.T) {
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

	// 开仓
	_, err = svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		Quantity:     decimal.NewFromFloat(0.1),
	})
	require.NoError(t, err)

	// 等待开仓成交
	<-klineChan

	// 等待几根K线
	for i := 0; i < 3; i++ {
		<-klineChan
	}

	// 平仓
	closeOrderId, err := svc.ClosePosition(ctx, exchange.ClosePositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		CloseAll:     true, // 全平
	})
	require.NoError(t, err)
	assert.NotEmpty(t, closeOrderId)

	// 等待平仓成交
	<-klineChan

	// 检查持仓已关闭
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	assert.Empty(t, positions)

	t.Logf("平仓订单ID: %s", closeOrderId)
}

// TestTradingService_ClosePositionByPercent 测试按百分比平仓
func TestTradingService_ClosePositionByPercent(t *testing.T) {
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

	// 开仓
	_, err = svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		Quantity:     decimal.NewFromFloat(1.0),
	})
	require.NoError(t, err)

	// 等待开仓成交
	<-klineChan

	// 检查初始持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)
	initialQuantity := positions[0].Quantity

	// 平仓50%
	_, err = svc.ClosePosition(ctx, exchange.ClosePositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		Percent:      decimal.NewFromFloat(50), // 平50%
	})
	require.NoError(t, err)

	// 等待平仓成交
	<-klineChan

	// 检查持仓数量
	positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	expectedQuantity := initialQuantity.Mul(decimal.NewFromFloat(0.5))
	assert.True(t, positions[0].Quantity.Equal(expectedQuantity),
		"平仓50%后，持仓数量应该减半")

	t.Logf("初始持仓: %s", initialQuantity)
	t.Logf("平仓50%%后: %s (预期: %s)", positions[0].Quantity, expectedQuantity)
}

// TestTradingService_ShortPosition 测试做空
func TestTradingService_ShortPosition(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	// 生成下跌趋势的K线
	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "down")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 记录开仓前余额
	accountBefore, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	// 开空仓
	_, err = svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideShort,
		Price:        decimal.Zero,
		Quantity:     decimal.NewFromFloat(0.1),
	})
	require.NoError(t, err)

	// 等待开仓成交
	<-klineChan

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)
	assert.Equal(t, exchange.PositionSideShort, positions[0].PositionSide)

	entryPrice := positions[0].EntryPrice

	// 等待价格下跌
	for i := 0; i < 10; i++ {
		<-klineChan
	}

	// 检查未实现盈亏
	positions, err = svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	// 价格下跌，空头应该盈利
	assert.True(t, positions[0].UnrealizedPnl.GreaterThan(decimal.Zero),
		"价格下跌，空头未实现盈亏应该为正")
	assert.True(t, positions[0].MarkPrice.LessThan(entryPrice),
		"当前价格应该低于入场价")

	// 平仓
	_, err = svc.ClosePosition(ctx, exchange.ClosePositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideShort,
		Price:        decimal.Zero,
		CloseAll:     true,
	})
	require.NoError(t, err)

	// 等待平仓成交
	<-klineChan

	// 检查盈利
	accountAfter, err := svc.GetAccountInfo(ctx)
	require.NoError(t, err)

	profit := accountAfter.TotalBalance.Sub(accountBefore.TotalBalance)
	assert.True(t, profit.GreaterThan(decimal.Zero), "做空下跌市场应该盈利")

	t.Logf("入场价: %s, 平仓价: %s", entryPrice, positions[0].MarkPrice)
	t.Logf("初始余额: %s, 最终余额: %s, 盈利: %s",
		accountBefore.TotalBalance, accountAfter.TotalBalance, profit)
}

// TestTradingService_SetStopOrders 测试设置止盈止损
func TestTradingService_SetStopOrders(t *testing.T) {
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

	// 开仓（不设置止盈止损）
	_, err = svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		Quantity:     decimal.NewFromFloat(0.1),
	})
	require.NoError(t, err)

	// 等待成交
	<-klineChan

	// 检查持仓
	positions, err := svc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	require.NoError(t, err)
	require.Len(t, positions, 1)

	entryPrice := positions[0].EntryPrice

	// 后续设置止盈止损
	stopResp, err := svc.SetStopOrders(ctx, exchange.SetStopOrdersReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		TakeProfit: exchange.StopOrder{
			Price: entryPrice.Mul(decimal.NewFromFloat(1.05)), // 止盈+5%
		},
		StopLoss: exchange.StopOrder{
			Price: entryPrice.Mul(decimal.NewFromFloat(0.95)), // 止损-5%
		},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, stopResp.TakeProfitId)
	assert.NotEmpty(t, stopResp.StopLossId)

	t.Logf("止盈订单ID: %s", stopResp.TakeProfitId)
	t.Logf("止损订单ID: %s", stopResp.StopLossId)
}

// TestTradingService_UpdateStopOrders 测试更新止盈止损
func TestTradingService_UpdateStopOrders(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(24 * time.Hour)
	initialBalance := 10000.0

	svc, provider := createTestExchange(t, initialBalance, startTime, endTime)

	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	interval := exchange.Interval5m

	provider.GenerateKlines(pair, interval, startTime, 50000.0, 20, "up")

	ctx := context.Background()

	// 订阅K线
	klineChan, err := svc.SubscribeKline(ctx, pair, interval)
	require.NoError(t, err)

	// 等待第一根K线
	<-klineChan

	// 开仓并设置止盈止损
	resp, err := svc.OpenPosition(ctx, exchange.OpenPositionReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.Zero,
		Quantity:     decimal.NewFromFloat(0.1),
		TakeProfit: exchange.StopOrder{
			Price: decimal.NewFromFloat(51000),
		},
		StopLoss: exchange.StopOrder{
			Price: decimal.NewFromFloat(49000),
		},
	})
	require.NoError(t, err)

	oldTakeProfitId := resp.TakeProfitId
	oldStopLossId := resp.StopLossId

	// 等待成交
	<-klineChan

	// 更新止盈止损（会取消旧的，创建新的）
	newStopResp, err := svc.SetStopOrders(ctx, exchange.SetStopOrdersReq{
		TradingPair:  pair,
		PositionSide: exchange.PositionSideLong,
		TakeProfit: exchange.StopOrder{
			Price: decimal.NewFromFloat(52000), // 提高止盈价
		},
		StopLoss: exchange.StopOrder{
			Price: decimal.NewFromFloat(48500), // 降低止损价
		},
	})
	require.NoError(t, err)

	// 新的订单ID应该不同
	assert.NotEqual(t, oldTakeProfitId, newStopResp.TakeProfitId)
	assert.NotEqual(t, oldStopLossId, newStopResp.StopLossId)

	t.Logf("旧止盈订单: %s, 新止盈订单: %s", oldTakeProfitId, newStopResp.TakeProfitId)
	t.Logf("旧止损订单: %s, 新止损订单: %s", oldStopLossId, newStopResp.StopLossId)
}

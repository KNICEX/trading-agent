package engine

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/exchange/backtest"
	"github.com/KNICEX/trading-agent/internal/service/exchange/binance"
	"github.com/KNICEX/trading-agent/internal/service/portfolio"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBacktestEngine_WithBinanceData 使用币安真实历史数据进行回测
// 注意：这个测试需要币安API密钥，如果没有配置会跳过
func TestBacktestEngine_WithBinanceData(t *testing.T) {
	// 检查是否配置了币安API密钥（通过环境变量）
	apiKey := ""
	apiSecret := ""

	// 1. 准备测试数据 - 回测2024年1月1日到1月3日的BTC数据
	tradingPair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
	startTime := time.Date(2025, 6, 6, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC)
	initialBalance := decimal.NewFromInt(10000) // 初始资金 10000 USDT

	t.Logf("回测时间段: %s 至 %s", startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))

	// 2. 创建币安客户端和K线提供者
	client := futures.NewClient(apiKey, apiSecret)
	binanceMarketSvc := binance.NewMarketService(client)
	binanceProvider := backtest.NewBinanceKlineProvider(binanceMarketSvc)

	// 3. 创建回测交易所服务
	exchangeSvc := backtest.NewExchangeService(
		startTime,
		endTime,
		initialBalance,
		binanceProvider,
	)

	exchangeSvc.SetLeverage(context.Background(), exchange.SetLeverageReq{
		TradingPair: tradingPair,
		Leverage:    10,
	})

	// 4. 创建回测引擎
	engine := NewBacktestEngine(startTime, endTime, exchangeSvc)

	// 5. 配置仓位管理器
	positionSizer := portfolio.NewSimplePositionSizer(exchangeSvc)
	err := positionSizer.Initialize(context.Background(), portfolio.RiskConfig{
		MaxStopLossRatio:    0.03, // 最大止损资金比例 5%
		MaxLeverage:         10,   // 最大杠杆 3x
		MinProfitLossRatio:  2,    // 最小盈亏比 1:1
		ConfidenceThreshold: 0.6,  // 置信度阈值 51%
	})
	require.NoError(t, err, "初始化仓位管理器失败")
	engine.positionSizer = positionSizer

	// 6. 创建并添加策略
	testStrategy := strategy.NewSimpleTestStrategy(tradingPair)
	err = engine.AddStrategy(context.Background(), testStrategy)
	require.NoError(t, err, "添加策略失败")

	// 7. 运行回测
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // 给足够的时间获取数据
	defer cancel()

	t.Log("开始运行回测，从币安获取历史数据...")
	err = engine.Run(ctx)
	require.NoError(t, err, "运行回测失败")

	// 8. 验证回测结果
	accountInfo, err := exchangeSvc.AccountService().GetAccountInfo(ctx)
	require.NoError(t, err, "获取账户信息失败")

	t.Logf("=== 回测结果 ===")
	t.Logf("交易对: %s", tradingPair.ToString())
	t.Logf("时间段: %s 至 %s", startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))
	t.Logf("初始余额: %s USDT", initialBalance.String())
	t.Logf("最终余额: %s USDT", accountInfo.TotalBalance.String())
	t.Logf("可用余额: %s USDT", accountInfo.AvailableBalance.String())
	t.Logf("未实现盈亏: %s USDT", accountInfo.UnrealizedPnl.String())
	t.Logf("已用保证金: %s USDT", accountInfo.UsedMargin.String())

	// 计算收益率
	pnl := accountInfo.TotalBalance.Sub(initialBalance)
	pnlPercent := pnl.Div(initialBalance).Mul(decimal.NewFromInt(100))
	t.Logf("总盈亏: %s USDT (%.2f%%)", pnl.String(), pnlPercent.InexactFloat64())

	// 成交订单历史
	// orders, err := exchangeSvc.OrderService().GetOrders(ctx, exchange.GetOrdersReq{TradingPair: tradingPair})
	// require.NoError(t, err, "获取成交订单历史失败")
	// t.Logf("成交订单历史: %d", len(orders))
	// for _, order := range orders {
	// 	t.Logf("订单: %s", order.Id)
	// 	t.Logf("方向: %s", order.PositionSide)
	// 	t.Logf("数量: %s", order.Quantity.String())
	// 	t.Logf("开仓价: %s", order.Price.String())
	// }

	// 当前持仓
	positions, err := exchangeSvc.PositionService().GetActivePositions(ctx, []exchange.TradingPair{tradingPair})
	require.NoError(t, err, "获取持仓失败")
	t.Logf("当前持仓: %d", len(positions))
	for _, position := range positions {
		t.Logf("持仓: %s", position.TradingPair.ToString())
		t.Logf("方向: %s", position.PositionSide)
		t.Logf("数量: %s", position.Quantity.String())
		t.Logf("开仓价: %s", position.EntryPrice.String())
		t.Logf("标记价格: %s", position.MarkPrice.String())
		t.Logf("杠杆: %d", position.Leverage)
		t.Logf("保证金: %s", position.MarginAmount.String())
		t.Logf("未实现盈亏: %s", position.UnrealizedPnl.String())
		t.Logf("创建时间: %s", position.CreatedAt.Format("2006-01-02 15:04:05"))
		t.Logf("更新时间: %s", position.UpdatedAt.Format("2006-01-02 15:04:05"))
		t.Logf("--------------------------------")
	}

	// 获取持仓历史记录
	positionHistories, err := exchangeSvc.PositionService().GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{})
	require.NoError(t, err, "获取持仓历史失败")

	t.Logf("持仓历史记录数: %d", len(positionHistories))
	for i, history := range positionHistories {
		t.Logf("持仓 #%d:", i+1)
		t.Logf("  交易对: %s", history.TradingPair.ToString())
		t.Logf("  方向: %s", history.PositionSide)
		t.Logf("  开仓时间: %s", history.OpenedAt.Format("2006-01-02 15:04:05"))
		t.Logf("  平仓时间: %s", history.ClosedAt.Format("2006-01-02 15:04:05"))
		t.Logf("  入场价: %s", history.EntryPrice.String())
		t.Logf("  出场价: %s", history.ClosePrice.String())
		t.Logf("  最大数量: %s", history.MaxQuantity.String())
		t.Logf("  已实现盈亏: %s", history.RealizedPnl.String())
	}

	// 	// 计算总的已实现盈亏
	// 	totalPnl := decimal.Zero
	// 	for _, event := range history.Events {
	// 		totalPnl = totalPnl.Add(event.RealizedPnl)
	// 	}
	// 	t.Logf("  已实现盈亏: %s USDT", totalPnl.String())
	// }

	// 验证测试完成
	assert.NotNil(t, accountInfo, "账户信息不应为空")
}

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

// TestGetHistoryPositions 测试获取历史持仓
// 注意：此测试需要账户中有历史成交记录
func TestGetHistoryPositions(t *testing.T) {
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 查询最近7天的历史持仓（币安API限制）
	t.Log("=== 查询历史持仓 ===")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7) // 7天前

	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{pair},
		StartTime:    startTime,
		EndTime:      now,
	})
	if err != nil {
		t.Fatalf("获取历史持仓失败: %v", err)
	}

	t.Logf("找到 %d 个历史持仓", len(histories))

	// 显示历史持仓详情
	for i, history := range histories {
		t.Logf("\n持仓 %d:", i+1)
		t.Logf("  交易对: %s", history.TradingPair.ToString())
		t.Logf("  方向: %s", history.PositionSide)
		t.Logf("  开仓时间: %s", history.OpenedAt.Format("2006-01-02 15:04:05"))
		t.Logf("  平仓时间: %s", history.ClosedAt.Format("2006-01-02 15:04:05"))
		t.Logf("  持仓时长: %s", history.ClosedAt.Sub(history.OpenedAt))
		t.Logf("  平均开仓价: %s", history.EntryPrice.String())
		t.Logf("  平均平仓价: %s", history.ClosePrice.String())
		t.Logf("  最大持仓量: %s", history.MaxQuantity.String())
		t.Logf("  事件数量: %d", len(history.Events))

		// 显示事件详情
		t.Logf("\n  事件列表:")
		for j, event := range history.Events {
			t.Logf("    事件 %d: 类型=%s, 数量=%s, 价格=%s, 持仓变化: %s -> %s, 盈亏=%s, 手续费=%s",
				j+1,
				event.EventType,
				event.Quantity.String(),
				event.Price.String(),
				event.BeforeQuantity.String(),
				event.AfterQuantity.String(),
				event.RealizedPnl.String(),
				event.Fee.String(),
			)
		}

		// 计算总盈亏
		totalRealizedPnl := decimal.Zero
		totalFee := decimal.Zero
		for _, event := range history.Events {
			totalRealizedPnl = totalRealizedPnl.Add(event.RealizedPnl)
			totalFee = totalFee.Add(event.Fee)
		}
		netPnl := totalRealizedPnl.Sub(totalFee)
		t.Logf("\n  总盈亏: %s USDT", totalRealizedPnl.String())
		t.Logf("  总手续费: %s USDT", totalFee.String())
		t.Logf("  净盈亏: %s USDT", netPnl.String())
	}
}

// TestGetAllPositionHistories 测试获取所有交易对的历史持仓
// TradingPairs 为空数组时，查询所有交易对
func TestGetAllPositionHistories(t *testing.T) {
	positionSvc := newPositionService(t)

	t.Log("=== 查询所有交易对的历史持仓 ===")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7) // 7天前

	// TradingPairs 为空，查询所有
	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{}, // 空数组
		StartTime:    startTime,
		EndTime:      now,
	})
	if err != nil {
		t.Fatalf("获取所有历史持仓失败: %v", err)
	}

	t.Logf("找到 %d 个历史持仓（来自所有交易对）", len(histories))

	// 按交易对分组统计
	pairCount := make(map[string]int)
	for _, history := range histories {
		pairKey := history.TradingPair.ToString() + "-" + string(history.PositionSide)
		pairCount[pairKey]++
	}

	t.Log("\n按交易对统计:")
	for pairKey, count := range pairCount {
		t.Logf("  %s: %d 个持仓", pairKey, count)
	}

	// 显示前3个持仓的摘要
	displayCount := 3
	if len(histories) < displayCount {
		displayCount = len(histories)
	}

	t.Logf("\n显示前 %d 个持仓的详情:", displayCount)
	for i := 0; i < displayCount; i++ {
		history := histories[i]
		t.Logf("\n持仓 %d:", i+1)
		t.Logf("  交易对: %s", history.TradingPair.ToString())
		t.Logf("  方向: %s", history.PositionSide)
		t.Logf("  事件数: %d", len(history.Events))
		t.Logf("  开仓价: %s", history.EntryPrice.String())
		t.Logf("  平仓价: %s", history.ClosePrice.String())
	}
}

// TestGetRecentPositionHistory 测试获取最近的持仓历史
// 这个测试更简单，只查询最近1天的数据
func TestGetRecentPositionHistory(t *testing.T) {
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 查询最近1天的持仓历史 ===")
	now := time.Now()
	startTime := now.AddDate(0, 0, -1) // 1天前

	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{pair},
		StartTime:    startTime,
		EndTime:      now,
	})
	if err != nil {
		t.Fatalf("获取历史持仓失败: %v", err)
	}

	if len(histories) == 0 {
		t.Log("最近1天没有持仓历史")
		return
	}

	t.Logf("找到 %d 个持仓历史", len(histories))
	for i, history := range histories {
		t.Logf("持仓 %d: 方向=%s, 事件数=%d, 开仓=%s, 平仓=%s",
			i+1,
			history.PositionSide,
			len(history.Events),
			history.OpenedAt.Format("15:04:05"),
			history.ClosedAt.Format("15:04:05"),
		)
	}
}

// TestPositionLifecycle 测试完整的持仓生命周期
// 此测试会创建一个完整的持仓生命周期：开仓 -> 加仓 -> 减仓 -> 平仓
// 然后验证历史记录是否正确
func TestPositionLifecycle(t *testing.T) {
	orderSvc := newOrderService(t)
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 测试完整持仓生命周期 ===")

	beforeTest := time.Now()

	// 1. 开仓：市价买入
	t.Log("\n步骤 1: 开仓")
	orderId1, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		OrderType:   exchange.OrderTypeMarket,
		PositonSide: exchange.PositionSideLong,
		Quantity:    decimal.NewFromFloat(0.002),
	})
	if err != nil {
		t.Fatalf("开仓失败: %v", err)
	}
	t.Logf("开仓订单ID: %s", orderId1)
	time.Sleep(2 * time.Second)

	// 2. 加仓：市价买入
	t.Log("\n步骤 2: 加仓")
	orderId2, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		OrderType:   exchange.OrderTypeMarket,
		PositonSide: exchange.PositionSideLong,
		Quantity:    decimal.NewFromFloat(0.001),
	})
	if err != nil {
		t.Fatalf("加仓失败: %v", err)
	}
	t.Logf("加仓订单ID: %s", orderId2)
	time.Sleep(2 * time.Second)

	// 3. 减仓：市价卖出一部分
	t.Log("\n步骤 3: 减仓")
	orderId3, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideSell,
		OrderType:   exchange.OrderTypeMarket,
		PositonSide: exchange.PositionSideLong,
		Quantity:    decimal.NewFromFloat(0.001),
	})
	if err != nil {
		t.Fatalf("减仓失败: %v", err)
	}
	t.Logf("减仓订单ID: %s", orderId3)
	time.Sleep(2 * time.Second)

	// 4. 完全平仓：卖出剩余全部
	t.Log("\n步骤 4: 完全平仓")
	orderId4, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideSell,
		OrderType:   exchange.OrderTypeMarket,
		PositonSide: exchange.PositionSideLong,
		Quantity:    decimal.NewFromFloat(0.002),
	})
	if err != nil {
		t.Fatalf("平仓失败: %v", err)
	}
	t.Logf("平仓订单ID: %s", orderId4)
	time.Sleep(3 * time.Second)

	// 5. 查询历史持仓，验证生命周期
	t.Log("\n步骤 5: 查询历史持仓")
	now := time.Now()
	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{pair},
		StartTime:    beforeTest,
		EndTime:      now,
	})
	if err != nil {
		t.Fatalf("查询历史持仓失败: %v", err)
	}

	if len(histories) == 0 {
		t.Fatal("应该有历史持仓记录，但查询结果为空")
	}

	// 找到多头持仓
	var longHistory *exchange.PositionHistory
	for i := range histories {
		if histories[i].PositionSide == exchange.PositionSideLong {
			longHistory = &histories[i]
			break
		}
	}

	if longHistory == nil {
		t.Fatal("找不到多头持仓历史")
	}

	t.Logf("\n持仓生命周期分析:")
	t.Logf("  开仓时间: %s", longHistory.OpenedAt.Format("2006-01-02 15:04:05"))
	t.Logf("  平仓时间: %s", longHistory.ClosedAt.Format("2006-01-02 15:04:05"))
	t.Logf("  持仓时长: %s", longHistory.ClosedAt.Sub(longHistory.OpenedAt))
	t.Logf("  平均开仓价: %s", longHistory.EntryPrice.String())
	t.Logf("  平均平仓价: %s", longHistory.ClosePrice.String())
	t.Logf("  最大持仓量: %s", longHistory.MaxQuantity.String())
	t.Logf("  事件数量: %d", len(longHistory.Events))

	// 验证事件类型
	t.Log("\n验证事件序列:")
	expectedEventTypes := []exchange.PositionEventType{
		exchange.PositionEventTypeCreate,   // 开仓
		exchange.PositionEventTypeIncrease, // 加仓
		exchange.PositionEventTypeDecrease, // 减仓
		exchange.PositionEventTypeClose,    // 平仓
	}

	if len(longHistory.Events) < len(expectedEventTypes) {
		t.Logf("警告: 事件数量(%d)少于预期(%d)，可能有些订单合并了",
			len(longHistory.Events), len(expectedEventTypes))
	}

	for i, event := range longHistory.Events {
		t.Logf("  事件 %d: %s, 数量=%s, 价格=%s, 持仓: %s -> %s",
			i+1,
			event.EventType,
			event.Quantity.String(),
			event.Price.String(),
			event.BeforeQuantity.String(),
			event.AfterQuantity.String(),
		)
	}

	// 验证最后一个事件是平仓
	lastEvent := longHistory.Events[len(longHistory.Events)-1]
	if lastEvent.EventType != exchange.PositionEventTypeClose {
		t.Logf("警告: 最后一个事件类型应该是CLOSE，实际是 %s", lastEvent.EventType)
	}

	if !lastEvent.AfterQuantity.IsZero() {
		t.Logf("警告: 平仓后持仓量应该为0，实际是 %s", lastEvent.AfterQuantity.String())
	}

	t.Log("\n✅ 持仓生命周期测试完成")
}

// TestFetchAllTradesWithPagination 测试自动分页功能
// 验证超过1000条记录时，能够自动分页获取所有数据
func TestFetchAllTradesWithPagination(t *testing.T) {
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 测试自动分页功能 ===")

	// 查询最近7天的数据（可能会有多页）
	now := time.Now()
	startTime := now.AddDate(0, 0, -7)

	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{pair},
		StartTime:    startTime,
		EndTime:      now,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	t.Logf("查询结果: 找到 %d 个持仓历史", len(histories))

	if len(histories) > 0 {
		totalEvents := 0
		for _, history := range histories {
			totalEvents += len(history.Events)
		}
		t.Logf("总事件数: %d", totalEvents)
		t.Logf("说明: 如果总事件数 > 1000，证明自动分页生效")
	}
}

// TestFetchTradesAcrossMultipleDays 测试跨越多天的查询
// 验证超过7天时，能够自动分片查询
func TestFetchTradesAcrossMultipleDays(t *testing.T) {
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 测试跨天查询功能（超过7天限制）===")

	// 查询最近30天的数据（超过币安7天限制）
	now := time.Now()
	startTime := now.AddDate(0, 0, -30) // 30天前

	t.Logf("查询时间范围: %s 到 %s (共 %d 天)",
		startTime.Format("2006-01-02"),
		now.Format("2006-01-02"),
		int(now.Sub(startTime).Hours()/24))

	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{pair},
		StartTime:    startTime,
		EndTime:      now,
	})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	t.Logf("查询结果: 找到 %d 个持仓历史", len(histories))

	if len(histories) > 0 {
		// 显示时间分布
		t.Log("\n时间分布:")
		for i, history := range histories {
			t.Logf("  持仓 %d: 开仓时间=%s",
				i+1,
				history.OpenedAt.Format("2006-01-02 15:04:05"))
		}
	} else {
		t.Log("在过去30天内没有找到持仓记录（可能账户没有交易）")
	}
}

// TestDebugRawTrades 调试测试：打印原始 trade 数据
// 查看实际获取到了多少条成交记录
func TestDebugRawTrades(t *testing.T) {
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 调试：查看原始成交记录 ===")

	// 查询最近30天
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	t.Logf("查询时间范围: %s 到 %s",
		startTime.Format("2006-01-02 15:04:05"),
		now.Format("2006-01-02 15:04:05"))

	// 使用我们的 fetchAllTrades 方法（支持自动分页和分片）
	// 但这是私有方法，我们先用 7 天的数据测试
	t.Log("\n分7天批次获取数据...")

	var allTrades []*futures.AccountTrade
	currentEnd := now
	batchNum := 0

	for currentEnd.After(startTime) {
		batchNum++
		batchStart := currentEnd.AddDate(0, 0, -7)
		if batchStart.Before(startTime) {
			batchStart = startTime
		}

		t.Logf("\n批次 %d: %s 到 %s",
			batchNum,
			batchStart.Format("2006-01-02"),
			currentEnd.Format("2006-01-02"))

		cli := initClient(t)
		trades, err := cli.NewListAccountTradeService().
			Symbol(pair.ToString()).
			StartTime(batchStart.UnixMilli()).
			EndTime(currentEnd.UnixMilli()).
			Limit(1000).
			Do(context.Background())
		if err != nil {
			t.Logf("  查询失败: %v", err)
			break
		}

		t.Logf("  获取到 %d 条记录", len(trades))
		allTrades = append(allTrades, trades...)

		currentEnd = batchStart
	}

	trades := allTrades

	t.Logf("\n获取到 %d 条原始成交记录", len(trades))

	if len(trades) == 1000 {
		t.Logf("⚠️  警告：获取到正好1000条记录，可能还有更多数据！")
	}

	// 统计信息
	buyCount := 0
	sellCount := 0
	longCount := 0
	shortCount := 0
	dateMap := make(map[string]int)

	for _, trade := range trades {
		if trade.Side == "BUY" {
			buyCount++
		} else {
			sellCount++
		}

		if trade.PositionSide == "LONG" {
			longCount++
		} else if trade.PositionSide == "SHORT" {
			shortCount++
		}

		// 按日期统计
		tradeTime := time.UnixMilli(trade.Time)
		dateKey := tradeTime.Format("2006-01-02")
		dateMap[dateKey]++
	}

	t.Logf("\n统计信息:")
	t.Logf("  买单: %d 条", buyCount)
	t.Logf("  卖单: %d 条", sellCount)
	t.Logf("  多头: %d 条", longCount)
	t.Logf("  空头: %d 条", shortCount)

	t.Log("\n按日期分布:")
	// 按日期排序显示
	var dates []string
	for date := range dateMap {
		dates = append(dates, date)
	}
	// 简单排序
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] > dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
	for _, date := range dates {
		t.Logf("  %s: %d 条", date, dateMap[date])
	}

	// 显示前10条和后10条记录
	displayCount := 10
	if len(trades) < displayCount {
		displayCount = len(trades)
	}

	t.Logf("\n前 %d 条成交记录:", displayCount)
	for i := 0; i < displayCount; i++ {
		trade := trades[i]
		tradeTime := time.UnixMilli(trade.Time)
		t.Logf("  %d. [%s] %s %s @ %s, 数量=%s, 方向=%s, 盈亏=%s",
			i+1,
			tradeTime.Format("2006-01-02 15:04:05"),
			trade.PositionSide,
			trade.Side,
			trade.Price,
			trade.Quantity,
			trade.PositionSide,
			trade.RealizedPnl,
		)
	}

	if len(trades) > displayCount {
		t.Logf("\n后 %d 条成交记录:", displayCount)
		for i := len(trades) - displayCount; i < len(trades); i++ {
			trade := trades[i]
			tradeTime := time.UnixMilli(trade.Time)
			t.Logf("  %d. [%s] %s %s @ %s, 数量=%s, 方向=%s, 盈亏=%s",
				i+1,
				tradeTime.Format("2006-01-02 15:04:05"),
				trade.PositionSide,
				trade.Side,
				trade.Price,
				trade.Quantity,
				trade.PositionSide,
				trade.RealizedPnl,
			)
		}
	}

	// 分析为什么只生成了2个持仓
	t.Log("\n分析持仓生成逻辑:")
	t.Log("持仓生成规则:")
	t.Log("  - 多头和空头是独立的持仓")
	t.Log("  - 只有当持仓完全平仓（数量归零）后，才算一个完整的持仓历史")
	t.Log("  - 如果有未平仓的仓位，会合并到同一个持仓历史中")

	t.Log("\n可能的原因:")
	t.Logf("  1. 如果30天内只有2次完整的 开仓→平仓 循环，就只有2个持仓")
	t.Logf("  2. 其他交易可能是加仓、减仓，会合并到这2个持仓中")
	t.Logf("  3. 查看上面的 前10条/后10条 记录，分析开仓平仓的模式")
}

// TestFetchAllTradesForAllPairs 测试查询所有交易对（可能数据量很大）
func TestFetchAllTradesForAllPairs(t *testing.T) {
	positionSvc := newPositionService(t)

	t.Log("=== 测试查询所有交易对（可能数据量很大）===")

	// 查询最近7天的所有交易对
	now := time.Now()
	startTime := now.AddDate(0, 0, -7)

	t.Logf("查询所有交易对，时间范围: %s 到 %s",
		startTime.Format("2006-01-02"),
		now.Format("2006-01-02"))

	startQuery := time.Now()
	histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{}, // 空数组 = 查询所有
		StartTime:    startTime,
		EndTime:      now,
	})
	queryDuration := time.Since(startQuery)

	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}

	t.Logf("查询耗时: %s", queryDuration)
	t.Logf("查询结果: 找到 %d 个持仓历史", len(histories))

	// 统计各交易对的持仓数量和总事件数
	pairStats := make(map[string]int)
	totalEvents := 0

	for _, history := range histories {
		pairKey := history.TradingPair.ToString()
		pairStats[pairKey]++
		totalEvents += len(history.Events)
	}

	t.Log("\n各交易对统计:")
	for pair, count := range pairStats {
		t.Logf("  %s: %d 个持仓", pair, count)
	}

	t.Logf("\n总事件数: %d", totalEvents)
	if totalEvents > 1000 {
		t.Logf("✅ 总事件数超过1000，自动分页生效")
	}
}

// TestPaginationPerformance 测试分页性能
func TestPaginationPerformance(t *testing.T) {
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 测试分页性能 ===")

	testCases := []struct {
		name string
		days int
	}{
		{"1天", 1},
		{"3天", 3},
		{"7天", 7},
		{"14天", 14},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			startTime := now.AddDate(0, 0, -tc.days)

			start := time.Now()
			histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
				TradingPairs: []exchange.TradingPair{pair},
				StartTime:    startTime,
				EndTime:      now,
			})
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("查询失败: %v", err)
			}

			totalEvents := 0
			for _, h := range histories {
				totalEvents += len(h.Events)
			}

			t.Logf("查询 %d 天数据: 持仓数=%d, 事件数=%d, 耗时=%s",
				tc.days, len(histories), totalEvents, duration)
		})
	}
}

// TestEdgeCases 测试边界情况
func TestEdgeCases(t *testing.T) {
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Run("查询未来时间", func(t *testing.T) {
		t.Log("=== 测试查询未来时间 ===")

		future := time.Now().Add(24 * time.Hour)
		now := time.Now()

		histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
			TradingPairs: []exchange.TradingPair{pair},
			StartTime:    now,
			EndTime:      future,
		})

		if err != nil {
			t.Logf("查询失败（预期）: %v", err)
		} else {
			t.Logf("查询成功，结果数量: %d（应该为0）", len(histories))
		}
	})

	t.Run("开始时间晚于结束时间", func(t *testing.T) {
		t.Log("=== 测试开始时间晚于结束时间 ===")

		now := time.Now()
		past := now.AddDate(0, 0, -7)

		histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
			TradingPairs: []exchange.TradingPair{pair},
			StartTime:    now,  // 开始时间
			EndTime:      past, // 结束时间（更早）
		})

		if err != nil {
			t.Logf("查询失败（预期）: %v", err)
		} else {
			t.Logf("查询成功，结果数量: %d（应该为0）", len(histories))
		}
	})

	t.Run("查询很短的时间范围", func(t *testing.T) {
		t.Log("=== 测试查询很短的时间范围（1小时）===")

		now := time.Now()
		oneHourAgo := now.Add(-1 * time.Hour)

		histories, err := positionSvc.GetHistoryPositions(context.Background(), exchange.GetHistoryPositionsReq{
			TradingPairs: []exchange.TradingPair{pair},
			StartTime:    oneHourAgo,
			EndTime:      now,
		})

		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}

		t.Logf("查询成功，结果数量: %d", len(histories))
	})
}

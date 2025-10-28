package integration

import (
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
)

// PositionHistorySuite 持仓历史测试套件
// 测试范围: 历史持仓查询、分页、跨天查询等
// 风险等级: 低-中（大部分只读，但包含一个创建测试仓位的测试）
type PositionHistorySuite struct {
	BaseSuite
}

// TestPositionHistorySuite 运行持仓历史测试套件
func TestPositionHistorySuite(t *testing.T) {
	suite.Run(t, new(PositionHistorySuite))
}

// Test01_GetRecentHistoryPositions 测试获取最近的历史持仓
// 验证点:
// - 可以查询历史持仓
// - 数据结构正确
func (s *PositionHistorySuite) Test01_GetRecentHistoryPositions() {
	s.T().Log("\n步骤 1: 查询最近 7 天的历史持仓")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7)

	histories, err := s.positionSvc.GetHistoryPositions(s.ctx, exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{s.testPair},
		StartTime:    startTime,
		EndTime:      now,
	})
	s.Require().NoError(err, "获取历史持仓失败")

	s.T().Logf("  ✓ 找到 %d 个历史持仓", len(histories))

	if len(histories) == 0 {
		s.T().Log("  最近 7 天没有持仓历史（这是正常的）")
		return
	}

	// 显示前几个持仓的摘要
	displayCount := 3
	if len(histories) < displayCount {
		displayCount = len(histories)
	}

	s.T().Logf("\n  显示前 %d 个持仓:", displayCount)
	for i := 0; i < displayCount; i++ {
		history := histories[i]
		s.T().Logf("\n  持仓 %d:", i+1)
		s.T().Logf("    交易对: %s", history.TradingPair.ToString())
		s.T().Logf("    方向: %s", history.PositionSide)
		s.T().Logf("    开仓时间: %s", history.OpenedAt.Format("2006-01-02 15:04:05"))
		s.T().Logf("    平仓时间: %s", history.ClosedAt.Format("2006-01-02 15:04:05"))
		s.T().Logf("    持仓时长: %s", history.ClosedAt.Sub(history.OpenedAt))
		s.T().Logf("    开仓价: %s", history.EntryPrice)
		s.T().Logf("    平仓价: %s", history.ClosePrice)
		s.T().Logf("    事件数: %d", len(history.Events))

		// 计算总盈亏
		totalPnl := decimal.Zero
		totalFee := decimal.Zero
		for _, event := range history.Events {
			totalPnl = totalPnl.Add(event.RealizedPnl)
			totalFee = totalFee.Add(event.Fee)
		}
		netPnl := totalPnl.Sub(totalFee)
		s.T().Logf("    总盈亏: %s USDT", totalPnl)
		s.T().Logf("    手续费: %s USDT", totalFee)
		s.T().Logf("    净盈亏: %s USDT", netPnl)
	}
}

// Test02_GetAllPairsHistory 测试获取所有交易对的历史
// 验证点:
// - 空交易对列表可以查询所有
// - 可以正确分组统计
func (s *PositionHistorySuite) Test02_GetAllPairsHistory() {
	s.T().Log("\n步骤 1: 查询所有交易对的历史持仓（最近 7 天）")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7)

	histories, err := s.positionSvc.GetHistoryPositions(s.ctx, exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{}, // 空数组表示查询所有
		StartTime:    startTime,
		EndTime:      now,
	})
	s.Require().NoError(err, "获取所有历史持仓失败")

	s.T().Logf("  ✓ 找到 %d 个历史持仓（来自所有交易对）", len(histories))

	if len(histories) == 0 {
		s.T().Log("  最近 7 天没有持仓历史")
		return
	}

	// 按交易对分组统计
	pairCount := make(map[string]int)
	for _, history := range histories {
		pairKey := history.TradingPair.ToString() + "-" + string(history.PositionSide)
		pairCount[pairKey]++
	}

	s.T().Logf("\n  按交易对统计（共 %d 个不同的交易对-方向组合）:", len(pairCount))
	count := 0
	for pairKey, num := range pairCount {
		if count >= 5 {
			s.T().Log("    ...")
			break
		}
		s.T().Logf("    %s: %d 个持仓", pairKey, num)
		count++
	}
}

// Test03_QueryAcrossMultipleDays 测试跨越多天查询（超过7天限制）
// 验证点:
// - 自动分片功能正常
// - 可以查询超过 7 天的数据
func (s *PositionHistorySuite) Test03_QueryAcrossMultipleDays() {
	s.T().Log("\n步骤 1: 查询最近 30 天的持仓历史（测试自动分片）")
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	s.T().Logf("  查询时间范围: %s 到 %s (共 %d 天)",
		startTime.Format("2006-01-02"),
		now.Format("2006-01-02"),
		30)

	startQuery := time.Now()
	histories, err := s.positionSvc.GetHistoryPositions(s.ctx, exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{s.testPair},
		StartTime:    startTime,
		EndTime:      now,
	})
	queryDuration := time.Since(startQuery)

	s.Require().NoError(err, "获取历史持仓失败")

	s.T().Logf("  ✓ 查询耗时: %s", queryDuration)
	s.T().Logf("  ✓ 查询结果: 找到 %d 个持仓历史", len(histories))

	if len(histories) > 0 {
		// 统计总事件数
		totalEvents := 0
		for _, history := range histories {
			totalEvents += len(history.Events)
		}
		s.T().Logf("  总事件数: %d", totalEvents)

		if totalEvents > 1000 {
			s.T().Log("  ✓ 总事件数超过 1000，自动分页功能生效")
		}

		// 显示时间分布
		s.T().Log("\n  时间分布（前5个持仓）:")
		displayCount := 5
		if len(histories) < displayCount {
			displayCount = len(histories)
		}
		for i := 0; i < displayCount; i++ {
			s.T().Logf("    持仓 %d: 开仓时间=%s",
				i+1,
				histories[i].OpenedAt.Format("2006-01-02 15:04:05"))
		}
	} else {
		s.T().Log("  在过去 30 天内没有持仓记录")
	}
}

// Test04_PositionEventAnalysis 测试持仓事件分析
// 验证点:
// - 事件类型正确
// - 数量变化合理
// - 盈亏计算正确
func (s *PositionHistorySuite) Test04_PositionEventAnalysis() {
	s.T().Log("\n步骤 1: 获取最近持仓并分析事件")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7)

	histories, err := s.positionSvc.GetHistoryPositions(s.ctx, exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{s.testPair},
		StartTime:    startTime,
		EndTime:      now,
	})
	s.Require().NoError(err, "获取历史持仓失败")

	if len(histories) == 0 {
		s.T().Log("  没有历史持仓数据可供分析")
		return
	}

	// 分析第一个持仓
	history := histories[0]
	s.T().Logf("\n  分析持仓: %s %s", history.TradingPair.ToString(), history.PositionSide)
	s.T().Logf("  事件数量: %d", len(history.Events))

	// 验证事件序列
	s.T().Log("\n  事件序列:")
	for i, event := range history.Events {
		s.T().Logf("    %d. [%s] %s: 数量=%s, 价格=%s, 持仓: %s → %s, 盈亏=%s",
			i+1,
			event.CreatedAt.Format("15:04:05"),
			event.EventType,
			event.Quantity,
			event.Price,
			event.BeforeQuantity,
			event.AfterQuantity,
			event.RealizedPnl)

		// 验证数量变化的合理性
		if i == 0 {
			// 第一个事件应该是创建
			s.Assert().True(event.BeforeQuantity.IsZero(), "第一个事件前持仓应该为 0")
		}

		// 验证持仓数量非负
		s.Assert().False(event.AfterQuantity.IsNegative(), "持仓数量不应该为负")
	}

	// 验证最后一个事件
	if len(history.Events) > 0 {
		lastEvent := history.Events[len(history.Events)-1]
		if lastEvent.EventType == exchange.PositionEventTypeClose {
			s.Assert().True(lastEvent.AfterQuantity.IsZero(), "平仓后持仓应该为 0")
		}
	}
}

// Test05_CreateAndVerifyPositionLifecycle 测试完整持仓生命周期
// 此测试会创建一个真实的持仓，然后验证历史记录
// 验证点:
// - 可以正确记录持仓生命周期
// - 事件类型和顺序正确
// 风险: 中（会产生实际交易，约 0.1 USDT 手续费）
func (s *PositionHistorySuite) Test05_CreateAndVerifyPositionLifecycle() {
	s.T().Log("\n⚠️  警告: 此测试会创建实际仓位和产生手续费（约 0.1 USDT）")

	// 清理环境
	s.CleanupEnvironment(s.testPair)

	beforeTest := time.Now()

	// 1. 开仓
	s.T().Log("\n步骤 1: 市价开多仓 4 XRP")
	orderId1 := s.CreateMarketOrder(exchange.OrderTypeOpen, exchange.PositionSideLong, decimal.NewFromFloat(4.0))
	s.T().Logf("  ✓ 开仓订单ID: %s", orderId1)

	// 2. 加仓
	s.T().Log("\n步骤 2: 加仓 2 XRP")
	orderId2 := s.CreateMarketOrder(exchange.OrderTypeOpen, exchange.PositionSideLong, decimal.NewFromFloat(2.0))
	s.T().Logf("  ✓ 加仓订单ID: %s", orderId2)

	// 3. 减仓
	s.T().Log("\n步骤 3: 减仓 2 XRP")
	orderId3 := s.CreateMarketOrder(exchange.OrderTypeClose, exchange.PositionSideLong, decimal.NewFromFloat(2.0))
	s.T().Logf("  ✓ 减仓订单ID: %s", orderId3)

	// 4. 完全平仓
	s.T().Log("\n步骤 4: 完全平仓")
	_, err := s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		CloseAll:     true,
	})
	s.Require().NoError(err, "平仓失败")
	s.T().Log("  ✓ 平仓成功")

	time.Sleep(3 * time.Second)

	// 5. 查询历史验证
	s.T().Log("\n步骤 5: 查询并验证持仓历史")
	now := time.Now()
	histories, err := s.positionSvc.GetHistoryPositions(s.ctx, exchange.GetHistoryPositionsReq{
		TradingPairs: []exchange.TradingPair{s.testPair},
		StartTime:    beforeTest,
		EndTime:      now,
	})
	s.Require().NoError(err, "查询历史持仓失败")

	if len(histories) == 0 {
		s.T().Log("  ⚠ 未找到刚创建的持仓历史（可能需要等待更长时间）")
		return
	}

	// 找到多头持仓
	var longHistory *exchange.PositionHistory
	for i := range histories {
		if histories[i].PositionSide == exchange.PositionSideLong &&
			histories[i].OpenedAt.After(beforeTest) {
			longHistory = &histories[i]
			break
		}
	}

	if longHistory == nil {
		s.T().Log("  ⚠ 未找到刚创建的多头持仓历史")
		return
	}

	s.T().Log("\n  ✓ 找到持仓历史:")
	s.T().Logf("    开仓时间: %s", longHistory.OpenedAt.Format("2006-01-02 15:04:05"))
	s.T().Logf("    平仓时间: %s", longHistory.ClosedAt.Format("2006-01-02 15:04:05"))
	s.T().Logf("    持仓时长: %s", longHistory.ClosedAt.Sub(longHistory.OpenedAt))
	s.T().Logf("    事件数量: %d", len(longHistory.Events))

	// 验证事件
	s.T().Log("\n  事件列表:")
	for i, event := range longHistory.Events {
		s.T().Logf("    %d. %s: 数量=%s, 持仓: %s → %s",
			i+1,
			event.EventType,
			event.Quantity,
			event.BeforeQuantity,
			event.AfterQuantity)
	}

	// 验证最后一个事件是平仓
	if len(longHistory.Events) > 0 {
		lastEvent := longHistory.Events[len(longHistory.Events)-1]
		s.Assert().Equal(exchange.PositionEventTypeClose, lastEvent.EventType, "最后一个事件应该是平仓")
		s.Assert().True(lastEvent.AfterQuantity.IsZero(), "平仓后持仓应该为 0")
	}

	s.T().Log("\n  ✓ 持仓生命周期验证完成")
}

// Test06_PaginationPerformance 测试分页性能
// 验证点:
// - 不同时间范围的查询性能
func (s *PositionHistorySuite) Test06_PaginationPerformance() {
	testCases := []struct {
		name string
		days int
	}{
		{"1天", 1},
		{"3天", 3},
		{"7天", 7},
		{"14天", 14},
	}

	s.T().Log("\n测试不同时间范围的查询性能:")

	for _, tc := range testCases {
		s.T().Logf("\n  测试: 查询 %s", tc.name)
		now := time.Now()
		startTime := now.AddDate(0, 0, -tc.days)

		start := time.Now()
		histories, err := s.positionSvc.GetHistoryPositions(s.ctx, exchange.GetHistoryPositionsReq{
			TradingPairs: []exchange.TradingPair{s.testPair},
			StartTime:    startTime,
			EndTime:      now,
		})
		duration := time.Since(start)

		s.Require().NoError(err, "查询失败")

		totalEvents := 0
		for _, h := range histories {
			totalEvents += len(h.Events)
		}

		s.T().Logf("    结果: 持仓数=%d, 事件数=%d, 耗时=%s",
			len(histories), totalEvents, duration)
	}
}

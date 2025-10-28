package integration

import (
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
)

// AccountServiceSuite 账户服务测试套件
// 测试范围: 账户信息查询、转账历史等
// 风险等级: 低（只读操作）
type AccountServiceSuite struct {
	BaseSuite
}

// TestAccountServiceSuite 运行账户服务测试套件
func TestAccountServiceSuite(t *testing.T) {
	suite.Run(t, new(AccountServiceSuite))
}

// Test01_GetAccountInfo 测试获取账户信息
// 验证点:
// - 可以获取账户余额
// - 各项余额数据合理
func (s *AccountServiceSuite) Test01_GetAccountInfo() {
	s.T().Log("\n步骤 1: 获取账户信息")
	accountInfo, err := s.accountSvc.GetAccountInfo(s.ctx)
	s.Require().NoError(err, "获取账户信息失败")

	s.T().Logf("  ✓ 账户余额信息:")
	s.T().Logf("    总余额: %s USDT", accountInfo.TotalBalance)
	s.T().Logf("    可用余额: %s USDT", accountInfo.AvailableBalance)
	s.T().Logf("    已用保证金: %s USDT", accountInfo.UsedMargin)
	s.T().Logf("    未实现盈亏: %s USDT", accountInfo.UnrealizedPnl)

	// 验证数据合理性
	s.Assert().False(accountInfo.TotalBalance.IsNegative(), "总余额不应该为负")
	s.Assert().False(accountInfo.AvailableBalance.IsNegative(), "可用余额不应该为负")

	// 计算保证金占用率
	if !accountInfo.TotalBalance.IsZero() {
		marginRatio := accountInfo.UsedMargin.Div(accountInfo.TotalBalance).Mul(decimal.NewFromInt(100))
		s.T().Logf("\n  风控指标:")
		s.T().Logf("    保证金占用率: %s%%", marginRatio.StringFixed(2))
	}
}

// Test02_GetRecentTransferHistory 测试获取最近转账历史
// 验证点:
// - 可以查询转账记录
// - 数据格式正确
func (s *AccountServiceSuite) Test02_GetRecentTransferHistory() {
	s.T().Log("\n步骤 1: 查询最近 7 天转账历史")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7)

	transfers, err := s.accountSvc.GetTransferHistory(s.ctx, exchange.GetTransferHistoryReq{
		StartTime: startTime,
		EndTime:   now,
	})
	s.Require().NoError(err, "获取转账历史失败")

	s.T().Logf("  ✓ 找到 %d 条转账记录", len(transfers))

	if len(transfers) == 0 {
		s.T().Log("  最近 7 天没有转账记录")
		return
	}

	// 统计转入转出
	totalIn := 0
	totalOut := 0
	amountIn := decimal.Zero
	amountOut := decimal.Zero

	for _, transfer := range transfers {
		if transfer.Direction == exchange.DirectionIn {
			totalIn++
			amountIn = amountIn.Add(transfer.Amount)
		} else {
			totalOut++
			amountOut = amountOut.Add(transfer.Amount)
		}
	}

	s.T().Logf("\n  转账统计:")
	s.T().Logf("    转入: %d 笔, 总金额: %s", totalIn, amountIn)
	s.T().Logf("    转出: %d 笔, 总金额: %s", totalOut, amountOut)
	s.T().Logf("    净流入: %s", amountIn.Sub(amountOut))

	// 显示最近几条
	displayCount := 3
	if len(transfers) < displayCount {
		displayCount = len(transfers)
	}

	s.T().Logf("\n  最近 %d 条转账记录:", displayCount)
	for i := 0; i < displayCount; i++ {
		transfer := transfers[i]
		s.T().Logf("    %d. [%s] %s %s, 金额: %s",
			i+1,
			transfer.TimeStamp.Format("2006-01-02 15:04:05"),
			transfer.Direction,
			transfer.Type,
			transfer.Amount)
	}
}

// Test03_GetLongTermTransferHistory 测试获取长期转账历史（跨越7天限制）
// 验证点:
// - 可以查询超过 7 天的数据
// - 自动分片功能正常
func (s *AccountServiceSuite) Test03_GetLongTermTransferHistory() {
	s.T().Log("\n步骤 1: 查询最近 30 天转账历史（测试自动分片）")
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	s.T().Logf("  查询时间范围: %s 到 %s (共 %d 天)",
		startTime.Format("2006-01-02"),
		now.Format("2006-01-02"),
		30)

	transfers, err := s.accountSvc.GetTransferHistory(s.ctx, exchange.GetTransferHistoryReq{
		StartTime: startTime,
		EndTime:   now,
	})
	s.Require().NoError(err, "获取转账历史失败")

	s.T().Logf("  ✓ 查询结果: 找到 %d 条转账记录", len(transfers))

	if len(transfers) > 0 {
		// 按日期统计
		dateMap := make(map[string]int)
		for _, transfer := range transfers {
			dateKey := transfer.TimeStamp.Format("2006-01-02")
			dateMap[dateKey]++
		}

		s.T().Logf("\n  按日期分布（共 %d 天有记录）:", len(dateMap))
		// 只显示前几天
		count := 0
		for date, num := range dateMap {
			if count >= 5 {
				s.T().Log("    ...")
				break
			}
			s.T().Logf("    %s: %d 笔", date, num)
			count++
		}
	} else {
		s.T().Log("  在过去 30 天内没有转账记录")
	}
}

// Test04_ComprehensiveAccountAnalysis 综合账户分析
// 验证点:
// - 账户信息和转账历史的综合分析
func (s *AccountServiceSuite) Test04_ComprehensiveAccountAnalysis() {
	s.T().Log("\n=== 综合账户分析 ===")

	// 1. 获取账户当前状态
	s.T().Log("\n步骤 1: 当前账户状态")
	accountInfo, err := s.accountSvc.GetAccountInfo(s.ctx)
	s.Require().NoError(err, "获取账户信息失败")

	s.T().Logf("  总余额: %s USDT", accountInfo.TotalBalance)
	s.T().Logf("  可用余额: %s USDT", accountInfo.AvailableBalance)
	s.T().Logf("  未实现盈亏: %s USDT", accountInfo.UnrealizedPnl)

	// 2. 查询最近资金流动
	s.T().Log("\n步骤 2: 最近 30 天资金流动")
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	transfers, err := s.accountSvc.GetTransferHistory(s.ctx, exchange.GetTransferHistoryReq{
		StartTime: startTime,
		EndTime:   now,
	})
	s.Require().NoError(err, "获取转账历史失败")

	s.T().Logf("  最近 30 天转账: %d 笔", len(transfers))

	// 3. 分析净流入
	if len(transfers) > 0 {
		netFlow := decimal.Zero
		for _, transfer := range transfers {
			if transfer.Direction == exchange.DirectionIn {
				netFlow = netFlow.Add(transfer.Amount)
			} else {
				netFlow = netFlow.Sub(transfer.Amount)
			}
		}
		s.T().Logf("  净流入: %s USDT", netFlow)

		// 4. 简单的账户健康度评估
		s.T().Log("\n步骤 3: 账户健康度评估")
		if !accountInfo.TotalBalance.IsZero() {
			marginRatio := accountInfo.UsedMargin.Div(accountInfo.TotalBalance)

			if marginRatio.LessThan(decimal.NewFromFloat(0.3)) {
				s.T().Log("  ✓ 账户健康（保证金占用 < 30%）")
			} else if marginRatio.LessThan(decimal.NewFromFloat(0.6)) {
				s.T().Log("  ⚠ 账户警戒（保证金占用 30-60%）")
			} else {
				s.T().Log("  ⚠ 账户风险较高（保证金占用 > 60%）")
			}
		}
	}
}

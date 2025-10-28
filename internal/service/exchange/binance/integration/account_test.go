package integration

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/exchange/binance"
	"github.com/shopspring/decimal"
)

// newAccountService 创建账户服务
func newAccountService(t *testing.T) exchange.AccountService {
	return binance.NewAccountService(initClient(t))
}

// TestGetAccountInfo 测试获取账户信息
func TestGetAccountInfo(t *testing.T) {
	accountSvc := newAccountService(t)

	t.Log("=== 获取账户信息 ===")
	accountInfo, err := accountSvc.GetAccountInfo(context.Background())
	if err != nil {
		t.Fatalf("获取账户信息失败: %v", err)
	}

	t.Logf("账户余额信息:")
	t.Logf("  总余额: %s USDT", accountInfo.TotalBalance.String())
	t.Logf("  可用余额: %s USDT", accountInfo.AvailableBalance.String())
	t.Logf("  已用保证金: %s USDT", accountInfo.UsedMargin.String())
	t.Logf("  未实现盈亏: %s USDT", accountInfo.UnrealizedPnl.String())

	// 计算一些有用的指标
	if !accountInfo.TotalBalance.IsZero() {
		marginRatio := accountInfo.UsedMargin.Div(accountInfo.TotalBalance).Mul(decimal.NewFromInt(100))
		t.Logf("\n风控指标:")
		t.Logf("  保证金占用率: %s%%", marginRatio.String())
	}
}

// TestGetTransferHistory 测试获取转账历史
func TestGetTransferHistory(t *testing.T) {
	accountSvc := newAccountService(t)

	t.Log("=== 获取转账历史 ===")
	now := time.Now()
	startTime := now.AddDate(0, 0, -7) // 最近7天

	t.Logf("查询时间范围: %s 到 %s",
		startTime.Format("2006-01-02"),
		now.Format("2006-01-02"))

	transfers, err := accountSvc.GetTransferHistory(context.Background(), exchange.GetTransferHistoryReq{
		StartTime: startTime,
		EndTime:   now,
	})
	if err != nil {
		t.Fatalf("获取转账历史失败: %v", err)
	}

	t.Logf("找到 %d 条转账记录", len(transfers))

	if len(transfers) == 0 {
		t.Log("最近7天没有转账记录")
		return
	}

	// 统计转入转出
	var totalIn, totalOut int
	var amountIn, amountOut = decimal.Zero, decimal.Zero

	for _, transfer := range transfers {
		if transfer.Direction == exchange.DirectionIn {
			totalIn++
			amountIn = amountIn.Add(transfer.Amount)
		} else {
			totalOut++
			amountOut = amountOut.Add(transfer.Amount)
		}
	}

	t.Logf("\n转账统计:")
	t.Logf("  转入: %d 笔, 总金额: %s", totalIn, amountIn.String())
	t.Logf("  转出: %d 笔, 总金额: %s", totalOut, amountOut.String())
	t.Logf("  净流入: %s", amountIn.Sub(amountOut).String())

	// 显示最近5条记录
	displayCount := 5
	if len(transfers) < displayCount {
		displayCount = len(transfers)
	}

	t.Logf("\n最近 %d 条转账记录:", displayCount)
	for i := 0; i < displayCount; i++ {
		transfer := transfers[i]
		t.Logf("  %d. [%s] %s %s, 金额: %s",
			i+1,
			transfer.TimeStamp.Format("2006-01-02 15:04:05"),
			transfer.Direction,
			transfer.Type,
			transfer.Amount.String(),
		)
	}
}

// TestGetTransferHistoryAcrossMultipleDays 测试跨越多天的转账历史查询
func TestGetTransferHistoryAcrossMultipleDays(t *testing.T) {
	accountSvc := newAccountService(t)

	t.Log("=== 测试跨天查询转账历史（超过7天限制）===")

	// 查询最近30天
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	t.Logf("查询时间范围: %s 到 %s (共 %d 天)",
		startTime.Format("2006-01-02"),
		now.Format("2006-01-02"),
		int(now.Sub(startTime).Hours()/24))

	transfers, err := accountSvc.GetTransferHistory(context.Background(), exchange.GetTransferHistoryReq{
		StartTime: startTime,
		EndTime:   now,
	})
	if err != nil {
		t.Fatalf("获取转账历史失败: %v", err)
	}

	t.Logf("查询结果: 找到 %d 条转账记录", len(transfers))

	if len(transfers) > 0 {
		// 按日期统计
		dateMap := make(map[string]int)
		for _, transfer := range transfers {
			dateKey := transfer.TimeStamp.Format("2006-01-02")
			dateMap[dateKey]++
		}

		t.Log("\n按日期分布:")
		for date, count := range dateMap {
			t.Logf("  %s: %d 笔", date, count)
		}
	} else {
		t.Log("在过去30天内没有转账记录")
	}
}

// TestAccountInfoAndTransfer 综合测试：账户信息 + 转账历史
func TestAccountInfoAndTransfer(t *testing.T) {
	accountSvc := newAccountService(t)

	// 1. 获取账户信息
	t.Log("=== 步骤 1: 获取账户信息 ===")
	accountInfo, err := accountSvc.GetAccountInfo(context.Background())
	if err != nil {
		t.Fatalf("获取账户信息失败: %v", err)
	}

	t.Logf("当前账户状态:")
	t.Logf("  总余额: %s", accountInfo.TotalBalance.String())
	t.Logf("  可用余额: %s", accountInfo.AvailableBalance.String())
	t.Logf("  未实现盈亏: %s", accountInfo.UnrealizedPnl.String())

	// 2. 查询最近转账
	t.Log("\n=== 步骤 2: 查询最近转账 ===")
	now := time.Now()
	startTime := now.AddDate(0, 0, -30)

	transfers, err := accountSvc.GetTransferHistory(context.Background(), exchange.GetTransferHistoryReq{
		StartTime: startTime,
		EndTime:   now,
	})
	if err != nil {
		t.Fatalf("获取转账历史失败: %v", err)
	}

	t.Logf("最近30天转账: %d 笔", len(transfers))

	// 3. 分析资金流动
	if len(transfers) > 0 {
		netFlow := decimal.Zero
		for _, transfer := range transfers {
			if transfer.Direction == exchange.DirectionIn {
				netFlow = netFlow.Add(transfer.Amount)
			} else {
				netFlow = netFlow.Sub(transfer.Amount)
			}
		}
		t.Logf("净流入: %s", netFlow.String())
	}
}

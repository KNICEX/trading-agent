package backtest

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// ============ AccountService 实现 ============

// GetAccountInfo 获取账户信息
func (svc *ExchangeService) GetAccountInfo(ctx context.Context) (exchange.AccountInfo, error) {
	svc.accountMu.RLock()
	accountCopy := *svc.account
	svc.accountMu.RUnlock()

	// 🔑 计算总的未实现盈亏（遍历所有持仓）
	svc.positionMu.RLock()
	totalUnrealizedPnl := decimal.Zero
	for _, position := range svc.positions {
		totalUnrealizedPnl = totalUnrealizedPnl.Add(position.UnrealizedPnl)
	}
	svc.positionMu.RUnlock()

	// 更新账户的未实现盈亏
	accountCopy.UnrealizedPnl = totalUnrealizedPnl

	return accountCopy, nil
}

// GetTransferHistory 获取转账历史（回测模式：不支持）
func (svc *ExchangeService) GetTransferHistory(ctx context.Context, req exchange.GetTransferHistoryReq) ([]exchange.TransferHistory, error) {
	// 回测模式没有转账历史
	return []exchange.TransferHistory{}, nil
}

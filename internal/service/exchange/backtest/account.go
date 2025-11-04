package backtest

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

// ============ AccountService 实现 ============

// GetAccountInfo 获取账户信息
func (svc *BinanceExchangeService) GetAccountInfo(ctx context.Context) (exchange.AccountInfo, error) {
	svc.accountMu.RLock()
	defer svc.accountMu.RUnlock()

	return *svc.account, nil
}

// GetTransferHistory 获取转账历史（回测模式：不支持）
func (svc *BinanceExchangeService) GetTransferHistory(ctx context.Context, req exchange.GetTransferHistoryReq) ([]exchange.TransferHistory, error) {
	// 回测模式没有转账历史
	return []exchange.TransferHistory{}, nil
}

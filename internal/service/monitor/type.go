package monitor

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
)

// AbnormalService 监控服务接口
type AbnormalService interface {
	Scan(ctx context.Context, symbols []exchange.Symbol) error
}

type Notifier interface {
	Notify(ctx context.Context, signal strategy.AbnormalSignal) error
}

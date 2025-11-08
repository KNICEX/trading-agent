package backtest

import (
	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

var _ exchange.QuantityPrecisionProvider = (*PercisionProvider)(nil)

type PercisionProvider struct {
}

func (p *PercisionProvider) GetQuantityPrecision(pair exchange.TradingPair) int32 {
	return 3
}

package backtest

import "github.com/KNICEX/trading-agent/internal/service/exchange"

type ExchangeService interface {
	exchange.MarketService
	exchange.PositionService
	exchange.OrderService
	exchange.AccountService
	exchange.TradingService
}

package binance

import (
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
)

var _ exchange.Service = (*Service)(nil)

type Service struct {
	marketSvc   exchange.MarketService
	orderSvc    exchange.OrderService
	accountSvc  exchange.AccountService
	positionSvc exchange.PositionService
	tradingSvc  exchange.TradingService
}

func NewService(cli *futures.Client) *Service {
	orderSvc := NewOrderService(cli)
	accountSvc := NewAccountService(cli)
	positionSvc := NewPositionService(cli)
	marketSvc := NewMarketService(cli)
	return &Service{
		marketSvc:   marketSvc,
		positionSvc: positionSvc,
		orderSvc:    orderSvc,
		accountSvc:  accountSvc,
		tradingSvc:  NewTradingService(cli, orderSvc, accountSvc, positionSvc, marketSvc),
	}
}

func (s *Service) MarketService() exchange.MarketService {
	return s.marketSvc
}

func (s *Service) PositionService() exchange.PositionService {
	return s.positionSvc
}

func (s *Service) OrderService() exchange.OrderService {
	return s.orderSvc
}

func (s *Service) AccountService() exchange.AccountService {
	return s.accountSvc
}

func (s *Service) TradingService() exchange.TradingService {
	return s.tradingSvc
}

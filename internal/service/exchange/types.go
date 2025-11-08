package exchange

type Service interface {
	MarketService() MarketService
	PositionService() PositionService
	AccountService() AccountService
	OrderService() OrderService
}

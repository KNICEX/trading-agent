package binance

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

type TradingService struct {
	positonSvc PositonService
	orderSvc   OrderService
}

func (t *TradingService) OpenLong(ctx context.Context, req exchange.TradingReq) (exchange.OrderId, error) {
	orderId, err := t.orderSvc.CreateOrder(ctx, exchange.CreateOrderReq{
		Symbol:      req.Symbol,
		Side:        exchange.OrderSideBuy,
		PositonSide: exchange.PositionSideLong,
		OrderType:   req.Type,
		Price:       req.Price,
		Quantity:    req.Amount,
	})
	if err != nil {
		return "", err
	}
	return orderId, nil
}

func (t *TradingService) OpenShort(ctx context.Context, req exchange.TradingReq) (exchange.OrderId, error) {
	orderId, err := t.orderSvc.CreateOrder(ctx, exchange.CreateOrderReq{
		Symbol:      req.Symbol,
		Side:        exchange.OrderSideSell,
		PositonSide: exchange.PositionSideShort,
		OrderType:   req.Type,
		Price:       req.Price,
		Quantity:    req.Amount,
	})
	if err != nil {
		return "", err
	}
	return orderId, nil
}

func (t *TradingService) CloseLong(ctx context.Context, req exchange.TradingReq) error {
	_, err := t.orderSvc.CreateOrder(ctx, exchange.CreateOrderReq{
		Symbol:      req.Symbol,
		Side:        exchange.OrderSideSell,
		PositonSide: exchange.PositionSideLong,
		Price:       req.Price,
		Quantity:    req.Amount,
	})
	if err != nil {
		return err
	}
	return nil
}

func (t *TradingService) CloseShort(ctx context.Context, req exchange.TradingReq) error {
	_, err := t.orderSvc.CreateOrder(ctx, exchange.CreateOrderReq{
		Symbol:      req.Symbol,
		Side:        exchange.OrderSideBuy,
		PositonSide: exchange.PositionSideShort,
		Price:       req.Price,
		Quantity:    req.Amount,
	})
	if err != nil {
		return err
	}
	return nil
}

package binance

import (
	"context"
	"testing"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

func newOrderService(t *testing.T) *OrderService {
	return &OrderService{
		cli: initClient(t),
	}
}

func TestCreateOrder(t *testing.T) {
	order := exchange.CreateOrderReq{
		Symbol:    exchange.Symbol{Base: "BTC", Quote: "USDT"},
		Side:      exchange.OrderSideLong,
		Amount:    decimal.NewFromFloat(10),
		Price:     decimal.NewFromFloat(100000),
		OrderType: exchange.OrderTypeLimit,
	}
	svc := newOrderService(t)
	orderId, err := svc.CreateOrder(context.Background(), order)
	if err != nil {
		t.Errorf("Error creating order: %v", err)
	}
	t.Logf("Order ID: %s", orderId)
}

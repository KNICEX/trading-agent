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
		Symbol:    exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Side:      exchange.OrderSideLong,
		Quantity:  decimal.NewFromFloat(0.001),
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

func TestCancelOrder(t *testing.T) {
	svc := newOrderService(t)
	err := svc.CancelOrder(context.Background(), exchange.CancelOrderReq{
		Symbol: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Id:     exchange.OrderId("800749073792"),
	})
	if err != nil {
		t.Errorf("Error canceling order: %v", err)
	}
	t.Logf("Order canceled")
}

func TestListOrders(t *testing.T) {
	svc := newOrderService(t)
	orders, err := svc.ListOrders(context.Background(), exchange.ListOrdersReq{
		Symbol: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
	})
	if err != nil {
		t.Errorf("Error listing orders: %v", err)
	}
	t.Logf("Orders: %+v", orders)
}

func TestModifyOrder(t *testing.T) {
	svc := newOrderService(t)
	err := svc.ModifyOrder(context.Background(), exchange.ModifyOrderReq{
		Symbol:   exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Id:       exchange.OrderId("800749073792"),
		Quantity: decimal.NewFromFloat(0.002),
		Side:     exchange.OrderSideShort,
	})
	if err != nil {
		t.Errorf("Error modifying order: %v", err)
	}
	t.Logf("Order modified")
}

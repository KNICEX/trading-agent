package binance

import (
	"context"
	"testing"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

func newOrderService(t *testing.T) *OrderService {
	return NewOrderService(initClient(t))
}

func TestCreateOrder(t *testing.T) {
	order := exchange.CreateOrderReq{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Side:        exchange.OrderSideBuy,
		Quantity:    decimal.NewFromFloat(0.001),
		Price:       decimal.NewFromFloat(100000),
		OrderType:   exchange.OrderTypeLimit,
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
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Id:          exchange.OrderId("800749073792"),
	})
	if err != nil {
		t.Errorf("Error canceling order: %v", err)
	}
	t.Logf("Order canceled")
}

func TestListOrders(t *testing.T) {
	svc := newOrderService(t)
	orders, err := svc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
	})
	if err != nil {
		t.Errorf("Error listing orders: %v", err)
	}
	t.Logf("Orders: %+v", orders)
}

func TestModifyOrder(t *testing.T) {
	svc := newOrderService(t)
	err := svc.ModifyOrder(context.Background(), exchange.ModifyOrderReq{
		Id:          exchange.OrderId("800749073792"),
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Quantity:    decimal.NewFromFloat(0.002),
		Side:        exchange.OrderSideSell,
	})
	if err != nil {
		t.Errorf("Error modifying order: %v", err)
	}
	t.Logf("Order modified")
}

func TestMarketClosePosiiton(t *testing.T) {
	svc := newOrderService(t)
	positionSvc := newPositionService(t)
	position, err := positionSvc.GetActivePosition(context.Background(), exchange.TradingPair{Base: "BTC", Quote: "USDT"})
	if err != nil {
		t.Fatalf("Error getting position: %v", err)
	}
	if len(position) == 0 {
		t.Skip("No position found, skipping close position test")
	}
	t.Logf("Position: %+v", position[0])

	// 确定平仓方向
	var orderSide exchange.OrderSide
	if position[0].PositionAmount.IsNegative() {
		// 空头持仓，需要买入平仓
		orderSide = exchange.OrderSideBuy
	} else {
		// 多头持仓，需要卖出平仓
		orderSide = exchange.OrderSideSell
	}

	orderId, err := svc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Side:        orderSide,
		PositonSide: position[0].PositionSide,
		Quantity:    position[0].PositionAmount.Abs(),
		OrderType:   exchange.OrderTypeMarket,
	})
	if err != nil {
		t.Fatalf("Error creating order: %v", err)
	}
	t.Logf("Order ID: %s", orderId)
}

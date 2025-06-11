package binance

import (
	"context"
	"strconv"
	"sync"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2"
	"github.com/samber/lo"
)

type OrderService struct {
	cli *binance.Client

	orderSlTpMap *orderSlTpMap
}

func (svc *OrderService) CreateOrder(ctx context.Context, orders []exchange.Order) ([]exchange.Order, error) {
	for i, order := range orders {
		side := binanceSide(order.Side)
		closeSide := reverseSide(side)
		orderResp, err := svc.cli.NewCreateOrderService().Symbol(order.Symbol.ToString()).
			Side(binanceSide(order.Side)).Price(order.Price).
			Type(binanceOrderType(order.Type)).Quantity(order.Amount).Do(ctx)
		if err != nil {
			return nil, err
		}

		// 止损订单
		slResp, err := svc.cli.NewCreateOrderService().Symbol(order.Symbol.ToString()).Side(closeSide).
			Type(binance.OrderTypeStopLossLimit).StopPrice(order.StopLossPrice).Do(ctx)
		if err != nil {
			return nil, err
		}

		// 止盈订单
		tpResp, err := svc.cli.NewCreateOrderService().Symbol(order.Symbol.ToString()).Side(closeSide).
			Type(binance.OrderTypeTakeProfitLimit).StopPrice(order.TakeProfitPrice).Do(ctx)
		if err != nil {
			return nil, err
		}

		orders[i].Id = strconv.FormatInt(orderResp.OrderID, 10)
		orders[i].Status = fromBinanceOrderStatus(orderResp.Status)
		svc.orderSlTpMap.Add(orderResp.OrderID, svc.createOrderRespToOrder(slResp), svc.createOrderRespToOrder(tpResp))
	}
	return orders, nil
}

func (svc *OrderService) createOrderRespToOrder(orderResp *binance.CreateOrderResponse) *binance.Order {
	return &binance.Order{
		OrderID:      orderResp.OrderID,
		Symbol:       orderResp.Symbol,
		Status:       orderResp.Status,
		Side:         orderResp.Side,
		Type:         orderResp.Type,
		Price:        orderResp.Price,
		OrigQuantity: orderResp.OrigQuantity,
	}
}

func (svc *OrderService) CancelOrder(ctx context.Context, orderIds []string) error {
	for _, orderId := range orderIds {
		orderIdInt, err := strconv.ParseInt(orderId, 10, 64)
		if err != nil {
			return err
		}
		_, err = svc.cli.NewCancelOrderService().OrderID(orderIdInt).Do(ctx)
		if err != nil {
			return err
		}

		// 删除止损止盈订单
		if slOrder, tpOrder, ok := svc.orderSlTpMap.Get(orderIdInt); ok {
			if slOrder != nil {
				_, err = svc.cli.NewCancelOrderService().OrderID(slOrder.OrderID).Do(ctx)
				if err != nil {
					return err
				}
			}
			if tpOrder != nil {
				_, err = svc.cli.NewCancelOrderService().OrderID(tpOrder.OrderID).Do(ctx)
				if err != nil {
					return err
				}
			}
			svc.orderSlTpMap.Remove(orderId, orderIdInt)
		}
	}
	return nil
}

func (svc *OrderService) GetOrder(ctx context.Context, orderId string) (exchange.Order, error) {
	orderIdInt, err := strconv.ParseInt(orderId, 10, 64)
	if err != nil {
		return exchange.Order{}, err
	}

	order, err := svc.cli.NewGetOrderService().OrderID(orderIdInt).Do(ctx)
	if err != nil {
		return exchange.Order{}, err
	}

	return svc.parseOrder(order), nil
}

func (svc *OrderService) GetOpenOrders(ctx context.Context, symbol exchange.Symbol) ([]exchange.Order, error) {
	orders, err := svc.cli.NewListOpenOrdersService().Symbol(symbol.ToString()).Do(ctx)
	if err != nil {
		return nil, err
	}
	return lo.Map(orders, func(item *binance.Order, index int) exchange.Order {
		return svc.parseOrder(item)
	}), nil
}

func (svc *OrderService) parseOrder(order *binance.Order) exchange.Order {
	return exchange.Order{
		Id:     strconv.FormatInt(order.OrderID, 10),
		Symbol: fromBinanceSymbol(order.Symbol),
		Status: fromBinanceOrderStatus(order.Status),
		Side:   fromBinanceSide(order.Side),
		Type:   fromBinanceOrderType(order.Type),
		Price:  order.Price,
		Amount: order.OrigQuantity,
		// TODO 获取止盈止损价格
	}
}

type orderSlTpMap struct {
	mu sync.Mutex
	m  map[int64][2]*binance.Order
}

func (om *orderSlTpMap) Add(orderId int64, slOrderId, tpOderId *binance.Order) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.m[orderId] = [2]*binance.Order{
		slOrderId, tpOderId,
	}
}

func (om *orderSlTpMap) Get(orderId int64) (sl, tp *binance.Order, ok bool) {
	om.mu.Lock()
	defer om.mu.Unlock()
	if orders, exists := om.m[orderId]; exists {
		return orders[0], orders[1], true
	}
	return nil, nil, false
}

func (om *orderSlTpMap) Remove(symbol string, orderId int64) {
	om.mu.Lock()
	defer om.mu.Unlock()
	delete(om.m, orderId)
}

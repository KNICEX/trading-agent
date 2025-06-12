package binance

import (
	"context"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"strconv"
)

type OrderFuturesService struct {
	cli          *futures.Client
	orderSlTpMap *orderSlTpMap
}

func (o *OrderFuturesService) CreateOrder(ctx context.Context, orders []exchange.Order) ([]exchange.Order, error) {
	var result []exchange.Order
	for i, order := range orders {
		mainOrderResp, err := o.cli.NewCreateOrderService().
			Symbol(order.Symbol.ToString()).
			Side(futures.SideType(binanceSide(order.Side))).
			PositionSide(futures.PositionSideType(order.PositionSide)).
			Type(futures.OrderType(binanceOrderType(order.Type))).
			Quantity(order.Amount).
			Price(order.Price).
			TimeInForce(futures.TimeInForceTypeGTC).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("main order failed: %w", err)
		}

		var slResp *futures.CreateOrderResponse
		if order.StopLossPrice != "" {
			slResp, err = o.cli.NewCreateOrderService().
				Symbol(order.Symbol.ToString()).
				Side(futures.SideType(reverseSide(binance.SideType(order.Side)))).
				PositionSide(futures.PositionSideType(order.PositionSide)).
				Type(futures.OrderTypeStopMarket).
				StopPrice(order.StopLossPrice).
				ReduceOnly(true).
				Quantity(order.Amount).
				Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("stop-loss order failed: %w", err)
			}
		}

		var tpResp *futures.CreateOrderResponse
		if order.TakeProfitPrice != "" {
			tpResp, err = o.cli.NewCreateOrderService().
				Symbol(order.Symbol.ToString()).
				Side(futures.SideType(reverseSide(binance.SideType(order.Side)))).
				PositionSide(futures.PositionSideType(order.PositionSide)).
				Type(futures.OrderTypeTakeProfitMarket).
				StopPrice(order.TakeProfitPrice).
				ReduceOnly(true).
				Quantity(order.Amount).
				Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("take-profit order failed: %w", err)
			}
		}

		orders[i].Id = strconv.FormatInt(mainOrderResp.OrderID, 10)
		orders[i].Status = fromBinanceOrderStatus(binance.OrderStatusType(mainOrderResp.Status))
		o.orderSlTpMap.Add(mainOrderResp.OrderID, o.createFutureOrderRespToOrder(slResp), o.createFutureOrderRespToOrder(tpResp))

		result = append(result, exchange.Order{
			Id:           fmt.Sprintf("%d", mainOrderResp.OrderID),
			Symbol:       order.Symbol,
			Side:         order.Side,
			PositionSide: order.PositionSide,
			Type:         order.Type,
			Price:        order.Price,
			Amount:       order.Amount,
			Status:       fromBinanceOrderStatus(binance.OrderStatusType(mainOrderResp.Status)),
		})
	}

	return result, nil
}

func (o *OrderFuturesService) createFutureOrderRespToOrder(orderResp *futures.CreateOrderResponse) *binance.Order {
	return &binance.Order{
		OrderID:      orderResp.OrderID,
		Symbol:       orderResp.Symbol,
		Status:       binance.OrderStatusType(orderResp.Status),
		Side:         binance.SideType(orderResp.Side),
		Type:         binance.OrderType(orderResp.Type),
		Price:        orderResp.Price,
		OrigQuantity: orderResp.OrigQuantity,
	}
}

func (o *OrderFuturesService) CancelOrder(ctx context.Context, orderIds []string) error {
	for _, id := range orderIds {
		_, err := o.cli.NewCancelOrderService().
			OrderID(mustParseInt64(id)).
			Do(ctx)
		if err != nil {
			return fmt.Errorf("cancel order %s failed: %w", id, err)
		}
		// 删除止损止盈订单
		if slOrder, tpOrder, ok := o.orderSlTpMap.Get(mustParseInt64(id)); ok {
			if slOrder != nil {
				_, err = o.cli.NewCancelOrderService().OrderID(slOrder.OrderID).Do(ctx)
				if err != nil {
					return err
				}
			}
			if tpOrder != nil {
				_, err = o.cli.NewCancelOrderService().OrderID(tpOrder.OrderID).Do(ctx)
				if err != nil {
					return err
				}
			}
			o.orderSlTpMap.Remove(id, mustParseInt64(id))
		}
	}
	return nil
}

func (o *OrderFuturesService) GetOrder(ctx context.Context, orderId string) (exchange.Order, error) {
	resp, err := o.cli.NewGetOrderService().
		OrderID(mustParseInt64(orderId)).
		Do(ctx)
	if err != nil {
		return exchange.Order{}, fmt.Errorf("get order %s failed: %w", orderId, err)
	}

	return exchange.Order{
		Id:     fmt.Sprintf("%d", resp.OrderID),
		Symbol: fromBinanceSymbol(resp.Symbol),
		Side:   fromBinanceSide(binance.SideType(resp.Side)),
		Type:   fromBinanceOrderType(binance.OrderType(resp.Type)),
		Price:  resp.Price,
		Amount: resp.OrigQuantity,
		Status: fromBinanceOrderStatus(binance.OrderStatusType(resp.Status)),
	}, nil
}

func mustParseInt64(id string) int64 {
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

func (o *OrderFuturesService) GetOpenOrders(ctx context.Context, symbol exchange.Symbol) ([]exchange.Order, error) {
	resp, err := o.cli.NewListOpenOrdersService().
		Symbol(symbol.ToString()).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get open orders failed: %w", err)
	}

	var orders []exchange.Order
	for _, ord := range resp {
		orders = append(orders, exchange.Order{
			Id:     fmt.Sprintf("%d", ord.OrderID),
			Symbol: fromBinanceSymbol(ord.Symbol),
			Side:   fromBinanceSide(binance.SideType(ord.Side)),
			Type:   fromBinanceOrderType(binance.OrderType(ord.Type)),
			Price:  ord.Price,
			Amount: ord.OrigQuantity,
			Status: fromBinanceOrderStatus(binance.OrderStatusType(ord.Status)),
		})
	}
	return orders, nil
}

func NewOrderFuturesService(cli *futures.Client) *OrderFuturesService {
	return &OrderFuturesService{
		cli: cli,
	}
}

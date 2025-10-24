package binance

import (
	"context"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
	"strconv"
	"time"
)

type OrderService struct {
	cli                   *futures.Client
	changeLeverageService *futures.ChangeLeverageService
}

func (o *OrderService) CreateOrder(ctx context.Context, req exchange.CreateOrderReq) (exchange.OrderId, error) {
	order, err := o.cli.NewCreateOrderService().
		Symbol(req.Symbol.ToString()).
		Side(futures.SideType(req.Side)).                 // BUY / SELL
		Type(futures.OrderType(req.OrderType)).           // LIMIT / MARKET
		Quantity(req.Amount.String()).                    // 下单数量
		Price(req.Price.String()).                        // 限价单才需要
		PositionSide(futures.PositionSideType(req.Side)). // LONG / SHORT
		TimeInForce(futures.TimeInForceTypeGTC).          // 挂单时效
		Do(ctx)
	if err != nil {
		return "", fmt.Errorf("create order failed: %w", err)
	}

	return exchange.OrderId(strconv.FormatInt(order.OrderID, 10)), nil
}

func (o *OrderService) CreateBatchOrders(ctx context.Context, req []exchange.CreateOrderReq) ([]exchange.OrderId, error) {
	var orderList []*futures.CreateOrderService
	for _, orderReq := range req {
		orderList = append(orderList, o.cli.NewCreateOrderService().
			Symbol(orderReq.Symbol.ToString()).
			Side(futures.SideType(orderReq.Side)).                 // BUY / SELL
			Type(futures.OrderType(orderReq.OrderType)).           // LIMIT / MARKET
			Quantity(orderReq.Amount.String()).                    // 下单数量
			Price(orderReq.Price.String()).                        // 限价单才需要
			PositionSide(futures.PositionSideType(orderReq.Side)). // LONG / SHORT
			TimeInForce(futures.TimeInForceTypeGTC))
	}

	orders, err := o.cli.NewCreateBatchOrdersService().
		OrderList(orderList). // 挂单时效
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("create order failed: %w", err)
	}
	var orderIds []exchange.OrderId
	for _, order := range orders.Orders {
		orderIds = append(orderIds, exchange.OrderId(strconv.FormatInt(order.OrderID, 10)))
	}
	return orderIds, nil
}

func (o *OrderService) ModifyOrder(ctx context.Context, req exchange.ModifyOrderReq) error {
	service := o.cli.NewModifyOrderService().
		Symbol(req.Symbol.ToString()).
		Side(futures.SideType(req.Side)).
		Quantity(req.Amount.String())

	if !req.Price.IsZero() {
		service = service.Price(req.Price.String())
	}

	if !req.Id.IsZero() {
		service = service.OrderID(req.Id.ToInt64())
	}

	_, err := service.Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to modify order: %w", err)
	}

	return nil
}

func (o *OrderService) ModifyBatchOrders(ctx context.Context, req []exchange.ModifyOrderReq) error {
	batch := o.cli.NewModifyBatchOrdersService()
	var orders []*futures.ModifyOrder

	for _, orderReq := range req {
		var modifyOrder *futures.ModifyOrder
		modifyOrder.
			Symbol(orderReq.Symbol.ToString()).
			Side(futures.SideType(orderReq.Side)).
			Quantity(orderReq.Amount.String())

		if !orderReq.Price.IsZero() {
			modifyOrder.Price(orderReq.Price.String())
		}

		if !orderReq.Id.IsZero() {
			modifyOrder.OrderID(orderReq.Id.ToInt64())
		}
		orders = append(orders, modifyOrder)
	}

	_, err := batch.OrderList(orders).Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to modify batch orders: %w", err)
	}
	return nil
}
func (o *OrderService) GetOrder(ctx context.Context, req exchange.GetOrderReq) (*exchange.OrderInfo, error) {
	order, err := o.cli.NewGetOrderService().
		Symbol(req.Symbol.ToString()).
		OrderID(req.Id.ToInt64()).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get order failed: %w", err)
	}

	price, _ := decimal.NewFromString(order.Price)
	amount, _ := decimal.NewFromString(order.OrigQuantity)

	return &exchange.OrderInfo{
		Id:        strconv.FormatInt(order.OrderID, 10),
		Symbol:    req.Symbol,
		Side:      exchange.OrderSide(order.Side),
		Price:     price,
		Amount:    amount,
		Status:    exchange.OrderStatus(order.Status),
		CreatedAt: time.UnixMilli(order.Time),
		UpdatedAt: time.UnixMilli(order.UpdateTime),
	}, nil
}

func (o *OrderService) GetOpenOrder(ctx context.Context, req exchange.GetOpenOrderReq) (*exchange.OrderInfo, error) {
	order, err := o.cli.NewGetOpenOrderService().
		Symbol(req.Symbol.ToString()).
		OrderID(req.Id.ToInt64()).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get open order failed: %w", err)
	}

	price, _ := decimal.NewFromString(order.Price)
	amount, _ := decimal.NewFromString(order.OrigQuantity)

	return &exchange.OrderInfo{
		Id:        strconv.FormatInt(order.OrderID, 10),
		Symbol:    req.Symbol,
		Side:      exchange.OrderSide(order.Side),
		Price:     price,
		Amount:    amount,
		Status:    exchange.OrderStatus(order.Status),
		CreatedAt: time.UnixMilli(order.Time),
		UpdatedAt: time.UnixMilli(order.UpdateTime),
	}, nil
}

func (o *OrderService) ListOrders(ctx context.Context, req exchange.ListOrdersReq) ([]exchange.OrderInfo, error) {
	svc := o.cli.NewListOrdersService().
		Symbol(req.Symbol.ToString())

	if req.Limit > 0 {
		svc = svc.Limit(req.Limit)
	}
	if !req.StartTime.IsZero() {
		svc = svc.StartTime(req.StartTime.UnixMilli())
	}
	if !req.EndTime.IsZero() {
		svc = svc.EndTime(req.EndTime.UnixMilli())
	}

	orders, err := svc.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list orders failed: %w", err)
	}

	results := make([]exchange.OrderInfo, 0, len(orders))
	for _, oinfo := range orders {
		price, _ := decimal.NewFromString(oinfo.Price)
		amount, _ := decimal.NewFromString(oinfo.OrigQuantity)
		results = append(results, exchange.OrderInfo{
			Id:        strconv.FormatInt(oinfo.OrderID, 10),
			Symbol:    req.Symbol,
			Side:      exchange.OrderSide(oinfo.Side),
			Price:     price,
			Amount:    amount,
			Status:    exchange.OrderStatus(oinfo.Status),
			CreatedAt: time.UnixMilli(oinfo.Time),
			UpdatedAt: time.UnixMilli(oinfo.UpdateTime),
		})
	}
	return results, nil
}

func (o *OrderService) ListOpenOrders(ctx context.Context, req exchange.ListOpenOrdersReq) ([]exchange.OrderInfo, error) {
	svc := o.cli.NewListOpenOrdersService()
	// symbol 可选 不传时返回所有
	if req.Symbol.ToString() != "" {
		svc = svc.Symbol(req.Symbol.ToString())
	}

	orders, err := svc.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list open orders failed: %w", err)
	}

	results := make([]exchange.OrderInfo, 0, len(orders))
	for _, oinfo := range orders {
		price, _ := decimal.NewFromString(oinfo.Price)
		amount, _ := decimal.NewFromString(oinfo.OrigQuantity)
		base, quote := exchange.SplitSymbol(oinfo.Symbol)
		results = append(results, exchange.OrderInfo{
			Id:        strconv.FormatInt(oinfo.OrderID, 10),
			Symbol:    exchange.Symbol{Base: base, Quote: quote},
			Side:      exchange.OrderSide(oinfo.Side),
			Price:     price,
			Amount:    amount,
			Status:    exchange.OrderStatus(oinfo.Status),
			CreatedAt: time.UnixMilli(oinfo.Time),
			UpdatedAt: time.UnixMilli(oinfo.UpdateTime),
		})
	}
	return results, nil
}

func (o *OrderService) CancelOrder(ctx context.Context, req exchange.CancelOrderReq) error {
	_, err := o.cli.NewCancelOrderService().
		Symbol(req.Symbol.ToString()).
		OrderID(req.Id.ToInt64()).
		Do(ctx)
	return err
}

func (o *OrderService) CancelAllOpenOrders(ctx context.Context, req exchange.CancelAllOpenOrdersReq) error {
	err := o.cli.NewCancelAllOpenOrdersService().
		Symbol(req.Symbol.ToString()).
		Do(ctx)
	return err
}

func (o *OrderService) CancelMultipleOrders(ctx context.Context, req exchange.CancelMultipleOrdersReq) error {
	var orderIds []int64
	for _, id := range req.Ids {
		orderIds = append(orderIds, id.ToInt64())
	}
	_, err := o.cli.NewCancelMultipleOrdersService().
		Symbol(req.Symbol.ToString()).
		OrderIDList(orderIds).
		Do(ctx)
	return err
}

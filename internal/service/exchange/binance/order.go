package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

var _ exchange.OrderService = (*OrderService)(nil)

type OrderService struct {
	cli *futures.Client
}

// NewOrderService 创建订单服务
func NewOrderService(cli *futures.Client) *OrderService {
	return &OrderService{cli: cli}
}

func (o *OrderService) CreateOrder(ctx context.Context, req exchange.CreateOrderReq) (exchange.OrderId, error) {
	service := o.cli.NewCreateOrderService().
		Symbol(req.TradingPair.ToString()).
		Side(futures.SideType(req.Side)).                       // BUY / SELL
		Type(futures.OrderType(req.OrderType)).                 // LIMIT / MARKET
		Quantity(req.Quantity.String()).                        // 下单数量
		PositionSide(futures.PositionSideType(req.PositonSide)) // LONG / SHORT
	if req.OrderType != exchange.OrderTypeMarket {
		service = service.Price(req.Price.String())
		service = service.TimeInForce(futures.TimeInForceTypeGTC)
	}
	order, err := service.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("create order failed: %w", err)
	}

	return exchange.OrderId(strconv.FormatInt(order.OrderID, 10)), nil
}

func (o *OrderService) CreateOrders(ctx context.Context, req []exchange.CreateOrderReq) ([]exchange.OrderId, error) {
	var orderList []*futures.CreateOrderService
	for _, orderReq := range req {
		orderList = append(orderList, o.cli.NewCreateOrderService().
			Symbol(orderReq.TradingPair.ToString()).
			Side(futures.SideType(orderReq.Side)).                        // BUY / SELL
			Type(futures.OrderType(orderReq.OrderType)).                  // LIMIT / MARKET
			Quantity(orderReq.Quantity.String()).                         // 下单数量
			Price(orderReq.Price.String()).                               // 限价单才需要
			PositionSide(futures.PositionSideType(orderReq.PositonSide)). // LONG / SHORT
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
		Symbol(req.TradingPair.ToString()).
		Side(futures.SideType(req.Side)).
		Quantity(req.Quantity.String())

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

func (o *OrderService) ModifyOrders(ctx context.Context, req []exchange.ModifyOrderReq) error {
	var orderList []*futures.ModifyOrder
	for _, orderReq := range req {
		orderList = append(orderList, (&futures.ModifyOrder{}).
			Symbol(orderReq.TradingPair.ToString()).
			Side(futures.SideType(orderReq.Side)).
			Quantity(orderReq.Quantity.String()).
			Price(orderReq.Price.String()).
			OrderID(orderReq.Id.ToInt64()))
	}

	_, err := o.cli.NewModifyBatchOrdersService().
		OrderList(orderList).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to modify orders: %w", err)
	}
	return nil
}

func (o *OrderService) GetOrder(ctx context.Context, req exchange.GetOrderReq) (*exchange.OrderInfo, error) {
	order, err := o.cli.NewGetOrderService().
		Symbol(req.TradingPair.ToString()).
		OrderID(req.Id.ToInt64()).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("get order failed: %w", err)
	}

	price, _ := decimal.NewFromString(order.Price)
	amount, _ := decimal.NewFromString(order.OrigQuantity)
	executedQty, _ := decimal.NewFromString(order.ExecutedQuantity)

	return &exchange.OrderInfo{
		Id:               strconv.FormatInt(order.OrderID, 10),
		TradingPair:      req.TradingPair,
		Side:             exchange.OrderSide(order.Side),
		Price:            price,
		Quantity:         amount,
		ExecutedQuantity: executedQty,
		Status:           exchange.OrderStatus(order.Status),
		CreatedAt:        time.UnixMilli(order.Time),
		UpdatedAt:        time.UnixMilli(order.UpdateTime),
	}, nil
}

func (o *OrderService) GetOrders(ctx context.Context, req exchange.GetOrdersReq) ([]exchange.OrderInfo, error) {
	svc := o.cli.NewListOrdersService()
	var binanceOrders []*futures.Order
	var err error
	// symbol 可选 不传时返回所有
	if req.TradingPair.IsZero() {
		binanceOrders, err = svc.Do(ctx)
	} else {
		binanceOrders, err = svc.Symbol(req.TradingPair.ToString()).Do(ctx)
	}

	if err != nil {
		return nil, err
	}

	results := make([]exchange.OrderInfo, 0, len(binanceOrders))
	for _, oinfo := range binanceOrders {
		price, _ := decimal.NewFromString(oinfo.Price)
		amount, _ := decimal.NewFromString(oinfo.OrigQuantity)
		executedQty, _ := decimal.NewFromString(oinfo.ExecutedQuantity)
		base, quote := exchange.SplitSymbol(oinfo.Symbol)
		results = append(results, exchange.OrderInfo{
			Id:               strconv.FormatInt(oinfo.OrderID, 10),
			TradingPair:      exchange.TradingPair{Base: base, Quote: quote},
			Side:             exchange.OrderSide(oinfo.Side),
			Price:            price,
			Quantity:         amount,
			ExecutedQuantity: executedQty,
			Status:           o.orderStatus(oinfo.Status),
			CreatedAt:        time.UnixMilli(oinfo.Time),
			UpdatedAt:        time.UnixMilli(oinfo.UpdateTime),
		})
	}
	return results, nil
}

func (o *OrderService) CancelOrder(ctx context.Context, req exchange.CancelOrderReq) error {
	_, err := o.cli.NewCancelOrderService().
		Symbol(req.TradingPair.ToString()).
		OrderID(req.Id.ToInt64()).
		Do(ctx)
	return err
}

func (o *OrderService) CancelOrders(ctx context.Context, req exchange.CancelOrdersReq) error {
	if req.TradingPair.IsZero() {
		return o.cli.NewCancelAllOpenOrdersService().
			Do(ctx)

	}
	if len(req.Ids) == 0 {
		return o.cli.NewCancelAllOpenOrdersService().Symbol(req.TradingPair.ToString()).Do(ctx)
	}

	_, err := o.cli.NewCancelMultipleOrdersService().
		Symbol(req.TradingPair.ToString()).
		OrderIDList(lo.Map(req.Ids, func(id exchange.OrderId, _ int) int64 { return id.ToInt64() })).
		Do(ctx)
	return err
}

func (o *OrderService) orderStatus(status futures.OrderStatusType) exchange.OrderStatus {
	switch status {
	case futures.OrderStatusTypeNew:
		return exchange.OrderStatusPending
	case futures.OrderStatusTypeFilled:
		return exchange.OrderStatusFilled
	case futures.OrderStatusTypePartiallyFilled:
		return exchange.OrderStatusPartiallyFilled
	}
	return exchange.OrderStatus(status)
}

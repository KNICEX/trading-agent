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
	// 将接口层的 OrderType 转换为币安的 OrderType
	binanceType := o.binanceOrderType(req.OrderType, req.Price)

	// 根据 OrderType 和 PositionSide 自动计算 Side
	side := o.calculateOrderSide(req.OrderType, req.PositonSide)

	service := o.cli.NewCreateOrderService().
		Symbol(req.TradingPair.ToString()).
		Side(futures.SideType(side)).                           // 自动计算的 BUY / SELL
		Type(binanceType).                                      // 币安的订单类型
		Quantity(req.Quantity.String()).                        // 下单数量
		PositionSide(futures.PositionSideType(req.PositonSide)) // LONG / SHORT

	// 限价单需要设置价格和有效期
	if binanceType == futures.OrderTypeLimit {
		service = service.Price(req.Price.String())
		service = service.TimeInForce(futures.TimeInForceTypeGTC)
	}

	order, err := service.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("create order failed: %w", err)
	}

	return exchange.OrderId(strconv.FormatInt(order.OrderID, 10)), nil
}

// calculateOrderSide 根据 OrderType 和 PositionSide 自动计算 Side
func (o *OrderService) calculateOrderSide(orderType exchange.OrderType, positionSide exchange.PositionSide) exchange.OrderSide {
	switch orderType {
	case exchange.OrderTypeOpen:
		// 开仓：LONG 用买单，SHORT 用卖单
		if positionSide == exchange.PositionSideLong {
			return exchange.OrderSideBuy
		}
		return exchange.OrderSideSell

	case exchange.OrderTypeClose:
		// 平仓：LONG 用卖单，SHORT 用买单
		if positionSide == exchange.PositionSideLong {
			return exchange.OrderSideSell
		}
		return exchange.OrderSideBuy

	default:
		return exchange.OrderSideBuy
	}
}

// binanceOrderType 将接口层的 OrderType 转换为币安的 OrderType
// 根据 orderType（开仓/平仓）和 price（是否为0）判断具体的币安订单类型
func (o *OrderService) binanceOrderType(orderType exchange.OrderType, price decimal.Decimal) futures.OrderType {
	// 根据是否有价格判断市价还是限价
	isMarket := price.IsZero()

	switch orderType {
	case exchange.OrderTypeOpen:
		// 开仓
		if isMarket {
			return futures.OrderTypeMarket
		}
		return futures.OrderTypeLimit

	case exchange.OrderTypeClose:
		// 平仓
		if isMarket {
			return futures.OrderTypeMarket
		}
		return futures.OrderTypeLimit

	default:
		// 默认限价
		return futures.OrderTypeLimit
	}
}

func (o *OrderService) CreateOrders(ctx context.Context, req []exchange.CreateOrderReq) ([]exchange.OrderId, error) {
	var orderList []*futures.CreateOrderService
	for _, orderReq := range req {
		// 自动计算 Side 和 BinanceOrderType
		side := o.calculateOrderSide(orderReq.OrderType, orderReq.PositonSide)
		binanceType := o.binanceOrderType(orderReq.OrderType, orderReq.Price)

		service := o.cli.NewCreateOrderService().
			Symbol(orderReq.TradingPair.ToString()).
			Side(futures.SideType(side)).                                // 自动计算的 BUY / SELL
			Type(binanceType).                                           // 转换后的币安订单类型
			Quantity(orderReq.Quantity.String()).                        // 下单数量
			PositionSide(futures.PositionSideType(orderReq.PositonSide)) // LONG / SHORT

		// 限价单需要设置价格
		if binanceType == futures.OrderTypeLimit {
			service = service.Price(orderReq.Price.String())
			service = service.TimeInForce(futures.TimeInForceTypeGTC)
		}

		orderList = append(orderList, service)
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

func (o *OrderService) GetOrder(ctx context.Context, req exchange.GetOrderReq) (exchange.OrderInfo, error) {
	order, err := o.cli.NewGetOrderService().
		Symbol(req.TradingPair.ToString()).
		OrderID(req.Id.ToInt64()).
		Do(ctx)
	if err != nil {
		return exchange.OrderInfo{}, fmt.Errorf("get order failed: %w", err)
	}

	if order.Status != futures.OrderStatusTypeNew && order.Status != futures.OrderStatusTypePartiallyFilled {
		// 只需要返回未成交或部分成交的订单
		return exchange.OrderInfo{}, fmt.Errorf("order is not active: %s", order.Status)
	}

	price, _ := decimal.NewFromString(order.Price)
	stopPrice, _ := decimal.NewFromString(order.StopPrice)
	amount, _ := decimal.NewFromString(order.OrigQuantity)
	executedQty, _ := decimal.NewFromString(order.ExecutedQuantity)

	return exchange.OrderInfo{
		Id:               strconv.FormatInt(order.OrderID, 10),
		TradingPair:      req.TradingPair,
		Side:             exchange.OrderSide(order.Side),
		Price:            price,
		StopPrice:        stopPrice,
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
		if oinfo.Status != futures.OrderStatusTypeNew && oinfo.Status != futures.OrderStatusTypePartiallyFilled {
			// 只需要返回未成交或部分成交的订单
			continue
		}
		price, _ := decimal.NewFromString(oinfo.Price)
		stopPrice, _ := decimal.NewFromString(oinfo.StopPrice)
		amount, _ := decimal.NewFromString(oinfo.OrigQuantity)
		executedQty, _ := decimal.NewFromString(oinfo.ExecutedQuantity)
		base, quote := exchange.SplitSymbol(oinfo.Symbol)
		results = append(results, exchange.OrderInfo{
			Id:               strconv.FormatInt(oinfo.OrderID, 10),
			TradingPair:      exchange.TradingPair{Base: base, Quote: quote},
			Side:             exchange.OrderSide(oinfo.Side),
			Price:            price,
			StopPrice:        stopPrice,
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

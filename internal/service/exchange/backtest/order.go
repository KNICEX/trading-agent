package backtest

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// CreateOrder 创建订单（回测模式：创建挂单，等待K线触发成交）
func (svc *BinanceExchangeService) CreateOrder(ctx context.Context, req exchange.CreateOrderReq) (exchange.OrderId, error) {
	orderId := svc.generateOrderId()
	now := svc.now()

	// 计算订单方向
	side := calculateOrderSide(req.OrderType, req.PositonSide)

	// 创建订单记录（扩展版本）
	order := &OrderInfo{
		OrderInfo: exchange.OrderInfo{
			Id:               orderId.ToString(),
			TradingPair:      req.TradingPair,
			Side:             side,
			Price:            req.Price,
			Quantity:         req.Quantity,
			ExecutedQuantity: decimal.Zero,                // 初始未成交
			Status:           exchange.OrderStatusPending, // 挂单状态
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		OrderType:    req.OrderType,
		PositionSide: req.PositonSide,
	}

	// 保存订单
	svc.orderMu.Lock()
	svc.orders[orderId] = order
	// 添加到待成交订单列表
	svc.pendingOrders[orderId] = order
	svc.orderMu.Unlock()

	return orderId, nil
}

// CreateOrders 批量创建订单
func (svc *BinanceExchangeService) CreateOrders(ctx context.Context, reqs []exchange.CreateOrderReq) ([]exchange.OrderId, error) {
	ids := make([]exchange.OrderId, len(reqs))
	for i, req := range reqs {
		id, err := svc.CreateOrder(ctx, req)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

// ModifyOrder 修改订单（回测模式：不支持）
func (svc *BinanceExchangeService) ModifyOrder(ctx context.Context, req exchange.ModifyOrderReq) error {
	return fmt.Errorf("modify order not supported in backtest mode")
}

// ModifyOrders 批量修改订单（回测模式：不支持）
func (svc *BinanceExchangeService) ModifyOrders(ctx context.Context, req []exchange.ModifyOrderReq) error {
	return fmt.Errorf("modify orders not supported in backtest mode")
}

// GetOrder 获取订单信息
func (svc *BinanceExchangeService) GetOrder(ctx context.Context, req exchange.GetOrderReq) (exchange.OrderInfo, error) {
	svc.orderMu.RLock()
	defer svc.orderMu.RUnlock()

	order, exists := svc.orders[req.Id]
	if !exists {
		return exchange.OrderInfo{}, fmt.Errorf("order not found: %s", req.Id)
	}

	return order.OrderInfo, nil
}

// GetOrders 获取待成交订单列表
func (svc *BinanceExchangeService) GetOrders(ctx context.Context, req exchange.GetOrdersReq) ([]exchange.OrderInfo, error) {
	svc.orderMu.RLock()
	defer svc.orderMu.RUnlock()

	var orders []exchange.OrderInfo
	for _, order := range svc.pendingOrders {
		if req.TradingPair.IsZero() || order.OrderInfo.TradingPair == req.TradingPair {
			orders = append(orders, order.OrderInfo)
		}
	}

	return orders, nil
}

// CancelOrder 取消订单
func (svc *BinanceExchangeService) CancelOrder(ctx context.Context, req exchange.CancelOrderReq) error {
	svc.orderMu.Lock()
	defer svc.orderMu.Unlock()

	order, exists := svc.pendingOrders[req.Id]
	if !exists {
		return fmt.Errorf("order not found or already filled: %s", req.Id)
	}

	// 从待成交列表移除
	delete(svc.pendingOrders, req.Id)

	// 更新订单状态为已取消
	order.Status = exchange.OrderStatus("cancelled")
	order.UpdatedAt = svc.now()

	return nil
}

// CancelOrders 批量取消订单
func (svc *BinanceExchangeService) CancelOrders(ctx context.Context, req exchange.CancelOrdersReq) error {
	// 获取需要取消的订单ID列表
	orderIds := req.Ids
	if len(orderIds) == 0 {
		// 取消指定交易对的所有订单
		svc.orderMu.RLock()
		for id, order := range svc.pendingOrders {
			if req.TradingPair.IsZero() || order.OrderInfo.TradingPair == req.TradingPair {
				orderIds = append(orderIds, id)
			}
		}
		svc.orderMu.RUnlock()
	}

	// 批量取消
	for _, id := range orderIds {
		svc.CancelOrder(ctx, exchange.CancelOrderReq{
			Id:          id,
			TradingPair: req.TradingPair,
		})
	}

	return nil
}

// ============ 辅助方法 ============

// calculateOrderSide 根据订单类型和持仓方向计算订单方向
func calculateOrderSide(orderType exchange.OrderType, positionSide exchange.PositionSide) exchange.OrderSide {
	if orderType == exchange.OrderTypeOpen {
		// 开仓
		if positionSide == exchange.PositionSideLong {
			return exchange.OrderSideBuy
		}
		return exchange.OrderSideSell
	} else {
		// 平仓
		if positionSide == exchange.PositionSideLong {
			return exchange.OrderSideSell
		}
		return exchange.OrderSideBuy
	}
}

// openPosition 开仓或加仓
func (svc *BinanceExchangeService) openPosition(posKey string, order *OrderInfo, price decimal.Decimal) error {
	svc.positionMu.Lock()
	defer svc.positionMu.Unlock()

	// 计算所需保证金（假设杠杆为1）
	cost := price.Mul(order.Quantity)

	// 检查账户余额
	if svc.account.AvailableBalance.LessThan(cost) {
		return fmt.Errorf("insufficient balance: available=%s, required=%s",
			svc.account.AvailableBalance, cost)
	}

	position, exists := svc.positions[posKey]
	now := svc.now()

	if !exists {
		// 创建新仓位
		position = &exchange.Position{
			TradingPair:      order.OrderInfo.TradingPair,
			PositionSide:     order.PositionSide,
			EntryPrice:       price,
			BreakEvenPrice:   price,
			MarginType:       exchange.MarginTypeCross,
			Leverage:         1,
			LiquidationPrice: decimal.Zero,
			MarkPrice:        price,
			Quantity:         order.Quantity,
			MarginAmount:     cost,
			UnrealizedPnl:    decimal.Zero,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		svc.positions[posKey] = position
	} else {
		// 加仓：计算新的平均入场价
		totalCost := position.EntryPrice.Mul(position.Quantity).Add(price.Mul(order.Quantity))
		totalQuantity := position.Quantity.Add(order.Quantity)
		position.EntryPrice = totalCost.Div(totalQuantity)
		position.BreakEvenPrice = position.EntryPrice
		position.Quantity = totalQuantity
		position.MarginAmount = position.MarginAmount.Add(cost)
		position.UpdatedAt = now
	}

	// 更新账户余额
	svc.accountMu.Lock()
	svc.account.AvailableBalance = svc.account.AvailableBalance.Sub(cost)
	svc.account.UsedMargin = svc.account.UsedMargin.Add(cost)
	svc.accountMu.Unlock()

	return nil
}

// closePosition 平仓或减仓
func (svc *BinanceExchangeService) closePosition(posKey string, order *OrderInfo, price decimal.Decimal) error {
	svc.positionMu.Lock()
	defer svc.positionMu.Unlock()

	position, exists := svc.positions[posKey]
	if !exists {
		return fmt.Errorf("position not found: %s", posKey)
	}

	if position.Quantity.LessThan(order.Quantity) {
		return fmt.Errorf("insufficient position quantity: have=%s, want=%s",
			position.Quantity, order.Quantity)
	}

	// 计算盈亏
	var pnl decimal.Decimal
	if order.PositionSide == exchange.PositionSideLong {
		// 多头：(卖出价 - 买入价) * 数量
		pnl = price.Sub(position.EntryPrice).Mul(order.Quantity)
	} else {
		// 空头：(买入价 - 卖出价) * 数量
		pnl = position.EntryPrice.Sub(price).Mul(order.Quantity)
	}

	// 释放保证金
	releasedMargin := position.MarginAmount.Mul(order.Quantity).Div(position.Quantity)

	// 更新账户
	svc.accountMu.Lock()
	svc.account.AvailableBalance = svc.account.AvailableBalance.Add(releasedMargin).Add(pnl)
	svc.account.UsedMargin = svc.account.UsedMargin.Sub(releasedMargin)
	svc.account.TotalBalance = svc.account.TotalBalance.Add(pnl)
	svc.accountMu.Unlock()

	// 更新或关闭仓位
	position.Quantity = position.Quantity.Sub(order.Quantity)
	position.MarginAmount = position.MarginAmount.Sub(releasedMargin)
	position.UpdatedAt = svc.now()

	if position.Quantity.IsZero() {
		// 完全平仓，删除仓位
		delete(svc.positions, posKey)
	}

	return nil
}

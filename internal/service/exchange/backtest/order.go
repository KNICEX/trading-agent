package backtest

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// CreateOrder 创建订单（回测模式：创建挂单，等待K线触发成交）
func (svc *ExchangeService) CreateOrder(ctx context.Context, req exchange.CreateOrderReq) (exchange.OrderId, error) {
	orderId := svc.generateOrderId()
	now := svc.now(req.TradingPair)

	// 计算订单方向
	side := calculateOrderSide(req.OrderType, req.PositonSide)

	if req.OrderType == exchange.OrderTypeOpen {
		// 🔑 开仓订单：冻结资金（应用杠杆）
		// 获取订单价格（限价单用限价，市价单用当前价）
		price := req.Price
		if price.IsZero() {
			// 市价单，使用当前市价估算
			currentPrice, err := svc.Ticker(ctx, req.TradingPair)
			if err != nil {
				return "", fmt.Errorf("failed to get current price for market order: %w", err)
			}
			price = currentPrice
		}

		// 🔑 获取杠杆倍数
		leverage := svc.getLeverage(req.TradingPair)

		// 计算所需保证金（价格 × 数量 ÷ 杠杆）
		frozenAmount := price.Mul(req.Quantity).Div(decimal.NewFromInt(int64(leverage)))

		// 检查可用余额
		svc.accountMu.RLock()
		availableBalance := svc.account.AvailableBalance
		svc.accountMu.RUnlock()

		if availableBalance.LessThan(frozenAmount) {
			return "", fmt.Errorf("insufficient balance: available=%s, required=%s (leverage: %dx)",
				availableBalance, frozenAmount, leverage)
		}

		// 冻结资金
		svc.accountMu.Lock()
		svc.account.AvailableBalance = svc.account.AvailableBalance.Sub(frozenAmount)
		svc.frozenFunds[orderId] = frozenAmount
		svc.accountMu.Unlock()
	} else {
		// 🔑 平仓订单：冻结持仓数量
		posKey := svc.getPositionKey(req.TradingPair, req.PositonSide)

		svc.positionMu.RLock()
		position, exists := svc.positions[posKey]
		svc.positionMu.RUnlock()

		if !exists {
			return "", fmt.Errorf("position not found: %s", posKey)
		}

		// 计算可用持仓数量（总持仓 - 已冻结）
		svc.orderMu.RLock()
		totalFrozen := decimal.Zero
		for _, frozenQty := range svc.frozenPositions {
			totalFrozen = totalFrozen.Add(frozenQty)
		}
		svc.orderMu.RUnlock()

		availableQty := position.Quantity.Sub(totalFrozen)
		if availableQty.LessThan(req.Quantity) {
			return "", fmt.Errorf("insufficient position quantity: available=%s, required=%s",
				availableQty, req.Quantity)
		}

		// 冻结持仓数量
		svc.orderMu.Lock()
		svc.frozenPositions[orderId] = req.Quantity
		svc.orderMu.Unlock()
	}

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
	fmt.Printf("[DEBUG] CreateOrder: 订单 %s 已添加到pendingOrders, 总数=%d\n", orderId, len(svc.pendingOrders))
	svc.orderMu.Unlock()

	return orderId, nil
}

// CreateOrders 批量创建订单
func (svc *ExchangeService) CreateOrders(ctx context.Context, reqs []exchange.CreateOrderReq) ([]exchange.OrderId, error) {
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
func (svc *ExchangeService) ModifyOrder(ctx context.Context, req exchange.ModifyOrderReq) error {
	return fmt.Errorf("modify order not supported in backtest mode")
}

// ModifyOrders 批量修改订单（回测模式：不支持）
func (svc *ExchangeService) ModifyOrders(ctx context.Context, req []exchange.ModifyOrderReq) error {
	return fmt.Errorf("modify orders not supported in backtest mode")
}

// GetOrder 获取订单信息
func (svc *ExchangeService) GetOrder(ctx context.Context, req exchange.GetOrderReq) (exchange.OrderInfo, error) {
	svc.orderMu.RLock()
	defer svc.orderMu.RUnlock()

	order, exists := svc.orders[req.Id]
	if !exists {
		return exchange.OrderInfo{}, fmt.Errorf("order not found: %s", req.Id)
	}

	return order.OrderInfo, nil
}

// GetOrders 获取待成交订单列表
func (svc *ExchangeService) GetOrders(ctx context.Context, req exchange.GetOrdersReq) ([]exchange.OrderInfo, error) {
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
func (svc *ExchangeService) CancelOrder(ctx context.Context, req exchange.CancelOrderReq) error {
	svc.orderMu.Lock()
	order, exists := svc.pendingOrders[req.Id]
	if !exists {
		svc.orderMu.Unlock()
		return fmt.Errorf("order not found or already filled: %s", req.Id)
	}

	// 从待成交列表移除
	delete(svc.pendingOrders, req.Id)

	// 🔑 清理待设置的止盈止损订单（如果有）
	delete(svc.pendingStopOrders, req.Id)

	// 更新订单状态为已取消
	order.Status = exchange.OrderStatus("cancelled")
	order.UpdatedAt = svc.now(order.OrderInfo.TradingPair)

	// 🔑 释放冻结的资金或持仓
	if order.OrderType == exchange.OrderTypeOpen {
		// 开仓订单：释放冻结资金
		frozenAmount, wasFrozen := svc.frozenFunds[req.Id]
		if wasFrozen {
			delete(svc.frozenFunds, req.Id)
			svc.orderMu.Unlock()

			// 返还冻结资金到可用余额
			svc.accountMu.Lock()
			svc.account.AvailableBalance = svc.account.AvailableBalance.Add(frozenAmount)
			svc.accountMu.Unlock()
		} else {
			svc.orderMu.Unlock()
		}
	} else {
		// 平仓订单：释放冻结持仓
		frozenQty, wasFrozen := svc.frozenPositions[req.Id]
		if wasFrozen {
			delete(svc.frozenPositions, req.Id)
		}
		svc.orderMu.Unlock()
		// 持仓数量冻结不需要额外操作，只是从map中删除即可
		_ = frozenQty // 避免unused警告
	}

	return nil
}

// CancelOrders 批量取消订单
func (svc *ExchangeService) CancelOrders(ctx context.Context, req exchange.CancelOrdersReq) error {
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
func (svc *ExchangeService) openPosition(posKey string, order *OrderInfo, price decimal.Decimal) error {
	svc.positionMu.Lock()
	defer svc.positionMu.Unlock()

	// 🔑 获取杠杆倍数
	leverage := svc.getLeverage(order.OrderInfo.TradingPair)

	// 计算实际所需保证金（价格 × 数量 ÷ 杠杆）
	actualCost := price.Mul(order.Quantity).Div(decimal.NewFromInt(int64(leverage)))

	// 🔑 从冻结资金转为已用保证金
	orderId := exchange.OrderId(order.Id)
	svc.accountMu.Lock()
	frozenAmount, wasFrozen := svc.frozenFunds[orderId]
	if wasFrozen {
		// ✅ 挂单已冻结资金，现在转为保证金
		delete(svc.frozenFunds, orderId)

		// 计算冻结金额与实际成交金额的差额
		// 对于市价单，冻结时使用估算价格，成交时使用实际价格
		diff := frozenAmount.Sub(actualCost)

		if diff.IsPositive() {
			// 冻结金额 > 实际成交金额，返还多余部分到可用余额
			svc.account.AvailableBalance = svc.account.AvailableBalance.Add(diff)
			svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
		} else if diff.IsNegative() {
			// 冻结金额 < 实际成交金额，需要额外扣除可用余额
			shortage := diff.Abs()
			if svc.account.AvailableBalance.LessThan(shortage) {
				svc.accountMu.Unlock()
				return fmt.Errorf("insufficient balance for price difference: available=%s, need=%s",
					svc.account.AvailableBalance, shortage)
			}
			svc.account.AvailableBalance = svc.account.AvailableBalance.Sub(shortage)
			svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
		} else {
			// 正好相等
			svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
		}
	} else {
		// ⚠️ 没有冻结资金（止盈止损触发、或其他特殊情况）
		// 这种情况下无法开仓，因为没有预留资金
		svc.accountMu.Unlock()
		return fmt.Errorf("no frozen funds for order %s, cannot open position", orderId)
	}
	svc.accountMu.Unlock()

	position, exists := svc.positions[posKey]
	now := svc.now(order.OrderInfo.TradingPair)

	// 📝 持仓历史记录
	svc.historyMu.Lock()
	history, historyExists := svc.activeHistories[posKey]

	if !exists {
		// 创建新仓位
		position = &exchange.Position{
			TradingPair:      order.OrderInfo.TradingPair,
			PositionSide:     order.PositionSide,
			EntryPrice:       price,
			BreakEvenPrice:   price,
			MarginType:       exchange.MarginTypeCross,
			Leverage:         leverage, // 使用实际杠杆
			LiquidationPrice: decimal.Zero,
			MarkPrice:        price,
			Quantity:         order.Quantity,
			MarginAmount:     actualCost,
			UnrealizedPnl:    decimal.Zero,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		svc.positions[posKey] = position

		// 创建持仓历史记录
		if !historyExists {
			history = &exchange.PositionHistory{
				TradingPair:  order.OrderInfo.TradingPair,
				PositionSide: order.PositionSide,
				EntryPrice:   price,
				MaxQuantity:  order.Quantity,
				OpenedAt:     now,
				Events:       []exchange.PositionEvent{},
			}
			svc.activeHistories[posKey] = history
		}

		// 记录创建事件
		history.Events = append(history.Events, exchange.PositionEvent{
			OrderId:        exchange.OrderId(order.Id),
			EventType:      exchange.PositionEventTypeCreate,
			Quantity:       order.Quantity,
			BeforeQuantity: decimal.Zero,
			AfterQuantity:  order.Quantity,
			Price:          price,
			RealizedPnl:    decimal.Zero,
			Fee:            decimal.Zero,
			CreatedAt:      order.CreatedAt,
			UpdatedAt:      order.UpdatedAt,
			CompletedAt:    now,
		})
	} else {
		// 加仓：计算新的平均入场价
		oldQuantity := position.Quantity
		totalCost := position.EntryPrice.Mul(position.Quantity).Add(price.Mul(order.Quantity))
		totalQuantity := position.Quantity.Add(order.Quantity)
		position.EntryPrice = totalCost.Div(totalQuantity)
		position.BreakEvenPrice = position.EntryPrice
		position.Quantity = totalQuantity
		position.MarginAmount = position.MarginAmount.Add(actualCost)
		position.UpdatedAt = now

		// 更新最大持仓数量
		if history != nil && totalQuantity.GreaterThan(history.MaxQuantity) {
			history.MaxQuantity = totalQuantity
		}

		// 记录加仓事件
		if history != nil {
			history.Events = append(history.Events, exchange.PositionEvent{
				OrderId:        exchange.OrderId(order.Id),
				EventType:      exchange.PositionEventTypeIncrease,
				Quantity:       order.Quantity,
				BeforeQuantity: oldQuantity,
				AfterQuantity:  totalQuantity,
				Price:          price,
				RealizedPnl:    decimal.Zero,
				Fee:            decimal.Zero,
				CreatedAt:      order.CreatedAt,
				UpdatedAt:      order.UpdatedAt,
				CompletedAt:    now,
			})
		}
	}
	svc.historyMu.Unlock()

	// 🔑 检查是否有待设置的止盈止损订单
	svc.orderMu.Lock()
	pendingStop, hasPendingStop := svc.pendingStopOrders[orderId]
	if hasPendingStop {
		// 从待处理列表移除
		delete(svc.pendingStopOrders, orderId)
	}
	svc.orderMu.Unlock()

	// 如果有待设置的止盈止损订单，现在设置到持仓上
	if hasPendingStop {
		// 创建止盈订单（使用预分配的订单ID）
		if pendingStop.TakeProfit.IsValid() {
			stopOrder := &StopOrderInfo{
				Id:           pendingStop.TakeProfitId,
				TradingPair:  pendingStop.TradingPair,
				PositionSide: pendingStop.PositionSide,
				StopType:     StopOrderTypeTakeProfit,
				OrderSide:    pendingStop.PositionSide.GetCloseOrderSide(),
				TriggerPrice: pendingStop.TakeProfit.Price,
				Quantity:     position.Quantity, // 使用当前持仓数量
				PositionKey:  posKey,
			}

			svc.orderMu.Lock()
			svc.stopOrders[pendingStop.TakeProfitId] = stopOrder
			svc.orderMu.Unlock()

			fmt.Printf("[DEBUG] openPosition: 开仓成交后设置止盈订单 %s (触发价=%s)\n",
				pendingStop.TakeProfitId, pendingStop.TakeProfit.Price)
		}

		// 创建止损订单（使用预分配的订单ID）
		if pendingStop.StopLoss.IsValid() {
			stopOrder := &StopOrderInfo{
				Id:           pendingStop.StopLossId,
				TradingPair:  pendingStop.TradingPair,
				PositionSide: pendingStop.PositionSide,
				StopType:     StopOrderTypeStopLoss,
				OrderSide:    pendingStop.PositionSide.GetCloseOrderSide(),
				TriggerPrice: pendingStop.StopLoss.Price,
				Quantity:     position.Quantity, // 使用当前持仓数量
				PositionKey:  posKey,
			}

			svc.orderMu.Lock()
			svc.stopOrders[pendingStop.StopLossId] = stopOrder
			svc.orderMu.Unlock()

			fmt.Printf("[DEBUG] openPosition: 开仓成交后设置止损订单 %s (触发价=%s)\n",
				pendingStop.StopLossId, pendingStop.StopLoss.Price)
		}
	}

	// ✅ 资金流转完成：冻结资金 → 保证金，差额已调整可用余额
	return nil
}

// closePosition 平仓或减仓
func (svc *ExchangeService) closePosition(posKey string, order *OrderInfo, price decimal.Decimal) error {
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

	// 🔑 释放冻结的持仓数量（如果有）
	orderId := exchange.OrderId(order.Id)
	svc.orderMu.Lock()
	frozenQty, wasFrozen := svc.frozenPositions[orderId]
	if wasFrozen {
		delete(svc.frozenPositions, orderId)
	}
	svc.orderMu.Unlock()
	_ = frozenQty // 避免unused警告

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

	// ✅ 更新账户：保证金 + 盈亏 → 可用余额
	svc.accountMu.Lock()
	svc.account.AvailableBalance = svc.account.AvailableBalance.Add(releasedMargin).Add(pnl)
	svc.account.UsedMargin = svc.account.UsedMargin.Sub(releasedMargin)
	svc.account.TotalBalance = svc.account.TotalBalance.Add(pnl)
	svc.accountMu.Unlock()

	// 更新或关闭仓位
	oldQuantity := position.Quantity
	position.Quantity = position.Quantity.Sub(order.Quantity)
	position.MarginAmount = position.MarginAmount.Sub(releasedMargin)
	now := svc.now(order.OrderInfo.TradingPair)
	position.UpdatedAt = now

	// 📝 持仓历史记录
	svc.historyMu.Lock()
	history, historyExists := svc.activeHistories[posKey]

	if position.Quantity.IsZero() {
		// 完全平仓，删除仓位
		delete(svc.positions, posKey)

		// 完成持仓历史记录
		if historyExists && history != nil {
			history.ClosePrice = price
			history.ClosedAt = now

			// 记录平仓事件
			history.Events = append(history.Events, exchange.PositionEvent{
				OrderId:        exchange.OrderId(order.Id),
				EventType:      exchange.PositionEventTypeClose,
				Quantity:       order.Quantity,
				BeforeQuantity: oldQuantity,
				AfterQuantity:  decimal.Zero,
				Price:          price,
				RealizedPnl:    pnl,
				Fee:            decimal.Zero,
				CreatedAt:      order.CreatedAt,
				UpdatedAt:      order.UpdatedAt,
				CompletedAt:    now,
			})

			// 移动到历史记录列表
			svc.positionHistories = append(svc.positionHistories, *history)
			delete(svc.activeHistories, posKey)
		}
	} else {
		// 部分平仓，记录减仓事件
		if historyExists && history != nil {
			history.Events = append(history.Events, exchange.PositionEvent{
				OrderId:        exchange.OrderId(order.Id),
				EventType:      exchange.PositionEventTypeDecrease,
				Quantity:       order.Quantity,
				BeforeQuantity: oldQuantity,
				AfterQuantity:  position.Quantity,
				Price:          price,
				RealizedPnl:    pnl,
				Fee:            decimal.Zero,
				CreatedAt:      order.CreatedAt,
				UpdatedAt:      order.UpdatedAt,
				CompletedAt:    now,
			})
		}
	}
	svc.historyMu.Unlock()

	return nil
}

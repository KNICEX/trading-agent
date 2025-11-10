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
		// 🔑 平仓订单：检查持仓数量是否足够
		// posKey := svc.getPositionKey(req.TradingPair, req.PositonSide)

		// svc.positionMu.RLock()
		// position, exists := svc.positions[posKey]
		// svc.positionMu.RUnlock()

		// if !exists {
		// 	return "", fmt.Errorf("position not found: %s", posKey)
		// }

		// // 检查持仓数量是否足够
		// if position.Quantity.LessThan(req.Quantity) {
		// 	return "", fmt.Errorf("insufficient position quantity: have=%s, required=%s",
		// 		position.Quantity, req.Quantity)
		// }
	}

	// 创建订单记录（扩展版本）
	order := &exchange.OrderInfo{
		Id:               orderId.ToString(),
		TradingPair:      req.TradingPair,
		OrderType:        req.OrderType,
		PositionSide:     req.PositonSide,
		Price:            req.Price,
		Quantity:         req.Quantity,
		ExecutedQuantity: decimal.Zero,                // 初始未成交
		Status:           exchange.OrderStatusPending, // 挂单状态
		CreatedAt:        now,
		UpdatedAt:        now,
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

	return *order, nil
}

// GetOrders 获取待成交订单列表
func (svc *ExchangeService) GetOrders(ctx context.Context, req exchange.GetOrdersReq) ([]exchange.OrderInfo, error) {
	svc.orderMu.RLock()
	defer svc.orderMu.RUnlock()

	var orders []exchange.OrderInfo
	for _, order := range svc.pendingOrders {
		if req.TradingPair.IsZero() || order.TradingPair == req.TradingPair {
			orders = append(orders, *order)
		}
	}

	return orders, nil
}

// CancelOrder 取消订单
// - 如果 Id 为空，撤销该交易对的所有挂单
// - 如果有 Id，则匹配 TradingPair + Id 来撤销特定订单
func (svc *ExchangeService) CancelOrder(ctx context.Context, req exchange.CancelOrderReq) error {
	if req.Id.IsZero() {
		// Id 为空：撤销该交易对的所有挂单
		return svc.cancelOrdersByTradingPair(ctx, req.TradingPair)
	}

	// 有 Id：撤销特定订单
	svc.orderMu.Lock()
	order, exists := svc.pendingOrders[req.Id]
	if !exists {
		svc.orderMu.Unlock()
		return fmt.Errorf("order not found or already filled: %s", req.Id)
	}

	// 检查交易对是否匹配
	if !req.TradingPair.IsZero() && order.TradingPair != req.TradingPair {
		svc.orderMu.Unlock()
		return fmt.Errorf("order %s does not belong to trading pair %s", req.Id, req.TradingPair.ToString())
	}

	// 从待成交列表移除
	delete(svc.pendingOrders, req.Id)

	// 更新订单状态为已取消
	order.Status = exchange.OrderStatus("cancelled")
	order.UpdatedAt = svc.now(order.TradingPair)

	// 🔑 释放冻结的资金（仅开仓订单）
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
		// 平仓订单：无需释放冻结持仓
		svc.orderMu.Unlock()
	}

	return nil
}

// cancelOrdersByTradingPair 撤销指定交易对的所有挂单
func (svc *ExchangeService) cancelOrdersByTradingPair(ctx context.Context, tradingPair exchange.TradingPair) error {
	svc.orderMu.RLock()
	// 收集需要撤销的订单ID
	var orderIds []exchange.OrderId
	for id, order := range svc.pendingOrders {
		if tradingPair.IsZero() || order.TradingPair == tradingPair {
			orderIds = append(orderIds, id)
		}
	}
	svc.orderMu.RUnlock()

	if len(orderIds) == 0 {
		return nil // 没有需要撤销的订单
	}

	// 逐个撤销订单
	for _, id := range orderIds {
		err := svc.CancelOrder(ctx, exchange.CancelOrderReq{
			Id:          id,
			TradingPair: tradingPair,
		})
		if err != nil {
			// 如果订单已经成交或不存在，忽略错误继续处理其他订单
			continue
		}
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
			if req.TradingPair.IsZero() || order.TradingPair == req.TradingPair {
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

// openPosition 开仓或加仓
// 返回实际成交的数量（可能因资金不足而部分成交）
func (svc *ExchangeService) openPosition(posKey string, order *exchange.OrderInfo, price decimal.Decimal) (decimal.Decimal, error) {
	svc.positionMu.Lock()
	defer svc.positionMu.Unlock()

	// 🔑 获取杠杆倍数
	leverage := svc.getLeverage(order.TradingPair)

	// 计算实际所需保证金（价格 × 数量 ÷ 杠杆）
	actualCost := price.Mul(order.Quantity).Div(decimal.NewFromInt(int64(leverage)))

	// 🔑 从冻结资金转为已用保证金
	orderId := exchange.OrderId(order.Id)
	svc.accountMu.Lock()
	frozenAmount, wasFrozen := svc.frozenFunds[orderId]
	if !wasFrozen {
		// ⚠️ 没有冻结资金（止盈止损触发、或其他特殊情况）
		// 这种情况下无法开仓，因为没有预留资金
		svc.accountMu.Unlock()
		return decimal.Zero, fmt.Errorf("no frozen funds for order %s, cannot open position", orderId)
	}

	// ✅ 挂单已冻结资金，现在转为保证金
	delete(svc.frozenFunds, orderId)

	// 计算冻结金额与实际成交金额的差额
	// 对于市价单，冻结时使用估算价格，成交时使用实际价格
	diff := frozenAmount.Sub(actualCost)

	// 实际成交的数量（默认为订单数量）
	executedQuantity := order.Quantity
	actualMargin := actualCost

	if diff.IsPositive() {
		// 冻结金额 > 实际成交金额，返还多余部分到可用余额
		svc.account.AvailableBalance = svc.account.AvailableBalance.Add(diff)
		svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
	} else if diff.IsNegative() {
		// 冻结金额 < 实际成交金额，需要额外扣除可用余额
		shortage := diff.Abs()

		if svc.account.AvailableBalance.LessThan(shortage) {
			// 🔑 可用余额不足，计算能够成交的最大数量（部分成交）
			// 可用总资金 = 冻结资金 + 剩余可用余额
			totalAvailable := frozenAmount.Add(svc.account.AvailableBalance)

			// 能够开仓的最大数量 = 可用总资金 × 杠杆 ÷ 成交价格
			maxQuantity := totalAvailable.Mul(decimal.NewFromInt(int64(leverage))).Div(price)

			if maxQuantity.LessThan(order.Quantity) {
				// 部分成交：使用全部可用资金
				executedQuantity = maxQuantity
				actualMargin = totalAvailable
				svc.account.AvailableBalance = decimal.Zero
				svc.account.UsedMargin = svc.account.UsedMargin.Add(actualMargin)
			} else {
				// 理论上不应该到这里（计算误差导致）
				svc.account.AvailableBalance = svc.account.AvailableBalance.Sub(shortage)
				svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
			}
		} else {
			// 余额充足，完全成交
			svc.account.AvailableBalance = svc.account.AvailableBalance.Sub(shortage)
			svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
		}
	} else {
		// 正好相等
		svc.account.UsedMargin = svc.account.UsedMargin.Add(actualCost)
	}
	svc.accountMu.Unlock()

	position, exists := svc.positions[posKey]
	now := svc.now(order.TradingPair)

	// 📝 持仓历史记录
	svc.historyMu.Lock()
	history, historyExists := svc.activeHistories[posKey]

	if !exists {
		// 创建新仓位
		position = &exchange.Position{
			TradingPair:      order.TradingPair,
			PositionSide:     order.PositionSide,
			EntryPrice:       price,
			BreakEvenPrice:   price,
			MarginType:       exchange.MarginTypeCross,
			Leverage:         leverage, // 使用实际杠杆
			LiquidationPrice: decimal.Zero,
			MarkPrice:        price,
			Quantity:         executedQuantity, // 使用实际成交数量
			MarginAmount:     actualMargin,     // 使用实际保证金
			UnrealizedPnl:    decimal.Zero,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		svc.positions[posKey] = position

		// 创建持仓历史记录
		if !historyExists {
			history = &exchange.PositionHistory{
				TradingPair:  order.TradingPair,
				PositionSide: order.PositionSide,
				EntryPrice:   price,
				MaxQuantity:  executedQuantity,
				OpenedAt:     now,
				Events:       []exchange.PositionEvent{},
			}
			svc.activeHistories[posKey] = history
		}

		// 记录创建事件
		history.Events = append(history.Events, exchange.PositionEvent{
			OrderId:        exchange.OrderId(order.Id),
			EventType:      exchange.PositionEventTypeCreate,
			Quantity:       executedQuantity,
			BeforeQuantity: decimal.Zero,
			AfterQuantity:  executedQuantity,
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
		totalCost := position.EntryPrice.Mul(position.Quantity).Add(price.Mul(executedQuantity))
		totalQuantity := position.Quantity.Add(executedQuantity)
		position.EntryPrice = totalCost.Div(totalQuantity)
		position.BreakEvenPrice = position.EntryPrice
		position.Quantity = totalQuantity
		position.MarginAmount = position.MarginAmount.Add(actualMargin)
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
				Quantity:       executedQuantity,
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

	// ✅ 资金流转完成：冻结资金 → 保证金，差额已调整可用余额
	return executedQuantity, nil
}

// closePosition 平仓或减仓
func (svc *ExchangeService) closePosition(posKey string, order *exchange.OrderInfo, price decimal.Decimal) error {
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

	// ✅ 更新账户：保证金 + 盈亏 → 可用余额
	svc.accountMu.Lock()
	svc.account.AvailableBalance = svc.account.AvailableBalance.Add(releasedMargin).Add(pnl)
	svc.account.UsedMargin = svc.account.UsedMargin.Sub(releasedMargin)
	svc.account.TotalBalance = svc.account.TotalBalance.Add(pnl)
	svc.accountMu.Unlock()

	// 更新或关闭仓位
	oldQuantity := position.Quantity
	if order.Quantity.GreaterThan(position.Quantity) {
		order.Quantity = position.Quantity
	}
	position.Quantity = position.Quantity.Sub(order.Quantity)
	position.MarginAmount = position.MarginAmount.Sub(releasedMargin)
	now := svc.now(order.TradingPair)
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

			for _, event := range history.Events {
				history.RealizedPnl = history.RealizedPnl.Add(event.RealizedPnl)
			}

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

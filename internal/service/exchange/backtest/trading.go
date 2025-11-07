package backtest

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// ============ TradingService 实现 ============

// OpenPosition 开仓/加仓
func (svc *ExchangeService) OpenPosition(ctx context.Context, req exchange.OpenPositionReq) (*exchange.OpenPositionResp, error) {
	// 计算开仓数量
	quantity := req.Quantity
	if !req.BalancePercent.IsZero() {
		// 使用账户余额百分比计算数量
		price := req.Price
		if price.IsZero() {
			// 获取当前市价（从最近的K线收盘价更新而来）
			currentPrice, err := svc.Ticker(ctx, req.TradingPair)
			if err != nil {
				return nil, err
			}
			price = currentPrice
		}

		svc.accountMu.RLock()
		availableBalance := svc.account.AvailableBalance
		svc.accountMu.RUnlock()

		// 计算可用于开仓的金额
		balanceToUse := availableBalance.Mul(req.BalancePercent).Div(decimal.NewFromInt(100))
		quantity = balanceToUse.Div(price)
	}

	// 创建开仓订单
	orderReq := exchange.CreateOrderReq{
		TradingPair: req.TradingPair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: req.PositionSide,
		Price:       req.Price,
		Quantity:    quantity,
		Timestamp:   svc.now(req.TradingPair),
	}

	orderId, err := svc.CreateOrder(ctx, orderReq)
	if err != nil {
		return nil, err
	}

	// 获取成交价格（从最近的K线收盘价更新而来）
	price := req.Price
	if price.IsZero() {
		currentPrice, _ := svc.Ticker(ctx, req.TradingPair)
		price = currentPrice
	}

	resp := &exchange.OpenPositionResp{
		OrderId:        orderId,
		EstimatedCost:  price.Mul(quantity),
		EstimatedPrice: price,
	}

	// 🔑 保存止盈止损订单到待处理列表（等待开仓订单成交后再设置）
	if req.TakeProfit.IsValid() || req.StopLoss.IsValid() {
		pendingStop := &PendingStopOrders{
			TradingPair:  req.TradingPair,
			PositionSide: req.PositionSide,
			TakeProfit:   req.TakeProfit,
			StopLoss:     req.StopLoss,
		}

		// 预分配止盈止损订单ID（用于返回给调用方）
		if req.TakeProfit.IsValid() {
			pendingStop.TakeProfitId = svc.generateOrderId()
			resp.TakeProfitId = pendingStop.TakeProfitId
		}
		if req.StopLoss.IsValid() {
			pendingStop.StopLossId = svc.generateOrderId()
			resp.StopLossId = pendingStop.StopLossId
		}

		svc.orderMu.Lock()
		svc.pendingStopOrders[orderId] = pendingStop
		svc.orderMu.Unlock()
	}

	return resp, nil
}

// ClosePosition 平仓
func (svc *ExchangeService) ClosePosition(ctx context.Context, req exchange.ClosePositionReq) (exchange.OrderId, error) {
	// 获取当前持仓
	posKey := svc.getPositionKey(req.TradingPair, req.PositionSide)

	svc.positionMu.RLock()
	position, exists := svc.positions[posKey]
	svc.positionMu.RUnlock()

	if !exists {
		return "", fmt.Errorf("position not found: %s", posKey)
	}

	// 计算平仓数量
	quantity := req.Quantity
	if req.CloseAll {
		quantity = position.Quantity
	} else if !req.Percent.IsZero() {
		quantity = position.Quantity.Mul(req.Percent).Div(decimal.NewFromInt(100))
	}

	// 创建平仓订单
	orderReq := exchange.CreateOrderReq{
		TradingPair: req.TradingPair,
		OrderType:   exchange.OrderTypeClose,
		PositonSide: req.PositionSide,
		Price:       req.Price,
		Quantity:    quantity,
		Timestamp:   svc.now(req.TradingPair),
	}

	return svc.CreateOrder(ctx, orderReq)
}

// SetStopOrders 设置止盈止损订单
func (svc *ExchangeService) SetStopOrders(ctx context.Context, req exchange.SetStopOrdersReq) (*exchange.SetStopOrdersResp, error) {
	resp := &exchange.SetStopOrdersResp{}
	posKey := svc.getPositionKey(req.TradingPair, req.PositionSide)

	// 检查持仓是否存在
	svc.positionMu.RLock()
	position, exists := svc.positions[posKey]
	svc.positionMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("position not found: %s", posKey)
	}

	svc.orderMu.Lock()
	defer svc.orderMu.Unlock()

	// 🔑 先取消该持仓的旧止盈止损订单（防止重复）
	for id, stopOrder := range svc.stopOrders {
		if stopOrder.PositionKey == posKey {
			delete(svc.stopOrders, id)
		}
	}

	// 创建止盈订单
	if req.TakeProfit.IsValid() {
		takeProfitId := svc.generateOrderId()
		stopOrder := &StopOrderInfo{
			Id:           takeProfitId,
			TradingPair:  req.TradingPair,
			PositionSide: req.PositionSide,
			StopType:     StopOrderTypeTakeProfit,
			OrderSide:    req.PositionSide.GetCloseOrderSide(), // 多头用卖，空头用买
			TriggerPrice: req.TakeProfit.Price,
			Quantity:     position.Quantity, // 使用当前持仓数量（避免过度平仓）
			PositionKey:  posKey,
		}

		svc.stopOrders[takeProfitId] = stopOrder
		resp.TakeProfitId = takeProfitId
	}

	// 创建止损订单
	if req.StopLoss.IsValid() {
		stopLossId := svc.generateOrderId()
		stopOrder := &StopOrderInfo{
			Id:           stopLossId,
			TradingPair:  req.TradingPair,
			PositionSide: req.PositionSide,
			StopType:     StopOrderTypeStopLoss,
			OrderSide:    req.PositionSide.GetCloseOrderSide(),
			TriggerPrice: req.StopLoss.Price,
			Quantity:     position.Quantity, // 使用当前持仓数量（避免过度平仓）
			PositionKey:  posKey,
		}

		svc.stopOrders[stopLossId] = stopOrder
		resp.StopLossId = stopLossId
	}

	return resp, nil
}

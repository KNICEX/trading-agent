package backtest

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// ============ TradingService 实现 ============

// OpenPosition 开仓/加仓
func (svc *BinanceExchangeService) OpenPosition(ctx context.Context, req exchange.OpenPositionReq) (*exchange.OpenPositionResp, error) {
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

	// 处理止盈止损订单
	if req.TakeProfit.IsValid() || req.StopLoss.IsValid() {
		stopResp, err := svc.SetStopOrders(ctx, exchange.SetStopOrdersReq{
			TradingPair:  req.TradingPair,
			PositionSide: req.PositionSide,
			TakeProfit:   req.TakeProfit,
			StopLoss:     req.StopLoss,
		})
		if err == nil {
			resp.TakeProfitId = stopResp.TakeProfitId
			resp.StopLossId = stopResp.StopLossId
		}
	}

	return resp, nil
}

// ClosePosition 平仓
func (svc *BinanceExchangeService) ClosePosition(ctx context.Context, req exchange.ClosePositionReq) (exchange.OrderId, error) {
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
func (svc *BinanceExchangeService) SetStopOrders(ctx context.Context, req exchange.SetStopOrdersReq) (*exchange.SetStopOrdersResp, error) {
	resp := &exchange.SetStopOrdersResp{}
	posKey := svc.getPositionKey(req.TradingPair, req.PositionSide)

	// 检查持仓是否存在
	svc.positionMu.RLock()
	_, exists := svc.positions[posKey]
	svc.positionMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("position not found: %s", posKey)
	}

	// 创建止盈订单
	if req.TakeProfit.IsValid() {
		takeProfitId := svc.generateOrderId()
		stopOrder := &StopOrderInfo{
			Id:           takeProfitId,
			TradingPair:  req.TradingPair,
			PositionSide: req.PositionSide,
			Type:         req.PositionSide.GetCloseOrderSide(), // 多头用卖，空头用买
			TriggerPrice: req.TakeProfit.Price,
			Quantity:     decimal.Zero, // 0表示全平
			PositionKey:  posKey,
		}

		svc.orderMu.Lock()
		svc.stopOrders[takeProfitId] = stopOrder
		svc.orderMu.Unlock()

		resp.TakeProfitId = takeProfitId
	}

	// 创建止损订单
	if req.StopLoss.IsValid() {
		stopLossId := svc.generateOrderId()
		stopOrder := &StopOrderInfo{
			Id:           stopLossId,
			TradingPair:  req.TradingPair,
			PositionSide: req.PositionSide,
			Type:         req.PositionSide.GetCloseOrderSide(),
			TriggerPrice: req.StopLoss.Price,
			Quantity:     decimal.Zero, // 0表示全平
			PositionKey:  posKey,
		}

		svc.orderMu.Lock()
		svc.stopOrders[stopLossId] = stopOrder
		svc.orderMu.Unlock()

		resp.StopLossId = stopLossId
	}

	return resp, nil
}

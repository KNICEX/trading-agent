package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ============ TradingService 相关类型定义 ============

// OpenPositionReq 开仓/加仓请求
type OpenPositionReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide    // LONG / SHORT
	Price        decimal.Decimal // 限价：有值则为限价单，为空则为市价单
	Quantity     decimal.Decimal // 开仓数量（具体值）

	// 使用账户余额的百分比开仓（与 Quantity 互斥）
	// 例如：BalancePercent = 50 表示使用 50% 的可用余额开仓
	BalancePercent decimal.Decimal

	// 止盈止损（可选）
	TakeProfit StopOrder // 止盈单
	StopLoss   StopOrder // 止损单

	Timestamp time.Time
}

// ClosePositionReq 平仓请求
type ClosePositionReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide    // LONG / SHORT（平哪个方向的仓位）
	Price        decimal.Decimal // 限价：有值则为限价单，为空则为市价单

	// 平仓数量（具体值）
	Quantity decimal.Decimal

	// 平仓百分比（与 Quantity 互斥）
	// 例如：Percent = 50 表示平掉当前持仓的 50%
	Percent decimal.Decimal

	// 是否平掉该方向的全部仓位
	CloseAll bool

	Timestamp time.Time
}

// StopOrder 止盈止损订单
type StopOrder struct {
	Price decimal.Decimal // 触发价格
	// 内部会自动判断：有触发价格 = 市价止盈止损（TAKE_PROFIT_MARKET / STOP_MARKET）
}

func (s StopOrder) IsValid() bool {
	return !s.Price.IsZero()
}

// TradingService 交易服务接口
type TradingService interface {
	// OpenPosition 开仓/加仓
	// 返回开仓订单 ID，如果设置了止盈止损，还会返回对应的订单 ID
	OpenPosition(ctx context.Context, req OpenPositionReq) (*OpenPositionResp, error)

	// ClosePosition 平仓
	ClosePosition(ctx context.Context, req ClosePositionReq) (OrderId, error)

	// SetStopOrders 为已有仓位设置止盈止损
	SetStopOrders(ctx context.Context, req SetStopOrdersReq) (*SetStopOrdersResp, error)
}

// OpenPositionResp 开仓响应
type OpenPositionResp struct {
	OrderId        OrderId         // 开仓订单 ID
	TakeProfitId   OrderId         // 止盈单 ID（如果设置了）
	StopLossId     OrderId         // 止损单 ID（如果设置了）
	EstimatedCost  decimal.Decimal // 预估占用保证金
	EstimatedPrice decimal.Decimal // 预估成交价格（市价单为当前市价）
}

// SetStopOrdersReq 设置止盈止损请求
type SetStopOrdersReq struct {
	TradingPair  TradingPair
	PositionSide PositionSide // 针对哪个方向的仓位
	TakeProfit   StopOrder    // 止盈单（可选）
	StopLoss     StopOrder    // 止损单（可选）
	Timestamp    time.Time
}

// SetStopOrdersResp 设置止盈止损响应
type SetStopOrdersResp struct {
	TakeProfitId OrderId // 止盈单 ID
	StopLossId   OrderId // 止损单 ID
}

// QuantityPrecisionProvider 交易对精度提供器接口
// 不同交易所可以实现这个接口来提供交易对精度信息
type QuantityPrecisionProvider interface {
	// GetQuantityPrecision 获取交易对的数量精度
	GetQuantityPrecision(pair TradingPair) int32
}

// tradingService 通用交易服务
// 完全基于 exchange 包的接口实现，不依赖任何具体交易所或第三方库
type tradingService struct {
	orderSvc          OrderService
	accountSvc        AccountService
	positionSvc       PositionService
	marketSvc         MarketService
	precisionProvider QuantityPrecisionProvider
}

// NewTradingService 创建通用交易服务
func NewTradingService(
	svc Service,
	precisionProvider QuantityPrecisionProvider,
) *tradingService {
	return &tradingService{
		orderSvc:          svc.OrderService(),
		accountSvc:        svc.AccountService(),
		positionSvc:       svc.PositionService(),
		marketSvc:         svc.MarketService(),
		precisionProvider: precisionProvider,
	}
}

// OpenPosition 开仓/加仓
func (s *tradingService) OpenPosition(ctx context.Context, req OpenPositionReq) (*OpenPositionResp, error) {
	// 1. 计算开仓数量
	quantity, estimatedCost, estimatedPrice, err := s.calculateOpenQuantity(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calculate open quantity failed: %w", err)
	}

	// 2. 创建开仓订单（Side 会自动计算）
	orderId, err := s.orderSvc.CreateOrder(ctx, CreateOrderReq{
		TradingPair: req.TradingPair,
		OrderType:   OrderTypeOpen, // 开仓类型
		PositonSide: req.PositionSide,
		Price:       req.Price,
		Quantity:    quantity,
		Timestamp:   req.Timestamp,
	})
	if err != nil {
		return nil, fmt.Errorf("create open position order failed: %w", err)
	}

	resp := &OpenPositionResp{
		OrderId:        orderId,
		EstimatedCost:  estimatedCost,
		EstimatedPrice: estimatedPrice,
	}

	// 3. 如果设置了止盈止损，创建对应订单
	if req.TakeProfit.IsValid() {
		tpId, err := s.createTakeProfitOrder(ctx, req.TradingPair, req.PositionSide, req.TakeProfit.Price, quantity, req.Timestamp)
		if err != nil {
			// 止盈单失败不影响主订单，只记录错误
			return resp, fmt.Errorf("main order created successfully, but take profit order failed: %w", err)
		}
		resp.TakeProfitId = tpId
	}

	if req.StopLoss.IsValid() {
		slId, err := s.createStopLossOrder(ctx, req.TradingPair, req.PositionSide, req.StopLoss.Price, quantity, req.Timestamp)
		if err != nil {
			// 止损单失败不影响主订单，只记录错误
			return resp, fmt.Errorf("main order created successfully, but stop loss order failed: %w", err)
		}
		resp.StopLossId = slId
	}

	return resp, nil
}

// ClosePosition 平仓
func (s *tradingService) ClosePosition(ctx context.Context, req ClosePositionReq) (OrderId, error) {
	// 1. 计算平仓数量
	quantity, err := s.calculateCloseQuantity(ctx, req)
	if err != nil {
		return "", fmt.Errorf("calculate close quantity failed: %w", err)
	}

	// 2. 创建平仓订单（Side 会自动计算）
	orderId, err := s.orderSvc.CreateOrder(ctx, CreateOrderReq{
		TradingPair: req.TradingPair,
		OrderType:   OrderTypeClose, // 平仓类型
		PositonSide: req.PositionSide,
		Price:       req.Price,
		Quantity:    quantity,
		Timestamp:   req.Timestamp,
	})
	if err != nil {
		return "", fmt.Errorf("create close position order failed: %w", err)
	}

	return orderId, nil
}

// SetStopOrders 为已有仓位设置止盈止损
func (s *tradingService) SetStopOrders(ctx context.Context, req SetStopOrdersReq) (*SetStopOrdersResp, error) {
	// 1. 获取当前仓位数量
	positions, err := s.positionSvc.GetActivePositions(ctx, []TradingPair{req.TradingPair})
	if err != nil {
		return nil, fmt.Errorf("get active positions failed: %w", err)
	}

	// 2. 找到指定方向的仓位
	var position *Position
	for i := range positions {
		if positions[i].PositionSide == req.PositionSide && !positions[i].Quantity.IsZero() {
			position = &positions[i]
			break
		}
	}

	if position == nil {
		return nil, fmt.Errorf("no active position found for %s %s", req.TradingPair.ToString(), req.PositionSide)
	}

	resp := &SetStopOrdersResp{}

	// 3. 创建止盈单
	if req.TakeProfit.IsValid() {
		tpId, err := s.createTakeProfitOrder(ctx, req.TradingPair, req.PositionSide, req.TakeProfit.Price, position.Quantity, req.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("create take profit order failed: %w", err)
		}
		resp.TakeProfitId = tpId
	}

	// 4. 创建止损单
	if req.StopLoss.IsValid() {
		slId, err := s.createStopLossOrder(ctx, req.TradingPair, req.PositionSide, req.StopLoss.Price, position.Quantity, req.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("create stop loss order failed: %w", err)
		}
		resp.StopLossId = slId
	}

	return resp, nil
}

// ============ 私有辅助方法 ============

// calculateOpenQuantity 计算开仓数量
func (s *tradingService) calculateOpenQuantity(ctx context.Context, req OpenPositionReq) (
	quantity, estimatedCost, estimatedPrice decimal.Decimal, err error) {

	// 如果直接指定了数量，直接使用
	if !req.Quantity.IsZero() {
		quantity = s.roundQuantity(req.TradingPair, req.Quantity)
		estimatedPrice, err = s.getEstimatedPrice(ctx, req.TradingPair, req.Price)
		if err != nil {
			return decimal.Zero, decimal.Zero, decimal.Zero, err
		}

		// 获取杠杆
		leverage, err := s.getCurrentLeverage(ctx, req.TradingPair)
		if err != nil {
			return decimal.Zero, decimal.Zero, decimal.Zero, err
		}

		// 计算保证金占用：(数量 * 价格) / 杠杆
		estimatedCost = quantity.Mul(estimatedPrice).Div(decimal.NewFromInt(int64(leverage)))
		return quantity, estimatedCost, estimatedPrice, nil
	}

	// 如果使用余额百分比，需要计算数量
	if !req.BalancePercent.IsZero() {
		accountInfo, err := s.accountSvc.GetAccountInfo(ctx)
		if err != nil {
			return decimal.Zero, decimal.Zero, decimal.Zero, fmt.Errorf("get account info failed: %w", err)
		}

		// 可用于开仓的资金 = 可用余额 * 百分比
		availableFunds := accountInfo.AvailableBalance.Mul(req.BalancePercent).Div(decimal.NewFromInt(100))

		// 获取预估价格
		estimatedPrice, err = s.getEstimatedPrice(ctx, req.TradingPair, req.Price)
		if err != nil {
			return decimal.Zero, decimal.Zero, decimal.Zero, err
		}

		// 获取杠杆
		leverage, err := s.getCurrentLeverage(ctx, req.TradingPair)
		if err != nil {
			return decimal.Zero, decimal.Zero, decimal.Zero, err
		}

		// 计算可开仓数量：(可用资金 * 杠杆) / 价格
		quantity = availableFunds.Mul(decimal.NewFromInt(int64(leverage))).Div(estimatedPrice)
		estimatedCost = availableFunds

		// 对数量进行精度处理
		quantity = s.roundQuantity(req.TradingPair, quantity)

		return quantity, estimatedCost, estimatedPrice, nil
	}

	return decimal.Zero, decimal.Zero, decimal.Zero, fmt.Errorf("must specify either Quantity or BalancePercent")
}

// calculateCloseQuantity 计算平仓数量
func (s *tradingService) calculateCloseQuantity(ctx context.Context, req ClosePositionReq) (decimal.Decimal, error) {
	// 如果直接指定了数量，直接使用
	if !req.Quantity.IsZero() {
		return s.roundQuantity(req.TradingPair, req.Quantity), nil
	}

	// 需要获取当前仓位
	positions, err := s.positionSvc.GetActivePositions(ctx, []TradingPair{req.TradingPair})
	if err != nil {
		return decimal.Zero, fmt.Errorf("get active positions failed: %w", err)
	}

	// 找到指定方向的仓位
	var position *Position
	for i := range positions {
		if positions[i].PositionSide == req.PositionSide && !positions[i].Quantity.IsZero() {
			position = &positions[i]
			break
		}
	}

	if position == nil {
		return decimal.Zero, fmt.Errorf("no active position found for %s %s", req.TradingPair.ToString(), req.PositionSide)
	}

	// 如果是全部平仓
	if req.CloseAll {
		return position.Quantity.Abs(), nil // 使用绝对值，因为空仓数量是负数
	}

	// 如果使用百分比
	if !req.Percent.IsZero() {
		quantity := position.Quantity.Abs().Mul(req.Percent).Div(decimal.NewFromInt(100))
		rounded := s.roundQuantity(req.TradingPair, quantity)

		// 如果截断后为0，说明数量太小，至少保留最小精度单位
		if rounded.IsZero() && !quantity.IsZero() {
			precision := s.getQuantityPrecision(req.TradingPair)
			minQuantity := decimal.New(1, -precision) // 例如：precision=3 -> 0.001
			return minQuantity, nil
		}

		return rounded, nil
	}

	return decimal.Zero, fmt.Errorf("must specify either Quantity, Percent or CloseAll")
}

// createTakeProfitOrder 创建止盈订单
func (s *tradingService) createTakeProfitOrder(
	ctx context.Context,
	pair TradingPair,
	positionSide PositionSide,
	triggerPrice decimal.Decimal,
	quantity decimal.Decimal,
	timestamp time.Time,
) (OrderId, error) {
	return s.orderSvc.CreateOrder(ctx, CreateOrderReq{
		TradingPair: pair,
		OrderType:   OrderTypeClose,
		PositonSide: positionSide,
		Price:       triggerPrice,
		Quantity:    quantity,
		Timestamp:   timestamp,
	})
}

// createStopLossOrder 创建止损订单
func (s *tradingService) createStopLossOrder(
	ctx context.Context,
	pair TradingPair,
	positionSide PositionSide,
	triggerPrice decimal.Decimal,
	quantity decimal.Decimal,
	timestamp time.Time,
) (OrderId, error) {
	return s.orderSvc.CreateOrder(ctx, CreateOrderReq{
		TradingPair: pair,
		OrderType:   OrderTypeClose,
		PositonSide: positionSide,
		Price:       triggerPrice,
		Quantity:    quantity,
		Timestamp:   timestamp,
	})
}

// getEstimatedPrice 获取预估成交价格
func (s *tradingService) getEstimatedPrice(
	ctx context.Context,
	pair TradingPair,
	price decimal.Decimal,
) (decimal.Decimal, error) {
	// 如果有价格（限价单），使用指定价格
	if !price.IsZero() {
		return price, nil
	}

	// 如果没有价格（市价单），获取当前市价
	price, err := s.marketSvc.Ticker(ctx, pair)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get market price failed: %w", err)
	}
	return price, nil
}

// getCurrentLeverage 获取当前杠杆倍数
func (s *tradingService) getCurrentLeverage(ctx context.Context, pair TradingPair) (int, error) {
	// 获取当前仓位信息（包含杠杆信息）
	positions, err := s.positionSvc.GetActivePositions(ctx, []TradingPair{pair})
	if err != nil {
		return 0, fmt.Errorf("get positions failed: %w", err)
	}

	// 如果已有仓位，使用仓位的杠杆
	for _, pos := range positions {
		if pos.TradingPair.ToString() == pair.ToString() && pos.Leverage > 0 {
			return pos.Leverage, nil
		}
	}

	// 如果没有仓位，使用默认杠杆（合约默认为20倍）
	// 注意：实际使用时可能需要从配置或账户设置中获取
	return 20, nil
}

// getQuantityPrecision 获取交易对的数量精度
func (s *tradingService) getQuantityPrecision(pair TradingPair) int32 {
	if s.precisionProvider != nil {
		return s.precisionProvider.GetQuantityPrecision(pair)
	}
	// 默认精度为3位小数
	return 3
}

// roundQuantity 根据交易对对数量进行精度处理
func (s *tradingService) roundQuantity(pair TradingPair, quantity decimal.Decimal) decimal.Decimal {
	precision := s.getQuantityPrecision(pair)
	// 向下取整到指定精度
	return quantity.Truncate(precision)
}

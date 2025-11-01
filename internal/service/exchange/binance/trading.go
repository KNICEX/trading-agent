package binance

import (
	"context"
	"fmt"
	"strconv"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

var _ exchange.TradingService = (*TradingService)(nil)

type TradingService struct {
	cli         *futures.Client
	orderSvc    exchange.OrderService
	accountSvc  exchange.AccountService
	positionSvc exchange.PositionService
	marketSvc   exchange.MarketService
}

// NewTradingService 创建交易服务
func NewTradingService(
	cli *futures.Client,
	orderSvc exchange.OrderService,
	accountSvc exchange.AccountService,
	positionSvc exchange.PositionService,
	marketSvc exchange.MarketService,
) *TradingService {
	return &TradingService{
		cli:         cli,
		orderSvc:    orderSvc,
		accountSvc:  accountSvc,
		positionSvc: positionSvc,
		marketSvc:   marketSvc,
	}
}

// OpenPosition 开仓/加仓
func (s *TradingService) OpenPosition(ctx context.Context, req exchange.OpenPositionReq) (*exchange.OpenPositionResp, error) {
	// 1. 计算开仓数量
	quantity, estimatedCost, estimatedPrice, err := s.calculateOpenQuantity(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calculate open quantity failed: %w", err)
	}

	// 2. 创建开仓订单（Side 会自动计算）
	orderId, err := s.orderSvc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: req.TradingPair,
		OrderType:   exchange.OrderTypeOpen, // 开仓类型
		PositonSide: req.PositionSide,
		Price:       req.Price,
		Quantity:    quantity,
	})
	if err != nil {
		return nil, fmt.Errorf("create open position order failed: %w", err)
	}

	resp := &exchange.OpenPositionResp{
		OrderId:        orderId,
		EstimatedCost:  estimatedCost,
		EstimatedPrice: estimatedPrice,
	}

	// 4. 如果设置了止盈止损，创建对应订单
	if req.TakeProfit.IsValid() {
		tpId, err := s.createTakeProfitOrder(ctx, req.TradingPair, req.PositionSide, req.TakeProfit.Price, quantity)
		if err != nil {
			// 止盈单失败不影响主订单，只记录错误
			return resp, fmt.Errorf("main order created successfully, but take profit order failed: %w", err)
		}
		resp.TakeProfitId = tpId
	}

	if req.StopLoss.IsValid() {
		slId, err := s.createStopLossOrder(ctx, req.TradingPair, req.PositionSide, req.StopLoss.Price, quantity)
		if err != nil {
			// 止损单失败不影响主订单，只记录错误
			return resp, fmt.Errorf("main order created successfully, but stop loss order failed: %w", err)
		}
		resp.StopLossId = slId
	}

	return resp, nil
}

// ClosePosition 平仓
func (s *TradingService) ClosePosition(ctx context.Context, req exchange.ClosePositionReq) (exchange.OrderId, error) {
	// 1. 计算平仓数量
	quantity, err := s.calculateCloseQuantity(ctx, req)
	if err != nil {
		return "", fmt.Errorf("calculate close quantity failed: %w", err)
	}

	// 2. 创建平仓订单（Side 会自动计算）
	orderId, err := s.orderSvc.CreateOrder(ctx, exchange.CreateOrderReq{
		TradingPair: req.TradingPair,
		OrderType:   exchange.OrderTypeClose, // 平仓类型
		PositonSide: req.PositionSide,
		Price:       req.Price,
		Quantity:    quantity,
	})
	if err != nil {
		return "", fmt.Errorf("create close position order failed: %w", err)
	}

	return orderId, nil
}

// SetStopOrders 为已有仓位设置止盈止损
func (s *TradingService) SetStopOrders(ctx context.Context, req exchange.SetStopOrdersReq) (*exchange.SetStopOrdersResp, error) {
	// 1. 获取当前仓位数量
	positions, err := s.positionSvc.GetActivePositions(ctx, []exchange.TradingPair{req.TradingPair})
	if err != nil {
		return nil, fmt.Errorf("get active positions failed: %w", err)
	}

	// 2. 找到指定方向的仓位
	var position *exchange.Position
	for i := range positions {
		if positions[i].PositionSide == req.PositionSide && !positions[i].Quantity.IsZero() {
			position = &positions[i]
			break
		}
	}

	if position == nil {
		return nil, fmt.Errorf("no active position found for %s %s", req.TradingPair.ToString(), req.PositionSide)
	}

	resp := &exchange.SetStopOrdersResp{}

	// 3. 创建止盈单
	if req.TakeProfit.IsValid() {
		tpId, err := s.createTakeProfitOrder(ctx, req.TradingPair, req.PositionSide, req.TakeProfit.Price, position.Quantity)
		if err != nil {
			return nil, fmt.Errorf("create take profit order failed: %w", err)
		}
		resp.TakeProfitId = tpId
	}

	// 4. 创建止损单
	if req.StopLoss.IsValid() {
		slId, err := s.createStopLossOrder(ctx, req.TradingPair, req.PositionSide, req.StopLoss.Price, position.Quantity)
		if err != nil {
			return nil, fmt.Errorf("create stop loss order failed: %w", err)
		}
		resp.StopLossId = slId
	}

	return resp, nil
}

// calculateOpenQuantity 计算开仓数量
func (s *TradingService) calculateOpenQuantity(ctx context.Context, req exchange.OpenPositionReq) (
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

		// 对数量进行精度处理（BTC合约精度为3位小数）
		quantity = s.roundQuantity(req.TradingPair, quantity)

		return quantity, estimatedCost, estimatedPrice, nil
	}

	return decimal.Zero, decimal.Zero, decimal.Zero, fmt.Errorf("must specify either Quantity or BalancePercent")
}

// calculateCloseQuantity 计算平仓数量
func (s *TradingService) calculateCloseQuantity(ctx context.Context, req exchange.ClosePositionReq) (decimal.Decimal, error) {
	// 如果直接指定了数量，直接使用
	if !req.Quantity.IsZero() {
		return s.roundQuantity(req.TradingPair, req.Quantity), nil
	}

	// 需要获取当前仓位
	positions, err := s.positionSvc.GetActivePositions(ctx, []exchange.TradingPair{req.TradingPair})
	if err != nil {
		return decimal.Zero, fmt.Errorf("get active positions failed: %w", err)
	}

	// 找到指定方向的仓位
	var position *exchange.Position
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

// createTakeProfitOrder 创建止盈订单（直接使用币安API）
func (s *TradingService) createTakeProfitOrder(
	ctx context.Context,
	pair exchange.TradingPair,
	positionSide exchange.PositionSide,
	triggerPrice decimal.Decimal,
	quantity decimal.Decimal,
) (exchange.OrderId, error) {
	// 止盈的订单方向与平仓方向相同
	orderSide := positionSide.GetCloseOrderSide()

	// 直接使用币安的市价止盈类型
	service := s.cli.NewCreateOrderService().
		Symbol(pair.ToString()).
		Side(futures.SideType(orderSide)).
		Type(futures.OrderTypeTakeProfitMarket). // 币安的市价止盈
		Quantity(quantity.String()).
		PositionSide(futures.PositionSideType(positionSide)).
		StopPrice(triggerPrice.String()).
		WorkingType(futures.WorkingTypeMarkPrice)

	order, err := service.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("create take profit order failed: %w", err)
	}

	return exchange.OrderId(strconv.FormatInt(order.OrderID, 10)), nil
}

// createStopLossOrder 创建止损订单（直接使用币安API）
func (s *TradingService) createStopLossOrder(
	ctx context.Context,
	pair exchange.TradingPair,
	positionSide exchange.PositionSide,
	triggerPrice decimal.Decimal,
	quantity decimal.Decimal,
) (exchange.OrderId, error) {
	// 止损的订单方向与平仓方向相同
	orderSide := positionSide.GetCloseOrderSide()

	// 直接使用币安的市价止损类型
	service := s.cli.NewCreateOrderService().
		Symbol(pair.ToString()).
		Side(futures.SideType(orderSide)).
		Type(futures.OrderTypeStopMarket). // 币安的市价止损
		Quantity(quantity.String()).
		PositionSide(futures.PositionSideType(positionSide)).
		StopPrice(triggerPrice.String()).
		WorkingType(futures.WorkingTypeMarkPrice)

	order, err := service.Do(ctx)
	if err != nil {
		return "", fmt.Errorf("create stop loss order failed: %w", err)
	}

	return exchange.OrderId(strconv.FormatInt(order.OrderID, 10)), nil
}

// getEstimatedPrice 获取预估成交价格
func (s *TradingService) getEstimatedPrice(
	ctx context.Context,
	pair exchange.TradingPair,
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
func (s *TradingService) getCurrentLeverage(ctx context.Context, pair exchange.TradingPair) (int, error) {
	// 获取当前仓位信息（包含杠杆信息）
	positions, err := s.positionSvc.GetActivePositions(ctx, []exchange.TradingPair{pair})
	if err != nil {
		return 0, fmt.Errorf("get positions failed: %w", err)
	}

	// 如果已有仓位，使用仓位的杠杆
	for _, pos := range positions {
		if pos.TradingPair.ToString() == pair.ToString() && pos.Leverage > 0 {
			return pos.Leverage, nil
		}
	}

	// 如果没有仓位，使用默认杠杆（币安合约默认为20倍）
	// 注意：实际使用时可能需要从配置或账户设置中获取
	return 20, nil
}

// getQuantityPrecision 获取交易对的数量精度
func (s *TradingService) getQuantityPrecision(pair exchange.TradingPair) int32 {
	// 常见交易对的精度配置
	// 参考: https://www.binance.com/en/futures/trading-rules
	precisionMap := map[string]int32{
		"BTC":   3, // 0.001
		"ETH":   3, // 0.001
		"BNB":   2, // 0.01
		"SOL":   1, // 0.1
		"DOGE":  0, // 1
		"SHIB":  0, // 1
		"XRP":   1, // 0.1
		"ADA":   0, // 1
		"AVAX":  1, // 0.1
		"DOT":   1, // 0.1
		"MATIC": 0, // 1
	}

	// 获取精度，默认为3位小数
	precision, exists := precisionMap[pair.Base]
	if !exists {
		precision = 3
	}
	return precision
}

// roundQuantity 根据交易对对数量进行精度处理
// 币安合约不同交易对有不同的精度要求
func (s *TradingService) roundQuantity(pair exchange.TradingPair, quantity decimal.Decimal) decimal.Decimal {
	precision := s.getQuantityPrecision(pair)
	// 向下取整到指定精度
	return quantity.Truncate(precision)
}

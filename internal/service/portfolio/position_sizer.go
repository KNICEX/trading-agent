package portfolio

import (
	"context"
	"fmt"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"github.com/shopspring/decimal"
)

var _ PositionSizer = (*SimplePositionSizer)(nil)

// SimplePositionSizer 简单的仓位管理器实现
type SimplePositionSizer struct {
	exchangeSvc exchange.Service
	riskConfig  RiskConfig
}

// NewSimplePositionSizer 创建新的仓位管理器
func NewSimplePositionSizer(exchangeSvc exchange.Service) *SimplePositionSizer {
	return &SimplePositionSizer{
		exchangeSvc: exchangeSvc,
	}
}

// Initialize 初始化仓位管理器
func (s *SimplePositionSizer) Initialize(ctx context.Context, riskConfig RiskConfig) error {
	// 验证风控配置
	if riskConfig.MaxStopLossRatio <= 0 || riskConfig.MaxStopLossRatio >= 1 {
		return fmt.Errorf("MaxStopLossRatio 必须在 (0, 1) 之间，当前值: %f", riskConfig.MaxStopLossRatio)
	}

	if riskConfig.MaxLeverage <= 0 {
		return fmt.Errorf("MaxLeverage 必须大于 0，当前值: %d", riskConfig.MaxLeverage)
	}

	if riskConfig.MinProfitLossRatio < 0 {
		return fmt.Errorf("MinProfitLossRatio 必须大于等于 0，当前值: %f", riskConfig.MinProfitLossRatio)
	}

	if riskConfig.ConfidenceThreshold <= 50 || riskConfig.ConfidenceThreshold > 100 {
		return fmt.Errorf("ConfidenceThreshold 必须在 (50, 100] 之间，当前值: %f", riskConfig.ConfidenceThreshold)
	}

	s.riskConfig = riskConfig
	return nil
}

// HandleSignal 处理策略信号，进行风控检查并计算仓位
func (s *SimplePositionSizer) HandleSignal(ctx context.Context, signal strategy.Signal) (HandleSignalResult, error) {
	result := HandleSignalResult{
		Validated: false,
	}

	// 1. 检查信号类型
	if signal.Action == strategy.SignalActionHold {
		result.Reason = "信号为观望，无需开仓"
		return result, nil
	}

	if signal.Action != strategy.SignalActionLong && signal.Action != strategy.SignalActionShort {
		result.Reason = fmt.Sprintf("不支持的信号类型: %s", signal.Action)
		return result, nil
	}

	// 2. 检查置信度阈值
	if signal.Confidence < s.riskConfig.ConfidenceThreshold {
		result.Reason = fmt.Sprintf("置信度 %.2f%% 低于阈值 %.2f%%", signal.Confidence, s.riskConfig.ConfidenceThreshold)
		return result, nil
	}

	// 3. 检查止损价格是否设置
	if signal.StopLoss.IsZero() {
		result.Reason = "止损价格未设置"
		return result, nil
	}

	// 4. 获取当前市场价格
	currentPrice, err := s.exchangeSvc.MarketService().Ticker(ctx, signal.TradingPair)
	if err != nil {
		return result, fmt.Errorf("获取市场价格失败: %w", err)
	}

	// 5. 计算止损距离比例
	stopLossRatio, err := s.calculateStopLossRatio(signal.Action, currentPrice, signal.StopLoss)
	if err != nil {
		result.Reason = err.Error()
		return result, nil
	}

	// 6. 检查止盈止损比例（如果设置了止盈）
	if !signal.TakeProfit.IsZero() {
		profitLossRatio, err := s.calculateProfitLossRatio(signal.Action, currentPrice, signal.TakeProfit, signal.StopLoss)
		if err != nil {
			result.Reason = err.Error()
			return result, nil
		}

		if profitLossRatio.LessThan(decimal.NewFromFloat(s.riskConfig.MinProfitLossRatio)) {
			result.Reason = fmt.Sprintf("盈亏比 %.2f 低于最小值 %.2f",
				profitLossRatio.InexactFloat64(), s.riskConfig.MinProfitLossRatio)
			return result, nil
		}
	}

	// 7. 获取账户信息
	accountInfo, err := s.exchangeSvc.AccountService().GetAccountInfo(ctx)
	if err != nil {
		return result, fmt.Errorf("获取账户信息失败: %w", err)
	}

	// 8. 计算当前所有持仓的总杠杆
	currentLeverage, err := s.calculateCurrentLeverage(ctx, accountInfo)
	if err != nil {
		return result, fmt.Errorf("计算当前杠杆失败: %w", err)
	}

	// 9. 计算本次开仓的最大可用杠杆
	availableLeverage := s.riskConfig.MaxLeverage - currentLeverage
	if availableLeverage <= 0 {
		result.Reason = fmt.Sprintf("当前总杠杆 %d 已达到或超过最大杠杆 %d",
			currentLeverage, s.riskConfig.MaxLeverage)
		return result, nil
	}

	// 10. 根据止损比例和最大止损资金比例计算仓位杠杆
	// 公式：仓位杠杆 = 最大止损资金比例 / 止损距离比例
	// 例如：止损距离1%，最大止损资金5%，则最大可开5x杠杆
	positionLeverage := decimal.NewFromFloat(s.riskConfig.MaxStopLossRatio).Div(stopLossRatio)

	// 11. 根据置信度调整仓位
	// 简单的线性调整：置信度越高，使用的杠杆比例越大
	// 置信度范围：[ConfidenceThreshold, 100]
	// 调整因子范围：[0, 1]
	confidenceAdjustment := (signal.Confidence - s.riskConfig.ConfidenceThreshold) / (100 - s.riskConfig.ConfidenceThreshold)
	// 至少使用 50% 的计算杠杆，最多 100%
	leverageMultiplier := 0.5 + (confidenceAdjustment * 0.5)
	adjustedLeverage := positionLeverage.Mul(decimal.NewFromFloat(leverageMultiplier))

	// 12. 限制仓位杠杆不超过可用杠杆
	if adjustedLeverage.GreaterThan(decimal.NewFromInt(int64(availableLeverage))) {
		adjustedLeverage = decimal.NewFromInt(int64(availableLeverage))
	}

	// 13. 确保杠杆至少为 1
	if adjustedLeverage.LessThan(decimal.NewFromInt(1)) {
		result.Reason = fmt.Sprintf("计算出的杠杆 %.2f 小于 1，无法开仓", adjustedLeverage.InexactFloat64())
		return result, nil
	}

	// 14. 计算开仓数量
	// 开仓价值 = 可用余额 * 调整后杠杆
	// 开仓数量 = 开仓价值 / 当前价格
	positionValue := accountInfo.AvailableBalance.Mul(adjustedLeverage)
	quantity := positionValue.Div(currentPrice)

	// 15. 构建增强信号
	positionSide := exchange.PositionSideLong
	if signal.Action == strategy.SignalActionShort {
		positionSide = exchange.PositionSideShort
	}

	result.EnhancedSignal = EnhancedSignal{
		TradingPair:  signal.TradingPair,
		PositionSide: positionSide,
		Quantity:     quantity,
		TakeProfit:   signal.TakeProfit,
		StopLoss:     signal.StopLoss,
		Timestamp:    signal.Timestamp,
	}
	result.Validated = true
	result.Reason = fmt.Sprintf("通过风控检查 - 置信度: %.2f%%, 止损比例: %.2f%%, 仓位杠杆: %.2fx",
		signal.Confidence, stopLossRatio.Mul(decimal.NewFromInt(100)).InexactFloat64(), adjustedLeverage.InexactFloat64())

	return result, nil
}

// calculateStopLossRatio 计算止损距离比例
func (s *SimplePositionSizer) calculateStopLossRatio(
	action strategy.SignalAction,
	currentPrice, stopLoss decimal.Decimal,
) (decimal.Decimal, error) {
	if currentPrice.IsZero() {
		return decimal.Zero, fmt.Errorf("当前价格为 0")
	}

	var stopLossDistance decimal.Decimal
	if action == strategy.SignalActionLong {
		// 做多：止损价应该低于当前价
		if stopLoss.GreaterThanOrEqual(currentPrice) {
			return decimal.Zero, fmt.Errorf("做多止损价 %s 应低于当前价 %s",
				stopLoss.String(), currentPrice.String())
		}
		stopLossDistance = currentPrice.Sub(stopLoss)
	} else {
		// 做空：止损价应该高于当前价
		if stopLoss.LessThanOrEqual(currentPrice) {
			return decimal.Zero, fmt.Errorf("做空止损价 %s 应高于当前价 %s",
				stopLoss.String(), currentPrice.String())
		}
		stopLossDistance = stopLoss.Sub(currentPrice)
	}

	// 计算止损距离占当前价格的比例
	stopLossRatio := stopLossDistance.Div(currentPrice)
	return stopLossRatio, nil
}

// calculateProfitLossRatio 计算盈亏比
func (s *SimplePositionSizer) calculateProfitLossRatio(
	action strategy.SignalAction,
	currentPrice, takeProfit, stopLoss decimal.Decimal,
) (decimal.Decimal, error) {
	if currentPrice.IsZero() {
		return decimal.Zero, fmt.Errorf("当前价格为 0")
	}

	var profitDistance, lossDistance decimal.Decimal

	if action == strategy.SignalActionLong {
		// 做多
		if takeProfit.LessThanOrEqual(currentPrice) {
			return decimal.Zero, fmt.Errorf("做多止盈价 %s 应高于当前价 %s",
				takeProfit.String(), currentPrice.String())
		}
		profitDistance = takeProfit.Sub(currentPrice)
		lossDistance = currentPrice.Sub(stopLoss)
	} else {
		// 做空
		if takeProfit.GreaterThanOrEqual(currentPrice) {
			return decimal.Zero, fmt.Errorf("做空止盈价 %s 应低于当前价 %s",
				takeProfit.String(), currentPrice.String())
		}
		profitDistance = currentPrice.Sub(takeProfit)
		lossDistance = stopLoss.Sub(currentPrice)
	}

	if lossDistance.IsZero() {
		return decimal.Zero, fmt.Errorf("止损距离为 0")
	}

	// 盈亏比 = 盈利距离 / 亏损距离
	profitLossRatio := profitDistance.Div(lossDistance)
	return profitLossRatio, nil
}

// calculateCurrentLeverage 计算当前所有持仓的总杠杆
func (s *SimplePositionSizer) calculateCurrentLeverage(
	ctx context.Context,
	accountInfo exchange.AccountInfo,
) (int, error) {
	// 获取所有持仓
	positions, err := s.exchangeSvc.PositionService().GetActivePositions(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("获取持仓信息失败: %w", err)
	}

	if len(positions) == 0 {
		return 0, nil
	}

	// 计算所有持仓的总价值
	totalPositionValue := decimal.Zero
	for _, position := range positions {
		// 持仓价值 = 数量 * 标记价格
		positionValue := position.Quantity.Abs().Mul(position.MarkPrice)
		totalPositionValue = totalPositionValue.Add(positionValue)
	}

	// 总杠杆 = 总持仓价值 / 总余额
	// 注意：这里使用 TotalBalance（总余额）而不是 AvailableBalance
	if accountInfo.TotalBalance.IsZero() {
		return 0, nil
	}

	currentLeverage := totalPositionValue.Div(accountInfo.TotalBalance)
	return int(currentLeverage.IntPart()), nil
}

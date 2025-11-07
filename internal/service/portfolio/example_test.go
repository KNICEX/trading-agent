package portfolio_test

import (
	"context"
	"fmt"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/portfolio"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"github.com/shopspring/decimal"
)

// 这个示例展示了如何使用 PositionSizer 进行风控和仓位管理
func Example_positionSizer() {
	ctx := context.Background()

	// 1. 创建交易所服务（这里使用模拟数据）
	// 实际使用时应该创建真实的交易所服务
	var exchangeSvc exchange.Service
	// exchangeSvc = createExchangeService()

	// 2. 创建仓位管理器
	sizer := portfolio.NewSimplePositionSizer(exchangeSvc)

	// 3. 配置风控参数
	riskConfig := portfolio.RiskConfig{
		MaxStopLossRatio:    0.05, // 单笔最大止损 5%
		MaxLeverage:         10,   // 最大总杠杆 10x
		MinProfitLossRatio:  1.5,  // 最小盈亏比 1.5:1
		ConfidenceThreshold: 60,   // 最小置信度 60%
	}

	// 4. 初始化仓位管理器
	err := sizer.Initialize(ctx, riskConfig)
	if err != nil {
		fmt.Printf("初始化失败: %v\n", err)
		return
	}

	// 5. 处理策略信号
	signal := strategy.Signal{
		TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
		Action:      strategy.SignalActionLong,
		Confidence:  75, // 75% 置信度
		TakeProfit:  decimal.NewFromInt(52000),
		StopLoss:    decimal.NewFromInt(49500),
		Timestamp:   time.Now(),
		Reason:      "突破关键阻力位，MACD 金叉",
	}

	result, err := sizer.HandleSignal(ctx, signal)
	if err != nil {
		fmt.Printf("处理信号失败: %v\n", err)
		return
	}

	// 6. 检查风控结果
	if !result.Validated {
		fmt.Printf("❌ 信号未通过风控: %s\n", result.Reason)
		return
	}

	// 7. 使用增强信号
	fmt.Printf("✅ 信号通过风控: %s\n", result.Reason)
	fmt.Printf("交易对: %s\n", result.EnhancedSignal.TradingPair.ToString())
	fmt.Printf("方向: %s\n", result.EnhancedSignal.PositionSide)
	fmt.Printf("数量: %s\n", result.EnhancedSignal.Quantity.String())
	fmt.Printf("止盈: %s\n", result.EnhancedSignal.TakeProfit.String())
	fmt.Printf("止损: %s\n", result.EnhancedSignal.StopLoss.String())

	// 8. 可以继续用增强信号进行交易
	// trading.OpenPosition(ctx, result.EnhancedSignal)
}

// 这个示例展示了不同置信度对仓位大小的影响
func Example_confidenceImpact() {
	// 假设其他条件相同，只改变置信度

	confidences := []float64{60, 70, 80, 90, 100}

	fmt.Println("置信度对仓位大小的影响（其他条件相同）：")
	fmt.Println("假设：止损 1%，MaxStopLossRatio 5%，理论最大杠杆 5x")
	fmt.Println()

	for _, confidence := range confidences {
		// 调整因子 = (confidence - 60) / (100 - 60)
		adjustmentFactor := (confidence - 60) / (100 - 60)
		// 杠杆倍数 = 0.5 + adjustmentFactor * 0.5
		leverageMultiplier := 0.5 + adjustmentFactor*0.5
		// 实际杠杆 = 5 * leverageMultiplier
		actualLeverage := 5 * leverageMultiplier

		fmt.Printf("置信度 %.0f%% → 杠杆倍数 %.2f → 实际杠杆 %.2fx\n",
			confidence, leverageMultiplier, actualLeverage)
	}

	// Output:
	// 置信度对仓位大小的影响（其他条件相同）：
	// 假设：止损 1%，MaxStopLossRatio 5%，理论最大杠杆 5x
	//
	// 置信度 60% → 杠杆倍数 0.50 → 实际杠杆 2.50x
	// 置信度 70% → 杠杆倍数 0.62 → 实际杠杆 3.12x
	// 置信度 80% → 杠杆倍数 0.75 → 实际杠杆 3.75x
	// 置信度 90% → 杠杆倍数 0.88 → 实际杠杆 4.38x
	// 置信度 100% → 杠杆倍数 1.00 → 实际杠杆 5.00x
}

// 这个示例展示了止损距离对仓位大小的影响
func Example_stopLossImpact() {
	fmt.Println("止损距离对仓位大小的影响（其他条件相同）：")
	fmt.Println("假设：MaxStopLossRatio 5%，置信度 80%（杠杆倍数 0.75）")
	fmt.Println()

	stopLossRatios := []float64{0.005, 0.01, 0.02, 0.05}

	for _, stopLossRatio := range stopLossRatios {
		// 理论最大杠杆 = 0.05 / stopLossRatio
		maxLeverage := 0.05 / stopLossRatio
		// 实际杠杆 = maxLeverage * 0.75
		actualLeverage := maxLeverage * 0.75

		fmt.Printf("止损距离 %.1f%% → 理论杠杆 %.2fx → 实际杠杆 %.2fx\n",
			stopLossRatio*100, maxLeverage, actualLeverage)
	}

	// Output:
	// 止损距离对仓位大小的影响（其他条件相同）：
	// 假设：MaxStopLossRatio 5%，置信度 80%（杠杆倍数 0.75）
	//
	// 止损距离 0.5% → 理论杠杆 10.00x → 实际杠杆 7.50x
	// 止损距离 1.0% → 理论杠杆 5.00x → 实际杠杆 3.75x
	// 止损距离 2.0% → 理论杠杆 2.50x → 实际杠杆 1.88x
	// 止损距离 5.0% → 理论杠杆 1.00x → 实际杠杆 0.75x
}

// 这个示例展示了如何处理多个信号
func Example_multipleSignals() {
	ctx := context.Background()

	// 创建仓位管理器
	var exchangeSvc exchange.Service
	sizer := portfolio.NewSimplePositionSizer(exchangeSvc)

	// 初始化
	riskConfig := portfolio.RiskConfig{
		MaxStopLossRatio:    0.05,
		MaxLeverage:         10,
		MinProfitLossRatio:  1.5,
		ConfidenceThreshold: 60,
	}
	sizer.Initialize(ctx, riskConfig)

	// 多个策略信号
	signals := []strategy.Signal{
		{
			TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
			Action:      strategy.SignalActionLong,
			Confidence:  80,
			StopLoss:    decimal.NewFromInt(49500),
			TakeProfit:  decimal.NewFromInt(52000),
			Timestamp:   time.Now(),
		},
		{
			TradingPair: exchange.TradingPair{Base: "ETH", Quote: "USDT"},
			Action:      strategy.SignalActionShort,
			Confidence:  65,
			StopLoss:    decimal.NewFromInt(3050),
			TakeProfit:  decimal.NewFromInt(2850),
			Timestamp:   time.Now(),
		},
		{
			TradingPair: exchange.TradingPair{Base: "SOL", Quote: "USDT"},
			Action:      strategy.SignalActionLong,
			Confidence:  55, // 低于阈值
			StopLoss:    decimal.NewFromInt(95),
			TakeProfit:  decimal.NewFromInt(110),
			Timestamp:   time.Now(),
		},
	}

	// 处理每个信号
	for i, signal := range signals {
		result, err := sizer.HandleSignal(ctx, signal)
		if err != nil {
			fmt.Printf("信号 %d 处理失败: %v\n", i+1, err)
			continue
		}

		if result.Validated {
			fmt.Printf("✅ 信号 %d (%s): 通过风控\n",
				i+1, signal.TradingPair.ToString())
		} else {
			fmt.Printf("❌ 信号 %d (%s): %s\n",
				i+1, signal.TradingPair.ToString(), result.Reason)
		}
	}
}

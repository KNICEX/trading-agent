# Portfolio 仓位管理服务

## 概述

Portfolio 服务提供了风控和仓位管理功能，主要负责：
1. 根据风险配置和策略信号计算合理的开仓数量
2. 进行风险控制检查，确保交易符合风控要求
3. 根据置信度动态调整仓位大小

## 核心组件

### PositionSizer 接口

仓位管理器接口，定义了风控和仓位计算的核心方法。

```go
type PositionSizer interface {
    Initialize(ctx context.Context, riskConfig RiskConfig) error
    HandleSignal(ctx context.Context, signal strategy.Signal) (HandleSignalResult, error)
}
```

### SimplePositionSizer 实现

`SimplePositionSizer` 是 `PositionSizer` 接口的基础实现，提供了完整的风控检查和仓位计算功能。

## 风控配置

### RiskConfig

```go
type RiskConfig struct {
    // 最大止损全仓资金比例 (0, 1)
    // 例如：0.05 表示单笔最大止损为总资金的 5%
    MaxStopLossRatio float64
    
    // 全仓最大杠杆
    // 例如：10 表示所有持仓总杠杆不超过 10x
    MaxLeverage int
    
    // 最小盈亏比（仅限止盈止损订单有效，跟踪止盈无效）
    // 例如：1.5 表示盈利目标至少是亏损的 1.5 倍
    MinProfitLossRatio float64
    
    // 置信度阈值 > 50
    // 例如：60 表示只接受置信度高于 60% 的信号
    ConfidenceThreshold float64
}
```

## 仓位计算逻辑

### 1. 基于止损比例的杠杆计算

这是最核心的风控逻辑，确保每笔交易的最大损失不超过设定的资金比例。

**公式：**
```
仓位杠杆 = MaxStopLossRatio / 止损距离比例
```

**示例：**
- 当前价格：50000 USDT
- 止损价格：49500 USDT（做多）
- 止损距离比例：(50000 - 49500) / 50000 = 1%
- MaxStopLossRatio：5%
- **理论最大杠杆 = 5% / 1% = 5x**

这意味着：
- 如果开 5x 杠杆，价格触及止损时，损失正好是总资金的 5%
- 如果开 3x 杠杆，价格触及止损时，损失是总资金的 3%

### 2. 置信度调整

根据策略给出的置信度动态调整仓位，置信度越高，使用的杠杆比例越大。

**公式：**
```
调整因子 = (当前置信度 - 置信度阈值) / (100 - 置信度阈值)
杠杆倍数 = 0.5 + 调整因子 * 0.5
实际杠杆 = 理论最大杠杆 * 杠杆倍数
```

**示例：**（接上面的例子）
- 置信度阈值：60%
- 当前置信度：80%
- 调整因子 = (80 - 60) / (100 - 60) = 0.5
- 杠杆倍数 = 0.5 + 0.5 * 0.5 = 0.75
- **实际杠杆 = 5 * 0.75 = 3.75x**

置信度对应的杠杆倍数：
- 置信度 = 60%（阈值）→ 杠杆倍数 = 50%
- 置信度 = 80% → 杠杆倍数 = 75%
- 置信度 = 100% → 杠杆倍数 = 100%

### 3. 总杠杆限制

确保所有持仓的总杠杆不超过配置的最大值。

**计算方式：**
```
当前总杠杆 = Σ(持仓价值) / 总余额
可用杠杆 = MaxLeverage - 当前总杠杆
实际使用杠杆 = min(计算出的杠杆, 可用杠杆)
```

### 4. 最终开仓数量

```
开仓价值 = 可用余额 * 实际杠杆
开仓数量 = 开仓价值 / 当前价格
```

## 风控检查项

`HandleSignal` 方法会依次执行以下检查：

1. **信号类型检查**：只处理 LONG 和 SHORT 信号
2. **置信度检查**：置信度必须 ≥ ConfidenceThreshold
3. **止损设置检查**：止损价格必须设置（不能为 0）
4. **止损价格合理性检查**：
   - 做多：止损价必须低于当前价
   - 做空：止损价必须高于当前价
5. **盈亏比检查**（如果设置了止盈）：盈亏比必须 ≥ MinProfitLossRatio
6. **总杠杆检查**：确保不超过 MaxLeverage

所有检查通过后，才会计算开仓数量。

## 使用示例

### 初始化

```go
import (
    "github.com/KNICEX/trading-agent/internal/service/exchange"
    "github.com/KNICEX/trading-agent/internal/service/portfolio"
)

// 创建交易所服务
exchangeSvc := exchange.NewService(...)

// 创建仓位管理器
sizer := portfolio.NewSimplePositionSizer(exchangeSvc)

// 配置风控参数
riskConfig := portfolio.RiskConfig{
    MaxStopLossRatio:    0.05,  // 单笔最大止损 5%
    MaxLeverage:         10,    // 最大总杠杆 10x
    MinProfitLossRatio:  1.5,   // 最小盈亏比 1.5:1
    ConfidenceThreshold: 60,    // 最小置信度 60%
}

// 初始化
err := sizer.Initialize(ctx, riskConfig)
if err != nil {
    log.Fatal(err)
}
```

### 处理策略信号

```go
import "github.com/KNICEX/trading-agent/internal/service/strategy"

// 策略生成的信号
signal := strategy.Signal{
    TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    Action:      strategy.SignalActionLong,
    Confidence:  75,  // 75% 置信度
    TakeProfit:  decimal.NewFromInt(52000),
    StopLoss:    decimal.NewFromInt(49000),
    Timestamp:   time.Now(),
}

// 经过风控处理
result, err := sizer.HandleSignal(ctx, signal)
if err != nil {
    log.Printf("处理信号失败: %v", err)
    return
}

if !result.Validated {
    log.Printf("信号未通过风控: %s", result.Reason)
    return
}

// 通过风控，可以使用增强信号进行交易
enhancedSignal := result.EnhancedSignal
log.Printf("开仓数量: %s %s", 
    enhancedSignal.Quantity.String(), 
    enhancedSignal.TradingPair.Base)

// 使用增强信号开仓
// ...
```

## 完整示例

### 场景：BTC 做多

**市场状态：**
- 当前 BTC 价格：50,000 USDT
- 账户总余额：10,000 USDT
- 可用余额：10,000 USDT
- 当前持仓：无

**策略信号：**
- 操作：做多
- 置信度：80%
- 止盈：52,000 USDT
- 止损：49,500 USDT

**风控配置：**
- MaxStopLossRatio：5%
- MaxLeverage：10x
- MinProfitLossRatio：1.5
- ConfidenceThreshold：60%

**计算过程：**

1. **止损距离比例**
   ```
   止损距离 = 50,000 - 49,500 = 500 USDT
   止损比例 = 500 / 50,000 = 1%
   ```

2. **盈亏比检查**
   ```
   盈利距离 = 52,000 - 50,000 = 2,000 USDT
   亏损距离 = 50,000 - 49,500 = 500 USDT
   盈亏比 = 2,000 / 500 = 4.0 ✓ (> 1.5)
   ```

3. **理论最大杠杆**
   ```
   理论杠杆 = 5% / 1% = 5x
   ```

4. **置信度调整**
   ```
   调整因子 = (80 - 60) / (100 - 60) = 0.5
   杠杆倍数 = 0.5 + 0.5 * 0.5 = 0.75
   实际杠杆 = 5 * 0.75 = 3.75x
   ```

5. **总杠杆检查**
   ```
   当前总杠杆 = 0（无持仓）
   可用杠杆 = 10 - 0 = 10x ✓
   最终杠杆 = min(3.75, 10) = 3.75x
   ```

6. **开仓数量**
   ```
   开仓价值 = 10,000 * 3.75 = 37,500 USDT
   开仓数量 = 37,500 / 50,000 = 0.75 BTC
   ```

**结果：**
- ✅ 通过风控
- 开仓 0.75 BTC 做多
- 如果触及止损（49,500），损失约 375 USDT（3.75%）
- 如果触及止盈（52,000），盈利约 1,500 USDT（15%）

## 注意事项

1. **止损必须设置**：系统要求所有信号必须设置止损价格，这是风控的基础
2. **置信度阈值**：必须大于 50%，确保只接受有一定把握的信号
3. **杠杆计算**：实际杠杆会根据置信度动态调整，避免过度激进
4. **总杠杆限制**：所有持仓的总杠杆不能超过配置的最大值
5. **价格精度**：实际交易时需要根据交易所的精度规则对数量进行舍入

## 扩展

如果需要更复杂的仓位管理策略，可以：

1. 实现自己的 `PositionSizer` 接口
2. 添加更多风控指标（如最大回撤、胜率等）
3. 使用更复杂的仓位调整算法（如 Kelly 公式）
4. 根据不同的市场状态使用不同的风控参数

## 测试

运行单元测试：

```bash
go test -v ./internal/service/portfolio
```

测试覆盖了以下场景：
- 配置验证
- 置信度检查
- 止损设置检查
- 盈亏比检查
- 总杠杆限制
- 做多/做空场景
- 止损比例计算
- 盈亏比计算


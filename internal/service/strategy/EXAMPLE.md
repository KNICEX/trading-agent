# 策略使用示例

## SimpleTestStrategy - 简单测试策略

这是一个基于双均线交叉的简单测试策略，实现了 `Strategy` 接口的所有方法。

### 策略说明

- **类型**: 双均线交叉策略
- **短期均线**: 5周期
- **长期均线**: 20周期
- **时间周期**: 5分钟
- **交易信号**:
  - 金叉（短期均线上穿长期均线）→ 做多信号
  - 死叉（短期均线下穿长期均线）→ 做空信号
  - 无交叉 → 观望

### 风险控制

- **止盈**: 入场价格的 ±2%
- **止损**: 入场价格的 ±1%

### 使用示例

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/KNICEX/trading-agent/internal/service/exchange"
    "github.com/KNICEX/trading-agent/internal/service/strategy"
)

func main() {
    // 创建交易对
    tradingPair := exchange.TradingPair{
        Base:  "BTC",
        Quote: "USDT",
    }
    
    // 创建策略实例
    testStrategy := strategy.NewSimpleTestStrategy(tradingPair)
    
    // 获取策略需要的K线周期
    interval := testStrategy.Interval() // 返回 exchange.Interval5m
    fmt.Printf("策略需要 %s 周期的K线数据\n", interval.ToString())
    
    // 创建策略上下文（需要实现 strategy.Context 接口）
    // strategyCtx := ... 
    
    // 初始化策略
    err := testStrategy.Initialize(context.Background(), strategyCtx)
    if err != nil {
        panic(err)
    }
    
    // 订阅K线数据（根据策略要求的周期）
    // klineChan, err := marketService.SubscribeKline(ctx, tradingPair, interval)
    
    // 处理新的K线数据
    // kline := <-klineChan
    signal, err := testStrategy.OnKline(context.Background(), kline)
    if err != nil {
        panic(err)
    }
    
    // 根据信号执行交易
    switch signal.Action {
    case strategy.SignalActionLong:
        fmt.Println("做多信号:", signal.Reason)
    case strategy.SignalActionShort:
        fmt.Println("做空信号:", signal.Reason)
    case strategy.SignalActionHold:
        fmt.Println("观望:", signal.Reason)
    }
    
    // 关闭策略
    err = testStrategy.Shutdown(context.Background())
    if err != nil {
        panic(err)
    }
}
```

### 运行测试

```bash
# 运行所有策略测试
go test -v ./internal/service/strategy/...

# 只运行 SimpleTestStrategy 的测试
go test -v ./internal/service/strategy/... -run TestSimpleTestStrategy
```

### 测试覆盖

- ✅ 策略初始化
- ✅ 金叉信号（做多）
- ✅ 死叉信号（做空）
- ✅ 数据不足处理
- ✅ 策略关闭
- ✅ 重复信号过滤

### 自定义参数

如果需要自定义参数，可以修改策略结构：

```go
strategy := &strategy.SimpleTestStrategy{
    name:        "custom_strategy",
    tradingPair: tradingPair,
    shortPeriod: 10,  // 自定义短期周期
    longPeriod:  30,  // 自定义长期周期
    interval:    exchange.Interval15m, // 自定义时间周期
    klines:      make([]exchange.Kline, 0, 100),
    lastSignal:  strategy.SignalActionHold,
}
```

### 注意事项

1. 这是一个**测试策略**，不建议直接用于实盘交易
2. 策略需要至少 20 根K线数据才能开始计算信号
3. 策略会自动过滤重复信号，避免频繁交易
4. 信号的置信度固定为 0.7
5. 止盈止损是基于简单的百分比计算，实际应用中需要更复杂的风险管理

### 进一步改进建议

- [ ] 添加更多技术指标（RSI、MACD、布林带等）
- [ ] 实现动态止盈止损
- [ ] 添加仓位管理
- [ ] 实现多时间周期分析
- [ ] 添加回测性能统计
- [ ] 实现参数优化功能


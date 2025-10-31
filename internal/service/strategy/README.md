# Strategy 策略引擎

## 主循环工作流程

```
启动
 ↓
初始化策略
 ↓
┌─────────────────────────────────┐
│    主循环 (K线轮询)              │
│                                 │
│  1. 每隔 N 秒轮询一次            │
│     (N = K线周期 / 2)            │
│     ↓                            │
│  2. 获取最新K线                  │
│     ↓                            │
│  3. 检查是否已处理过              │
│     ↓ (未处理)                   │
│  4. 等待K线完成                  │
│     ↓                            │
│  5. 调用 Strategy.OnBar()        │ ← 这里调用策略
│     ↓                            │
│  6. 策略返回 Signal              │
│     ↓                            │
│  7. Executor 执行信号            │
│     ├─ 风控检查                  │
│     ├─ 调用 exchange             │
│     └─ 记录日志                  │
│     ↓                            │
│  回到步骤1                       │
│                                 │
└─────────────────────────────────┘
 ↓ (收到停止信号)
关闭策略
 ↓
退出
```

## 什么时候调用策略？

### 1. K线完成时（主要触发点）

```go
// 引擎每隔一段时间轮询
ticker := time.NewTicker(pollInterval)

for range ticker.C {
    // 获取最新K线
    klines := GetKlines(limit: 1)
    
    // 检查K线是否完成（CloseTime已过）
    if time.Now().After(kline.CloseTime) {
        // 调用策略
        signal := strategy.OnBar(bar)
        
        // 执行信号
        executor.Execute(signal)
    }
}
```

### 2. 轮询间隔

根据K线周期自动计算：

| K线周期 | 轮询间隔 | 说明 |
|---------|---------|------|
| 5分钟   | 2.5分钟 | 确保能及时获取新K线 |
| 15分钟  | 7.5分钟 | |
| 1小时   | 30分钟  | |
| 4小时   | 2小时   | |

### 3. 防重复处理

```go
// 使用 map 记录已处理的K线
processedBars[kline.OpenTime] = true

// 每次检查是否处理过
if processedBars[kline.OpenTime] {
    return // 跳过
}
```

## 使用方法

### 1. 实现你的策略

```go
type MyStrategy struct {
    ctx Context
}

func (s *MyStrategy) OnBar(ctx context.Context, bar *Bar) (*Signal, error) {
    // 1. 获取历史数据
    klines, _ := s.ctx.GetKlines(ctx, exchange.GetKlinesReq{
        TradingPair: bar.TradingPair,
        Interval:    bar.Interval,
        Limit:       50,
    })
    
    // 2. 计算指标
    ma20 := calculateMA(klines, 20)
    
    // 3. 获取持仓
    position, _ := s.ctx.GetPositions(ctx, bar.TradingPair)
    
    // 4. 判断信号
    if shouldBuy {
        return &Signal{
            Action: SignalActionLong,
            Size:   30,
            Reason: "买入信号",
        }, nil
    }
    
    return &Signal{
        Action: SignalActionHold,
        Reason: "观望",
    }, nil
}
```

### 2. 创建并启动引擎

```go
func main() {
    ctx := context.Background()
    
    // 创建 exchange 服务
    marketSvc := ...
    tradingSvc := ...
    positionSvc := ...
    
    // 创建上下文
    strategyCtx := NewLiveContext(marketSvc, positionSvc, ...)
    
    // 创建策略
    myStrategy := NewMyStrategy()
    
    // 创建执行器
    executor := NewSimpleExecutor(tradingSvc, positionSvc)
    
    // 创建引擎
    engine := NewEngine(
        myStrategy,
        strategyCtx,
        executor,
        tradingPair,    // BTC/USDT
        interval,       // 1小时
    )
    
    // 启动引擎（启动主循环）
    engine.Start(ctx)
    
    // 等待退出信号
    <-sigChan
    
    // 停止引擎
    engine.Stop(ctx)
}
```

## 核心组件

### 1. Engine 引擎

- 负责主循环
- 轮询K线数据
- 调用策略
- 分发信号

### 2. Strategy 策略

- 实现 `OnBar()` 方法
- 返回 `Signal`
- 不直接交易

### 3. Executor 执行器

- 接收 `Signal`
- 风控检查
- 调用 `exchange` 下单

### 4. Context 上下文

- 提供数据访问
- 隔离策略和实现
- 支持回测/实盘切换

## 信号类型

### 无持仓时
- `LONG` - 做多
- `SHORT` - 做空
- `HOLD` - 观望

### 有持仓时
- `ADD` - 加仓
- `REDUCE` - 减仓
- `CLOSE` - 平仓

## 完整示例

见 `example_strategy.go` - 简单的均线策略

```go
// 创建策略
strategy := NewSimpleMAStrategy(
    "MA_Strategy",
    tradingPair,
    20,  // 快线
    50,  // 慢线
)

// 启动
engine := NewEngine(strategy, ctx, executor, pair, interval)
engine.Start(context.Background())
```

## 日志输出示例

```
[Engine] 策略引擎启动: MA_Strategy, 交易对: BTCUSDT, 周期: 1h
[Engine] 策略初始化完成: MA_Strategy
[Engine] K线轮询启动，间隔: 30m0s
[Engine] 新K线: BTCUSDT 2024-01-15 10:00:00, Open: 45000, Close: 45200
[MA_Strategy] 收到K线: 2024-01-15 10:00:00
[MA_Strategy] MA(20)=44800.00, MA(50)=44500.00
[MA_Strategy] 金叉信号，准备做多
[Engine] 策略信号: LONG, Size: 30.00%, Reason: MA金叉信号
[Executor] 执行信号: LONG, Size: 30.00%, Reason: MA金叉信号
[Executor] 开多仓成功: OrderID=123456, 预估成本=1000.00
```

## 关键时间点

### 1. K线轮询
- 每 `pollInterval` 执行一次
- 检查是否有新K线

### 2. K线完成判断
```go
if time.Now().After(kline.CloseTime) {
    // K线已完成，可以处理
}
```

### 3. 策略调用
```go
signal, err := strategy.OnBar(ctx, bar)
```

### 4. 信号执行
```go
executor.Execute(ctx, signal)
```

## 注意事项

1. **防止重复处理**
   - 使用 `processedBars` map 记录
   - 每根K线只处理一次

2. **等待K线完成**
   - 不在K线形成中处理（可配置）
   - 避免数据不完整

3. **错误处理**
   - 获取数据失败 → 记录日志，继续轮询
   - 策略出错 → 记录日志，不影响主循环
   - 执行失败 → 记录日志，等待下次信号

4. **优雅退出**
   - 监听 `SIGINT`/`SIGTERM`
   - 调用 `strategy.Shutdown()`
   - 清理资源

## 扩展功能

### 1. 订单监控（可选）
```go
go engine.runOrderMonitor(ctx)
```

### 2. 持仓监控（可选）
```go
go engine.runPositionMonitor(ctx)
```

### 3. 多策略运行
```go
engine1 := NewEngine(strategy1, ...)
engine2 := NewEngine(strategy2, ...)

engine1.Start(ctx)
engine2.Start(ctx)
```

### 4. 风控层
```go
type RiskExecutor struct {
    baseExecutor Executor
    riskManager  *RiskManager
}

func (e *RiskExecutor) Execute(ctx context.Context, signal *Signal) error {
    // 风控检查
    if err := e.riskManager.Check(signal); err != nil {
        return err
    }
    // 执行
    return e.baseExecutor.Execute(ctx, signal)
}
```

## 性能优化

1. **轮询间隔不要太短**
   - 避免频繁请求API
   - 根据K线周期合理设置

2. **历史数据缓存**
   - 缓存最近的K线数据
   - 减少API调用

3. **并发处理**
   - 多个策略并发运行
   - 使用 goroutine

## 总结

主循环就是：**定时轮询 → 获取K线 → 调用策略 → 执行信号**

简单、清晰、易于理解！🚀


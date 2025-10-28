# 持仓历史功能说明

## 功能概述

`GetHistoryPositions` 方法通过分析币安的成交记录（Account Trades），推导出持仓的完整生命周期。

## 实现原理

### 数据来源

使用币安 API：`/fapi/v1/userTrades` (ListAccountTradeService)

每条成交记录包含：
- `OrderID` - 订单ID
- `Side` - BUY/SELL
- `PositionSide` - LONG/SHORT
- `Price` - 成交价格
- `Quantity` - 成交数量
- `RealizedPnl` - 已实现盈亏（重要！）
- `Commission` - 手续费
- `Time` - 成交时间

### 推导逻辑

```
1. 按 PositionSide 分组（LONG/SHORT 是两个独立的持仓）
2. 按时间排序
3. 逐条分析：
   - LONG + BUY  = 开仓/加仓
   - LONG + SELL = 减仓/平仓
   - SHORT + SELL = 开仓/加仓
   - SHORT + BUY  = 减仓/平仓
4. 追踪持仓数量变化，生成事件
5. 计算平均开仓价、平均平仓价
```

### 事件类型

- `CREATE` - 开仓（首次建仓）
- `INCREASE` - 加仓（增加持仓）
- `DECREASE` - 减仓（部分平仓）
- `CLOSE` - 完全平仓（持仓归零）

## 使用示例

### 1. 查询指定交易对的持仓历史

```go
positionSvc := binance.NewPositionService(client)

histories, err := positionSvc.GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{
    TradingPairs: []exchange.TradingPair{
        {Base: "BTC", Quote: "USDT"},
    },
    StartTime: time.Now().AddDate(0, 0, -7), // 7天前
    EndTime:   time.Now(),
})

for _, history := range histories {
    fmt.Printf("持仓方向: %s\n", history.PositionSide)
    fmt.Printf("开仓价格: %s\n", history.EntryPrice)
    fmt.Printf("平仓价格: %s\n", history.ClosePrice)
    fmt.Printf("事件数量: %d\n", len(history.Events))
    
    // 查看每个事件
    for _, event := range history.Events {
        fmt.Printf("  %s: %s @ %s, 持仓 %s -> %s\n",
            event.EventType,
            event.Quantity,
            event.Price,
            event.BeforeQuantity,
            event.AfterQuantity,
        )
    }
}
```

### 2. 查询所有交易对的持仓历史

```go
// TradingPairs 为空数组时，查询所有交易对
histories, err := positionSvc.GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{
    TradingPairs: []exchange.TradingPair{}, // 空数组 = 查询所有
    StartTime:    time.Now().AddDate(0, 0, -7),
    EndTime:      time.Now(),
})

// 按交易对分组统计
pairStats := make(map[string]int)
for _, history := range histories {
    key := history.TradingPair.ToString()
    pairStats[key]++
}

fmt.Println("各交易对的持仓数量:")
for pair, count := range pairStats {
    fmt.Printf("  %s: %d 个\n", pair, count)
}
```

### 3. 分析持仓盈亏

```go
for _, history := range histories {
    totalPnl := decimal.Zero
    totalFee := decimal.Zero
    
    for _, event := range history.Events {
        totalPnl = totalPnl.Add(event.RealizedPnl)
        totalFee = totalFee.Add(event.Fee)
    }
    
    netPnl := totalPnl.Sub(totalFee)
    
    fmt.Printf("总盈亏: %s\n", totalPnl)
    fmt.Printf("手续费: %s\n", totalFee)
    fmt.Printf("净盈亏: %s\n", netPnl)
}
```

### 4. 追踪持仓生命周期

```go
history := histories[0]

fmt.Printf("持仓周期: %s -> %s\n", 
    history.OpenedAt.Format("2006-01-02 15:04:05"),
    history.ClosedAt.Format("2006-01-02 15:04:05"))

fmt.Printf("持仓时长: %s\n", history.ClosedAt.Sub(history.OpenedAt))

fmt.Printf("生命周期:\n")
for i, event := range history.Events {
    fmt.Printf("%d. [%s] %s %s @ %s, 持仓量: %s\n",
        i+1,
        event.CreatedAt.Format("15:04:05"),
        event.EventType,
        event.Quantity,
        event.Price,
        event.AfterQuantity,
    )
}
```

## 自动分页和时间分片

**重要特性**：该实现已经**自动隐藏**了币安 API 的限制，您无需关心分页逻辑！

### 自动处理的限制

1. **7天限制** ✅ 已处理
   - 币安限制：单次查询最多7天
   - 自动方案：自动分片为多个7天的请求
   - 示例：查询30天会自动拆分为5个请求

2. **1000条限制** ✅ 已处理
   - 币安限制：单次最多返回1000条记录
   - 自动方案：自动使用 `FromID` 分页
   - 示例：有3000条记录会自动分3次查询

### 工作原理

```
查询30天，有5000条记录：

1. 时间分片：30天 → 按7天拆分
   Day 1-7   → Request 1
   Day 8-14  → Request 2  
   Day 15-21 → Request 3
   Day 22-28 → Request 4
   Day 29-30 → Request 5

2. 每个时间片内分页（如果数据 > 1000）：
   Request 1: 
     - Page 1: records 1-1000
     - Page 2: records 1001-2000
     - Page 3: records 2001-2500
   
3. 合并所有结果 → 返回完整数据
```

### 使用示例

```go
// 查询任意时间范围，自动处理分页和分片
histories, err := positionSvc.GetHistoryPositions(ctx, exchange.GetHistoryPositionsReq{
    TradingPairs: []exchange.TradingPair{{Base: "BTC", Quote: "USDT"}},
    StartTime:    time.Now().AddDate(0, 0, -30), // 30天前
    EndTime:      time.Now(),
})

// 无需关心：
// - 是否超过7天限制 ✅ 自动分片
// - 是否超过1000条 ✅ 自动分页
// - FromID 如何使用 ✅ 自动处理
```

## 限制和注意事项

### 1. **推导准确性**
- 依赖于成交记录的完整性
- 如果成交记录不完整，推导结果可能不准确
- 建议配合实时记录使用（事件驱动）

### 2. **性能考虑**
- 查询时间越长，API 请求次数越多
- 示例：30天且有5000条记录可能需要 5-10 个 API 请求
- 频繁查询可能触发 API 限流
- 建议添加缓存或本地存储

### 3. **API 限流**
- 币安有请求频率限制
- 建议在查询大量数据时添加适当延迟
- 生产环境建议实现本地存储方案

## 运行测试

```bash
# 测试查询指定交易对的历史
go test -v ./internal/service/exchange/binance/integration -run TestGetHistoryPositions

# 测试查询所有交易对的历史（空数组）
go test -v ./internal/service/exchange/binance/integration -run TestGetAllPositionHistories

# 测试最近1天的数据
go test -v ./internal/service/exchange/binance/integration -run TestGetRecentPositionHistory

# 测试完整生命周期（会创建真实订单）
go test -v ./internal/service/exchange/binance/integration -run TestPositionLifecycle

# 测试自动分页功能
go test -v ./internal/service/exchange/binance/integration -run TestFetchAllTradesWithPagination

# 测试跨多天查询（超过7天限制）
go test -v ./internal/service/exchange/binance/integration -run TestFetchTradesAcrossMultipleDays

# 测试性能（不同时间范围）
go test -v ./internal/service/exchange/binance/integration -run TestPaginationPerformance

# 测试边界情况
go test -v ./internal/service/exchange/binance/integration -run TestEdgeCases
```

## 数据结构

### PositionHistory

```go
type PositionHistory struct {
    TradingPair  TradingPair     // 交易对
    PositionSide PositionSide    // 方向（LONG/SHORT）
    EntryPrice   decimal.Decimal // 平均开仓价
    ClosePrice   decimal.Decimal // 平均平仓价
    MaxQuantity  decimal.Decimal // 最大持仓量
    OpenedAt     time.Time       // 开仓时间
    ClosedAt     time.Time       // 平仓时间
    Events       []PositionEvent // 事件列表
}
```

### PositionEvent

```go
type PositionEvent struct {
    OrderId        OrderId           // 订单ID
    EventType      PositionEventType // 事件类型
    Quantity       decimal.Decimal   // 变动数量
    BeforeQuantity decimal.Decimal   // 变动前持仓
    AfterQuantity  decimal.Decimal   // 变动后持仓
    Price          decimal.Decimal   // 成交价格
    RealizedPnl    decimal.Decimal   // 已实现盈亏
    Fee            decimal.Decimal   // 手续费
    CreatedAt      time.Time         // 时间
}
```

## 未来优化方向

### 短期优化
1. **分页支持** - 处理超过1000条的成交记录
2. **缓存机制** - 避免重复查询相同时间段
3. **并发查询** - 同时查询多个交易对

### 长期优化
1. **本地存储** - 突破7天限制
2. **事件驱动** - 实时记录持仓变动
3. **统计分析** - 添加更多统计指标（ROI、胜率等）
4. **可视化** - 生成持仓生命周期图表

## 示例输出

```
=== 持仓历史 ===
持仓 1: LONG
  开仓时间: 2025-10-28 10:23:15
  平仓时间: 2025-10-28 10:35:42
  持仓时长: 12m27s
  平均开仓价: 95823.50
  平均平仓价: 95956.20
  最大持仓量: 0.003
  
  事件列表:
    1. CREATE: 0.002 @ 95800.00, 0 -> 0.002
    2. INCREASE: 0.001 @ 95870.00, 0.002 -> 0.003
    3. DECREASE: 0.001 @ 95950.00, 0.003 -> 0.002
    4. CLOSE: 0.002 @ 95960.00, 0.002 -> 0
  
  总盈亏: 125.30 USDT
  手续费: 15.20 USDT
  净盈亏: 110.10 USDT
```

## 常见问题

### Q: 为什么查询不到历史数据？
A: 检查以下几点：
1. 时间范围是否在7天内
2. 账户是否有实际成交记录
3. 交易对是否正确

### Q: 事件数量比预期少？
A: 可能原因：
1. 某些小额成交被合并了
2. 查询时间范围太小
3. 实际成交次数就是这么多

### Q: 盈亏计算不准确？
A: 注意：
1. `RealizedPnl` 来自币安API，已经是准确的
2. 需要减去手续费才是净盈亏
3. 确认手续费的资产类型（USDT）

### Q: 如何处理超过1000条的成交记录？
A: 当前版本暂不支持，后续会添加分页功能。临时方案：
1. 缩短查询时间范围
2. 分多次查询不同时间段
3. 实现本地存储方案


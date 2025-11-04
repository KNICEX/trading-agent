# 回测订单系统 - 挂单与止盈止损机制

## 概述

回测系统现在支持**真实的挂单机制**和**止盈止损订单**，通过K线驱动实现订单成交和止盈止损触发。

## 核心特性

### ✅ 挂单机制
- 创建订单后进入 `pending` 状态（不再立即成交）
- 每次K线推送时扫描所有待成交订单
- 根据K线的高低价判断订单是否成交

### ✅ 止盈止损
- 独立管理止盈止损订单
- K线推送时自动检查触发条件
- 触发后自动平仓

### ✅ K线驱动
- 所有订单操作都由K线推送驱动
- 更真实地模拟实际交易过程

## 订单状态

### 订单状态类型
```go
const (
    OrderStatusPending         OrderStatus = "pending"          // 挂单中
    OrderStatusFilled          OrderStatus = "filled"           // 已成交
    OrderStatusPartiallyFilled OrderStatus = "partially_filled" // 部分成交（暂未实现）
    OrderStatusCancelled       OrderStatus = "cancelled"        // 已取消
)
```

### 订单分类
```go
type BinanceExchangeService struct {
    orders        map[OrderId]*OrderInfo        // 所有订单（完整历史）
    pendingOrders map[OrderId]*OrderInfo        // 待成交订单（挂单）
    stopOrders    map[OrderId]*StopOrderInfo    // 止盈止损订单
}
```

## 工作流程

### 1. 创建挂单

```go
// 创建限价买单
orderId, err := backtestSvc.CreateOrder(ctx, exchange.CreateOrderReq{
    TradingPair: btcPair,
    OrderType:   exchange.OrderTypeOpen,
    PositonSide: exchange.PositionSideLong,
    Price:       decimal.NewFromInt(48000), // 限价
    Quantity:    decimal.NewFromFloat(0.1),
})
// 订单创建后进入 pending 状态，等待K线触发成交
```

### 2. K线推送与订单扫描

```go
// 每次K线推送时的处理流程：
for kline := range klineChan {
    // 1. 更新当前价格
    updatePrice(kline.Close)
    
    // 2. 扫描待成交订单
    scanPendingOrders(kline)
    
    // 3. 扫描止盈止损订单
    scanStopOrders(kline)
    
    // 4. 推送K线给策略
    strategy.OnKline(kline)
}
```

### 3. 订单成交判断

#### 限价买单
```
成交条件：K线最低价 <= 限价
成交价格：限价
```

#### 限价卖单
```
成交条件：K线最高价 >= 限价
成交价格：限价
```

#### 市价单
```
成交条件：立即成交（下一个K线）
成交价格：K线收盘价
```

### 4. 止盈止损触发

#### 多头持仓
```
止盈触发：K线最高价 >= 止盈价
止损触发：K线最低价 <= 止损价
```

#### 空头持仓
```
止盈触发：K线最低价 <= 止盈价
止损触发：K线最高价 >= 止损价
```

## 使用示例

### 示例1：限价单开仓

```go
// 创建限价买单，等待价格回落到 48000 时买入
orderId, err := backtestSvc.CreateOrder(ctx, exchange.CreateOrderReq{
    TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    OrderType:   exchange.OrderTypeOpen,
    PositonSide: exchange.PositionSideLong,
    Price:       decimal.NewFromInt(48000),
    Quantity:    decimal.NewFromFloat(0.1),
})

// 订单状态：pending
// 当K线 Low <= 48000 时，订单成交
// 成交后状态：filled
```

### 示例2：市价单开仓

```go
// 创建市价买单，下一个K线立即成交
orderId, err := backtestSvc.CreateOrder(ctx, exchange.CreateOrderReq{
    TradingPair: btcPair,
    OrderType:   exchange.OrderTypeOpen,
    PositonSide: exchange.PositionSideLong,
    Price:       decimal.Zero, // 市价单
    Quantity:    decimal.NewFromFloat(0.1),
})

// 下一个K线推送时立即成交，使用K线收盘价
```

### 示例3：开仓并设置止盈止损

```go
// 开多仓，同时设置止盈止损
resp, err := backtestSvc.OpenPosition(ctx, exchange.OpenPositionReq{
    TradingPair:  btcPair,
    PositionSide: exchange.PositionSideLong,
    Quantity:     decimal.NewFromFloat(0.1),
    
    // 止盈：价格涨到 55000 时自动平仓
    TakeProfit: exchange.StopOrder{
        Price: decimal.NewFromInt(55000),
    },
    
    // 止损：价格跌到 45000 时自动平仓
    StopLoss: exchange.StopOrder{
        Price: decimal.NewFromInt(45000),
    },
})

fmt.Printf("开仓订单ID: %s\n", resp.OrderId)
fmt.Printf("止盈订单ID: %s\n", resp.TakeProfitId)
fmt.Printf("止损订单ID: %s\n", resp.StopLossId)
```

### 示例4：查询和取消挂单

```go
// 查询所有待成交订单
pendingOrders, err := backtestSvc.GetOrders(ctx, exchange.GetOrdersReq{
    TradingPair: btcPair,
})

for _, order := range pendingOrders {
    fmt.Printf("订单 %s: 价格=%s 数量=%s 状态=%s\n",
        order.Id, order.Price, order.Quantity, order.Status)
}

// 取消指定订单
err = backtestSvc.CancelOrder(ctx, exchange.CancelOrderReq{
    Id:          orderId,
    TradingPair: btcPair,
})

// 取消所有待成交订单
err = backtestSvc.CancelOrders(ctx, exchange.CancelOrdersReq{
    TradingPair: btcPair,
})
```

## 时序图

### 限价单成交流程

```
时间  K线1        K线2        K线3        K线4
     (50000)     (49000)     (47000)     (48500)
       |           |           |           |
创建订单 ---+         |           |           |
限价48000  |         |           |           |
pending    |         |           |           |
           |         |           |           |
扫描订单 <----------扫描------扫描------扫描
           |         |          ✓          |
           |         |      Low=47000      |
           |         |      触发成交       |
           |         |      状态=filled    |
           |         |      开仓完成       |
```

### 止盈止损流程

```
时间  开仓         K线1        K线2        K线3
     (50000)     (52000)     (54000)     (55500)
       |           |           |           |
开多仓 ---+         |           |           |
入场价50000         |           |           |
止盈55000-----------+           |           |
止损45000           |           |           |
                    |           |           |
扫描止盈止损 <------扫描------扫描------扫描
                    |           |          ✓
                    |           |      High=55500
                    |           |      触发止盈
                    |           |      ⚡立即平仓（不创建挂单）
                    |           |      按触发价成交
```

## 数据结构

### OrderInfo（扩展版）

```go
type OrderInfo struct {
    exchange.OrderInfo  // 基础订单信息
    
    OrderType    exchange.OrderType    // OPEN / CLOSE
    PositionSide exchange.PositionSide // LONG / SHORT
}
```

### StopOrderInfo（止盈止损）

```go
type StopOrderInfo struct {
    Id           exchange.OrderId
    TradingPair  exchange.TradingPair
    PositionSide exchange.PositionSide
    
    Type         exchange.OrderSide    // BUY=止损（空头）, SELL=止盈（多头）
    TriggerPrice decimal.Decimal       // 触发价格
    Quantity     decimal.Decimal       // 成交数量（0=全平）
    
    PositionKey  string                // 关联的持仓
}
```

## 实现细节

### 订单扫描优化

- 使用读写锁避免阻塞
- 只扫描当前K线对应的交易对
- 复制订单列表后释放锁，避免长时间占用

### 止盈止损立即执行

止盈止损触发后**不会创建挂单**，而是立即执行平仓：

```go
// 触发止盈止损时的流程
func triggerStopOrder() {
    // 1. 从止盈止损列表移除
    delete(stopOrders, stopOrderId)
    
    // 2. 直接调用内部平仓方法（不走CreateOrder）
    closePosition(posKey, order, triggerPrice)
    
    // 3. 立即成交，按触发价格
    // ✅ 不会进入pending状态
    // ✅ 不会等待下一个K线
}
```

这样可以：
- ✅ 避免止盈止损挂单再次被扫描
- ✅ 确保触发后立即执行
- ✅ 更符合实际止盈止损的行为

```go
func (svc *BinanceExchangeService) scanPendingOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
    svc.orderMu.RLock()
    // 复制待扫描的订单列表
    pendingList := make([]*OrderInfo, 0)
    for _, order := range svc.pendingOrders {
        if order.OrderInfo.TradingPair == tradingPair {
            pendingList = append(pendingList, order)
        }
    }
    svc.orderMu.RUnlock()
    
    // 释放锁后再处理订单
    for _, order := range pendingList {
        if svc.checkOrderFilled(order, kline) {
            svc.fillOrder(ctx, order, kline)
        }
    }
}
```

### 并发安全

所有订单操作都使用互斥锁保护：
- `orderMu` - 保护订单相关数据
- `positionMu` - 保护持仓数据
- `accountMu` - 保护账户数据
- `priceMu` - 保护价格数据

## 注意事项

### ⚠️ 重要提示

1. **K线精度限制**
   - 成交判断基于K线的高低价
   - 实际成交可能在K线周期内的任意时刻
   - 无法模拟盘口深度和tick级别数据

2. **订单成交顺序**
   - 同一K线触发多个订单时，按扫描顺序成交
   - 不保证与实际市场的成交顺序一致

3. **止盈止损机制**
   - ✅ 触发后**立即执行平仓**，不创建挂单
   - ✅ 按触发价格直接成交
   - ✅ 避免挂单延迟导致的风险
   - ❌ 未考虑滑点和流动性影响

4. **部分成交**
   - 当前版本暂不支持部分成交
   - 订单要么完全成交，要么保持挂单状态

### 💡 最佳实践

1. **合理设置限价**
   - 限价过于激进可能永远无法成交
   - 建议结合K线数据设置合理的限价区间

2. **止盈止损设置**
   - 止盈止损价格应考虑市场波动
   - 避免设置过近的止损价（容易被误触发）

3. **订单管理**
   - 定期检查和清理过期的挂单
   - 避免创建过多的挂单影响性能

## 文件结构

```
backtest/
├── exchange.go      - 核心服务、K线扫描机制
├── order_types.go   - 订单类型定义
├── order.go         - 订单管理（创建、查询、取消）
├── position.go      - 持仓管理
├── account.go       - 账户管理
├── trading.go       - 交易服务（开平仓、止盈止损）
├── types.go         - 接口定义
├── README.md        - 总体介绍
└── ORDER_SYSTEM.md  - 本文档
```

## 与实时交易的差异

| 特性 | 回测 | 实时交易 |
|------|------|----------|
| 订单成交 | K线驱动 | Tick级别 |
| 成交判断 | 高低价触及 | 盘口匹配 |
| 滑点 | 无 | 有 |
| 流动性 | 无限 | 有限 |
| 部分成交 | 不支持 | 支持 |
| 手续费 | 暂无 | 有 |

## 性能考虑

- 每个K线推送触发一次完整扫描
- 订单数量多时可能影响性能
- 建议控制单个交易对的挂单数量在 100 以内

## 未来改进

- [ ] 支持部分成交
- [ ] 添加手续费计算
- [ ] 模拟滑点
- [ ] 支持冰山订单
- [ ] 添加订单优先级
- [ ] 性能优化（订单索引）


# Exchange 服务分层设计说明

## 架构概览

```
┌─────────────────────────────────────────────┐
│  接口层 (exchange 包)                        │
│  -----------------------------------------  │
│  · TradingService  (高层交易逻辑)            │
│  · OrderService    (订单管理)               │
│  · PositionService (持仓管理)               │
│  · AccountService  (账户信息)               │
│  · MarketService   (行情数据)               │
│  -----------------------------------------  │
│  只定义业务概念，不涉及交易所实现细节         │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│  实现层 (binance 包)                         │
│  -----------------------------------------  │
│  · TradingService  (币安交易逻辑)            │
│  · OrderService    (币安订单API)             │
│  · PositionService (币安持仓API)             │
│  · AccountService  (币安账户API)             │
│  · MarketService   (币安行情API)             │
│  -----------------------------------------  │
│  处理所有币安特定的技术细节和API调用         │
└─────────────────────────────────────────────┘
```

## 核心设计原则

### 1. 接口层只定义业务概念

**OrderType 只有两种：**
```go
const (
    OrderTypeOpen  OrderType = "OPEN"   // 开仓
    OrderTypeClose OrderType = "CLOSE"  // 平仓
)
```

**不涉及技术细节：**
- ❌ 不定义 LIMIT、MARKET、TAKE_PROFIT 等
- ❌ 不定义 BUY、SELL（自动推导）
- ❌ 不定义具体的止盈止损类型

### 2. 实现层处理所有技术细节

**自动推导逻辑：**

#### OrderSide（买卖方向）
```go
func calculateOrderSide(orderType OrderType, positionSide PositionSide) OrderSide {
    switch orderType {
    case OrderTypeOpen:
        // 开仓：LONG 买入，SHORT 卖出
        if positionSide == PositionSideLong {
            return OrderSideBuy
        }
        return OrderSideSell
    
    case OrderTypeClose:
        // 平仓：LONG 卖出，SHORT 买入
        if positionSide == PositionSideLong {
            return OrderSideSell
        }
        return OrderSideBuy
    }
}
```

#### 币安 OrderType（订单类型）
```go
func binanceOrderType(orderType OrderType, price Decimal) futures.OrderType {
    isMarket := price.IsZero()
    
    switch orderType {
    case OrderTypeOpen:
        if isMarket {
            return futures.OrderTypeMarket
        }
        return futures.OrderTypeLimit
    
    case OrderTypeClose:
        if isMarket {
            return futures.OrderTypeMarket
        }
        return futures.OrderTypeLimit
    }
}
```

#### 止盈止损订单
```go
// 止盈：自动使用 TAKE_PROFIT_MARKET
func createTakeProfitOrder(...) {
    service.Type(futures.OrderTypeTakeProfitMarket)
    service.StopPrice(triggerPrice)
}

// 止损：自动使用 STOP_MARKET
func createStopLossOrder(...) {
    service.Type(futures.OrderTypeStopMarket)
    service.StopPrice(triggerPrice)
}
```

## 用户视角

### 开仓示例

```go
// 市价开多仓，使用30%余额，带止盈止损
tradingSvc.OpenPosition(ctx, OpenPositionReq{
    TradingPair:    pair,
    PositionSide:   PositionSideLong,      // 多仓
    BalancePercent: decimal.NewFromInt(30),
    TakeProfit:     StopOrder{Price: decimal.NewFromInt(50000)},
    StopLoss:       StopOrder{Price: decimal.NewFromInt(45000)},
})

// 限价开空仓，指定数量
tradingSvc.OpenPosition(ctx, OpenPositionReq{
    TradingPair:  pair,
    PositionSide: PositionSideShort,      // 空仓
    Price:        decimal.NewFromInt(48000), // 有价格 = 限价
    Quantity:     decimal.NewFromFloat(0.01),
})
```

### 平仓示例

```go
// 市价平掉50%多仓
tradingSvc.ClosePosition(ctx, ClosePositionReq{
    TradingPair:  pair,
    PositionSide: PositionSideLong,
    Percent:      decimal.NewFromInt(50),
})

// 限价全部平空仓
tradingSvc.ClosePosition(ctx, ClosePositionReq{
    TradingPair:  pair,
    PositionSide: PositionSideShort,
    Price:        decimal.NewFromInt(49000), // 有价格 = 限价
    CloseAll:     true,
})
```

## CreateOrderReq 简化对比

### 优化前
```go
CreateOrderReq{
    TradingPair: pair,
    Side:        OrderSideBuy,              // 需要手动指定
    OrderType:   OrderTypeLimit,            // 需要手动指定
    PositonSide: PositionSideLong,
    Price:       decimal.NewFromInt(48000),
    Quantity:    decimal.NewFromFloat(0.01),
}
```

### 优化后
```go
CreateOrderReq{
    TradingPair: pair,
    OrderType:   OrderTypeOpen,             // 只需指定开仓
    PositonSide: PositionSideLong,          // 只需指定方向
    Price:       decimal.NewFromInt(48000), // 有价格 = 限价
    Quantity:    decimal.NewFromFloat(0.01),
}
// Side 自动推导：OPEN + LONG = BUY
// 币安类型自动转换：有价格 = LIMIT
```

## 设计优势

### 1. 接口极简
- OrderType 只有 OPEN/CLOSE
- 不需要指定 Side（自动推导）
- 不需要指定具体订单类型（自动转换）

### 2. 关注点分离
- 接口层：业务逻辑（开仓、平仓、止盈止损）
- 实现层：技术细节（买卖方向、订单类型、API调用）

### 3. 易于扩展
- 新增交易所时，实现自己的转换逻辑
- 接口层保持稳定，不需要修改

### 4. 降低学习成本
用户只需理解：
- OPEN（开仓）vs CLOSE（平仓）
- LONG（多头）vs SHORT（空头）
- 有价格 = 限价，无价格 = 市价

不需要学习：
- BUY/SELL 的组合逻辑
- LIMIT/MARKET 的使用场景
- TAKE_PROFIT_MARKET/STOP_MARKET 的区别

## 自动推导规则

| OrderType | PositionSide | → Side | → 币安类型 (有价格) | → 币安类型 (无价格) |
|-----------|-------------|--------|-------------------|-------------------|
| OPEN      | LONG        | BUY    | LIMIT             | MARKET            |
| OPEN      | SHORT       | SELL   | LIMIT             | MARKET            |
| CLOSE     | LONG        | SELL   | LIMIT             | MARKET            |
| CLOSE     | SHORT       | BUY    | LIMIT             | MARKET            |

**止盈止损：**
- TakeProfit → TAKE_PROFIT_MARKET
- StopLoss → STOP_MARKET

## 实现细节处理

### 1. 数量精度自动处理
```go
// BTC: 3位小数 (0.001)
// ETH: 3位小数 (0.001)
// SOL: 1位小数 (0.1)
// ...

roundQuantity(pair, quantity) // 自动截断到正确精度
```

### 2. 余额计算
```go
// 使用 MaxWithdrawAmount（真正可用余额）
// 自动扣除挂单锁定的保证金
AvailableBalance: MaxWithdrawAmount
```

### 3. 杠杆倍数
```go
// 自动从现有仓位获取杠杆
// 没有仓位时默认使用 20x
getCurrentLeverage(ctx, pair)
```

## 总结

这个设计实现了**完美的关注点分离**：
- 用户只需关心**业务逻辑**（开仓/平仓）
- 系统自动处理**技术细节**（买卖方向、订单类型、精度处理等）

这就是优雅的 API 设计！


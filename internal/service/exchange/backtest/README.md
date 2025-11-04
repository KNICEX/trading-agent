# 回测交易所服务实现

## 概述

这是一个用于回测的交易所服务模拟实现。它实现了完整的 `exchange.Service` 接口，但不会真正调用交易所API进行下单，而是在内存中模拟订单、持仓和账户状态。

## 特性

### ✅ 真实API调用
- `GetKlines()` - 调用币安真实API获取历史K线数据
- `SubscribeKline()` - 通过时钟模拟推送K线数据（用于回测时间加速）

### 🎯 本地模拟
- **订单管理** - 在内存中模拟订单创建和成交
- **持仓管理** - 在内存中维护持仓状态
- **账户管理** - 在内存中跟踪账户余额和盈亏
- **交易执行** - 模拟开仓、平仓、加仓、减仓操作

## 核心设计

### 1. 历史价格驱动
回测使用**历史K线数据**作为价格来源：
- ✅ K线推送时自动更新当前价格（使用K线收盘价）
- ✅ 市价单按当前K线收盘价成交
- ✅ 限价单按指定价格成交
- ❌ 不调用实时API获取价格

价格更新流程：
```
历史K线数据 → SubscribeKline → 更新currentPrices → 订单扫描 → 成交判断
```

### 2. 挂单机制（K线驱动）
✨ **新特性**：支持真实的挂单功能
- ✅ 创建订单后进入 `pending` 状态（不再立即成交）
- ✅ 每次K线推送时自动扫描待成交订单
- ✅ 根据K线高低价判断是否触及限价
- ✅ 支持取消挂单
- ✅ 限价买单：当K线Low <= 限价时成交
- ✅ 限价卖单：当K线High >= 限价时成交
- ✅ 市价单：下一个K线立即成交

### 3. 止盈止损
✨ **新特性**：完整的止盈止损支持
- ✅ 开仓时可设置止盈止损价格
- ✅ K线推送时自动检查触发条件
- ✅ 触发后自动平仓
- ✅ 多头止盈：价格 >= 止盈价时卖出
- ✅ 多头止损：价格 <= 止损价时卖出
- ✅ 空头止盈：价格 <= 止盈价时买入
- ✅ 空头止损：价格 >= 止损价时买入

### 4. 时钟系统
内置时钟系统支持时间加速：
```go
// 创建服务时设置时间倍速
svc := NewBinanceExchangeService(
    client, 
    startTime,   // 回测开始时间
    endTime,     // 回测结束时间
    100,         // 时间倍速：100倍速
    decimal.NewFromInt(10000), // 初始资金
)
```

### 5. 资金管理
- 初始资金在创建服务时设置
- 自动计算和更新账户余额
- 跟踪已用保证金和可用余额
- 计算盈亏并实时更新账户

## 快速开始

### 📚 完整文档

- [README.md](README.md) - 本文档（总体介绍）
- [ORDER_SYSTEM.md](ORDER_SYSTEM.md) - **挂单与止盈止损详细文档**

## 使用示例

### 基础设置

```go
import (
    "github.com/KNICEX/trading-agent/internal/service/exchange/backtest"
    "github.com/adshao/go-binance/v2/futures"
    "github.com/shopspring/decimal"
)

// 创建币安客户端
client := futures.NewClient("api_key", "secret_key")

// 设置回测时间范围
startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
endTime := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

// 创建回测交易所服务
// 参数：客户端、开始时间、结束时间、时间倍速、初始资金
backtestSvc := backtest.NewBinanceExchangeService(
    client,
    startTime,
    endTime,
    100, // 100倍速执行回测
    decimal.NewFromInt(10000), // 初始资金 10000 USDT
)
```

### 挂单开仓

```go
// 方式1：直接创建挂单（更灵活）
orderId, err := backtestSvc.CreateOrder(ctx, exchange.CreateOrderReq{
    TradingPair: exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    OrderType:   exchange.OrderTypeOpen,
    PositonSide: exchange.PositionSideLong,
    Price:       decimal.NewFromInt(48000), // 限价48000，等待价格回落
    Quantity:    decimal.NewFromFloat(0.1),
})
// 订单创建后进入 pending 状态
// 当K线Low <= 48000时自动成交

// 方式2：使用OpenPosition（支持止盈止损）
resp, err := backtestSvc.OpenPosition(ctx, exchange.OpenPositionReq{
    TradingPair:  exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    PositionSide: exchange.PositionSideLong,
    Quantity:     decimal.NewFromFloat(0.1),
    
    // 开仓价格（为空则市价）
    Price: decimal.NewFromInt(50000),
    
    // 止盈：价格涨到55000时自动平仓
    TakeProfit: exchange.StopOrder{
        Price: decimal.NewFromInt(55000),
    },
    
    // 止损：价格跌到45000时自动平仓
    StopLoss: exchange.StopOrder{
        Price: decimal.NewFromInt(45000),
    },
})
```

### 平仓交易

```go
// 平仓 50%
orderId, err := backtestSvc.ClosePosition(ctx, exchange.ClosePositionReq{
    TradingPair:  exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    PositionSide: exchange.PositionSideLong,
    Percent:      decimal.NewFromInt(50), // 平仓 50%
})

// 完全平仓
orderId, err := backtestSvc.ClosePosition(ctx, exchange.ClosePositionReq{
    TradingPair:  exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    PositionSide: exchange.PositionSideLong,
    CloseAll:     true, // 全部平仓
})
```

### 查询订单和持仓

```go
// 查询所有待成交订单（挂单）
pendingOrders, err := backtestSvc.GetOrders(ctx, exchange.GetOrdersReq{
    TradingPair: btcPair,
})

// 查询指定订单
order, err := backtestSvc.GetOrder(ctx, exchange.GetOrderReq{
    Id:          orderId,
    TradingPair: btcPair,
})

// 取消挂单
err = backtestSvc.CancelOrder(ctx, exchange.CancelOrderReq{
    Id:          orderId,
    TradingPair: btcPair,
})

// 获取所有活跃持仓
positions, err := backtestSvc.GetActivePositions(ctx, []exchange.TradingPair{})

// 获取指定交易对的持仓
btcPair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}
positions, err := backtestSvc.GetActivePositions(ctx, []exchange.TradingPair{btcPair})
```

### 查询账户

```go
// 获取账户信息
account, err := backtestSvc.GetAccountInfo(ctx)
fmt.Printf("总余额: %s\n", account.TotalBalance)
fmt.Printf("可用余额: %s\n", account.AvailableBalance)
fmt.Printf("未实现盈亏: %s\n", account.UnrealizedPnl)
fmt.Printf("已用保证金: %s\n", account.UsedMargin)
```

### K线订阅

```go
// 订阅K线推送（回测模式会根据时钟模拟推送）
klineChan, err := backtestSvc.SubscribeKline(
    ctx, 
    exchange.TradingPair{Base: "BTC", Quote: "USDT"},
    exchange.Interval5m,
)

// 接收K线数据
for kline := range klineChan {
    fmt.Printf("收到K线: 时间=%s 收盘价=%s\n", 
        kline.CloseTime, kline.Close)
    
    // 在这里执行策略逻辑
}
```

## 与 engine.BacktestEngine 集成

回测引擎会自动使用这个服务：

```go
// 创建回测引擎
engine := engine.NewBacktestEngine(startTime, endTime, backtestSvc)

// 添加策略
engine.AddStrategy(ctx, myStrategy)

// 运行回测
err := engine.Run(ctx)
```

## 工作流程

```
历史K线API
   ↓
SubscribeKline (时钟驱动)
   ↓
更新当前价格 (K线收盘价)
   ↓
🔑 扫描待成交订单 (检查限价是否触及)
   ↓
🔑 扫描止盈止损订单 (检查是否触发)
   ↓
推送K线到策略
   ↓
策略生成信号
   ↓
CreateOrder (创建挂单)
   ↓
订单进入pending状态 (等待下一个K线扫描)
   ↓
(K线触发成交)
   ↓
更新持仓 (内存)
   ↓
更新账户 (内存)
   ↓
计算盈亏
```

## 实现细节

### 文件结构
```
backtest/
├── exchange.go   - 核心服务和市场数据
├── order.go      - 订单管理实现
├── position.go   - 持仓管理实现
├── account.go    - 账户管理实现
├── trading.go    - 交易服务实现
├── types.go      - 接口定义
└── README.md     - 本文档
```

### 数据结构

系统在内存中维护以下状态：

```go
type BinanceExchangeService struct {
    // 订单管理
    orders        map[OrderId]*OrderInfo        // 所有订单历史
    pendingOrders map[OrderId]*OrderInfo        // 待成交订单（挂单）✨新增
    stopOrders    map[OrderId]*StopOrderInfo    // 止盈止损订单 ✨新增
    
    // 持仓管理
    positions         map[string]*Position       // 当前活跃持仓 (key: pair_side)
    positionHistories []PositionHistory          // 已平仓位历史
    
    // 账户管理
    account       *AccountInfo                  // 账户信息（余额、保证金等）
    
    // 价格管理（关键！）
    currentPrices map[string]decimal.Decimal    // 当前价格 (从K线更新)
}
```

### 盈亏计算

**多头盈亏**：
```
PnL = (平仓价格 - 开仓价格) × 数量
```

**空头盈亏**：
```
PnL = (开仓价格 - 平仓价格) × 数量
```

### 保证金计算

当前实现使用 **1倍杠杆**（逐仓模式）：
```
所需保证金 = 开仓价格 × 数量 ÷ 杠杆
           = 开仓价格 × 数量 ÷ 1
           = 开仓价格 × 数量
```

### 加仓时的平均成本计算

```
新平均价格 = (原持仓成本 + 新开仓成本) ÷ 总数量
          = (原价格 × 原数量 + 新价格 × 新数量) ÷ (原数量 + 新数量)
```

## 注意事项

### ⚠️ 限制

1. **K线级别精度** - 基于K线的高低价判断成交，无法模拟tick级别
2. **固定杠杆** - 当前固定为1倍杠杆
3. **无滑点模拟** - 按限价或K线收盘价精确成交
4. **无手续费** - 暂未实现手续费计算
5. **部分成交** - 暂不支持部分成交
6. **流动性** - 假设流动性无限，订单可以完全成交

### 💡 最佳实践

1. **合理设置初始资金** - 根据策略需求设置
2. **时间倍速** - 建议设置为 50-1000 倍
3. **数据准备** - 确保回测时间范围内有足够的历史数据
4. **风险控制** - 在策略中实现仓位管理和风险控制

## TODO

- [ ] 实现手续费计算
- [ ] 添加滑点模拟
- [ ] 支持多级杠杆设置
- [ ] 完整的止盈止损实现
- [ ] 订单部分成交模拟
- [ ] 性能优化和并发安全测试


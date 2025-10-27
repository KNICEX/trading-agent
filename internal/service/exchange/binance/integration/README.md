# 币安交易集成测试

这个目录包含了币安交易服务的集成测试，涵盖订单管理和持仓管理的完整功能。

## 测试覆盖

### 1. TestCompleteTradeFlow - 完整交易流程测试
测试一个完整的订单生命周期：
- 查询当前持仓
- 创建限价买单（价格设置得很低，不会立即成交）
- 查询订单状态
- 修改订单（调整数量）
- 取消订单

### 2. TestMarketOrderClosePosition - 市价单平仓测试
测试使用市价单平仓功能：
- 获取当前持仓
- 根据持仓方向（多头/空头）确定平仓订单方向
- 创建市价单平仓
- 验证订单状态
- 确认持仓已平

**注意**: 此测试需要账户中有实际持仓才能执行，否则会跳过。

### 3. TestListAndManageOrders - 批量订单管理测试
测试批量订单操作：
- 创建多个限价单
- 查询未完成订单列表
- 批量取消订单
- 确认订单已取消

### 4. TestGetAllPositions - 获取所有持仓测试
测试获取账户所有持仓功能：
- 获取所有持仓
- 显示每个持仓的详细信息（交易对、方向、数量、价格、杠杆、保证金、盈亏等）

### 5. TestOrderHistory - 历史订单查询测试
测试查询历史订单功能：
- 查询最近24小时内的订单记录
- 显示订单详情（ID、方向、价格、数量、状态、时间等）

## 运行测试

### 运行所有集成测试
```bash
go test -v ./internal/service/exchange/binance/integration -timeout 60s
```

### 运行特定测试
```bash
# 测试完整交易流程
go test -v ./internal/service/exchange/binance/integration -run TestCompleteTradeFlow

# 测试市价单平仓
go test -v ./internal/service/exchange/binance/integration -run TestMarketOrderClosePosition

# 测试批量订单管理
go test -v ./internal/service/exchange/binance/integration -run TestListAndManageOrders

# 测试获取所有持仓
go test -v ./internal/service/exchange/binance/integration -run TestGetAllPositions

# 测试历史订单查询
go test -v ./internal/service/exchange/binance/integration -run TestOrderHistory
```

## 配置要求

测试需要配置文件 `config/config.dev.yaml`，包含币安 API 密钥：

```yaml
exchange:
  binance:
    api_key: "your_api_key"
    api_secret: "your_api_secret"
```

## 重要提示

1. **测试环境**: 这些是集成测试，会真实调用币安 API。建议在测试网或小额资金账户上运行。

2. **最小订单要求**: 币安要求订单的名义价值（价格 × 数量）至少为 100 USDT。测试中已经考虑了这个限制。

3. **限价单价格**: 测试中的限价单价格设置得很低（50000 USDT），不会立即成交，可以安全地进行订单管理测试。

4. **市价单测试**: `TestMarketOrderClosePosition` 会实际执行市价单，如果账户中有持仓，将会被平仓。请谨慎运行。

5. **API 限制**: 币安有 API 调用频率限制，如果测试运行失败，可能需要等待一段时间后重试。

## 故障排除

### 错误: "Order's notional must be no smaller than 100"
订单金额太小，不满足币安最小100 USDT的要求。调整订单数量或价格。

### 错误: "Parameter 'timeinforce' sent when not required"
市价单不应该包含 `timeInForce` 参数。这个问题已在 `order.go` 中修复。

### 测试超时
增加超时时间：`go test -v ./internal/service/exchange/binance/integration -timeout 120s`

## 代码结构

```
integration/
├── README.md              # 本文档
└── integration_test.go    # 集成测试实现
```

测试使用了以下服务构造函数：
- `binance.NewOrderService(client)` - 创建订单服务
- `binance.NewPositionService(client)` - 创建持仓服务


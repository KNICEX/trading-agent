# 币安交易集成测试

这个目录包含了币安交易服务的集成测试，专注于订单管理的核心功能。

## 重要说明

**OrderService 职责**: 
- OrderService 只负责订单的创建、修改和取消
- `GetOrders()` 只返回**未完全成交**的订单（不包括已成交或已取消的订单）
- 市价单通常会立即成交，成交后会自动从未成交列表中移除

## 测试覆盖

| 测试名称 | 主要功能 | 是否产生仓位 | 风险等级 |
|---------|---------|------------|---------|
| TestCreateAndQueryOrder | 创建、查询、取消订单 | 否 | 低 |
| TestModifyOrder | 修改订单 | 否 | 低 |
| TestBatchOrders | 批量操作 | 否 | 低 |
| TestCancelAllOrders | 取消所有订单 | 否 | 低 |
| TestMarketOrder | 市价单（开仓+平仓） | 是 | 中 |

### 1. TestCreateAndQueryOrder - 创建和查询订单测试
测试基本的订单创建和查询功能：
- 创建限价买单（价格设置得很低，不会立即成交）
- 查询单个订单状态（验证订单创建成功）
- 查询所有未成交订单（验证订单在列表中）
- 取消订单（清理测试数据）

**验证点**:
- 订单创建后应该处于活跃状态（未完全成交）
- 创建的订单应该出现在未成交订单列表中

### 2. TestModifyOrder - 修改订单测试
测试修改未成交订单的功能：
- 创建限价买单
- 修改订单价格和数量
- 查询验证修改结果
- 取消订单

**注意**: 某些交易所可能限制订单修改功能，测试会捕获并记录修改失败的情况。

### 3. TestBatchOrders - 批量订单操作测试
测试批量订单管理功能：
- 批量创建多个限价单
- 查询未成交订单列表，验证所有订单都在列表中
- 批量修改订单（价格和数量）
- 批量取消订单
- 验证订单已全部取消

**验证点**:
- 批量创建的订单数量应该与请求数量一致
- 取消后，订单应该从未成交列表中消失

### 4. TestCancelAllOrders - 取消所有订单测试
测试取消指定交易对的所有订单：
- 创建多个测试订单
- 查询未成交订单数量
- 取消该交易对的所有订单（通过传入空的订单ID列表）
- 验证订单已全部取消

**用途**: 这个功能常用于紧急情况下快速清空所有挂单。

### 5. TestMarketOrder - 市价单测试
测试市价单的特殊行为：
- 创建市价买单开仓
- 验证订单状态（市价单通常会立即成交）
- 验证未成交列表中不包含该订单（已成交订单不会出现在未成交列表中）
- 平仓（清理测试仓位）

**重要**: 
- 市价单会立即成交，成交后会从未成交订单列表中移除
- `GetOrder()` 可能仍能查询到已成交的订单（取决于交易所实现）
- `GetOrders()` 不会返回已成交的订单
- 此测试会创建实际仓位，测试结束时会自动平仓

## 运行测试

### 运行所有集成测试
```bash
go test -v ./internal/service/exchange/binance/integration -timeout 60s
```

### 运行特定测试
```bash
# 测试创建和查询订单
go test -v ./internal/service/exchange/binance/integration -run TestCreateAndQueryOrder

# 测试修改订单
go test -v ./internal/service/exchange/binance/integration -run TestModifyOrder

# 测试批量订单操作
go test -v ./internal/service/exchange/binance/integration -run TestBatchOrders

# 测试取消所有订单
go test -v ./internal/service/exchange/binance/integration -run TestCancelAllOrders

# 测试市价单（会产生实际仓位）
go test -v ./internal/service/exchange/binance/integration -run TestMarketOrder
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

4. **市价单测试**: `TestMarketOrder` 会实际执行市价单开仓和平仓操作。虽然测试会自动清理仓位，但请在小额资金账户上谨慎运行。

5. **未成交订单限制**: OrderService 的 `GetOrders()` 只返回未完全成交的订单。已成交或已取消的订单不会出现在列表中。

6. **API 限制**: 币安有 API 调用频率限制，如果测试运行失败，可能需要等待一段时间后重试。

7. **订单修改**: 某些交易所可能不支持或限制订单修改功能，`TestModifyOrder` 和 `TestBatchOrders` 中的修改操作可能会失败。

## 故障排除

### 错误: "Order's notional must be no smaller than 100"
订单金额太小，不满足币安最小100 USDT的要求。调整订单数量或价格。

### 错误: "Parameter 'timeinforce' sent when not required"
市价单不应该包含 `timeInForce` 参数。这个问题已在 `order.go` 中修复。

### 测试超时
增加超时时间：`go test -v ./internal/service/exchange/binance/integration -timeout 120s`

## 测试设计原则

1. **职责分离**: OrderService 只管理订单，不涉及历史数据查询
2. **未成交订单**: `GetOrders()` 只返回活跃的未成交订单
3. **自动清理**: 所有测试在结束时都会清理创建的订单和仓位
4. **独立性**: 每个测试都是独立的，可以单独运行
5. **真实场景**: 测试模拟真实的交易场景，包括限价单、市价单、批量操作等

## 代码结构

```
integration/
├── README.md              # 本文档
└── integration_test.go    # 集成测试实现
```

测试使用了以下服务构造函数：
- `binance.NewOrderService(client)` - 创建订单服务
- `binance.NewPositionService(client)` - 创建持仓服务（用于 TestMarketOrder 中的平仓操作）

## 快速开始

```bash
# 1. 确保配置文件存在
cat config/config.dev.yaml

# 2. 运行安全的测试（不产生仓位）
go test -v ./internal/service/exchange/binance/integration \
  -run "TestCreateAndQueryOrder|TestModifyOrder|TestBatchOrders|TestCancelAllOrders" \
  -timeout 60s

# 3. 如果需要测试市价单（会产生仓位）
go test -v ./internal/service/exchange/binance/integration -run TestMarketOrder
```

## 核心概念验证

通过这些测试，我们验证了以下核心概念：

1. **未成交订单管理**: 
   - 创建的限价单会出现在 `GetOrders()` 列表中
   - 取消订单后会从列表中移除
   - 市价单成交后不会出现在未成交列表中

2. **订单生命周期**:
   ```
   创建 → 未成交列表 → 修改（可选）→ 取消/成交 → 从列表移除
   ```

3. **批量操作效率**:
   - 批量创建订单比单个创建更高效
   - 批量取消订单适用于风控场景

4. **市价单特性**:
   - 市价单立即成交
   - 成交后订单不在未成交列表中
   - 需要通过仓位服务管理后续平仓


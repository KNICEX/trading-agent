# 币安交易集成测试套件

## 概述

本目录包含基于 `testify/suite` 重构的完整集成测试套件，采用模块化设计，测试覆盖全面，易于维护和扩展。

## 测试套件架构

```
integration/
├── suite_base.go                      # 基础测试套件（公共设施）
├── order_suite_test.go                # 订单服务测试套件
├── trading_suite_test.go              # 交易服务测试套件
├── account_suite_test.go              # 账户服务测试套件
├── position_history_suite_test.go     # 持仓历史测试套件
└── README_SUITES.md                   # 本文档
```

### 架构优势

1. **统一的测试基础设施** - 所有测试套件继承自 `BaseSuite`
2. **自动化环境管理** - 每个测试前后自动清理环境
3. **丰富的辅助方法** - 提供大量测试辅助函数
4. **清晰的测试流程** - 测试步骤编号，日志输出友好
5. **风险等级标注** - 明确标注每个测试的风险等级

## 测试套件详情

### 1. OrderServiceSuite - 订单服务测试套件

**测试文件**: `order_suite_test.go`  
**风险等级**: 低（大部分使用限价单，不会实际成交）  
**测试数量**: 7 个

| 测试方法 | 测试内容 | 是否实际交易 | 预计手续费 |
|---------|---------|------------|----------|
| Test01_CreateAndQueryOrder | 创建、查询、取消订单 | ❌ | 0 |
| Test02_ModifyOrder | 修改订单价格和数量 | ❌ | 0 |
| Test03_BatchCreateOrders | 批量创建订单 | ❌ | 0 |
| Test04_BatchModifyOrders | 批量修改订单 | ❌ | 0 |
| Test05_BatchCancelOrders | 批量取消订单 | ❌ | 0 |
| Test06_CancelAllOrders | 取消所有订单 | ❌ | 0 |
| Test07_MarketOrderBehavior | 市价单行为验证 | ✅ | ~0.1 USDT |

**运行命令**:
```bash
# 运行完整套件
go test -v ./internal/service/exchange/binance/integration -run TestOrderServiceSuite

# 运行单个测试
go test -v ./internal/service/exchange/binance/integration -run TestOrderServiceSuite/Test01_CreateAndQueryOrder
```

---

### 2. TradingServiceSuite - 交易服务测试套件

**测试文件**: `trading_suite_test.go`  
**风险等级**: 中-高（包含实际开仓平仓）  
**测试数量**: 5 个

| 测试方法 | 测试内容 | 是否实际交易 | 预计手续费 |
|---------|---------|------------|----------|
| Test01_OpenPositionWithBalance | 余额百分比开仓（限价） | ❌ | 0 |
| Test02_OpenPositionWithQuantity | 指定数量开仓（限价） | ❌ | 0 |
| Test03_OpenPositionWithStopOrders | 市价开仓+止盈止损 | ✅ | ~0.1 USDT |
| Test04_ClosePositionByPercent | 按百分比分批平仓 | ✅ | ~0.15 USDT |
| Test05_ClosePositionByQuantity | 按数量平仓 | ✅ | ~0.15 USDT |

**运行命令**:
```bash
# 运行完整套件
go test -v ./internal/service/exchange/binance/integration -run TestTradingServiceSuite

# 只运行安全测试（不产生费用）
go test -v ./internal/service/exchange/binance/integration -run "TestTradingServiceSuite/Test0[12]"

# 只运行实际交易测试
go test -v ./internal/service/exchange/binance/integration -run "TestTradingServiceSuite/Test0[345]"
```

---

### 3. AccountServiceSuite - 账户服务测试套件

**测试文件**: `account_suite_test.go`  
**风险等级**: 低（只读操作）  
**测试数量**: 4 个

| 测试方法 | 测试内容 | 是否实际交易 | 预计手续费 |
|---------|---------|------------|----------|
| Test01_GetAccountInfo | 获取账户余额信息 | ❌ | 0 |
| Test02_GetRecentTransferHistory | 查询最近7天转账 | ❌ | 0 |
| Test03_GetLongTermTransferHistory | 查询30天转账（分片） | ❌ | 0 |
| Test04_ComprehensiveAccountAnalysis | 综合账户分析 | ❌ | 0 |

**运行命令**:
```bash
# 运行完整套件
go test -v ./internal/service/exchange/binance/integration -run TestAccountServiceSuite
```

---

### 4. PositionHistorySuite - 持仓历史测试套件

**测试文件**: `position_history_suite_test.go`  
**风险等级**: 低-中（大部分只读，一个测试会创建仓位）  
**测试数量**: 6 个

| 测试方法 | 测试内容 | 是否实际交易 | 预计手续费 |
|---------|---------|------------|----------|
| Test01_GetRecentHistoryPositions | 查询最近持仓历史 | ❌ | 0 |
| Test02_GetAllPairsHistory | 查询所有交易对历史 | ❌ | 0 |
| Test03_QueryAcrossMultipleDays | 跨30天查询（分片） | ❌ | 0 |
| Test04_PositionEventAnalysis | 持仓事件分析 | ❌ | 0 |
| Test05_CreateAndVerifyPositionLifecycle | 创建并验证完整生命周期 | ✅ | ~0.1 USDT |
| Test06_PaginationPerformance | 分页性能测试 | ❌ | 0 |

**运行命令**:
```bash
# 运行完整套件
go test -v ./internal/service/exchange/binance/integration -run TestPositionHistorySuite

# 跳过实际交易测试
go test -v ./internal/service/exchange/binance/integration -run "TestPositionHistorySuite/Test0[1-46]"
```

---

## BaseSuite - 基础测试设施

`suite_base.go` 提供了所有测试套件共享的基础设施。

### 核心功能

#### 1. 服务初始化
- 自动读取配置文件
- 初始化所有服务（Order, Position, Account, Market, Trading）
- 设置测试交易对和上下文

#### 2. 环境管理
```go
SetupSuite()      // 套件开始前运行一次
TearDownSuite()   // 套件结束后运行一次
SetupTest()       // 每个测试前运行
TearDownTest()    // 每个测试后运行
```

#### 3. 清理方法
```go
CleanupOrders(pair)          // 清理所有未成交订单
CleanupPositions(pair)       // 清理所有持仓
CleanupEnvironment(pair)     // 清理订单+持仓
```

#### 4. 断言方法
```go
AssertOrderInList(orderId, pair)      // 断言订单在未成交列表中
AssertOrderNotInList(orderId, pair)   // 断言订单不在列表中
AssertPositionExists(pair, side)      // 断言持仓存在
AssertNoPosition(pair, side)          // 断言没有持仓
```

#### 5. 辅助方法
```go
CreateLimitOrder(side, quantity)      // 创建限价单（不会成交）
CreateMarketOrder(type, side, qty)    // 创建市价单（会成交）
GetAccountBalance()                   // 获取账户余额
WaitForOrderSettlement()              // 等待订单处理
```

---

## 运行测试

### 1. 运行所有测试套件

```bash
# 运行所有集成测试
go test -v ./internal/service/exchange/binance/integration -timeout 300s

# 并行运行（更快）
go test -v ./internal/service/exchange/binance/integration -timeout 300s -parallel 4
```

### 2. 运行特定套件

```bash
# 只运行订单服务测试
go test -v ./internal/service/exchange/binance/integration -run TestOrderServiceSuite

# 只运行交易服务测试
go test -v ./internal/service/exchange/binance/integration -run TestTradingServiceSuite

# 只运行账户服务测试
go test -v ./internal/service/exchange/binance/integration -run TestAccountServiceSuite

# 只运行持仓历史测试
go test -v ./internal/service/exchange/binance/integration -run TestPositionHistorySuite
```

### 3. 运行特定测试

```bash
# 运行订单套件中的特定测试
go test -v ./internal/service/exchange/binance/integration \
  -run TestOrderServiceSuite/Test01_CreateAndQueryOrder

# 使用正则表达式运行多个测试
go test -v ./internal/service/exchange/binance/integration \
  -run "TestOrderServiceSuite/Test0[123]"
```

### 4. 只运行安全测试（不产生费用）

```bash
# 运行所有不会产生实际交易的测试
go test -v ./internal/service/exchange/binance/integration \
  -run "TestOrderServiceSuite/Test0[1-6]|TestTradingServiceSuite/Test0[12]|TestAccountServiceSuite|TestPositionHistorySuite/Test0[1-46]"
```

### 5. 生成测试报告

```bash
# 生成覆盖率报告
go test -v ./internal/service/exchange/binance/integration \
  -coverprofile=coverage.out

# 查看覆盖率
go tool cover -html=coverage.out

# 生成 JSON 报告
go test -v ./internal/service/exchange/binance/integration \
  -json > test_results.json
```

---

## 配置要求

### 配置文件

测试需要配置文件 `config/config.dev.yaml`:

```yaml
exchange:
  binance:
    api_key: "your_api_key"
    api_secret: "your_api_secret"
```

### 环境要求

1. **Go 版本**: >= 1.18
2. **依赖包**:
   ```bash
   go get github.com/stretchr/testify/suite
   go get github.com/stretchr/testify/assert
   go get github.com/stretchr/testify/require
   ```

3. **账户要求**:
   - 币安合约账户
   - 建议余额: 200-500 USDT
   - 建议使用测试账户

---

## 费用预估

### 按套件统计

| 测试套件 | 实际交易测试数 | 预估总费用 |
|---------|------------|----------|
| OrderServiceSuite | 1 | ~0.1 USDT |
| TradingServiceSuite | 3 | ~0.4 USDT |
| AccountServiceSuite | 0 | 0 |
| PositionHistorySuite | 1 | ~0.1 USDT |
| **总计** | **5** | **~0.6 USDT** |

### 费用说明

- 以上费用基于币安合约的标准费率（Maker 0.02%, Taker 0.04%）
- 实际费用可能因市场价格波动而略有不同
- 如果使用 BNB 支付手续费可享受 10% 折扣
- 如果有 VIP 等级，费率会更低

---

## 测试最佳实践

### 1. 测试前准备

```bash
# 1. 确保配置文件正确
cat config/config.dev.yaml

# 2. 检查网络连接
ping api.binance.com

# 3. 验证账户余额充足
go test -v ./internal/service/exchange/binance/integration \
  -run TestAccountServiceSuite/Test01_GetAccountInfo
```

### 2. 分步骤运行

```bash
# Step 1: 先运行安全测试（不产生费用）
go test -v ./internal/service/exchange/binance/integration \
  -run TestAccountServiceSuite

# Step 2: 运行低风险测试
go test -v ./internal/service/exchange/binance/integration \
  -run "TestOrderServiceSuite/Test0[1-6]"

# Step 3: 运行中风险测试（确认无误后）
go test -v ./internal/service/exchange/binance/integration \
  -run TestOrderServiceSuite/Test07_MarketOrderBehavior

# Step 4: 运行高风险测试（仔细确认）
go test -v ./internal/service/exchange/binance/integration \
  -run TestTradingServiceSuite
```

### 3. 测试失败处理

如果测试失败:

1. **查看日志** - 测试会输出详细的步骤日志
2. **检查订单** - 登录币安查看是否有遗留订单
3. **检查持仓** - 确认是否有未平仓位
4. **手动清理** - 如有必要，手动取消订单和平仓
5. **重新运行** - 环境清理后重新运行失败的测试

### 4. CI/CD 集成

```yaml
# GitHub Actions 示例
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      
      - name: Create config
        run: |
          mkdir -p config
          echo "${{ secrets.BINANCE_CONFIG }}" > config/config.dev.yaml
      
      - name: Run safe tests only
        run: |
          go test -v ./internal/service/exchange/binance/integration \
            -run "TestAccountServiceSuite"
```

---

## 故障排除

### 常见问题

#### 1. 配置文件未找到
```
Error: Failed to read config: Config File "config.dev" Not Found
```
**解决**: 确保配置文件在正确位置 `config/config.dev.yaml`

#### 2. API 密钥错误
```
Error: code=-2015, msg=Invalid API-key, IP, or permissions for action
```
**解决**: 检查 API 密钥是否正确，是否启用了合约交易权限

#### 3. 余额不足
```
Error: code=-2019, msg=Margin is insufficient
```
**解决**: 充值更多 USDT 到合约账户

#### 4. 订单精度错误
```
Error: code=-1111, msg=Precision is over the maximum defined
```
**解决**: 已在代码中自动处理，如遇到新交易对，需要添加精度配置

#### 5. 测试超时
```
Error: test timed out after 60s
```
**解决**: 增加超时时间 `-timeout 300s`

---

## 与旧测试文件的对比

### 旧架构问题

1. ❌ 代码重复 - 每个测试都要初始化客户端
2. ❌ 环境管理混乱 - 手动清理，容易遗漏
3. ❌ 难以维护 - 分散在多个文件中
4. ❌ 测试隔离差 - 测试间可能相互影响

### 新架构优势

1. ✅ 代码复用 - 统一的基础设施
2. ✅ 自动清理 - 每个测试前后自动清理
3. ✅ 易于维护 - 清晰的套件结构
4. ✅ 完全隔离 - 每个测试独立运行
5. ✅ 更好的组织 - 按服务分组
6. ✅ 丰富的断言 - 专门的测试辅助方法
7. ✅ 友好的输出 - 步骤编号，清晰的日志

---

## 扩展测试套件

### 添加新的测试套件

1. 创建新文件（如 `new_service_suite_test.go`）
2. 定义套件结构:

```go
type NewServiceSuite struct {
    BaseSuite
}

func TestNewServiceSuite(t *testing.T) {
    suite.Run(t, new(NewServiceSuite))
}
```

3. 添加测试方法:

```go
func (s *NewServiceSuite) Test01_SomeFeature() {
    s.T().Log("\n步骤 1: ...")
    // 测试代码
}
```

### 添加新的辅助方法

在 `suite_base.go` 中添加:

```go
func (s *BaseSuite) YourHelperMethod() {
    // 辅助代码
}
```

---

## 最佳实践总结

### ✅ 推荐做法

1. **分步骤运行** - 先安全测试，再风险测试
2. **检查日志** - 仔细阅读测试输出
3. **小额测试** - 使用最小交易量
4. **测试账户** - 使用专门的测试账户
5. **定期清理** - 测试后检查是否有遗留
6. **版本控制** - 及时提交测试代码

### ❌ 避免做法

1. **跳过清理** - 不清理环境就运行测试
2. **并行风险测试** - 实际交易测试应串行运行
3. **大额测试** - 不要使用大量资金测试
4. **生产账户** - 不要在生产账户测试
5. **忽略错误** - 测试失败要仔细分析

---

## 维护建议

### 定期维护

1. **更新文档** - 保持文档与代码同步
2. **审查测试** - 定期审查测试覆盖率
3. **优化性能** - 优化慢速测试
4. **清理代码** - 移除过时的测试

### 监控指标

1. **测试通过率** - 目标 > 95%
2. **执行时间** - 全套测试 < 5分钟
3. **代码覆盖率** - 目标 > 80%
4. **失败率** - 关注频繁失败的测试

---

## 贡献指南

欢迎贡献新的测试用例或改进现有测试！

### 提交测试

1. Fork 项目
2. 创建特性分支
3. 添加测试
4. 确保所有测试通过
5. 提交 Pull Request

### 测试规范

1. **命名规范** - `Test<序号>_<功能描述>`
2. **日志规范** - 使用步骤编号和 ✓/⚠ 标记
3. **清理规范** - 确保清理测试数据
4. **文档规范** - 添加测试说明注释

---

## 联系方式

如有问题或建议，请:
- 提交 Issue
- 创建 Pull Request
- 联系维护者

---

**最后更新**: 2025-10-28
**维护者**: Trading Agent Team
**版本**: 2.0


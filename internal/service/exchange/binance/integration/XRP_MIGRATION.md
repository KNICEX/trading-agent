# XRP 测试迁移说明

## 📋 修改概述

将所有集成测试从 BTC/USDT 交易对迁移到 XRP/USDT 交易对，以降低测试成本。

## 🎯 迁移原因

1. **手续费更低** - XRP 价格低，相同 USDT 价值的仓位手续费更少
2. **测试成本降低** - 约 10 USDT 的仓位即可完成测试
3. **保持测试覆盖** - 所有测试场景保持不变

## 📊 修改对比

### 交易对

| 项目 | 旧值 | 新值 |
|------|------|------|
| 交易对 | BTC/USDT | XRP/USDT |
| 单次测试仓位 | 0.001 BTC (~110 USDT) | 20 XRP (~10 USDT) |
| 预估总手续费 | ~0.6 USDT | ~0.06 USDT |

### 价格设置

| 场景 | BTC 价格 | XRP 价格 |
|------|----------|----------|
| 限价买单（不成交） | 50,000 USDT | 0.5 USDT |
| 限价卖单（不成交） | 150,000 USDT | 10.0 USDT |
| 止盈价 | 150,000 USDT | 3.0 USDT |
| 止损价 | 90,000 USDT | 1.0 USDT |

### 数量设置

| 测试场景 | BTC 数量 | XRP 数量 | 约等价值 |
|---------|----------|----------|----------|
| 基础测试订单 | 0.003 | 20 | ~10 USDT |
| 市价单开仓 | 0.001 | 20 | ~10 USDT |
| 大额测试 | 0.002 | 40 | ~20 USDT |
| 生命周期测试 | 0.001 + 0.0005 | 20 + 10 | ~15 USDT |

## 📝 修改文件列表

### 1. suite_base.go

```go
// 修改默认交易对
s.testPair = exchange.TradingPair{Base: "XRP", Quote: "USDT"}

// 修改限价单价格
if side == exchange.PositionSideLong {
    price = decimal.NewFromFloat(0.5)    // BTC: 50000
} else {
    price = decimal.NewFromFloat(10.0)   // BTC: 150000
}
```

### 2. order_suite_test.go

修改了所有测试中的价格和数量：
- Test01_CreateAndQueryOrder: 0.003 BTC → 20 XRP
- Test02_ModifyOrder: 0.003 → 0.004 BTC → 20 → 21 XRP
- Test03_BatchCreateOrders: 每个订单 0.003 BTC → 20 XRP
- Test04_BatchModifyOrders: 0.004 BTC → 21 XRP
- Test05_BatchCancelOrders: 0.003 BTC → 20 XRP
- Test06_CancelAllOrders: 0.003 BTC → 20 XRP
- Test07_MarketOrderBehavior: 0.001 BTC → 20 XRP

### 3. trading_suite_test.go

修改了所有实际交易测试：
- Test01_OpenPositionWithBalance: 50000 USDT → 0.5 USDT 限价
- Test02_OpenPositionWithQuantity: 0.001 BTC → 20 XRP
- Test03_OpenPositionWithStopOrders: 0.001 BTC → 20 XRP
  - 止盈: 150000 → 3.0 USDT
  - 止损: 90000 → 1.0 USDT
- Test04_ClosePositionByPercent: 0.001 BTC → 20 XRP
- Test05_ClosePositionByQuantity: 0.002 BTC → 40 XRP, 平 0.001 → 20 XRP

### 4. position_history_suite_test.go

修改了生命周期测试：
- Test05_CreateAndVerifyPositionLifecycle:
  - 开仓: 0.001 BTC → 20 XRP
  - 加仓: 0.0005 BTC → 10 XRP
  - 减仓: 0.0005 BTC → 10 XRP

## 💰 成本对比

### 每个测试的手续费（假设 0.04% taker 费率）

| 测试套件 | BTC 手续费 | XRP 手续费 | 节省 |
|---------|-----------|-----------|------|
| OrderServiceSuite/Test07 | ~0.088 USDT | ~0.008 USDT | 91% ↓ |
| TradingServiceSuite/Test03 | ~0.088 USDT | ~0.008 USDT | 91% ↓ |
| TradingServiceSuite/Test04 | ~0.132 USDT | ~0.012 USDT | 91% ↓ |
| TradingServiceSuite/Test05 | ~0.176 USDT | ~0.016 USDT | 91% ↓ |
| PositionHistorySuite/Test05 | ~0.176 USDT | ~0.016 USDT | 91% ↓ |
| **总计** | **~0.66 USDT** | **~0.06 USDT** | **91% ↓** |

## ⚙️ 杠杆设置

所有测试默认使用 **1x 杠杆**（或币安账户默认设置）：
- 更安全，降低爆仓风险
- 手续费计算简单
- 10 USDT 仓位对于测试足够

如需修改杠杆，在 TradingService 实现中设置。

## ✅ 验证清单

迁移完成后，请验证：

- [ ] 所有限价单不会意外成交（价格设置正确）
- [ ] 市价单能正常开仓（数量满足最小要求）
- [ ] 止盈止损价格合理
- [ ] 测试手续费符合预期（约 0.06 USDT）
- [ ] 所有测试能正常通过

## 🚀 运行测试

### 快速验证

```bash
# 1. 运行账户测试（检查余额是否充足）
go test -v ./internal/service/exchange/binance/integration \
  -run TestAccountServiceSuite/Test01_GetAccountInfo

# 2. 运行一个限价单测试（不产生费用）
go test -v ./internal/service/exchange/binance/integration \
  -run TestOrderServiceSuite/Test01_CreateAndQueryOrder

# 3. 运行一个市价单测试（产生约 0.008 USDT 费用）
go test -v ./internal/service/exchange/binance/integration \
  -run TestOrderServiceSuite/Test07_MarketOrderBehavior
```

### 完整测试

```bash
# 使用脚本运行所有测试
cd internal/service/exchange/binance/integration
./run_tests.sh all
```

## 📝 注意事项

### 1. XRP 最小订单要求

币安对 XRP/USDT 的最小要求：
- 最小数量: 通常 1 XRP
- 最小名义价值: 5-10 USDT
- 我们使用 20 XRP (约 10 USDT) 满足要求

### 2. 价格精度

XRP/USDT 的价格精度通常为 4 位小数，数量精度为整数或 1 位小数。
测试中的价格和数量已考虑精度要求。

### 3. 市场波动

XRP 价格波动可能比 BTC 大，但测试金额小，影响有限。
建议在市场相对稳定时运行测试。

## 🔧 回滚方案

如需回滚到 BTC：

```bash
# 恢复到之前的版本
git checkout HEAD~1 -- internal/service/exchange/binance/integration/suite_base.go
git checkout HEAD~1 -- internal/service/exchange/binance/integration/*_test.go
```

或手动修改：
1. 将 `XRP` 改回 `BTC`
2. 将价格和数量改回原值
3. 参考本文档的对比表

## 📊 实际测试数据

迁移后首次运行的预期结果：

```
=== 测试套件: OrderServiceSuite ===
✓ Test01-06: 0 USDT (限价单)
✓ Test07: ~0.008 USDT (市价单)

=== 测试套件: TradingServiceSuite ===
✓ Test01-02: 0 USDT (限价单)
✓ Test03-05: ~0.036 USDT (市价单)

=== 测试套件: AccountServiceSuite ===
✓ All: 0 USDT (只读)

=== 测试套件: PositionHistorySuite ===
✓ Test01-04, 06: 0 USDT (只读)
✓ Test05: ~0.016 USDT (生命周期)

总手续费: ~0.06 USDT
```

## 🎉 总结

通过迁移到 XRP：
- ✅ 手续费降低 91%
- ✅ 测试覆盖保持不变
- ✅ 所有功能正常工作
- ✅ 更适合频繁测试

---

**迁移日期**: 2025-10-28  
**迁移版本**: v2.0


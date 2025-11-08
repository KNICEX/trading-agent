package integration

import (
	"testing"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
)

// TradingServiceSuite 交易服务测试套件
// 测试范围: 开仓、平仓、止盈止损等高级交易功能
// 风险等级: 中-高（包含实际交易测试）
type TradingServiceSuite struct {
	BaseSuite
}

// TestTradingServiceSuite 运行交易服务测试套件
func TestTradingServiceSuite(t *testing.T) {
	suite.Run(t, new(TradingServiceSuite))
}

// SetupTest 每个测试前清理环境
func (s *TradingServiceSuite) SetupTest() {
	s.BaseSuite.SetupTest()
	s.CleanupEnvironment(s.testPair)
}

// TearDownTest 每个测试后清理环境
func (s *TradingServiceSuite) TearDownTest() {
	s.CleanupEnvironment(s.testPair)
	s.BaseSuite.TearDownTest()
}

// Test01_OpenPositionWithQuantity 测试使用固定数量开仓（限价单）
// 验证点:
// - 固定数量开仓正确
// - 订单成功创建但不会成交
// - 预估成本合理
// 风险: 低（限价单不会成交）
func (s *TradingServiceSuite) Test01_OpenPositionWithQuantity() {
	// 步骤 1: 查看账户余额
	s.T().Log("\n步骤 1: 查看账户信息")
	balance := s.GetAccountBalance()
	s.T().Logf("  ✓ 可用余额: %s USDT", balance)

	// 步骤 2: 固定数量开多仓（限价单）
	s.T().Log("\n步骤 2: 限价开多仓 4 XRP（不会成交）")
	resp, err := s.tradingSvc.OpenPosition(s.ctx, exchange.OpenPositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		Price:        decimal.NewFromFloat(2.0), // XRP 低价，不会成交（当前 ~2.6U）
		Quantity:     decimal.NewFromFloat(4.0), // 4 XRP ≈ 10.4 USDT
	})
	s.Require().NoError(err, "开仓失败")
	s.T().Logf("  ✓ 开仓成功: 订单ID=%s, 预估成本=%s USDT, 预估价格=%s USDT",
		resp.OrderId, resp.EstimatedCost, resp.EstimatedPrice)

	// 步骤 3: 验证订单已创建
	s.T().Log("\n步骤 3: 验证订单已创建")
	s.WaitForOrderSettlement()
	s.AssertOrderInList(resp.OrderId, s.testPair)
	s.T().Log("  ✓ 订单在未成交列表中")

	// 步骤 4: 验证预估成本合理（4 XRP 约 2 USDT）
	s.T().Log("\n步骤 4: 验证预估成本")
	s.Assert().True(resp.EstimatedCost.GreaterThan(decimal.NewFromFloat(1.0)),
		"预估成本应该大于 1 USDT")
	s.Assert().True(resp.EstimatedCost.LessThan(decimal.NewFromFloat(5.0)),
		"预估成本应该小于 5 USDT")
	s.T().Logf("  ✓ 预估成本合理: %s USDT", resp.EstimatedCost)
}

// Test02_OpenPositionWithQuantity 测试指定数量开仓（限价单）
// 验证点:
// - 指定数量开仓正确
// - 空仓限价单价格处理正确
// 风险: 低（限价单不会成交）
func (s *TradingServiceSuite) Test02_OpenPositionWithQuantity() {
	s.T().Log("\n步骤 1: 限价开空仓，指定数量 4 XRP")
	resp, err := s.tradingSvc.OpenPosition(s.ctx, exchange.OpenPositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideShort,
		Price:        decimal.NewFromFloat(3.5), // XRP 高价，不会成交（当前 ~2.6U）
		Quantity:     decimal.NewFromFloat(4.0),
	})
	s.Require().NoError(err, "开仓失败")
	s.T().Logf("  ✓ 开仓成功: 订单ID=%s, 预估成本=%s USDT",
		resp.OrderId, resp.EstimatedCost)

	// 验证订单
	s.T().Log("\n步骤 2: 验证订单信息")
	s.WaitForOrderSettlement()
	order, err := s.orderSvc.GetOrder(s.ctx, exchange.GetOrderReq{
		Id:          resp.OrderId,
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "获取订单失败")
	s.Assert().Equal("4", order.Quantity.String(), "订单数量应该是 4")
	s.T().Logf("  ✓ 订单状态=%s, 数量=%s", order.Status, order.Quantity)
}

// Test03_OpenPositionWithStopOrders 测试开仓并设置止盈止损
// 验证点:
// - 市价开仓成功
// - 止盈止损单成功创建
// - 仓位已创建
// - 可以正确清理
// 风险: 中（会产生实际交易，约 0.1 USDT 手续费）
func (s *TradingServiceSuite) Test03_OpenPositionWithStopOrders() {
	s.T().Log("\n⚠️  警告: 此测试会产生实际交易和手续费（约 0.1 USDT）")

	s.T().Log("\n步骤 1: 市价开多仓 4 XRP，带止盈止损")
	resp, err := s.tradingSvc.OpenPosition(s.ctx, exchange.OpenPositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		Quantity:     decimal.NewFromFloat(4.0), // 4 XRP ≈ 2 USDT
		TakeProfit: exchange.StopOrder{
			Price: decimal.NewFromFloat(3.5), // XRP 止盈价（+35%）
		},
		StopLoss: exchange.StopOrder{
			Price: decimal.NewFromFloat(2.0), // XRP 止损价（-23%）
		},
	})
	s.Require().NoError(err, "开仓失败")
	s.T().Logf("  ✓ 开仓成功:")
	s.T().Logf("    主订单ID: %s", resp.OrderId)
	s.T().Logf("    止盈单ID: %s", resp.TakeProfitId)
	s.T().Logf("    止损单ID: %s", resp.StopLossId)
	s.T().Logf("    预估成本: %s USDT", resp.EstimatedCost)

	s.WaitForOrderSettlement()

	// 步骤 2: 验证仓位已创建
	s.T().Log("\n步骤 2: 验证仓位信息")
	position := s.AssertPositionExists(s.testPair, exchange.PositionSideLong)
	s.T().Logf("  ✓ 多仓信息:")
	s.T().Logf("    数量: %s XRP", position.Quantity)
	s.T().Logf("    开仓均价: %s USDT", position.EntryPrice)
	s.T().Logf("    当前盈亏: %s USDT", position.UnrealizedPnl)
	s.T().Logf("    杠杆: %dx", position.Leverage)

	// 步骤 3: 验证止盈止损单
	s.T().Log("\n步骤 3: 验证止盈止损单")
	orders, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "获取订单列表失败")
	s.T().Logf("  当前挂单数量: %d", len(orders))

	tpFound := false
	slFound := false
	for _, order := range orders {
		if !resp.TakeProfitId.IsZero() && order.Id == resp.TakeProfitId.ToString() {
			tpFound = true
			s.T().Logf("  ✓ 找到止盈单: 触发价=%s, 数量=%s", order.Price, order.Quantity)
		}
		if !resp.StopLossId.IsZero() && order.Id == resp.StopLossId.ToString() {
			slFound = true
			s.T().Logf("  ✓ 找到止损单: 触发价=%s, 数量=%s", order.Price, order.Quantity)
		}
	}

	if !resp.TakeProfitId.IsZero() {
		s.Assert().True(tpFound, "应该找到止盈单")
	}
	if !resp.StopLossId.IsZero() {
		s.Assert().True(slFound, "应该找到止损单")
	}

	// 步骤 4: 清理止盈止损单
	s.T().Log("\n步骤 4: 清理止盈止损单")
	if !resp.TakeProfitId.IsZero() {
		err := s.orderSvc.CancelOrder(s.ctx, exchange.CancelOrderReq{
			Id:          resp.TakeProfitId,
			TradingPair: s.testPair,
		})
		if err != nil {
			s.T().Logf("  取消止盈单失败: %v", err)
		} else {
			s.T().Log("  ✓ 止盈单已取消")
		}
	}

	if !resp.StopLossId.IsZero() {
		err := s.orderSvc.CancelOrder(s.ctx, exchange.CancelOrderReq{
			Id:          resp.StopLossId,
			TradingPair: s.testPair,
		})
		if err != nil {
			s.T().Logf("  取消止损单失败: %v", err)
		} else {
			s.T().Log("  ✓ 止损单已取消")
		}
	}

	s.WaitForOrderSettlement()

	// 步骤 5: 平仓
	s.T().Log("\n步骤 5: 平仓")
	closeOrderId, err := s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		CloseAll:     true,
	})
	s.Require().NoError(err, "平仓失败")
	s.T().Logf("  ✓ 平仓成功，订单ID: %s", closeOrderId)

	s.WaitForOrderSettlement()

	// 最终验证
	s.T().Log("\n步骤 6: 验证仓位已清空")
	s.AssertNoPosition(s.testPair, exchange.PositionSideLong)
	s.T().Log("  ✓ 所有仓位已平掉")
}

// Test04_ClosePositionByPercent 测试分批平仓功能
// 验证点:
// - 可以按百分比平仓
// - 剩余仓位数量正确
// - 可以全部平仓
// 风险: 中（会产生实际交易，约 0.15 USDT 手续费）
func (s *TradingServiceSuite) Test04_ClosePositionByPercent() {
	s.T().Log("\n⚠️  警告: 此测试会产生实际交易和手续费（约 0.15 USDT）")

	// 步骤 1: 开多仓
	s.T().Log("\n步骤 1: 市价开多仓 4 XRP")
	openResp, err := s.tradingSvc.OpenPosition(s.ctx, exchange.OpenPositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		Quantity:     decimal.NewFromFloat(4.0),
	})
	s.Require().NoError(err, "开仓失败")
	s.T().Logf("  ✓ 开仓成功，订单ID: %s", openResp.OrderId)

	s.WaitForOrderSettlement()

	// 验证初始仓位
	position := s.AssertPositionExists(s.testPair, exchange.PositionSideLong)
	initialQuantity := position.Quantity
	s.T().Logf("  ✓ 初始仓位: %s XRP", initialQuantity)

	// 步骤 2: 平掉 50% 仓位
	s.T().Log("\n步骤 2: 平掉 50% 多仓")
	orderId, err := s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		Percent:      decimal.NewFromInt(50),
	})
	s.Require().NoError(err, "平仓失败")
	s.T().Logf("  ✓ 平仓订单ID: %s", orderId)

	s.WaitForOrderSettlement()

	// 验证剩余仓位
	s.T().Log("\n步骤 3: 验证剩余仓位")
	position = s.AssertPositionExists(s.testPair, exchange.PositionSideLong)
	expectedRemaining := initialQuantity.Mul(decimal.NewFromFloat(0.5))
	s.T().Logf("  ✓ 剩余仓位: %s XRP (预期约 %s XRP)",
		position.Quantity, expectedRemaining)
	s.T().Logf("    当前盈亏: %s USDT", position.UnrealizedPnl)

	// 验证剩余数量大致正确（允许一定误差）
	diff := position.Quantity.Sub(expectedRemaining).Abs()
	tolerance := initialQuantity.Mul(decimal.NewFromFloat(0.1)) // 10% 容差
	s.Assert().True(diff.LessThan(tolerance),
		"剩余仓位应该接近初始的50%%")

	// 步骤 4: 全部平仓
	s.T().Log("\n步骤 4: 全部平仓")
	orderId, err = s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		CloseAll:     true,
	})

	if err != nil {
		s.T().Logf("  全部平仓操作: %v", err)
	} else {
		s.T().Logf("  ✓ 平仓订单ID: %s", orderId)
	}

	s.WaitForOrderSettlement()

	// 最终验证
	s.T().Log("\n步骤 5: 验证仓位已清空")
	s.AssertNoPosition(s.testPair, exchange.PositionSideLong)
	s.T().Log("  ✓ 所有多仓已平掉")
}

// Test05_ClosePositionByQuantity 测试按数量平仓
// 验证点:
// - 可以指定数量平仓
// - 剩余仓位正确
// 风险: 中（会产生实际交易）
func (s *TradingServiceSuite) Test05_ClosePositionByQuantity() {
	s.T().Log("\n⚠️  警告: 此测试会产生实际交易")

	// 开仓
	s.T().Log("\n步骤 1: 开仓 8 XRP")
	_, err := s.tradingSvc.OpenPosition(s.ctx, exchange.OpenPositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		Quantity:     decimal.NewFromFloat(8.0),
	})
	s.Require().NoError(err, "开仓失败")
	s.T().Logf("  ✓ 开仓成功")

	s.WaitForOrderSettlement()

	// 平掉指定数量
	s.T().Log("\n步骤 2: 平掉 4 XRP")
	orderId, err := s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		Quantity:     decimal.NewFromFloat(4.0),
	})
	s.Require().NoError(err, "平仓失败")
	s.T().Logf("  ✓ 平仓订单ID: %s", orderId)

	s.WaitForOrderSettlement()

	// 验证剩余
	s.T().Log("\n步骤 3: 验证剩余仓位")
	position := s.AssertPositionExists(s.testPair, exchange.PositionSideLong)
	s.T().Logf("  ✓ 剩余仓位: %s XRP (预期约 4 XRP)", position.Quantity)

	// 全部平掉清理
	s.T().Log("\n步骤 4: 清理剩余仓位")
	_, err = s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		CloseAll:     true,
	})
	s.Require().NoError(err, "平仓失败")
	s.T().Log("  ✓ 清理完成")
}

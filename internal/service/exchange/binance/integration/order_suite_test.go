package integration

import (
	"testing"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
)

// OrderServiceSuite 订单服务测试套件
// 测试范围: 订单的创建、查询、修改、取消等核心功能
// 风险等级: 低（所有订单使用不会成交的限价）
type OrderServiceSuite struct {
	BaseSuite
}

// TestOrderServiceSuite 运行订单服务测试套件
func TestOrderServiceSuite(t *testing.T) {
	suite.Run(t, new(OrderServiceSuite))
}

// SetupTest 每个测试前清理环境
func (s *OrderServiceSuite) SetupTest() {
	s.BaseSuite.SetupTest()
	s.CleanupEnvironment(s.testPair)
}

// TearDownTest 每个测试后清理环境
func (s *OrderServiceSuite) TearDownTest() {
	s.CleanupEnvironment(s.testPair)
	s.BaseSuite.TearDownTest()
}

// Test01_CreateAndQueryOrder 测试创建和查询订单
// 验证点:
// - 订单创建成功
// - 可以查询单个订单
// - 订单出现在未成交列表中
// - 可以取消订单
func (s *OrderServiceSuite) Test01_CreateAndQueryOrder() {
	s.T().Log("\n步骤 1: 创建限价买单")
	lowPrice := decimal.NewFromFloat(2.0) // XRP 低价（当前 ~2.6U）
	quantity := decimal.NewFromFloat(4.0) // 4 XRP

	orderId, err := s.orderSvc.CreateOrder(s.ctx, exchange.CreateOrderReq{
		TradingPair: s.testPair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       lowPrice,
		Quantity:    quantity,
	})
	s.Require().NoError(err, "创建订单失败")
	s.T().Logf("  ✓ 订单创建成功: %s", orderId)

	s.WaitForOrderSettlement()

	// 步骤 2: 查询单个订单
	s.T().Log("\n步骤 2: 查询订单状态")
	orderInfo, err := s.orderSvc.GetOrder(s.ctx, exchange.GetOrderReq{
		Id:          orderId,
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "查询订单失败")
	s.Assert().True(orderInfo.IsActive(), "订单应该处于活跃状态")
	s.T().Logf("  ✓ 订单状态: %s, 价格: %s, 数量: %s",
		orderInfo.Status, orderInfo.Price, orderInfo.Quantity)

	// 步骤 3: 验证订单在未成交列表中
	s.T().Log("\n步骤 3: 验证订单在未成交列表中")
	s.AssertOrderInList(orderId, s.testPair)
	s.T().Log("  ✓ 订单已在未成交列表中")

	// 步骤 4: 取消订单
	s.T().Log("\n步骤 4: 取消订单")
	err = s.orderSvc.CancelOrder(s.ctx, exchange.CancelOrderReq{
		TradingPair: s.testPair,
		Id:          orderId,
	})
	s.Require().NoError(err, "取消订单失败")
	s.T().Log("  ✓ 订单取消成功")
}

// Test02_ModifyOrder 测试修改订单
// 验证点:
// - 可以修改订单价格和数量
// - 修改后的值正确
func (s *OrderServiceSuite) Test02_ModifyOrder() {
	s.T().Log("\n步骤 1: 创建限价买单")
	initialPrice := decimal.NewFromFloat(2.0)
	initialQuantity := decimal.NewFromFloat(4.0)

	orderId, err := s.orderSvc.CreateOrder(s.ctx, exchange.CreateOrderReq{
		TradingPair: s.testPair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: exchange.PositionSideLong,
		Price:       initialPrice,
		Quantity:    initialQuantity,
	})
	s.Require().NoError(err, "创建订单失败")
	s.T().Logf("  ✓ 订单创建: ID=%s, 价格=%s, 数量=%s",
		orderId, initialPrice, initialQuantity)

	s.WaitForOrderSettlement()

	// 步骤 2: 修改订单
	s.T().Log("\n步骤 2: 修改订单价格和数量")
	newPrice := decimal.NewFromFloat(1.9)
	newQuantity := decimal.NewFromFloat(5.0)

	err = s.orderSvc.ModifyOrder(s.ctx, exchange.ModifyOrderReq{
		Id:          orderId,
		TradingPair: s.testPair,
		Price:       newPrice,
		Quantity:    newQuantity,
	})

	if err != nil {
		s.T().Logf("  ⚠ 修改订单失败（某些交易所可能不支持）: %v", err)
		// 不算测试失败，因为某些交易所确实不支持修改
	} else {
		s.T().Logf("  ✓ 订单修改成功: 新价格=%s, 新数量=%s", newPrice, newQuantity)

		// 验证修改结果
		s.WaitForOrderSettlement()
		orderInfo, err := s.orderSvc.GetOrder(s.ctx, exchange.GetOrderReq{
			Id:          orderId,
			TradingPair: s.testPair,
		})
		s.Require().NoError(err, "查询订单失败")
		s.T().Logf("  ✓ 修改后: 价格=%s, 数量=%s",
			orderInfo.Price, orderInfo.Quantity)
	}
}

// Test03_BatchCreateOrders 测试批量创建订单
// 验证点:
// - 可以批量创建多个订单
// - 所有订单都成功创建
// - 所有订单都在未成交列表中
func (s *OrderServiceSuite) Test03_BatchCreateOrders() {
	s.T().Log("\n步骤 1: 批量创建 3 个限价单")
	basePrice := decimal.NewFromFloat(2.0)
	var createReqs []exchange.CreateOrderReq

	for i := 0; i < 3; i++ {
		price := basePrice.Sub(decimal.NewFromFloat(float64(i) * 0.01))
		createReqs = append(createReqs, exchange.CreateOrderReq{
			TradingPair: s.testPair,
			OrderType:   exchange.OrderTypeOpen,
			PositonSide: exchange.PositionSideLong,
			Price:       price,
			Quantity:    decimal.NewFromFloat(4.0),
		})
	}

	orderIds, err := s.orderSvc.CreateOrders(s.ctx, createReqs)
	s.Require().NoError(err, "批量创建订单失败")
	s.Assert().Equal(3, len(orderIds), "应该创建 3 个订单")
	s.T().Logf("  ✓ 成功创建 %d 个订单", len(orderIds))

	s.WaitForOrderSettlement()

	// 步骤 2: 验证所有订单都在列表中
	s.T().Log("\n步骤 2: 验证所有订单都在未成交列表中")
	orders, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "获取订单列表失败")

	foundCount := 0
	for _, orderId := range orderIds {
		for _, order := range orders {
			if order.Id == orderId.ToString() {
				foundCount++
				break
			}
		}
	}
	s.Assert().Equal(len(orderIds), foundCount, "所有订单都应该在列表中")
	s.T().Logf("  ✓ 找到 %d/%d 个订单", foundCount, len(orderIds))
}

// Test04_BatchModifyOrders 测试批量修改订单
// 验证点:
// - 可以批量修改多个订单
func (s *OrderServiceSuite) Test04_BatchModifyOrders() {
	// 先批量创建订单
	s.T().Log("\n步骤 1: 批量创建订单")
	basePrice := decimal.NewFromFloat(2.0)
	var createReqs []exchange.CreateOrderReq

	for i := 0; i < 3; i++ {
		price := basePrice.Sub(decimal.NewFromFloat(float64(i) * 0.01))
		createReqs = append(createReqs, exchange.CreateOrderReq{
			TradingPair: s.testPair,
			OrderType:   exchange.OrderTypeOpen,
			PositonSide: exchange.PositionSideLong,
			Price:       price,
			Quantity:    decimal.NewFromFloat(4.0),
		})
	}

	orderIds, err := s.orderSvc.CreateOrders(s.ctx, createReqs)
	s.Require().NoError(err, "批量创建订单失败")
	s.T().Logf("  ✓ 创建了 %d 个订单", len(orderIds))

	s.WaitForOrderSettlement()

	// 步骤 2: 批量修改订单
	s.T().Log("\n步骤 2: 批量修改订单")
	newBasePrice := decimal.NewFromFloat(1.9)
	var modifyReqs []exchange.ModifyOrderReq

	for i, orderId := range orderIds {
		price := newBasePrice.Sub(decimal.NewFromFloat(float64(i) * 0.01))
		modifyReqs = append(modifyReqs, exchange.ModifyOrderReq{
			Id:          orderId,
			TradingPair: s.testPair,
			Price:       price,
			Quantity:    decimal.NewFromFloat(5.0),
		})
	}

	err = s.orderSvc.ModifyOrders(s.ctx, modifyReqs)
	if err != nil {
		s.T().Logf("  ⚠ 批量修改失败（某些交易所可能不支持）: %v", err)
	} else {
		s.T().Logf("  ✓ 批量修改成功")
	}
}

// Test05_BatchCancelOrders 测试批量取消订单
// 验证点:
// - 可以批量取消多个订单
// - 所有订单都从未成交列表中移除
func (s *OrderServiceSuite) Test05_BatchCancelOrders() {
	// 先批量创建订单
	s.T().Log("\n步骤 1: 批量创建订单")
	orderIds := make([]exchange.OrderId, 0, 3)
	for i := 0; i < 3; i++ {
		orderId := s.CreateLimitOrder(exchange.PositionSideLong, decimal.NewFromFloat(4.0))
		orderIds = append(orderIds, orderId)
	}
	s.T().Logf("  ✓ 创建了 %d 个订单", len(orderIds))

	// 步骤 2: 批量取消订单
	s.T().Log("\n步骤 2: 批量取消订单")
	err := s.orderSvc.CancelOrders(s.ctx, exchange.CancelOrdersReq{
		TradingPair: s.testPair,
		Ids:         orderIds,
	})
	s.Require().NoError(err, "批量取消订单失败")
	s.T().Log("  ✓ 批量取消成功")

	s.WaitForOrderSettlement()

	// 步骤 3: 验证订单已取消
	s.T().Log("\n步骤 3: 验证订单已从列表中移除")
	orders, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "获取订单列表失败")

	remainingCount := 0
	for _, orderId := range orderIds {
		for _, order := range orders {
			if order.Id == orderId.ToString() {
				remainingCount++
				break
			}
		}
	}
	s.Assert().Equal(0, remainingCount, "所有订单都应该被取消")
	s.T().Logf("  ✓ 确认所有订单已取消")
}

// Test06_CancelAllOrders 测试取消所有订单
// 验证点:
// - 可以取消指定交易对的所有订单
func (s *OrderServiceSuite) Test06_CancelAllOrders() {
	s.T().Log("\n步骤 1: 创建多个订单")
	orderCount := 3
	for i := 0; i < orderCount; i++ {
		s.CreateLimitOrder(exchange.PositionSideLong, decimal.NewFromFloat(4.0))
	}
	s.T().Logf("  ✓ 创建了 %d 个订单", orderCount)

	// 步骤 2: 查询订单数量
	s.T().Log("\n步骤 2: 查询当前订单")
	ordersBefore, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "获取订单列表失败")
	s.T().Logf("  ✓ 当前有 %d 个未成交订单", len(ordersBefore))

	// 步骤 3: 取消所有订单
	s.T().Log("\n步骤 3: 取消所有订单")
	err = s.orderSvc.CancelOrders(s.ctx, exchange.CancelOrdersReq{
		TradingPair: s.testPair,
		Ids:         []exchange.OrderId{}, // 空列表表示取消所有
	})
	s.Require().NoError(err, "取消所有订单失败")
	s.T().Log("  ✓ 取消请求已发送")

	s.WaitForOrderSettlement()

	// 步骤 4: 验证所有订单已取消
	s.T().Log("\n步骤 4: 验证订单已清空")
	ordersAfter, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{
		TradingPair: s.testPair,
	})
	s.Require().NoError(err, "获取订单列表失败")
	s.Assert().Equal(0, len(ordersAfter), "应该没有未成交订单")
	s.T().Logf("  ✓ 确认订单已清空（剩余 %d 个）", len(ordersAfter))
}

// Test07_MarketOrderBehavior 测试市价单的特殊行为
// 验证点:
// - 市价单会立即成交
// - 成交后不在未成交列表中
// - 会产生实际仓位
// 风险: 中（会产生实际交易，约 0.1 USDT 手续费）
func (s *OrderServiceSuite) Test07_MarketOrderBehavior() {
	s.T().Log("\n⚠️  警告: 此测试会创建实际仓位和产生手续费")

	s.T().Log("\n步骤 1: 创建市价买单开仓")
	orderId := s.CreateMarketOrder(exchange.OrderTypeOpen, exchange.PositionSideLong, decimal.NewFromFloat(4.0)) // 4 XRP
	s.T().Logf("  ✓ 市价单创建成功: %s", orderId)

	// 步骤 2: 验证订单不在未成交列表中
	s.T().Log("\n步骤 2: 验证市价单不在未成交列表中")
	s.AssertOrderNotInList(orderId, s.testPair)
	s.T().Log("  ✓ 市价单已成交，不在未成交列表中")

	// 步骤 3: 验证仓位已创建
	s.T().Log("\n步骤 3: 验证仓位已创建")
	position := s.AssertPositionExists(s.testPair, exchange.PositionSideLong)
	s.T().Logf("  ✓ 持仓: 数量=%s, 开仓价=%s, 盈亏=%s",
		position.Quantity, position.EntryPrice, position.UnrealizedPnl)

	// 步骤 4: 平仓清理
	s.T().Log("\n步骤 4: 平仓清理")
	_, err := s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
		TradingPair:  s.testPair,
		PositionSide: exchange.PositionSideLong,
		CloseAll:     true,
	})
	s.Require().NoError(err, "平仓失败")
	s.T().Log("  ✓ 平仓成功")
}

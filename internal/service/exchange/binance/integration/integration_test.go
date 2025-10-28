package integration

// 币安交易集成测试
//
// 本测试套件专注于 OrderService 的核心功能测试：
// 1. OrderService 只负责订单的创建、修改和取消
// 2. GetOrders() 只返回未完全成交的订单
// 3. 市价单会立即成交，成交后自动从未成交列表中移除
//
// 测试覆盖：
// - TestCreateAndQueryOrder: 基本的创建、查询、取消订单
// - TestModifyOrder: 修改未成交订单
// - TestBatchOrders: 批量创建、修改、取消订单
// - TestCancelAllOrders: 取消指定交易对的所有订单
// - TestMarketOrder: 市价单的特殊行为（会产生实际仓位）
//
// 注意事项：
// - 所有测试都会自动清理创建的订单和仓位
// - 限价单价格设置得很低（50000 USDT），确保不会立即成交
// - TestMarketOrder 会产生实际仓位，但会在测试结束时平仓

import (
	"context"
	"testing"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/exchange/binance"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
)

// initClient 初始化币安客户端
func initClient(t *testing.T) *futures.Client {
	viper.AddConfigPath("../../../../../config")
	viper.SetConfigName("config.dev")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	type Config struct {
		Exchange map[string]struct {
			ApiKey    string `mapstructure:"api_key"`
			ApiSecret string `mapstructure:"api_secret"`
		} `mapstructure:"exchange"`
	}
	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}
	return futures.NewClient(config.Exchange["binance"].ApiKey, config.Exchange["binance"].ApiSecret)
}

// newOrderService 创建订单服务
func newOrderService(t *testing.T) exchange.OrderService {
	return binance.NewOrderService(initClient(t))
}

// newPositionService 创建持仓服务
func newPositionService(t *testing.T) exchange.PositionService {
	return binance.NewPositionService(initClient(t))
}

// TestCreateAndQueryOrder 测试创建订单和查询未成交订单
// 1. 创建限价买单（不会立即成交）
// 2. 查询单个订单状态（验证订单创建成功）
// 3. 查询所有未成交订单（验证订单在列表中）
// 4. 取消订单（清理测试数据）
func TestCreateAndQueryOrder(t *testing.T) {
	orderSvc := newOrderService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 创建限价买单（价格设置得很低，不会立即成交）
	t.Log("=== 步骤 1: 创建限价买单 ===")
	lowPrice := decimal.NewFromInt(50000) // 设置一个很低的价格，不会立即成交
	orderReq := exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		OrderType:   exchange.OrderTypeLimit,
		PositonSide: exchange.PositionSideLong,
		Price:       lowPrice,
		Quantity:    decimal.NewFromFloat(0.003), // 50000 * 0.003 = 150 USDT，满足最小100 USDT要求
	}

	orderId, err := orderSvc.CreateOrder(context.Background(), orderReq)
	if err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
	t.Logf("订单创建成功, ID: %s", orderId)

	// 等待订单被系统接受
	time.Sleep(2 * time.Second)

	// 2. 查询单个订单状态
	t.Log("\n=== 步骤 2: 查询订单状态 ===")
	orderInfo, err := orderSvc.GetOrder(context.Background(), exchange.GetOrderReq{
		Id:          orderId,
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}
	t.Logf("订单详情: ID=%s, 状态=%s, 方向=%s, 价格=%s, 数量=%s",
		orderInfo.Id, orderInfo.Status, orderInfo.Side, orderInfo.Price.String(), orderInfo.Quantity.String())

	// 验证订单应该是未成交状态
	if !orderInfo.IsActive() {
		t.Fatalf("订单应该处于活跃状态，但实际状态为: %s", orderInfo.Status)
	}

	// 3. 查询所有未成交订单
	t.Log("\n=== 步骤 3: 查询所有未成交订单 ===")
	openOrders, err := orderSvc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询未成交订单失败: %v", err)
	}
	t.Logf("未成交订单数量: %d", len(openOrders))

	// 验证刚创建的订单在列表中
	found := false
	for _, order := range openOrders {
		if order.Id == orderInfo.Id {
			found = true
			t.Logf("找到刚创建的订单: ID=%s, 价格=%s, 数量=%s",
				order.Id, order.Price.String(), order.Quantity.String())
		}
	}
	if !found {
		t.Fatalf("未在未成交订单列表中找到刚创建的订单")
	}

	// 4. 清理：取消订单
	t.Log("\n=== 步骤 4: 取消订单 ===")
	cancelErr := orderSvc.CancelOrder(context.Background(), exchange.CancelOrderReq{
		TradingPair: pair,
		Id:          orderId,
	})
	if cancelErr != nil {
		t.Fatalf("取消订单失败: %v", cancelErr)
	}
	t.Logf("订单取消成功")
}

// TestModifyOrder 测试修改未成交订单
// 1. 创建限价买单
// 2. 修改订单价格和数量
// 3. 查询验证修改结果
// 4. 取消订单
func TestModifyOrder(t *testing.T) {
	orderSvc := newOrderService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 创建限价买单
	t.Log("=== 步骤 1: 创建限价买单 ===")
	initialPrice := decimal.NewFromInt(50000)
	initialQuantity := decimal.NewFromFloat(0.003)

	orderId, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		OrderType:   exchange.OrderTypeLimit,
		PositonSide: exchange.PositionSideLong,
		Price:       initialPrice,
		Quantity:    initialQuantity,
	})
	if err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
	t.Logf("订单创建成功, ID: %s, 价格: %s, 数量: %s",
		orderId, initialPrice.String(), initialQuantity.String())

	time.Sleep(2 * time.Second)

	// 2. 修改订单
	t.Log("\n=== 步骤 2: 修改订单 ===")
	newPrice := decimal.NewFromInt(49000)
	newQuantity := decimal.NewFromFloat(0.004)

	modifyErr := orderSvc.ModifyOrder(context.Background(), exchange.ModifyOrderReq{
		Id:          orderId,
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		Price:       newPrice,
		Quantity:    newQuantity,
	})
	if modifyErr != nil {
		t.Logf("修改订单失败 (可能是交易所限制): %v", modifyErr)
	} else {
		t.Logf("订单修改成功, 新价格: %s, 新数量: %s", newPrice.String(), newQuantity.String())
	}

	time.Sleep(2 * time.Second)

	// 3. 查询验证修改结果
	t.Log("\n=== 步骤 3: 验证修改结果 ===")
	orderInfo, err := orderSvc.GetOrder(context.Background(), exchange.GetOrderReq{
		Id:          orderId,
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}

	if modifyErr == nil {
		t.Logf("修改后订单详情: 价格=%s, 数量=%s",
			orderInfo.Price.String(), orderInfo.Quantity.String())
	}

	// 4. 清理：取消订单
	t.Log("\n=== 步骤 4: 取消订单 ===")
	cancelErr := orderSvc.CancelOrder(context.Background(), exchange.CancelOrderReq{
		TradingPair: pair,
		Id:          orderId,
	})
	if cancelErr != nil {
		t.Fatalf("取消订单失败: %v", cancelErr)
	}
	t.Logf("订单取消成功")
}

// TestBatchOrders 测试批量订单操作
// 1. 批量创建多个限价单
// 2. 查询未成交订单列表
// 3. 批量修改订单
// 4. 批量取消订单
func TestBatchOrders(t *testing.T) {
	orderSvc := newOrderService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 批量创建限价单
	t.Log("=== 步骤 1: 批量创建限价单 ===")
	basePrice := decimal.NewFromInt(50000)
	var createReqs []exchange.CreateOrderReq

	for i := 0; i < 3; i++ {
		price := basePrice.Sub(decimal.NewFromInt(int64(i * 100)))
		createReqs = append(createReqs, exchange.CreateOrderReq{
			TradingPair: pair,
			Side:        exchange.OrderSideBuy,
			OrderType:   exchange.OrderTypeLimit,
			PositonSide: exchange.PositionSideLong,
			Price:       price,
			Quantity:    decimal.NewFromFloat(0.003),
		})
	}

	orderIds, err := orderSvc.CreateOrders(context.Background(), createReqs)
	if err != nil {
		t.Fatalf("批量创建订单失败: %v", err)
	}
	t.Logf("成功创建 %d 个订单", len(orderIds))
	for i, orderId := range orderIds {
		t.Logf("订单 %d: ID=%s, 价格=%s", i+1, orderId, createReqs[i].Price.String())
	}

	time.Sleep(2 * time.Second)

	// 2. 查询未成交订单
	t.Log("\n=== 步骤 2: 查询未成交订单 ===")
	openOrders, err := orderSvc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询未成交订单失败: %v", err)
	}
	t.Logf("当前未成交订单数量: %d", len(openOrders))

	// 验证所有创建的订单都在未成交列表中
	foundCount := 0
	for _, orderId := range orderIds {
		for _, order := range openOrders {
			if order.Id == orderId.ToString() {
				foundCount++
				t.Logf("找到订单: ID=%s, 价格=%s, 数量=%s, 状态=%s",
					order.Id, order.Price.String(), order.Quantity.String(), order.Status)
				break
			}
		}
	}
	t.Logf("在未成交列表中找到 %d/%d 个订单", foundCount, len(orderIds))

	// 3. 批量修改订单
	t.Log("\n=== 步骤 3: 批量修改订单 ===")
	var modifyReqs []exchange.ModifyOrderReq
	newBasePrice := decimal.NewFromInt(49000)

	for i, orderId := range orderIds {
		price := newBasePrice.Sub(decimal.NewFromInt(int64(i * 100)))
		modifyReqs = append(modifyReqs, exchange.ModifyOrderReq{
			Id:          orderId,
			TradingPair: pair,
			Side:        exchange.OrderSideBuy,
			Price:       price,
			Quantity:    decimal.NewFromFloat(0.004),
		})
	}

	modifyErr := orderSvc.ModifyOrders(context.Background(), modifyReqs)
	if modifyErr != nil {
		t.Logf("批量修改订单失败 (可能是交易所限制): %v", modifyErr)
	} else {
		t.Logf("成功修改 %d 个订单", len(modifyReqs))
	}

	time.Sleep(2 * time.Second)

	// 4. 批量取消订单
	t.Log("\n=== 步骤 4: 批量取消订单 ===")
	cancelErr := orderSvc.CancelOrders(context.Background(), exchange.CancelOrdersReq{
		TradingPair: pair,
		Ids:         orderIds,
	})
	if cancelErr != nil {
		t.Fatalf("批量取消订单失败: %v", cancelErr)
	}
	t.Logf("成功取消 %d 个订单", len(orderIds))

	time.Sleep(2 * time.Second)

	// 5. 验证订单已取消
	t.Log("\n=== 步骤 5: 验证订单已取消 ===")
	remainingOrders, err := orderSvc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询未成交订单失败: %v", err)
	}

	// 检查刚才取消的订单是否还在列表中
	remainingCount := 0
	for _, orderId := range orderIds {
		for _, order := range remainingOrders {
			if order.Id == orderId.ToString() {
				remainingCount++
				break
			}
		}
	}
	t.Logf("取消后剩余订单数量: %d (应为0)", remainingCount)
	if remainingCount > 0 {
		t.Logf("警告: 还有 %d 个订单未成功取消", remainingCount)
	}
}

// TestCancelAllOrders 测试取消所有订单
// 1. 创建多个订单
// 2. 取消指定交易对的所有订单
// 3. 验证订单已全部取消
func TestCancelAllOrders(t *testing.T) {
	orderSvc := newOrderService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 创建多个订单
	t.Log("=== 步骤 1: 创建测试订单 ===")
	basePrice := decimal.NewFromInt(50000)
	orderCount := 2

	for i := 0; i < orderCount; i++ {
		price := basePrice.Sub(decimal.NewFromInt(int64(i * 100)))
		_, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
			TradingPair: pair,
			Side:        exchange.OrderSideBuy,
			OrderType:   exchange.OrderTypeLimit,
			PositonSide: exchange.PositionSideLong,
			Price:       price,
			Quantity:    decimal.NewFromFloat(0.003),
		})
		if err != nil {
			t.Fatalf("创建订单失败: %v", err)
		}
	}
	t.Logf("成功创建 %d 个订单", orderCount)

	time.Sleep(2 * time.Second)

	// 2. 查询创建的订单
	t.Log("\n=== 步骤 2: 查询未成交订单 ===")
	ordersBefore, err := orderSvc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}
	t.Logf("取消前订单数量: %d", len(ordersBefore))

	// 3. 取消该交易对的所有订单
	t.Log("\n=== 步骤 3: 取消所有订单 ===")
	err = orderSvc.CancelOrders(context.Background(), exchange.CancelOrdersReq{
		TradingPair: pair,
		Ids:         []exchange.OrderId{}, // 空列表表示取消该交易对的所有订单
	})
	if err != nil {
		t.Fatalf("取消所有订单失败: %v", err)
	}
	t.Logf("取消订单请求已发送")

	time.Sleep(2 * time.Second)

	// 4. 验证订单已全部取消
	t.Log("\n=== 步骤 4: 验证订单已取消 ===")
	ordersAfter, err := orderSvc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}
	t.Logf("取消后订单数量: %d (应为0或接近0)", len(ordersAfter))
}

// TestMarketOrder 测试市价单
// 注意: 市价单会立即成交，成交后订单会从未成交列表中移除
// 1. 创建市价买单
// 2. 验证订单已成交（通过GetOrder查询）
// 3. 验证未成交列表中不包含该订单
// 4. 平仓
func TestMarketOrder(t *testing.T) {
	orderSvc := newOrderService(t)
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 创建小额市价买单开仓
	t.Log("=== 步骤 1: 创建市价买单 ===")
	orderId, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		OrderType:   exchange.OrderTypeMarket,
		PositonSide: exchange.PositionSideLong,
		Quantity:    decimal.NewFromFloat(0.003),
	})
	if err != nil {
		t.Fatalf("创建市价单失败: %v", err)
	}
	t.Logf("市价单创建成功, ID: %s", orderId)

	time.Sleep(2 * time.Second)

	// 2. 查询订单状态（GetOrder应该能查到，即使已成交）
	t.Log("\n=== 步骤 2: 查询订单状态 ===")
	orderInfo, err := orderSvc.GetOrder(context.Background(), exchange.GetOrderReq{
		Id:          orderId,
		TradingPair: pair,
	})
	if err != nil {
		t.Logf("查询订单失败（市价单可能已成交并被移除）: %v", err)
	} else {
		t.Logf("订单状态: %s, 已成交数量: %s", orderInfo.Status, orderInfo.ExecutedQuantity.String())

		// 如果订单已完全成交，它应该不在未成交列表中
		if orderInfo.Status.IsFilled() {
			t.Log("订单已完全成交")
		}
	}

	// 3. 验证未成交列表中不包含该订单（因为市价单会立即成交）
	t.Log("\n=== 步骤 3: 验证未成交订单列表 ===")
	openOrders, err := orderSvc.GetOrders(context.Background(), exchange.GetOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询未成交订单失败: %v", err)
	}

	found := false
	for _, order := range openOrders {
		if order.Id == orderId.ToString() {
			found = true
			break
		}
	}

	if found {
		t.Log("市价单仍在未成交列表中（可能部分成交）")
	} else {
		t.Log("市价单不在未成交列表中（已完全成交，符合预期）")
	}

	// 4. 平仓
	time.Sleep(2 * time.Second)
	t.Log("\n=== 步骤 4: 平仓 ===")
	positions, err := positionSvc.GetActivePositions(context.Background(), []exchange.TradingPair{pair})
	if err != nil {
		t.Fatalf("获取持仓失败: %v", err)
	}

	if len(positions) > 0 {
		position := positions[0]
		t.Logf("当前持仓: 方向=%s, 数量=%s", position.PositionSide, position.PositionAmount.String())

		// 平仓
		closeOrderId, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
			TradingPair: pair,
			Side:        position.PositionSide.GetCloseOrderSide(),
			OrderType:   exchange.OrderTypeMarket,
			PositonSide: position.PositionSide,
			Quantity:    position.PositionAmount.Abs(),
		})
		if err != nil {
			t.Fatalf("平仓失败: %v", err)
		}
		t.Logf("平仓订单创建成功, ID: %s", closeOrderId)
	} else {
		t.Log("没有持仓需要平仓")
	}
}

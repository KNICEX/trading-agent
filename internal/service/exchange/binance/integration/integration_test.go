package integration

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

// TestCompleteTradeFlow 测试完整的交易流程
// 1. 查询当前持仓
// 2. 创建限价买单
// 3. 查询订单状态
// 4. 修改订单
// 5. 取消订单
func TestCompleteTradeFlow(t *testing.T) {
	orderSvc := newOrderService(t)
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 查询当前持仓
	t.Log("=== 步骤 1: 查询当前持仓 ===")
	positions, err := positionSvc.GetActivePosition(context.Background(), pair)
	if err != nil {
		t.Fatalf("获取持仓失败: %v", err)
	}
	t.Logf("当前持仓数量: %d", len(positions))
	for _, pos := range positions {
		t.Logf("持仓详情: 方向=%s, 数量=%s, 入场价=%s, 未实现盈亏=%s",
			pos.PositionSide, pos.PositionAmount.String(), pos.EntryPrice.String(), pos.UnrealizedProfit.String())
	}

	// 2. 创建限价买单（价格设置得很低，不会立即成交）
	t.Log("\n=== 步骤 2: 创建限价买单 ===")
	lowPrice := decimal.NewFromInt(50000) // 设置一个很低的价格，不会立即成交
	orderReq := exchange.CreateOrderReq{
		Symbol:      pair,
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

	// 3. 查询订单状态
	t.Log("\n=== 步骤 3: 查询订单状态 ===")
	orderInfo, err := orderSvc.GetOrder(context.Background(), exchange.GetOrderReq{
		Id:     orderId,
		Symbol: pair,
	})
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}
	t.Logf("订单状态: ID=%s, 状态=%s, 价格=%s, 数量=%s",
		orderInfo.Id, orderInfo.Status, orderInfo.Price.String(), orderInfo.Quantity.String())

	// 4. 修改订单（调整数量）
	t.Log("\n=== 步骤 4: 修改订单 ===")
	newQuantity := decimal.NewFromFloat(0.004) // 50000 * 0.004 = 200 USDT
	modifyErr := orderSvc.ModifyOrder(context.Background(), exchange.ModifyOrderReq{
		Id:          orderId,
		TradingPair: pair,
		Side:        exchange.OrderSideBuy,
		Quantity:    newQuantity,
		Price:       lowPrice,
	})
	if modifyErr != nil {
		t.Logf("修改订单失败 (可能是交易所限制): %v", modifyErr)
	} else {
		t.Logf("订单修改成功, 新数量: %s", newQuantity.String())
	}

	time.Sleep(1 * time.Second)

	// 5. 取消订单
	t.Log("\n=== 步骤 5: 取消订单 ===")
	cancelErr := orderSvc.CancelOrder(context.Background(), exchange.CancelOrderReq{
		TradingPair: pair,
		Id:          orderId,
	})
	if cancelErr != nil {
		t.Fatalf("取消订单失败: %v", cancelErr)
	}
	t.Logf("订单取消成功")
}

// TestMarketOrderClosePosition 测试使用市价单平仓
// 这个测试需要账户中有实际持仓才能执行
func TestMarketOrderClosePosition(t *testing.T) {
	orderSvc := newOrderService(t)
	positionSvc := newPositionService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 获取当前持仓
	t.Log("=== 查询当前持仓 ===")
	positions, err := positionSvc.GetActivePosition(context.Background(), pair)
	if err != nil {
		t.Fatalf("获取持仓失败: %v", err)
	}

	if len(positions) == 0 {
		t.Skip("没有持仓，跳过平仓测试")
		return
	}

	position := positions[0]
	t.Logf("持仓详情: %+v", position)
	t.Logf("持仓方向: %s, 数量: %s", position.PositionSide, position.PositionAmount.String())

	// 2. 确定平仓方向和数量
	var orderSide exchange.OrderSide
	if position.PositionAmount.IsNegative() {
		// 空头持仓，需要买入平仓
		orderSide = exchange.OrderSideBuy
		t.Log("检测到空头持仓，将使用买单平仓")
	} else {
		// 多头持仓，需要卖出平仓
		orderSide = exchange.OrderSideSell
		t.Log("检测到多头持仓，将使用卖单平仓")
	}

	// 3. 创建市价单平仓
	t.Log("\n=== 创建市价单平仓 ===")
	orderId, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
		Symbol:      pair,
		Side:        orderSide,
		PositonSide: position.PositionSide,
		Quantity:    position.PositionAmount.Abs(), // 使用绝对值
		OrderType:   exchange.OrderTypeMarket,
	})
	if err != nil {
		t.Fatalf("创建平仓订单失败: %v", err)
	}
	t.Logf("平仓订单创建成功, ID: %s", orderId)

	// 等待订单执行
	time.Sleep(2 * time.Second)

	// 4. 验证订单状态
	t.Log("\n=== 验证订单状态 ===")
	orderInfo, err := orderSvc.GetOrder(context.Background(), exchange.GetOrderReq{
		Id:     orderId,
		Symbol: pair,
	})
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}
	t.Logf("订单状态: %s", orderInfo.Status)

	// 5. 验证持仓是否已平
	time.Sleep(2 * time.Second)
	newPositions, err := positionSvc.GetActivePosition(context.Background(), pair)
	if err != nil {
		t.Fatalf("获取持仓失败: %v", err)
	}
	t.Logf("平仓后持仓数量: %d", len(newPositions))
}

// TestListAndManageOrders 测试批量订单管理
func TestListAndManageOrders(t *testing.T) {
	orderSvc := newOrderService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	// 1. 创建多个限价单
	t.Log("=== 步骤 1: 创建多个限价单 ===")
	var orderIds []exchange.OrderId
	basePrice := decimal.NewFromInt(50000)

	for i := 0; i < 3; i++ {
		price := basePrice.Sub(decimal.NewFromInt(int64(i * 100)))
		orderId, err := orderSvc.CreateOrder(context.Background(), exchange.CreateOrderReq{
			Symbol:      pair,
			Side:        exchange.OrderSideBuy,
			OrderType:   exchange.OrderTypeLimit,
			PositonSide: exchange.PositionSideLong,
			Price:       price,
			Quantity:    decimal.NewFromFloat(0.003), // 满足最小100 USDT要求
		})
		if err != nil {
			t.Fatalf("创建订单 %d 失败: %v", i+1, err)
		}
		orderIds = append(orderIds, orderId)
		t.Logf("订单 %d 创建成功, ID: %s, 价格: %s", i+1, orderId, price.String())
	}

	time.Sleep(2 * time.Second)

	// 2. 查询未完成订单列表
	t.Log("\n=== 步骤 2: 查询未完成订单 ===")
	openOrders, err := orderSvc.ListOpenOrders(context.Background(), exchange.ListOpenOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询未完成订单失败: %v", err)
	}
	t.Logf("当前未完成订单数量: %d", len(openOrders))
	for _, order := range openOrders {
		t.Logf("订单: ID=%s, 价格=%s, 数量=%s, 状态=%s",
			order.Id, order.Price.String(), order.Quantity.String(), order.Status)
	}

	// 3. 批量取消刚才创建的订单
	t.Log("\n=== 步骤 3: 批量取消订单 ===")
	if len(orderIds) > 0 {
		err = orderSvc.CancelMultipleOrders(context.Background(), exchange.CancelMultipleOrdersReq{
			TradingPair: pair,
			Ids:         orderIds,
		})
		if err != nil {
			t.Fatalf("批量取消订单失败: %v", err)
		}
		t.Logf("成功取消 %d 个订单", len(orderIds))
	}

	time.Sleep(1 * time.Second)

	// 4. 再次查询确认订单已取消
	t.Log("\n=== 步骤 4: 确认订单已取消 ===")
	openOrders, err = orderSvc.ListOpenOrders(context.Background(), exchange.ListOpenOrdersReq{
		TradingPair: pair,
	})
	if err != nil {
		t.Fatalf("查询未完成订单失败: %v", err)
	}
	t.Logf("当前未完成订单数量: %d", len(openOrders))
}

// TestGetAllPositions 测试获取所有持仓
func TestGetAllPositions(t *testing.T) {
	positionSvc := newPositionService(t)

	t.Log("=== 获取所有持仓 ===")
	positions, err := positionSvc.GetActivePositions(context.Background())
	if err != nil {
		t.Fatalf("获取所有持仓失败: %v", err)
	}

	t.Logf("总持仓数量: %d", len(positions))
	for i, pos := range positions {
		t.Logf("持仓 %d:", i+1)
		t.Logf("  交易对: %s", pos.TradingPair.ToString())
		t.Logf("  方向: %s", pos.PositionSide)
		t.Logf("  数量: %s", pos.PositionAmount.String())
		t.Logf("  入场价: %s", pos.EntryPrice.String())
		t.Logf("  标记价: %s", pos.MarkPrice.String())
		t.Logf("  杠杆: %d", pos.Leverage)
		t.Logf("  保证金: %s", pos.MarginAmount.String())
		t.Logf("  未实现盈亏: %s", pos.UnrealizedProfit.String())
		t.Logf("  ---")
	}
}

// TestOrderHistory 测试查询历史订单
func TestOrderHistory(t *testing.T) {
	orderSvc := newOrderService(t)
	pair := exchange.TradingPair{Base: "BTC", Quote: "USDT"}

	t.Log("=== 查询历史订单 ===")
	orders, err := orderSvc.ListOrders(context.Background(), exchange.ListOrdersReq{
		TradingPair: pair,
		Limit:       10,
		StartTime:   time.Now().Add(-24 * time.Hour),
		EndTime:     time.Now(),
	})
	if err != nil {
		t.Fatalf("查询历史订单失败: %v", err)
	}

	t.Logf("历史订单数量: %d", len(orders))
	for i, order := range orders {
		t.Logf("订单 %d:", i+1)
		t.Logf("  ID: %s", order.Id)
		t.Logf("  方向: %s", order.Side)
		t.Logf("  价格: %s", order.Price.String())
		t.Logf("  数量: %s", order.Quantity.String())
		t.Logf("  状态: %s", order.Status)
		t.Logf("  创建时间: %s", order.CreatedAt.Format("2006-01-02 15:04:05"))
		t.Logf("  ---")
	}
}

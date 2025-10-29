package integration

import (
	"context"
	"strings"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/exchange/binance"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

// BaseSuite 是所有集成测试的基础套件
type BaseSuite struct {
	suite.Suite
	client      *futures.Client
	orderSvc    exchange.OrderService
	positionSvc exchange.PositionService
	accountSvc  exchange.AccountService
	marketSvc   exchange.MarketService
	tradingSvc  exchange.TradingService

	// 测试配置
	testPair exchange.TradingPair
	ctx      context.Context
}

// SetupSuite 在测试套件开始前运行一次
func (s *BaseSuite) SetupSuite() {
	s.T().Log("=== 初始化测试套件 ===")

	// 读取配置
	viper.AddConfigPath("../../../../../config")
	viper.SetConfigName("config.dev")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	s.Require().NoError(err, "读取配置文件失败")

	type Config struct {
		Exchange map[string]struct {
			ApiKey    string `mapstructure:"api_key"`
			ApiSecret string `mapstructure:"api_secret"`
		} `mapstructure:"exchange"`
	}
	var config Config
	err = viper.Unmarshal(&config)
	s.Require().NoError(err, "解析配置失败")

	// 初始化币安客户端
	s.client = futures.NewClient(config.Exchange["binance"].ApiKey, config.Exchange["binance"].ApiSecret)
	s.Require().NotNil(s.client, "客户端初始化失败")

	// 初始化各个服务
	s.orderSvc = binance.NewOrderService(s.client)
	s.positionSvc = binance.NewPositionService(s.client)
	s.accountSvc = binance.NewAccountService(s.client)
	s.marketSvc = binance.NewMarketService(s.client)
	s.tradingSvc = binance.NewTradingService(
		s.client,
		s.orderSvc,
		s.accountSvc,
		s.positionSvc,
		s.marketSvc,
	)

	// 设置测试交易对和上下文
	s.testPair = exchange.TradingPair{Base: "XRP", Quote: "USDT"}
	s.ctx = context.Background()

	// 设置双向持仓模式（Hedge Mode）
	s.T().Log("  - 设置双向持仓模式...")
	err = s.client.NewChangePositionModeService().DualSide(true).Do(s.ctx)
	if err != nil {
		// 如果已经是双向持仓模式，会返回错误，这是正常的
		if strings.Contains(err.Error(), "No need to change position side") {
			s.T().Log("    ✓ 双向持仓模式已启用")
		} else {
			s.T().Logf("    设置双向持仓模式失败（可能已经是双向模式）: %v", err)
		}
	} else {
		s.T().Log("    ✓ 双向持仓模式已启用")
	}

	s.T().Log("✓ 测试套件初始化完成")
}

// TearDownSuite 在测试套件结束后运行一次
func (s *BaseSuite) TearDownSuite() {
	s.T().Log("=== 测试套件清理完成 ===")
}

// SetupTest 在每个测试用例开始前运行
func (s *BaseSuite) SetupTest() {
	s.T().Logf(">>> 开始测试: %s", s.T().Name())
}

// TearDownTest 在每个测试用例结束后运行
func (s *BaseSuite) TearDownTest() {
	s.T().Logf("<<< 结束测试: %s\n", s.T().Name())
}

// CleanupOrders 清理所有未成交订单
func (s *BaseSuite) CleanupOrders(pair exchange.TradingPair) {
	s.T().Log("  - 清理未成交订单...")
	err := s.orderSvc.CancelOrders(s.ctx, exchange.CancelOrdersReq{
		TradingPair: pair,
		Ids:         []exchange.OrderId{}, // 空列表表示取消所有
	})
	if err != nil {
		s.T().Logf("    清理订单失败（可能没有订单）: %v", err)
	} else {
		s.T().Log("    ✓ 订单清理成功")
	}
	time.Sleep(1 * time.Second)
}

// CleanupPositions 清理所有持仓
func (s *BaseSuite) CleanupPositions(pair exchange.TradingPair) {
	s.T().Log("  - 清理持仓...")
	positions, err := s.positionSvc.GetActivePositions(s.ctx, []exchange.TradingPair{pair})
	if err != nil {
		s.T().Logf("    获取持仓失败: %v", err)
		return
	}

	hasPosition := false
	for _, pos := range positions {
		if !pos.Quantity.IsZero() {
			hasPosition = true
			s.T().Logf("    平掉仓位: %s %s", pos.PositionSide, pos.Quantity.String())
			_, err := s.tradingSvc.ClosePosition(s.ctx, exchange.ClosePositionReq{
				TradingPair:  pair,
				PositionSide: pos.PositionSide,
				CloseAll:     true,
			})
			if err != nil {
				s.T().Logf("    平仓失败: %v", err)
			}
		}
	}

	if !hasPosition {
		s.T().Log("    ✓ 无持仓需要清理")
	} else {
		s.T().Log("    ✓ 持仓清理完成")
		time.Sleep(2 * time.Second)
	}
}

// CleanupEnvironment 清理测试环境（订单+持仓）
func (s *BaseSuite) CleanupEnvironment(pair exchange.TradingPair) {
	s.T().Log("清理测试环境:")
	s.CleanupOrders(pair)
	s.CleanupPositions(pair)
}

// AssertOrderInList 断言订单在未成交列表中
func (s *BaseSuite) AssertOrderInList(orderId exchange.OrderId, pair exchange.TradingPair) {
	orders, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{TradingPair: pair})
	s.Require().NoError(err, "获取订单列表失败")

	found := false
	for _, order := range orders {
		if order.Id == orderId.ToString() {
			found = true
			break
		}
	}
	s.Assert().True(found, "订单 %s 应该在未成交列表中", orderId)
}

// AssertOrderNotInList 断言订单不在未成交列表中
func (s *BaseSuite) AssertOrderNotInList(orderId exchange.OrderId, pair exchange.TradingPair) {
	orders, err := s.orderSvc.GetOrders(s.ctx, exchange.GetOrdersReq{TradingPair: pair})
	s.Require().NoError(err, "获取订单列表失败")

	found := false
	for _, order := range orders {
		if order.Id == orderId.ToString() {
			found = true
			break
		}
	}
	s.Assert().False(found, "订单 %s 不应该在未成交列表中", orderId)
}

// AssertPositionExists 断言持仓存在且数量不为零
func (s *BaseSuite) AssertPositionExists(pair exchange.TradingPair, side exchange.PositionSide) *exchange.Position {
	positions, err := s.positionSvc.GetActivePositions(s.ctx, []exchange.TradingPair{pair})
	s.Require().NoError(err, "获取持仓失败")

	for i := range positions {
		if positions[i].PositionSide == side && !positions[i].Quantity.IsZero() {
			return &positions[i]
		}
	}

	s.FailNow("未找到持仓", "持仓方向: %s", side)
	return nil
}

// AssertNoPosition 断言没有持仓或持仓为零
func (s *BaseSuite) AssertNoPosition(pair exchange.TradingPair, side exchange.PositionSide) {
	positions, err := s.positionSvc.GetActivePositions(s.ctx, []exchange.TradingPair{pair})
	s.Require().NoError(err, "获取持仓失败")

	for _, pos := range positions {
		if pos.PositionSide == side {
			s.Assert().True(pos.Quantity.IsZero(), "持仓 %s 应该为零，实际为 %s", side, pos.Quantity)
			return
		}
	}

	// 未找到持仓也是正常的
}

// CreateLimitOrder 创建限价单（不会立即成交的价格）
func (s *BaseSuite) CreateLimitOrder(side exchange.PositionSide, quantity decimal.Decimal) exchange.OrderId {
	// 根据方向选择不会成交的价格（XRP 当前约 2.6 USDT）
	var price decimal.Decimal
	if side == exchange.PositionSideLong {
		price = decimal.NewFromFloat(2.0) // 低于市价，买入不会成交
	} else {
		price = decimal.NewFromFloat(3.5) // 高于市价，卖出不会成交
	}

	orderId, err := s.orderSvc.CreateOrder(s.ctx, exchange.CreateOrderReq{
		TradingPair: s.testPair,
		OrderType:   exchange.OrderTypeOpen,
		PositonSide: side,
		Price:       price,
		Quantity:    quantity,
	})
	s.Require().NoError(err, "创建限价单失败")

	time.Sleep(1 * time.Second) // 等待订单被系统接受
	return orderId
}

// CreateMarketOrder 创建市价单（会立即成交）
func (s *BaseSuite) CreateMarketOrder(orderType exchange.OrderType, side exchange.PositionSide, quantity decimal.Decimal) exchange.OrderId {
	orderId, err := s.orderSvc.CreateOrder(s.ctx, exchange.CreateOrderReq{
		TradingPair: s.testPair,
		OrderType:   orderType,
		PositonSide: side,
		Quantity:    quantity,
	})
	s.Require().NoError(err, "创建市价单失败")

	time.Sleep(2 * time.Second) // 等待订单成交
	return orderId
}

// GetAccountBalance 获取账户余额
func (s *BaseSuite) GetAccountBalance() decimal.Decimal {
	accountInfo, err := s.accountSvc.GetAccountInfo(s.ctx)
	s.Require().NoError(err, "获取账户信息失败")
	return accountInfo.AvailableBalance
}

// WaitForOrderSettlement 等待订单处理完成
func (s *BaseSuite) WaitForOrderSettlement() {
	time.Sleep(2 * time.Second)
}

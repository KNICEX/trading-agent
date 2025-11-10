package backtest

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// 编译时检查接口实现
var _ exchange.Service = (*ExchangeService)(nil)

type ExchangeService struct {
	klineProvider KlineProvider // K线数据提供者
	startTime     time.Time
	endTime       time.Time

	// 每个交易对的当前时间（从K线更新）
	timeMu       sync.RWMutex
	currentTimes map[string]time.Time // key: tradingPair symbol

	// 模拟交易状态
	orderMu       sync.RWMutex
	orders        map[exchange.OrderId]*exchange.OrderInfo // 所有订单（含止盈止损）
	pendingOrders map[exchange.OrderId]*exchange.OrderInfo // 待成交订单（挂单）
	nextOrderId   int64

	positionMu sync.RWMutex
	positions  map[string]*exchange.Position // key: tradingPair_positionSide

	accountMu sync.RWMutex
	account   *exchange.AccountInfo

	// 持仓历史记录
	historyMu         sync.RWMutex
	positionHistories []exchange.PositionHistory
	// 当前持仓对应的历史记录（用于增量更新）
	activeHistories map[string]*exchange.PositionHistory // key: tradingPair_positionSide

	// 杠杆配置（每个交易对独立配置）
	leverageMu sync.RWMutex
	leverages  map[string]int // key: tradingPair symbol, default: 1

	// 当前市场价格（从K线更新）
	priceMu       sync.RWMutex
	currentPrices map[string]decimal.Decimal // key: tradingPair symbol

	// 冻结资金（开仓挂单占用）
	frozenFunds map[exchange.OrderId]decimal.Decimal // 每个开仓挂单冻结的资金
}

// NewExchangeService 使用自定义K线提供者创建服务
func NewExchangeService(startTime, endTime time.Time, initialBalance decimal.Decimal, provider KlineProvider) *ExchangeService {
	svc := &ExchangeService{
		klineProvider: provider,
		startTime:     startTime,
		endTime:       endTime,

		// 初始化模拟交易状态
		orders:        make(map[exchange.OrderId]*exchange.OrderInfo),
		pendingOrders: make(map[exchange.OrderId]*exchange.OrderInfo),

		nextOrderId: 1,
		positions:   make(map[string]*exchange.Position),
		account: &exchange.AccountInfo{
			TotalBalance:     initialBalance,
			AvailableBalance: initialBalance,
			UnrealizedPnl:    decimal.Zero,
			UsedMargin:       decimal.Zero,
		},
		positionHistories: []exchange.PositionHistory{},
		activeHistories:   make(map[string]*exchange.PositionHistory),
		leverages:         make(map[string]int),
		currentPrices:     make(map[string]decimal.Decimal),
		currentTimes:      make(map[string]time.Time),
		frozenFunds:       make(map[exchange.OrderId]decimal.Decimal),
	}

	return svc
}

// now 返回指定交易对的当前时间（从K线更新）
func (svc *ExchangeService) now(tradingPair exchange.TradingPair) time.Time {
	svc.timeMu.RLock()
	defer svc.timeMu.RUnlock()

	if t, exists := svc.currentTimes[tradingPair.ToString()]; exists {
		return t
	}
	return svc.startTime
}

// updateTime 更新交易对的当前时间
func (svc *ExchangeService) updateTime(tradingPair exchange.TradingPair, t time.Time) {
	svc.timeMu.Lock()
	defer svc.timeMu.Unlock()
	svc.currentTimes[tradingPair.ToString()] = t
}

func (svc *ExchangeService) Ticker(ctx context.Context, tradingPair exchange.TradingPair) (decimal.Decimal, error) {
	svc.priceMu.RLock()
	defer svc.priceMu.RUnlock()

	price, exists := svc.currentPrices[tradingPair.ToString()]
	if !exists {
		return decimal.Zero, fmt.Errorf("no price data for %s", tradingPair.ToString())
	}

	return price, nil
}

// updatePrice 更新交易对的当前价格（由K线数据驱动）
func (svc *ExchangeService) updatePrice(tradingPair exchange.TradingPair, price decimal.Decimal) {
	svc.priceMu.Lock()
	defer svc.priceMu.Unlock()
	svc.currentPrices[tradingPair.ToString()] = price
}

func (svc *ExchangeService) SubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) (chan exchange.Kline, error) {
	ch := make(chan exchange.Kline)

	// 🔑 优化：分批获取K线（每批200根），避免单次请求过大
	go func() {
		defer close(ch)

		const batchSize = 200 // 每批获取200根K线
		currentTime := svc.startTime
		totalKlines := 0

		for currentTime.Before(svc.endTime) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 计算当前批次的结束时间（最多200根K线）
			batchEndTime := currentTime.Add(interval.Duration() * batchSize)
			if batchEndTime.After(svc.endTime) {
				batchEndTime = svc.endTime
			}

			// 获取一批K线数据（使用K线提供者）
			klines, err := svc.klineProvider.GetKlines(ctx, exchange.GetKlinesReq{
				TradingPair: tradingPair,
				Interval:    interval,
				StartTime:   currentTime,
				EndTime:     batchEndTime,
			})

			if err != nil {
				fmt.Printf("failed to get klines for %s: %v\n", tradingPair.ToString(), err)
				return
			}

			if len(klines) == 0 {
				// 当前批次没有数据，跳到下一批
				currentTime = batchEndTime
				continue
			}

			totalKlines += len(klines)

			// 逐根推送K线
			for _, kline := range klines {
				select {
				case <-ctx.Done():
					return
				default:
				}

				time.Sleep(time.Millisecond * 10)
				// 更新当前价格为K线收盘价（用于市价单成交）
				svc.updatePrice(tradingPair, kline.Close)

				// 更新该交易对的当前时间
				svc.updateTime(tradingPair, kline.CloseTime)

				// 🔑 更新持仓的未实现盈亏和标记价格
				svc.updatePositionsPnl(tradingPair, kline.Close)

				// 🔑 第一次扫描：检查上一根K线后创建的订单
				// 检查挂单是否成交，检查止盈止损是否触发
				svc.scanOrders(ctx, tradingPair, kline)

				// 推送K线
				select {
				case ch <- kline:
				case <-ctx.Done():
					return
				}

			}

			// 移动到下一批
			currentTime = batchEndTime
		}

		fmt.Printf("loaded total %d klines for %s (%s)\n",
			totalKlines, tradingPair.ToString(), interval.ToString())
	}()

	return ch, nil
}

func (svc *ExchangeService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	klines, err := svc.klineProvider.GetKlines(ctx, req)
	if err != nil {
		return nil, err
	}
	// 转换K线数据
	result := make([]exchange.Kline, len(klines))
	for i, k := range klines {
		result[i] = exchange.Kline{
			OpenTime:         k.OpenTime,
			CloseTime:        k.CloseTime,
			Open:             k.Open,
			Close:            k.Close,
			High:             k.High,
			Low:              k.Low,
			Volume:           k.Volume,
			QuoteAssetVolume: k.QuoteAssetVolume,
		}
	}

	return result, nil
}

// ============ OrderService 实现 ============

func (svc *ExchangeService) generateOrderId() exchange.OrderId {
	svc.orderMu.Lock()
	defer svc.orderMu.Unlock()
	id := svc.nextOrderId
	svc.nextOrderId++
	return exchange.OrderId(strconv.FormatInt(id, 10))
}

func (svc *ExchangeService) getPositionKey(pair exchange.TradingPair, side exchange.PositionSide) string {
	return fmt.Sprintf("%s_%s", pair.ToString(), side)
}

// getLeverage 获取交易对的杠杆倍数（默认为1）
func (svc *ExchangeService) getLeverage(pair exchange.TradingPair) int {
	svc.leverageMu.RLock()
	defer svc.leverageMu.RUnlock()

	if leverage, exists := svc.leverages[pair.ToString()]; exists {
		return leverage
	}
	return 1 // 默认1倍杠杆
}

// ============ Service 接口实现 ============

func (svc *ExchangeService) MarketService() exchange.MarketService {
	return svc
}

func (svc *ExchangeService) PositionService() exchange.PositionService {
	return svc
}

func (svc *ExchangeService) AccountService() exchange.AccountService {
	return svc
}

func (svc *ExchangeService) OrderService() exchange.OrderService {
	return svc
}

// ============ 持仓未实现盈亏更新 ============

// updatePositionsPnl 更新指定交易对的持仓未实现盈亏和标记价格
func (svc *ExchangeService) updatePositionsPnl(tradingPair exchange.TradingPair, markPrice decimal.Decimal) {
	svc.positionMu.Lock()
	defer svc.positionMu.Unlock()

	for key, position := range svc.positions {
		// 只更新当前交易对的持仓
		if position.TradingPair != tradingPair {
			continue
		}

		// 更新标记价格
		position.MarkPrice = markPrice

		// 计算未实现盈亏
		if position.PositionSide == exchange.PositionSideLong {
			// 多头：(当前价 - 入场价) * 数量
			position.UnrealizedPnl = markPrice.Sub(position.EntryPrice).Mul(position.Quantity)
		} else {
			// 空头：(入场价 - 当前价) * 数量
			position.UnrealizedPnl = position.EntryPrice.Sub(markPrice).Mul(position.Quantity)
		}

		svc.positions[key] = position
	}
}

// ============ 订单扫描机制 ============

// scanOrders 扫描所有待成交订单
// 在每次K线推送时调用
func (svc *ExchangeService) scanOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
	// 扫描待成交的挂单
	svc.scanPendingOrders(ctx, tradingPair, kline)
}

// scanPendingOrders 扫描待成交订单，检查是否满足成交条件
func (svc *ExchangeService) scanPendingOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
	svc.orderMu.RLock()
	// 复制一份待扫描的订单列表（避免在锁内执行耗时操作）
	pendingList := make([]*exchange.OrderInfo, 0, len(svc.pendingOrders))
	for _, order := range svc.pendingOrders {
		// 只扫描当前K线对应的交易对
		if order.TradingPair == tradingPair {
			pendingList = append(pendingList, order)
		}
	}
	svc.orderMu.RUnlock()

	// 检查每个订单是否满足成交条件
	for _, order := range pendingList {
		if svc.checkOrderFilled(order, kline) {
			// 订单满足成交条件，执行成交
			svc.fillOrder(ctx, order, kline)
		}
	}
}

// checkOrderFilled 检查订单是否满足成交条件
// 根据 OrderType + PositionSide 组合判断订单方向：
// - Open + Long (开多)：买入，K线最低价 <= 限价时成交
// - Open + Short (开空)：卖出，K线最高价 >= 限价时成交
// - Close + Long (平多)：卖出，K线最高价 >= 限价时成交
// - Close + Short (平空)：买入，K线最低价 <= 限价时成交
func (svc *ExchangeService) checkOrderFilled(order *exchange.OrderInfo, kline exchange.Kline) bool {
	// 市价单，立即成交
	if order.Price.IsZero() {
		return true
	}

	// 限价单：判断K线价格区间是否触碰到限价
	isBuyOrder := (order.OrderType == exchange.OrderTypeOpen && order.PositionSide == exchange.PositionSideLong) ||
		(order.OrderType == exchange.OrderTypeClose && order.PositionSide == exchange.PositionSideShort)

	if isBuyOrder {
		// 买入订单：K线最低价触及或低于限价时成交
		return kline.Low.LessThanOrEqual(order.Price)
	} else {
		// 卖出订单：K线最高价触及或高于限价时成交
		return kline.High.GreaterThanOrEqual(order.Price)
	}
}

// fillOrder 执行订单成交
func (svc *ExchangeService) fillOrder(ctx context.Context, order *exchange.OrderInfo, kline exchange.Kline) error {
	// 确定成交价格
	fillPrice := order.Price
	if fillPrice.IsZero() {
		// 市价单使用当前K线开盘价
		fillPrice = kline.Open
	}
	// 否则使用限价单的挂单价格成交

	// 执行持仓变更
	posKey := svc.getPositionKey(order.TradingPair, order.PositionSide)

	var executedQuantity decimal.Decimal
	var err error

	if order.OrderType == exchange.OrderTypeOpen {
		// 开仓或加仓（可能部分成交）
		executedQuantity, err = svc.openPosition(posKey, order, fillPrice)
		if err != nil {
			return err
		}
	} else {
		// 平仓或减仓
		err = svc.closePosition(posKey, order, fillPrice)
		if err != nil {
			return err
		}
		executedQuantity = order.Quantity
	}

	// 更新订单状态
	svc.orderMu.Lock()

	// 从待成交列表移除
	delete(svc.pendingOrders, exchange.OrderId(order.Id))

	// 更新订单状态和成交数量
	order.ExecutedQuantity = executedQuantity
	if executedQuantity.Equal(order.Quantity) {
		order.Status = exchange.OrderStatusFilled
	} else {
		order.Status = exchange.OrderStatusPartiallyFilled
	}
	now := svc.now(order.TradingPair)
	order.UpdatedAt = now
	order.CompletedAt = now

	svc.orderMu.Unlock()

	return nil
}

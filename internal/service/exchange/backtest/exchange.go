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
var _ exchangeService = (*ExchangeService)(nil)

type ExchangeService struct {
	klineProvider KlineProvider // K线数据提供者
	startTime     time.Time
	endTime       time.Time

	// 每个交易对的当前时间（从K线更新）
	timeMu       sync.RWMutex
	currentTimes map[string]time.Time // key: tradingPair symbol

	// 模拟交易状态
	orderMu       sync.RWMutex
	orders        map[exchange.OrderId]*OrderInfo     // 所有订单（含止盈止损）
	pendingOrders map[exchange.OrderId]*OrderInfo     // 待成交订单（挂单）
	stopOrders    map[exchange.OrderId]*StopOrderInfo // 止盈止损订单
	nextOrderId   int64

	// 待设置的止盈止损订单（key: 开仓订单ID）
	pendingStopOrders map[exchange.OrderId]*PendingStopOrders

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

	// 冻结持仓数量（平仓挂单占用）
	frozenPositions map[exchange.OrderId]decimal.Decimal // 每个平仓挂单冻结的持仓数量
}

// NewExchangeService 使用自定义K线提供者创建服务
func NewExchangeService(startTime, endTime time.Time, initialBalance decimal.Decimal, provider KlineProvider) *ExchangeService {
	svc := &ExchangeService{
		klineProvider: provider,
		startTime:     startTime,
		endTime:       endTime,

		// 初始化模拟交易状态
		orders:            make(map[exchange.OrderId]*OrderInfo),
		pendingOrders:     make(map[exchange.OrderId]*OrderInfo),
		stopOrders:        make(map[exchange.OrderId]*StopOrderInfo),
		pendingStopOrders: make(map[exchange.OrderId]*PendingStopOrders),
		nextOrderId:       1,
		positions:         make(map[string]*exchange.Position),
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
		frozenPositions:   make(map[exchange.OrderId]decimal.Decimal),
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

				// 🔑 第二次扫描：处理基于当前K线创建的订单
				// 这样可以确保外部协程在收到K线后立即下单，订单能在当前K线被扫描到
				// 避免订单延迟到下一根K线才被处理
				svc.scanOrders(ctx, tradingPair, kline)
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

func (svc *ExchangeService) TradingService() exchange.TradingService {
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

// scanOrders 扫描所有待成交订单和止盈止损订单
// 在每次K线推送时调用
func (svc *ExchangeService) scanOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
	fmt.Printf("[DEBUG] scanOrders: pair=%s, price=%s\n", tradingPair.ToString(), kline.Close)

	// 1. 扫描待成交的挂单
	svc.scanPendingOrders(ctx, tradingPair, kline)

	// 2. 扫描止盈止损订单
	svc.scanStopOrders(ctx, tradingPair, kline)
}

// scanPendingOrders 扫描待成交订单，检查是否满足成交条件
func (svc *ExchangeService) scanPendingOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
	svc.orderMu.RLock()
	// 复制一份待扫描的订单列表（避免在锁内执行耗时操作）
	pendingList := make([]*OrderInfo, 0, len(svc.pendingOrders))
	for _, order := range svc.pendingOrders {
		// 只扫描当前K线对应的交易对
		if order.OrderInfo.TradingPair == tradingPair {
			pendingList = append(pendingList, order)
		}
	}
	svc.orderMu.RUnlock()

	fmt.Printf("[DEBUG] scanPendingOrders: 待扫描订单数=%d\n", len(pendingList))

	// 检查每个订单是否满足成交条件
	for _, order := range pendingList {
		fmt.Printf("[DEBUG] 检查订单 %s: 价格=%s, 市价=%v\n", order.Id, order.Price, order.Price.IsZero())
		if svc.checkOrderFilled(order, kline) {
			// 订单满足成交条件，执行成交
			fmt.Printf("[DEBUG] 订单 %s 满足成交条件，执行成交\n", order.Id)
			svc.fillOrder(ctx, order, kline)
		}
	}
}

// checkOrderFilled 检查订单是否满足成交条件
func (svc *ExchangeService) checkOrderFilled(order *OrderInfo, kline exchange.Kline) bool {
	// 限价单逻辑：
	// - 买单：当K线最低价 <= 限价，则成交
	// - 卖单：当K线最高价 >= 限价，则成交

	if order.Price.IsZero() {
		// 市价单，立即成交
		return true
	}

	if order.Side == exchange.OrderSideBuy {
		// 买单：K线最低价触及或低于限价
		return kline.Low.LessThanOrEqual(order.Price)
	} else {
		// 卖单：K线最高价触及或高于限价
		return kline.High.GreaterThanOrEqual(order.Price)
	}
}

// fillOrder 执行订单成交
func (svc *ExchangeService) fillOrder(ctx context.Context, order *OrderInfo, kline exchange.Kline) error {
	// 更新订单状态
	svc.orderMu.Lock()

	// 从待成交列表移除
	delete(svc.pendingOrders, exchange.OrderId(order.Id))

	// 更新订单状态为已成交
	order.Status = exchange.OrderStatusFilled
	order.ExecutedQuantity = order.Quantity
	now := svc.now(order.OrderInfo.TradingPair)
	order.UpdatedAt = now
	order.CompletedAt = now

	// 确定成交价格
	fillPrice := order.Price
	if fillPrice.IsZero() {
		// 市价单使用当前K线收盘价
		fillPrice = kline.Close
	}

	svc.orderMu.Unlock()

	// 执行持仓变更
	posKey := svc.getPositionKey(order.OrderInfo.TradingPair, order.PositionSide)

	if order.OrderType == exchange.OrderTypeOpen {
		// 开仓或加仓
		return svc.openPosition(posKey, order, fillPrice)
	} else {
		// 平仓或减仓
		return svc.closePosition(posKey, order, fillPrice)
	}
}

// scanStopOrders 扫描止盈止损订单
func (svc *ExchangeService) scanStopOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
	svc.orderMu.RLock()
	// 复制一份待扫描的止盈止损订单列表
	stopList := make([]*StopOrderInfo, 0, len(svc.stopOrders))
	for _, stopOrder := range svc.stopOrders {
		// 只扫描当前K线对应的交易对
		if stopOrder.TradingPair == tradingPair {
			stopList = append(stopList, stopOrder)
		}
	}
	svc.orderMu.RUnlock()

	// 检查每个止盈止损订单是否触发
	for _, stopOrder := range stopList {
		if svc.checkStopOrderTriggered(stopOrder, kline) {
			// 止盈止损触发，执行平仓
			svc.triggerStopOrder(ctx, stopOrder, kline)
		}
	}
}

// checkStopOrderTriggered 检查止盈止损订单是否触发
func (svc *ExchangeService) checkStopOrderTriggered(stopOrder *StopOrderInfo, kline exchange.Kline) bool {
	// 止盈止损触发逻辑：
	// 多头持仓：
	//   - 止盈：价格上涨到触发价 (high >= trigger)
	//   - 止损：价格下跌到触发价 (low <= trigger)
	// 空头持仓：
	//   - 止盈：价格下跌到触发价 (low <= trigger)
	//   - 止损：价格上涨到触发价 (high >= trigger)

	if stopOrder.StopType == StopOrderTypeTakeProfit {
		// 止盈订单
		if stopOrder.PositionSide == exchange.PositionSideLong {
			// 多头止盈：价格上涨触发
			return kline.High.GreaterThanOrEqual(stopOrder.TriggerPrice)
		} else {
			// 空头止盈：价格下跌触发
			return kline.Low.LessThanOrEqual(stopOrder.TriggerPrice)
		}
	} else {
		// 止损订单
		if stopOrder.PositionSide == exchange.PositionSideLong {
			// 多头止损：价格下跌触发
			return kline.Low.LessThanOrEqual(stopOrder.TriggerPrice)
		} else {
			// 空头止损：价格上涨触发
			return kline.High.GreaterThanOrEqual(stopOrder.TriggerPrice)
		}
	}
}

// triggerStopOrder 触发止盈止损订单
func (svc *ExchangeService) triggerStopOrder(ctx context.Context, stopOrder *StopOrderInfo, kline exchange.Kline) error {
	// 从止盈止损列表移除当前订单
	svc.orderMu.Lock()
	delete(svc.stopOrders, stopOrder.Id)

	// 🔑 同时删除该持仓的其他止盈止损订单（止盈触发后删除止损，止损触发后删除止盈）
	for id, otherStopOrder := range svc.stopOrders {
		if otherStopOrder.PositionKey == stopOrder.PositionKey && id != stopOrder.Id {
			delete(svc.stopOrders, id)
		}
	}
	svc.orderMu.Unlock()

	// 获取持仓
	posKey := stopOrder.PositionKey
	svc.positionMu.RLock()
	position, exists := svc.positions[posKey]
	svc.positionMu.RUnlock()

	if !exists {
		// 持仓已不存在（可能已被其他订单平仓）
		return nil
	}

	// 计算平仓数量（使用当前实际持仓数量，避免过度平仓）
	quantity := stopOrder.Quantity
	if quantity.IsZero() || quantity.GreaterThan(position.Quantity) {
		quantity = position.Quantity // 全平或调整为实际数量
	}

	// 创建一个虚拟订单信息（用于记录）
	orderId := svc.generateOrderId()
	now := svc.now(stopOrder.TradingPair)

	order := &OrderInfo{
		OrderInfo: exchange.OrderInfo{
			Id:               orderId.ToString(),
			TradingPair:      stopOrder.TradingPair,
			Side:             stopOrder.OrderSide, // BUY或SELL
			Price:            stopOrder.TriggerPrice,
			Quantity:         quantity,
			ExecutedQuantity: quantity,
			Status:           exchange.OrderStatusFilled, // 立即标记为已成交
			CreatedAt:        now,
			UpdatedAt:        now,
			CompletedAt:      now,
		},
		OrderType:    exchange.OrderTypeClose,
		PositionSide: stopOrder.PositionSide,
	}

	// 保存订单记录（用于历史查询）
	svc.orderMu.Lock()
	svc.orders[orderId] = order
	svc.orderMu.Unlock()

	// 🔑 直接执行平仓，不创建挂单
	return svc.closePosition(posKey, order, stopOrder.TriggerPrice)
}

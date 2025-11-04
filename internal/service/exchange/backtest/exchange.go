package backtest

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

// 编译时检查接口实现
var _ exchange.Service = (*BinanceExchangeService)(nil)
var _ ExchangeService = (*BinanceExchangeService)(nil)

type BinanceExchangeService struct {
	cli       *futures.Client
	startTime time.Time
	endTime   time.Time

	clockMu sync.RWMutex
	clock   time.Time

	clockUpdateCallbacks []func(t time.Time)
	timeMultiplier       int

	// 模拟交易状态
	orderMu       sync.RWMutex
	orders        map[exchange.OrderId]*OrderInfo     // 所有订单（含止盈止损）
	pendingOrders map[exchange.OrderId]*OrderInfo     // 待成交订单（挂单）
	stopOrders    map[exchange.OrderId]*StopOrderInfo // 止盈止损订单
	nextOrderId   int64

	positionMu sync.RWMutex
	positions  map[string]*exchange.Position // key: tradingPair_positionSide

	accountMu sync.RWMutex
	account   *exchange.AccountInfo

	// 交易历史
	positionHistories []exchange.PositionHistory

	// 当前市场价格（从K线更新）
	priceMu       sync.RWMutex
	currentPrices map[string]decimal.Decimal // key: tradingPair symbol
}

func NewBinanceExchangeService(cli *futures.Client, startTime, endTime time.Time, timeMultiplier int, initialBalance decimal.Decimal) *BinanceExchangeService {
	svc := &BinanceExchangeService{
		cli:                  cli,
		startTime:            startTime,
		endTime:              endTime,
		timeMultiplier:       timeMultiplier,
		clock:                startTime,
		clockUpdateCallbacks: []func(t time.Time){},

		// 初始化模拟交易状态
		orders:        make(map[exchange.OrderId]*OrderInfo),
		pendingOrders: make(map[exchange.OrderId]*OrderInfo),
		stopOrders:    make(map[exchange.OrderId]*StopOrderInfo),
		nextOrderId:   1,
		positions:     make(map[string]*exchange.Position),
		account: &exchange.AccountInfo{
			TotalBalance:     initialBalance,
			AvailableBalance: initialBalance,
			UnrealizedPnl:    decimal.Zero,
			UsedMargin:       decimal.Zero,
		},
		positionHistories: []exchange.PositionHistory{},
		currentPrices:     make(map[string]decimal.Decimal),
	}
	svc.clockLoop()
	return svc
}

func (svc *BinanceExchangeService) now() time.Time {
	svc.clockMu.RLock()
	defer svc.clockMu.RUnlock()
	return svc.clock
}

func (svc *BinanceExchangeService) updateClock(t time.Time) {
	svc.clockMu.Lock()
	defer svc.clockMu.Unlock()
	svc.clock = t
	go func() {
		for _, callback := range svc.clockUpdateCallbacks {
			callback(t)
		}
	}()
}

func (svc *BinanceExchangeService) onClockUpdate(callback func(t time.Time)) {
	svc.clockUpdateCallbacks = append(svc.clockUpdateCallbacks, callback)
}

// clockLoop 定时更新clock
func (svc *BinanceExchangeService) clockLoop() {
	startTime := svc.startTime
	go func() {
		baseInterval := time.Millisecond * 100
		for range time.Tick(baseInterval) {
			svc.updateClock(startTime.Add(baseInterval * time.Duration(svc.timeMultiplier)))
		}
	}()
}

func (svc *BinanceExchangeService) Ticker(ctx context.Context, tradingPair exchange.TradingPair) (decimal.Decimal, error) {
	svc.priceMu.RLock()
	defer svc.priceMu.RUnlock()

	price, exists := svc.currentPrices[tradingPair.ToString()]
	if !exists {
		return decimal.Zero, fmt.Errorf("no price data for %s", tradingPair.ToString())
	}

	return price, nil
}

// updatePrice 更新交易对的当前价格（由K线数据驱动）
func (svc *BinanceExchangeService) updatePrice(tradingPair exchange.TradingPair, price decimal.Decimal) {
	svc.priceMu.Lock()
	defer svc.priceMu.Unlock()
	svc.currentPrices[tradingPair.ToString()] = price
}

func (svc *BinanceExchangeService) SubscribeKline(ctx context.Context, tradingPair exchange.TradingPair, interval exchange.Interval) (chan exchange.Kline, error) {
	ch := make(chan exchange.Kline, 10)

	svc.onClockUpdate(func(t time.Time) {

		if t.Unix()%int64(interval.Duration().Seconds()) > 10 {
			// 误差大于10秒，跳过
			return
		}

		closeTime := t.Truncate(interval.Duration())
		openTime := closeTime.Add(-interval.Duration())

		klines, err := svc.GetKlines(ctx, exchange.GetKlinesReq{
			TradingPair: tradingPair,
			Interval:    interval,
			StartTime:   openTime,
			EndTime:     closeTime,
		})
		if err != nil {
			fmt.Println("get klines error", err)
			return
		}
		if len(klines) == 0 {
			fmt.Println("no klines found for ", openTime, " to ", closeTime)
			return
		}

		kline := klines[0]

		// 更新当前价格为K线收盘价（用于市价单成交）
		svc.updatePrice(tradingPair, kline.Close)

		// 🔑 关键：在推送K线前扫描所有订单
		// 检查挂单是否成交，检查止盈止损是否触发
		svc.scanOrders(ctx, tradingPair, kline)

		// 推送K线
		ch <- kline
	})
	return ch, nil
}

func (svc *BinanceExchangeService) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	klines, err := svc.cli.NewKlinesService().
		Symbol(req.TradingPair.ToString()).
		Interval(req.Interval.ToString()).
		StartTime(req.StartTime.UnixMilli()).
		EndTime(req.EndTime.UnixMilli()).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	// 转换K线数据
	result := make([]exchange.Kline, len(klines))
	for i, k := range klines {
		klineOpen, _ := decimal.NewFromString(k.Open)
		klineClose, _ := decimal.NewFromString(k.Close)
		klineHigh, _ := decimal.NewFromString(k.High)
		klineLow, _ := decimal.NewFromString(k.Low)
		klineVolume, _ := decimal.NewFromString(k.Volume)
		klineQuoteAssetVolume, _ := decimal.NewFromString(k.QuoteAssetVolume)

		result[i] = exchange.Kline{
			OpenTime:         time.UnixMilli(k.OpenTime),
			CloseTime:        time.UnixMilli(k.CloseTime),
			Open:             klineOpen,
			Close:            klineClose,
			High:             klineHigh,
			Low:              klineLow,
			Volume:           klineVolume,
			QuoteAssetVolume: klineQuoteAssetVolume,
		}
	}

	return result, nil
}

// ============ OrderService 实现 ============

func (svc *BinanceExchangeService) generateOrderId() exchange.OrderId {
	svc.orderMu.Lock()
	defer svc.orderMu.Unlock()
	id := svc.nextOrderId
	svc.nextOrderId++
	return exchange.OrderId(strconv.FormatInt(id, 10))
}

func (svc *BinanceExchangeService) getPositionKey(pair exchange.TradingPair, side exchange.PositionSide) string {
	return fmt.Sprintf("%s_%s", pair.ToString(), side)
}

// ============ Service 接口实现 ============

func (svc *BinanceExchangeService) MarketService() exchange.MarketService {
	return svc
}

func (svc *BinanceExchangeService) PositionService() exchange.PositionService {
	return svc
}

func (svc *BinanceExchangeService) AccountService() exchange.AccountService {
	return svc
}

func (svc *BinanceExchangeService) OrderService() exchange.OrderService {
	return svc
}

func (svc *BinanceExchangeService) TradingService() exchange.TradingService {
	return svc
}

// ============ 订单扫描机制 ============

// scanOrders 扫描所有待成交订单和止盈止损订单
// 在每次K线推送时调用
func (svc *BinanceExchangeService) scanOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
	// 1. 扫描待成交的挂单
	svc.scanPendingOrders(ctx, tradingPair, kline)

	// 2. 扫描止盈止损订单
	svc.scanStopOrders(ctx, tradingPair, kline)
}

// scanPendingOrders 扫描待成交订单，检查是否满足成交条件
func (svc *BinanceExchangeService) scanPendingOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
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

	// 检查每个订单是否满足成交条件
	for _, order := range pendingList {
		if svc.checkOrderFilled(order, kline) {
			// 订单满足成交条件，执行成交
			svc.fillOrder(ctx, order, kline)
		}
	}
}

// checkOrderFilled 检查订单是否满足成交条件
func (svc *BinanceExchangeService) checkOrderFilled(order *OrderInfo, kline exchange.Kline) bool {
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
func (svc *BinanceExchangeService) fillOrder(ctx context.Context, order *OrderInfo, kline exchange.Kline) error {
	// 更新订单状态
	svc.orderMu.Lock()

	// 从待成交列表移除
	delete(svc.pendingOrders, exchange.OrderId(order.Id))

	// 更新订单状态为已成交
	order.Status = exchange.OrderStatusFilled
	order.ExecutedQuantity = order.Quantity
	now := svc.now()
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
func (svc *BinanceExchangeService) scanStopOrders(ctx context.Context, tradingPair exchange.TradingPair, kline exchange.Kline) {
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
func (svc *BinanceExchangeService) checkStopOrderTriggered(stopOrder *StopOrderInfo, kline exchange.Kline) bool {
	// 止盈止损触发逻辑：
	// - 止盈（卖）：多头，当价格 >= 触发价
	// - 止损（卖）：多头，当价格 <= 触发价
	// - 止盈（买）：空头，当价格 <= 触发价
	// - 止损（买）：空头，当价格 >= 触发价

	// 简化逻辑：使用K线的最高价和最低价判断
	// 如果触发价在K线范围内，则触发

	if stopOrder.PositionSide == exchange.PositionSideLong {
		// 多头持仓
		// 止盈：价格上涨到触发价 (high >= trigger)
		// 止损：价格下跌到触发价 (low <= trigger)
		return kline.High.GreaterThanOrEqual(stopOrder.TriggerPrice) ||
			kline.Low.LessThanOrEqual(stopOrder.TriggerPrice)
	} else {
		// 空头持仓
		// 止盈：价格下跌到触发价 (low <= trigger)
		// 止损：价格上涨到触发价 (high >= trigger)
		return kline.Low.LessThanOrEqual(stopOrder.TriggerPrice) ||
			kline.High.GreaterThanOrEqual(stopOrder.TriggerPrice)
	}
}

// triggerStopOrder 触发止盈止损订单
func (svc *BinanceExchangeService) triggerStopOrder(ctx context.Context, stopOrder *StopOrderInfo, kline exchange.Kline) error {
	// 从止盈止损列表移除
	svc.orderMu.Lock()
	delete(svc.stopOrders, stopOrder.Id)
	svc.orderMu.Unlock()

	// 获取持仓
	posKey := stopOrder.PositionKey
	svc.positionMu.RLock()
	position, exists := svc.positions[posKey]
	svc.positionMu.RUnlock()

	if !exists {
		return fmt.Errorf("position not found: %s", posKey)
	}

	// 计算平仓数量
	quantity := stopOrder.Quantity
	if quantity.IsZero() {
		quantity = position.Quantity // 全平
	}

	// 创建一个虚拟订单信息（用于记录）
	orderId := svc.generateOrderId()
	now := svc.now()

	order := &OrderInfo{
		OrderInfo: exchange.OrderInfo{
			Id:               orderId.ToString(),
			TradingPair:      stopOrder.TradingPair,
			Side:             stopOrder.Type, // BUY或SELL
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

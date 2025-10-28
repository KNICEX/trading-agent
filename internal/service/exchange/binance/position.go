package binance

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

var _ exchange.PositionService = (*PositonService)(nil)

type PositonService struct {
	cli *futures.Client
}

// NewPositionService 创建持仓服务
func NewPositionService(cli *futures.Client) *PositonService {
	return &PositonService{cli: cli}
}

func (p *PositonService) SetLeverage(ctx context.Context, req exchange.SetLeverageReq) error {
	svc := p.cli.NewChangeLeverageService()
	svc.Symbol(req.TradingPair.ToString())
	svc.Leverage(req.Leverage)
	_, err := svc.Do(ctx)
	return err
}

// GetActivePositions 获取所有持仓
// notice: 币安有挂单，未成交的仓位也会返回，需要过滤掉
func (p *PositonService) GetActivePositions(ctx context.Context, pairs []exchange.TradingPair) ([]exchange.Position, error) {
	var binancePositions []*futures.PositionRisk
	var err error

	if len(pairs) == 0 {
		binancePositions, err = p.cli.NewGetPositionRiskService().Do(ctx)
	} else {
		for _, pair := range pairs {
			ps, err := p.cli.NewGetPositionRiskService().Symbol(pair.ToString()).Do(ctx)
			if err != nil {
				return nil, err
			}
			binancePositions = append(binancePositions, ps...)
		}
	}
	if err != nil {
		return nil, err
	}
	positions := make([]exchange.Position, 0, len(binancePositions))
	for _, v := range binancePositions {
		base, quote := exchange.SplitSymbol(v.Symbol)
		leverage, err := strconv.Atoi(v.Leverage)
		if err != nil {
			return nil, err
		}
		position := exchange.Position{
			TradingPair: exchange.TradingPair{
				Base:  base,
				Quote: quote,
			},
			PositionSide:     exchange.PositionSide(v.PositionSide),
			EntryPrice:       decimal.RequireFromString(v.EntryPrice),
			BreakEvenPrice:   decimal.RequireFromString(v.BreakEvenPrice),
			LiquidationPrice: decimal.RequireFromString(v.LiquidationPrice),
			MarkPrice:        decimal.RequireFromString(v.MarkPrice),
			MarginType:       exchange.MarginType(v.MarginType),
			Leverage:         leverage,
			Quantity:         decimal.RequireFromString(v.PositionAmt),
			MarginAmount:     decimal.RequireFromString(v.IsolatedMargin),
			UnrealizedPnl:    decimal.RequireFromString(v.UnRealizedProfit),
		}

		// 过滤掉未成交的仓位
		if position.Quantity.Equal(decimal.Zero) {
			continue
		}
		// TODO margin计算公式尚有疑问，需要确认
		margin := position.EntryPrice.Mul(position.Quantity).Div(decimal.NewFromInt(int64(position.Leverage))).Abs().Round(2)
		position.MarginAmount = margin
		positions = append(positions, position)
	}
	return positions, nil
}

func (p *PositonService) GetHistoryPositions(ctx context.Context, req exchange.GetHistoryPositionsReq) ([]exchange.PositionHistory, error) {
	var allHistories []exchange.PositionHistory

	// 如果 TradingPairs 为空，查询所有交易对的成交记录
	if len(req.TradingPairs) == 0 {
		// 获取所有成交记录（自动分页）
		trades, err := p.fetchAllTrades(ctx, "", req.StartTime, req.EndTime)
		if err != nil {
			return nil, fmt.Errorf("failed to get all trades: %w", err)
		}

		// 按交易对分组
		tradesBySymbol := p.groupTradesBySymbol(trades)

		// 为每个交易对推导持仓历史
		for symbol, symbolTrades := range tradesBySymbol {
			base, quote := exchange.SplitSymbol(symbol)
			pair := exchange.TradingPair{Base: base, Quote: quote}
			histories := p.derivePositionHistories(pair, symbolTrades)
			allHistories = append(allHistories, histories...)
		}
	} else {
		// 遍历每个交易对
		for _, pair := range req.TradingPairs {
			// 获取该交易对的成交记录（自动分页）
			trades, err := p.fetchAllTrades(ctx, pair.ToString(), req.StartTime, req.EndTime)
			if err != nil {
				return nil, fmt.Errorf("failed to get trades for %s: %w", pair.ToString(), err)
			}

			// 从成交记录推导持仓历史
			histories := p.derivePositionHistories(pair, trades)
			allHistories = append(allHistories, histories...)
		}
	}

	return allHistories, nil
}

// fetchAllTrades 获取所有成交记录，自动处理分页和时间分片
// 隐藏币安API的限制：
// 1. 最多查询7天的数据
// 2. 单次最多返回1000条
func (p *PositonService) fetchAllTrades(
	ctx context.Context,
	symbol string,
	startTime, endTime time.Time,
) ([]*futures.AccountTrade, error) {
	const (
		maxDays         = 7    // 币安限制最多查询7天
		limitPerRequest = 1000 // 单次请求最多1000条
		dayDuration     = 24 * time.Hour
	)

	var allTrades []*futures.AccountTrade

	// 计算时间跨度
	duration := endTime.Sub(startTime)

	// 如果时间跨度超过7天，需要分片
	if duration > maxDays*dayDuration {
		// 按7天分片
		currentStart := startTime
		for currentStart.Before(endTime) {
			currentEnd := currentStart.Add(maxDays * dayDuration)
			if currentEnd.After(endTime) {
				currentEnd = endTime
			}

			// 获取这个时间片的数据
			trades, err := p.fetchTradesWithPagination(ctx, symbol, currentStart, currentEnd)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch trades from %s to %s: %w",
					currentStart.Format("2006-01-02"), currentEnd.Format("2006-01-02"), err)
			}
			allTrades = append(allTrades, trades...)

			currentStart = currentEnd
		}
	} else {
		// 时间跨度在7天内，直接分页查询
		trades, err := p.fetchTradesWithPagination(ctx, symbol, startTime, endTime)
		if err != nil {
			return nil, err
		}
		allTrades = trades
	}

	return allTrades, nil
}

// fetchTradesWithPagination 在一个时间范围内分页获取所有成交记录
// 时间范围必须 <= 7天
func (p *PositonService) fetchTradesWithPagination(
	ctx context.Context,
	symbol string,
	startTime, endTime time.Time,
) ([]*futures.AccountTrade, error) {
	const limitPerRequest = 1000

	var allTrades []*futures.AccountTrade
	var lastTradeID int64 = 0

	for {
		// 构建请求
		svc := p.cli.NewListAccountTradeService().
			StartTime(startTime.UnixMilli()).
			EndTime(endTime.UnixMilli()).
			Limit(limitPerRequest)

		// 如果指定了交易对，添加 Symbol
		if symbol != "" {
			svc = svc.Symbol(symbol)
		}

		// 如果不是第一页，使用 FromID 分页
		if lastTradeID > 0 {
			svc = svc.FromID(lastTradeID)
		}

		// 执行请求
		trades, err := svc.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch trades: %w", err)
		}

		// 如果没有数据了，结束
		if len(trades) == 0 {
			break
		}

		// 添加到结果
		allTrades = append(allTrades, trades...)

		// 如果返回数量少于限制，说明已经是最后一页
		if len(trades) < limitPerRequest {
			break
		}

		// 更新 lastTradeID 为最后一条记录的 ID + 1
		// 币安的 FromID 是从指定ID开始（不包含该ID）
		lastTradeID = trades[len(trades)-1].ID + 1
	}

	return allTrades, nil
}

// groupTradesBySymbol 按交易对分组成交记录
func (p *PositonService) groupTradesBySymbol(trades []*futures.AccountTrade) map[string][]*futures.AccountTrade {
	grouped := make(map[string][]*futures.AccountTrade)
	for _, trade := range trades {
		grouped[trade.Symbol] = append(grouped[trade.Symbol], trade)
	}
	return grouped
}

// derivePositionHistories 从成交记录推导持仓历史
// 关键改进：识别每个独立的持仓周期（开仓→平仓为一个周期）
func (p *PositonService) derivePositionHistories(
	pair exchange.TradingPair,
	trades []*futures.AccountTrade,
) []exchange.PositionHistory {
	if len(trades) == 0 {
		return nil
	}

	// 按时间排序
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Time < trades[j].Time
	})

	var histories []exchange.PositionHistory

	// 按 PositionSide 分别处理，并识别每个独立的持仓周期
	longHistories := p.buildPositionHistoriesBySide(pair, exchange.PositionSideLong, trades)
	shortHistories := p.buildPositionHistoriesBySide(pair, exchange.PositionSideShort, trades)

	histories = append(histories, longHistories...)
	histories = append(histories, shortHistories...)

	return histories
}

// buildPositionHistoriesBySide 构建指定方向的所有持仓历史
// 识别每个独立的持仓周期：持仓归零后，下次开仓算新的持仓
func (p *PositonService) buildPositionHistoriesBySide(
	pair exchange.TradingPair,
	side exchange.PositionSide,
	allTrades []*futures.AccountTrade,
) []exchange.PositionHistory {
	// 筛选该方向的交易
	var trades []*futures.AccountTrade
	for _, trade := range allTrades {
		if (side == exchange.PositionSideLong && trade.PositionSide == futures.PositionSideTypeLong) ||
			(side == exchange.PositionSideShort && trade.PositionSide == futures.PositionSideTypeShort) {
			trades = append(trades, trade)
		}
	}

	if len(trades) == 0 {
		return nil
	}

	var histories []exchange.PositionHistory
	var currentTrades []*futures.AccountTrade
	currentPosition := decimal.Zero

	// 遍历交易，识别每个独立的持仓周期
	for _, trade := range trades {
		qty := decimal.RequireFromString(trade.Quantity)

		// 判断是开仓还是平仓
		isOpening := (side == exchange.PositionSideLong && trade.Side == futures.SideTypeBuy) ||
			(side == exchange.PositionSideShort && trade.Side == futures.SideTypeSell)

		currentTrades = append(currentTrades, trade)

		if isOpening {
			// 开仓/加仓
			currentPosition = currentPosition.Add(qty)
		} else {
			// 平仓/减仓
			currentPosition = currentPosition.Sub(qty)

			// 如果持仓归零，说明这个持仓周期结束
			if currentPosition.IsZero() || currentPosition.IsNegative() {
				// 构建这个持仓的历史
				history := p.buildPositionHistory(pair, side, currentTrades)
				histories = append(histories, history)

				// 重置，准备下一个持仓周期
				currentTrades = nil
				currentPosition = decimal.Zero
			}
		}
	}

	// 如果还有未平仓的交易（持仓未归零），也生成一个历史记录
	if len(currentTrades) > 0 {
		history := p.buildPositionHistory(pair, side, currentTrades)
		histories = append(histories, history)
	}

	return histories
}

// buildPositionHistory 构建单个持仓的历史记录
func (p *PositonService) buildPositionHistory(
	pair exchange.TradingPair,
	side exchange.PositionSide,
	trades []*futures.AccountTrade,
) exchange.PositionHistory {
	if len(trades) == 0 {
		return exchange.PositionHistory{}
	}

	// 按时间排序（确保按时间顺序处理）
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Time < trades[j].Time
	})

	var events []exchange.PositionEvent
	currentPosition := decimal.Zero
	totalEntryValue := decimal.Zero
	entryQuantity := decimal.Zero
	maxQuantity := decimal.Zero

	var openTime, closeTime time.Time

	for _, trade := range trades {
		price := decimal.RequireFromString(trade.Price)
		qty := decimal.RequireFromString(trade.Quantity)
		realizedPnl := decimal.RequireFromString(trade.RealizedPnl)
		fee := decimal.RequireFromString(trade.Commission)

		beforeAmount := currentPosition

		// 判断是开仓还是平仓
		// 多头：买入是开仓，卖出是平仓
		// 空头：卖出是开仓，买入是平仓
		isOpening := (side == exchange.PositionSideLong && trade.Side == futures.SideTypeBuy) ||
			(side == exchange.PositionSideShort && trade.Side == futures.SideTypeSell)

		var eventType exchange.PositionEventType

		if isOpening {
			// 开仓/加仓
			if currentPosition.IsZero() {
				eventType = exchange.PositionEventTypeCreate
				openTime = time.UnixMilli(trade.Time)
			} else {
				eventType = exchange.PositionEventTypeIncrease
			}
			currentPosition = currentPosition.Add(qty)
			totalEntryValue = totalEntryValue.Add(price.Mul(qty))
			entryQuantity = entryQuantity.Add(qty)
		} else {
			// 平仓/减仓
			currentPosition = currentPosition.Sub(qty)
			if currentPosition.IsZero() || currentPosition.IsNegative() {
				eventType = exchange.PositionEventTypeClose
				closeTime = time.UnixMilli(trade.Time)
				currentPosition = decimal.Zero // 确保不为负数
			} else {
				eventType = exchange.PositionEventTypeDecrease
			}
		}

		afterAmount := currentPosition

		// 记录最大持仓量
		if currentPosition.GreaterThan(maxQuantity) {
			maxQuantity = currentPosition
		}

		// 创建事件
		event := exchange.PositionEvent{
			OrderId:        exchange.OrderId(strconv.FormatInt(trade.OrderID, 10)),
			EventType:      eventType,
			Quantity:       qty,
			BeforeQuantity: beforeAmount,
			AfterQuantity:  afterAmount,
			Price:          price,
			RealizedPnl:    realizedPnl,
			Fee:            fee,
			CreatedAt:      time.UnixMilli(trade.Time),
		}
		events = append(events, event)
	}

	// 计算平均开仓价
	entryPrice := decimal.Zero
	if !entryQuantity.IsZero() {
		entryPrice = totalEntryValue.Div(entryQuantity)
	}

	// 计算平均平仓价
	closePrice := p.calculateAverageClosePrice(events)

	// 如果持仓没有完全平仓，closeTime 为最后一次交易时间
	if closeTime.IsZero() && len(trades) > 0 {
		closeTime = time.UnixMilli(trades[len(trades)-1].Time)
	}

	return exchange.PositionHistory{
		TradingPair:  pair,
		PositionSide: side,
		EntryPrice:   entryPrice,
		ClosePrice:   closePrice,
		MaxQuantity:  maxQuantity,
		OpenedAt:     openTime,
		ClosedAt:     closeTime,
		Events:       events,
	}
}

// calculateAverageClosePrice 计算平均平仓价
func (p *PositonService) calculateAverageClosePrice(events []exchange.PositionEvent) decimal.Decimal {
	totalCloseValue := decimal.Zero
	totalCloseQty := decimal.Zero

	for _, event := range events {
		if event.EventType == exchange.PositionEventTypeDecrease ||
			event.EventType == exchange.PositionEventTypeClose {
			totalCloseValue = totalCloseValue.Add(event.Price.Mul(event.Quantity))
			totalCloseQty = totalCloseQty.Add(event.Quantity)
		}
	}

	if totalCloseQty.IsZero() {
		return decimal.Zero
	}
	return totalCloseValue.Div(totalCloseQty)
}

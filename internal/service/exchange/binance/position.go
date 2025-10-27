package binance

import (
	"context"
	"strconv"

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

func (p *PositonService) ChangeLeverage(ctx context.Context, req exchange.ChangeLeverageReq) error {
	svc := p.cli.NewChangeLeverageService()
	svc.Symbol(req.TradingPair.ToString())
	svc.Leverage(req.Leverage)
	_, err := svc.Do(ctx)
	return err
}

// GetActivePosition 获取单个持仓
// notice: 币安有挂单，未成交的仓位也会返回，需要过滤掉
func (p *PositonService) GetActivePosition(ctx context.Context, pair exchange.TradingPair) ([]exchange.Position, error) {
	svc := p.cli.NewGetPositionRiskService().Symbol(pair.ToString())
	res, err := svc.Do(ctx)
	if err != nil {
		return nil, err
	}
	positions := make([]exchange.Position, 0, len(res))
	for _, v := range res {
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
			PositionAmount:   decimal.RequireFromString(v.PositionAmt),
			MarginAmount:     decimal.RequireFromString(v.IsolatedMargin),
			UnrealizedProfit: decimal.RequireFromString(v.UnRealizedProfit),
		}

		// 过滤掉未成交的仓位
		if position.PositionAmount.Equal(decimal.Zero) {
			continue
		}

		// TODO margin计算公式尚有疑问，需要确认
		margin := position.EntryPrice.Mul(position.PositionAmount).Div(decimal.NewFromInt(int64(position.Leverage))).Abs().Round(2)
		position.MarginAmount = margin
		positions = append(positions, position)
	}
	return positions, nil
}

// GetActivePositions 获取所有持仓
// notice: 币安有挂单，未成交的仓位也会返回，需要过滤掉
func (p *PositonService) GetActivePositions(ctx context.Context) ([]exchange.Position, error) {
	svc := p.cli.NewGetPositionRiskService()
	res, err := svc.Do(ctx)
	if err != nil {
		return nil, err
	}
	positions := make([]exchange.Position, 0, len(res))
	for _, v := range res {
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
			PositionAmount:   decimal.RequireFromString(v.PositionAmt),
			MarginAmount:     decimal.RequireFromString(v.IsolatedMargin),
			UnrealizedProfit: decimal.RequireFromString(v.UnRealizedProfit),
		}

		// 过滤掉未成交的仓位
		if position.PositionAmount.Equal(decimal.Zero) {
			continue
		}
		// TODO margin计算公式尚有疑问，需要确认
		margin := position.EntryPrice.Mul(position.PositionAmount).Div(decimal.NewFromInt(int64(position.Leverage))).Abs().Round(2)
		position.MarginAmount = margin
		positions = append(positions, position)
	}
	return positions, nil
}

func (p *PositonService) GetHistoryPositions(ctx context.Context, req exchange.GetHistoryPositionsReq) ([]exchange.Position, error) {
	// TODO 获取历史持仓
	return nil, nil
}

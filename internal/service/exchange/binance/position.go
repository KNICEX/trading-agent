package binance

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

type PositonService struct {
	cli *futures.Client
}

func (p *PositonService) ChangeLeverage(ctx context.Context, req exchange.ChangeLeverageReq) error {
	svc := p.cli.NewChangeLeverageService()
	svc.Symbol(req.Symbol.ToString())
	svc.Leverage(req.Leverage)
	_, err := svc.Do(ctx)
	return err
}
func (p *PositonService) GetPositionRisk(ctx context.Context, req exchange.GetPositionRiskReq) ([]*exchange.Position, error) {
	svc := p.cli.NewGetPositionRiskV3Service()
	if !req.Symbol.IsZero() {
		svc.Symbol(req.Symbol.ToString())
	}
	if req.RecvWindow > 0 {
		svc.RecvWindow(req.RecvWindow)
	}

	res, err := svc.Do(ctx)
	if err != nil {
		return nil, err
	}
	positions := make([]*exchange.Position, len(res))
	for i, v := range res {
		base, quote := exchange.SplitSymbol(v.Symbol)
		positions[i] = &exchange.Position{
			Symbol: exchange.TradingPair{
				Base:  base,
				Quote: quote,
			},
			PositionSide:     exchange.PositionSide(v.PositionSide),
			EntryPrice:       decimal.RequireFromString(v.EntryPrice),
			BreakEvenPrice:   decimal.RequireFromString(v.BreakEvenPrice),
			LiquidationPrice: decimal.RequireFromString(v.LiquidationPrice),
		}
	}
	return positions, nil
}

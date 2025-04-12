package binance

import (
	"context"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2"
	"github.com/samber/lo"
	"strings"
)

type SymbolService struct {
	cli *binance.Client
}

func NewSymbolService(cli *binance.Client) exchange.SymbolService {
	return &SymbolService{
		cli: cli,
	}
}
func (svc *SymbolService) GetAllSymbols(ctx context.Context) ([]exchange.Symbol, error) {
	symbols, err := svc.cli.NewListPricesService().Do(ctx)
	if err != nil {
		return nil, err
	}

	symbols = svc.onlyUSDT(symbols)

	res := lo.Map(symbols, func(item *binance.SymbolPrice, index int) exchange.Symbol {
		return exchange.Symbol{
			Base:  strings.TrimSuffix(item.Symbol, "USDT"),
			Quote: "USDT",
			Price: item.Price,
		}
	})
	return res, nil
}

func (svc *SymbolService) onlyUSDT(s []*binance.SymbolPrice) []*binance.SymbolPrice {
	return lo.Filter(s, func(item *binance.SymbolPrice, index int) bool {
		if strings.HasSuffix(item.Symbol, "USDT") {
			return true
		}
		return false
	})
}

func (svc *SymbolService) GetSymbolPrice(ctx context.Context, symbol exchange.Symbol) (exchange.Symbol, error) {
	s, err := svc.cli.NewListPricesService().Symbol(fmt.Sprintf("%s%s", symbol.Base, symbol.Quote)).Do(ctx)
	if err != nil {
		return exchange.Symbol{}, err
	}
	if len(s) == 0 {
		return exchange.Symbol{}, fmt.Errorf("symbol %s not found", symbol.Base)
	}
	return exchange.Symbol{
		Base:  strings.TrimSuffix(s[0].Symbol, symbol.Quote),
		Quote: symbol.Quote,
		Price: s[0].Price,
	}, nil
}

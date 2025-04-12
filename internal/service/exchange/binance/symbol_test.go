package binance

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"testing"
)

func initSymbolService() exchange.SymbolService {
	cli := initClient()
	return NewSymbolService(cli)
}

func TestSymbolService_GetAllSymbols(t *testing.T) {
	svc := initSymbolService()
	symbols, err := svc.GetAllSymbols(context.Background())
	if err != nil {
		t.Errorf("Error getting symbols: %v", err)
		return
	}
	for _, symbol := range symbols {
		t.Logf("Symbol: %+v\n", symbol)
	}
}

func TestSymbolService_GetSymbolPrice(t *testing.T) {
	svc := initSymbolService()
	ctx := context.Background()
	symbol, err := svc.GetSymbolPrice(ctx, exchange.Symbol{
		Base:  "ETH",
		Quote: "BTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v\n", symbol)

	symbol, err = svc.GetSymbolPrice(ctx, exchange.Symbol{
		Base:  "PARTI",
		Quote: "USDT",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v\n", symbol)
}

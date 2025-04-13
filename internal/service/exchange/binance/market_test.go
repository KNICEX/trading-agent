package binance

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"testing"
	"time"
)

func initMarketService() exchange.MarketService {
	cli := initClient()
	return NewMarketService(cli)
}

func TestMarketService_GetKlines(t *testing.T) {
	svc := initMarketService()
	klines, err := svc.GetKlines(context.Background(), exchange.Symbol{
		Base:  "BTC",
		Quote: "USDT",
	}, exchange.Interval15m, time.Now().Add(-time.Hour*10), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range klines {
		t.Logf("%+v\n", line)
	}
}

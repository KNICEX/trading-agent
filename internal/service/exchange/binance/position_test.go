package binance

import (
	"context"
	"testing"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

func newPositionService(t *testing.T) *PositonService {
	return NewPositionService(initClient(t))
}

func TestGetPosition(t *testing.T) {
	svc := newPositionService(t)
	positions, err := svc.GetActivePositions(context.Background(), []exchange.TradingPair{exchange.TradingPair{Base: "BTC", Quote: "USDT"}})
	if err != nil {
		t.Errorf("Error getting position: %v", err)
	}
	for _, position := range positions {
		t.Logf("Position: %+v", position)
	}
}

func TestGetPositions(t *testing.T) {
	svc := newPositionService(t)
	positions, err := svc.GetActivePositions(context.Background(), []exchange.TradingPair{exchange.TradingPair{Base: "BTC", Quote: "USDT"}})
	if err != nil {
		t.Errorf("Error getting positions: %v", err)
	}
	for _, position := range positions {
		t.Logf("Position: %+v", position)
	}
}

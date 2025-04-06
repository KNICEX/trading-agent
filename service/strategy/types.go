package strategy

import (
	"context"
	"github.com/KNICEX/trading-agent/service/exchange"
)

type OrderSide string

type Priority int

const (
	Buy  OrderSide = "buy"
	Sell OrderSide = "sell"
	None OrderSide = "none"

	Low    Priority = 100
	Medium Priority = 200
	High   Priority = 300
)

type Suggestion struct {
	OrderSide OrderSide // buy/sell/none
	Price     float64   // if buy/sell, the price to buy/sell
	Priority  Priority  // recommendation priority

	Reason string // reason for the recommendation
}

type MultiKline struct {
	Week     []exchange.Kline
	Day      []exchange.Kline
	Hour4    []exchange.Kline
	Hour     []exchange.Kline
	Minute15 []exchange.Kline
	Minute30 []exchange.Kline
	Minute5  []exchange.Kline
}

type Service interface {
	Analyze(ctx context.Context, kLines MultiKline) (Suggestion, error)
}

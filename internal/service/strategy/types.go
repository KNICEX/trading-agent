package strategy

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

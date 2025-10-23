package exchange

import "github.com/shopspring/decimal"

type AccountBalance struct {
	AccountAlias     string
	Asset            string
	Balance          decimal.Decimal
	UnrealizedPnl    decimal.Decimal
	AvailableBalance decimal.Decimal
}

type AccountService interface {
}

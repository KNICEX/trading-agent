package decimalx

import "github.com/shopspring/decimal"

func MustFromString(s string) decimal.Decimal {
	f, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return f
}

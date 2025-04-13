package decimalx

import "github.com/shopspring/decimal"

func MustFromString(s string) decimal.Decimal {
	res, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return res
}

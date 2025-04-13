package decimalx

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestSlope(t *testing.T) {
	testCases := []struct {
		name string
		ds   []decimal.Decimal
	}{
		{
			name: "1",
			ds: []decimal.Decimal{
				decimal.NewFromInt(1),
				decimal.NewFromInt(2),
				decimal.NewFromInt(3),
				decimal.NewFromInt(4),
			},
		},
		{
			name: "big num",
			ds: []decimal.Decimal{
				decimal.NewFromInt(100),
				decimal.NewFromInt(200),
				decimal.NewFromInt(300),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			slope := Slope(tc.ds)
			t.Log(slope)
		})
	}
}

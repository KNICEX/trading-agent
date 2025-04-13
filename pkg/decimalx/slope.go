package decimalx

import (
	"github.com/shopspring/decimal"
)

func Slope(ds []decimal.Decimal) decimal.Decimal {

	// 归一化
	maxY, minY := ds[0], ds[0]
	for _, d := range ds {
		maxY = decimal.Max(maxY, d)
		minY = decimal.Min(minY, d)
	}
	normalizedY := make([]decimal.Decimal, len(ds))
	diff := maxY.Sub(minY)
	if diff.IsZero() {
		return decimal.Zero // 如果所有值相同，返回默认值
	}
	for _, d := range ds {
		normalizedY = append(normalizedY, d.Sub(minY).Div(diff))
	}

	sumX, sumY, sumXY, sumX2 := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
	for i, d := range normalizedY {
		x := decimal.NewFromInt(int64(i))
		sumX = sumX.Add(x)
		sumY = sumY.Add(d)
		sumXY = sumXY.Add(x.Mul(d))
		sumX2 = sumX2.Add(x.Mul(x))
	}

	// 计算斜率
	n := decimal.NewFromInt(int64(len(ds)))
	denominator := n.Mul(sumX2).Sub(sumX.Mul(sumX))
	if denominator.IsZero() {
		return decimal.Zero // 除数为零，返回默认值
	}
	return n.Mul(sumXY).Sub(sumX.Mul(sumY)).Div(denominator)
}

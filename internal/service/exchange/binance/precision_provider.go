package binance

import (
	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

var _ exchange.QuantityPrecisionProvider = (*PrecisionProvider)(nil)

// PrecisionProvider 币安交易对精度提供器
type PrecisionProvider struct{}

// NewPrecisionProvider 创建币安精度提供器
func NewPrecisionProvider() *PrecisionProvider {
	return &PrecisionProvider{}
}

// GetQuantityPrecision 获取交易对的数量精度
func (p *PrecisionProvider) GetQuantityPrecision(pair exchange.TradingPair) int32 {
	// 常见交易对的精度配置
	// 参考: https://www.binance.com/en/futures/trading-rules
	precisionMap := map[string]int32{
		"BTC":   3, // 0.001
		"ETH":   3, // 0.001
		"BNB":   2, // 0.01
		"SOL":   1, // 0.1
		"DOGE":  0, // 1
		"SHIB":  0, // 1
		"XRP":   1, // 0.1
		"ADA":   0, // 1
		"AVAX":  1, // 0.1
		"DOT":   1, // 0.1
		"MATIC": 0, // 1
	}

	// 获取精度，默认为3位小数
	precision, exists := precisionMap[pair.Base]
	if !exists {
		precision = 3
	}
	return precision
}

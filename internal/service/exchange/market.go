package exchange

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// TradingPair 交易对
type TradingPair struct {
	Base  string
	Quote string
}

func SplitSymbol(s string) (string, string) {
	s = strings.ToUpper(s)
	// 常见 Quote 列表
	quotes := []string{"USDT", "BUSD", "USDC", "BTC", "ETH"}
	for _, q := range quotes {
		if strings.HasSuffix(s, q) {
			return strings.TrimSuffix(s, q), q
		}
	}
	// fallback
	return s, ""
}

func (s *TradingPair) IsZero() bool {
	return s.Base == "" || s.Quote == ""
}
func (s *TradingPair) ToString() string {
	return fmt.Sprintf("%s%s", s.Base, s.Quote)
}

func (s *TradingPair) ToSlashString() string {
	return fmt.Sprintf("%s/%s", s.Base, s.Quote)
}

type Interval string

func (i Interval) ToString() string {
	return string(i)
}

const (
	Interval1m  Interval = "1m"
	Interval3m  Interval = "3m"
	Interval5m  Interval = "5m"
	Interval15m Interval = "15m"
	Interval30m Interval = "30m"
	Interval1h  Interval = "1h"
	Interval2h  Interval = "2h"
	Interval4h  Interval = "4h"
	Interval6h  Interval = "6h"
	Interval8h  Interval = "8h"
	Interval12h Interval = "12h"
	Interval1d  Interval = "1d"
	Interval3d  Interval = "3d"
	Interval1w  Interval = "1w"
	Interval1M  Interval = "1M"
)

type Kline struct {
	OpenTime         time.Time
	CloseTime        time.Time
	Open             decimal.Decimal
	Close            decimal.Decimal
	High             decimal.Decimal
	Low              decimal.Decimal
	Volume           decimal.Decimal // 成交量
	QuoteAssetVolume decimal.Decimal // 成交额
	TradeNum         int64           // 成交笔数
}

type MarketService interface {
	GetKlines(ctx context.Context, symbol TradingPair, interval Interval, startTime, endTime time.Time) ([]Kline, error)
}
type SymbolService interface {
	GetAllSymbols(ctx context.Context) ([]TradingPair, error)
	GetSymbolPrice(ctx context.Context, symbol TradingPair) (TradingPair, error)
}

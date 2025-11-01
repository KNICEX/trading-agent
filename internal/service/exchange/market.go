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

type Interval struct {
	duration time.Duration
	str      string
}

func (i Interval) ToString() string {
	return i.str
}

func (i Interval) Duration() time.Duration {
	return i.duration
}

var (
	Interval5m  Interval = Interval{duration: time.Minute * 5, str: "5m"}
	Interval15m Interval = Interval{duration: time.Minute * 15, str: "15m"}
	Interval30m Interval = Interval{duration: time.Minute * 30, str: "30m"}
	Interval1h  Interval = Interval{duration: time.Hour, str: "1h"}
	Interval2h  Interval = Interval{duration: time.Hour * 2, str: "2h"}
	Interval4h  Interval = Interval{duration: time.Hour * 4, str: "4h"}
	Interval6h  Interval = Interval{duration: time.Hour * 6, str: "6h"}
	Interval8h  Interval = Interval{duration: time.Hour * 8, str: "8h"}
	Interval12h Interval = Interval{duration: time.Hour * 12, str: "12h"}
	Interval1d  Interval = Interval{duration: time.Hour * 24, str: "1d"}
	Interval3d  Interval = Interval{duration: time.Hour * 24 * 3, str: "3d"}
	Interval1w  Interval = Interval{duration: time.Hour * 24 * 7, str: "1w"}
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
}

type MarketService interface {
	Ticker(ctx context.Context, tradingPair TradingPair) (decimal.Decimal, error)
	GetKlines(ctx context.Context, req GetKlinesReq) ([]Kline, error)
	SubscribeKline(ctx context.Context, tradingPair TradingPair, interval Interval) (chan Kline, error)
}

type GetKlinesReq struct {
	TradingPair        TradingPair
	Interval           Interval
	StartTime, EndTime time.Time
}
type SymbolService interface {
	GetAllSymbols(ctx context.Context) ([]TradingPair, error)
	GetSymbolPrice(ctx context.Context, tradingPair TradingPair) (TradingPair, error)
}

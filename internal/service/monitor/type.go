package monitor

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"time"
)

type AbnormalType string

const (
	Bullish AbnormalType = "bullish"
	Bearish AbnormalType = "bearish"
)

type AbnormalSignal struct {
	Abnormal     bool            `json:"abnormal"`
	Symbol       exchange.Symbol `json:"symbol"`
	Reason       string          `json:"reason"`
	Type         AbnormalType    `json:"type"`
	Confidence   float64         `json:"confidence"`
	CurrentPrice string          `json:"current_price"`
	Timestamp    time.Time       `json:"timestamp"`
}

// AbnormalService 监控服务接口
type AbnormalService interface {
	Scan(ctx context.Context, symbols []exchange.Symbol) error
}

type Notifier interface {
	Notify(ctx context.Context, signal AbnormalSignal) error
}

type AbnormalAnalyzer interface {
	Analyze(ctx context.Context, kLines []exchange.Kline) (AbnormalSignal, error)
}

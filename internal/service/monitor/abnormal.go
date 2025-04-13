package monitor

import (
	"context"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/entity"
	"github.com/KNICEX/trading-agent/internal/repo"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/strategy"
	"log/slog"
	"time"
)

type AbnormalMonitor struct {
	analyzer strategy.AbnormalAnalyzer
	notifier Notifier

	repo repo.AbnormalRepo

	symbolSvc exchange.SymbolService
	marketSvc exchange.MarketService
}

type consoleNotifier struct {
}

func (c consoleNotifier) Notify(ctx context.Context, signal strategy.AbnormalSignal) error {
	fmt.Printf("find abnormal signal: %+v", signal)
	return nil
}

type Option func(m *AbnormalMonitor)

func WithNotifier(notifier Notifier) Option {
	return func(m *AbnormalMonitor) {
		m.notifier = notifier
	}
}

func NewAbnormalMonitor(analyzer strategy.AbnormalAnalyzer, repo repo.AbnormalRepo, symbolSvc exchange.SymbolService, marketSvc exchange.MarketService, opts ...Option) AbnormalService {
	monitor := &AbnormalMonitor{
		analyzer:  analyzer,
		repo:      repo,
		symbolSvc: symbolSvc,
		marketSvc: marketSvc,
		notifier:  consoleNotifier{},
	}
	for _, opt := range opts {
		opt(monitor)
	}
	return monitor
}

func (m *AbnormalMonitor) Scan(ctx context.Context, symbols []exchange.Symbol) error {
	for _, symbol := range symbols {
		kLines, err := m.marketSvc.GetKlines(
			ctx,
			symbol,
			exchange.Interval15m,
			time.Now().Add(-10*time.Hour),
			time.Now(),
		)
		if err != nil {
			slog.Error("failed to get k lines", "symbol", symbol, "error", err)
			continue
		}
		if len(kLines) < 2 {
			slog.Warn("skip analyze abnormal", "symbol", symbol, "reason", "too little k lines")
			continue
		}

		slog.Info("analyzing symbol abnormal", "symbol", symbol)
		signal, err := m.analyzer.Analyze(ctx, strategy.AnalyzeInput{
			Klines15Min: kLines,
		})
		if err != nil {
			slog.Error("failed to analyze symbol", "symbol", symbol, "error", err)
		}

		// 获取最新价格
		symbol, err = m.symbolSvc.GetSymbolPrice(ctx, symbol)
		if err != nil {
			slog.Error("failed to get symbol latest price", "symbol", symbol, "error", err)
		}
		signal.Symbol = symbol
		signal.CurrentPrice = symbol.Price

		if !signal.Abnormal {
			continue
		}

		// 保存异常信号
		_, err = m.repo.Create(ctx, entity.Abnormal{
			BaseSymbol:   symbol.Base,
			QuoteSymbol:  symbol.Quote,
			Price:        symbol.Price.String(),
			AbnormalType: string(signal.Type),
			Confidence:   signal.Confidence,
			Reason:       signal.Reason,
			CreatedAt:    time.Now(),
		})
		if err != nil {
			slog.Error("failed to save abnormal signal", "symbol", symbol, "signal", signal, "error", err)
		}

		// 发送通知
		go func() {
			err = m.notifier.Notify(ctx, signal)
			if err != nil {
				slog.Error("abnormal monitor notify signal err", "error", err, "signal", signal)
			}
		}()
	}
	return nil
}

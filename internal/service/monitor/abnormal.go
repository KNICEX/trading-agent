package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/entity"
	"github.com/KNICEX/trading-agent/internal/repo"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/llm"
	"log/slog"
	"strings"
	"time"
)

type AbnormalMonitor struct {
	analyzer AbnormalAnalyzer
	notifier Notifier

	repo repo.AbnormalRepo

	symbolSvc exchange.SymbolService
	marketSvc exchange.MarketService
}

type consoleNotifier struct {
}

func (c consoleNotifier) Notify(ctx context.Context, signal AbnormalSignal) error {
	fmt.Println("find abnormal signal", signal)
	return nil
}

type Option func(m *AbnormalMonitor)

func WithNotifier(notifier Notifier) Option {
	return func(m *AbnormalMonitor) {
		m.notifier = notifier
	}
}

func NewAbnormalMonitor(analyzer AbnormalAnalyzer, repo repo.AbnormalRepo, symbolSvc exchange.SymbolService, marketSvc exchange.MarketService, opts ...Option) AbnormalService {
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
			time.Now().Add(-8*time.Hour),
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
		signal, err := m.analyzer.Analyze(ctx, kLines)
		if err != nil {
			slog.Error("failed to analyze symbol", "symbol", symbol, "error", err)
		}

		// 获取最新价格
		symbol, err = m.symbolSvc.GetSymbolPrice(ctx, symbol)
		if err != nil {
			slog.Error("failed to get symbol latest price", "symbol", symbol, "error", err)
		}

		if !signal.Abnormal {
			continue
		}

		// 保存异常信号
		_, err = m.repo.Create(ctx, entity.Abnormal{
			BaseSymbol:   symbol.Base,
			QuoteSymbol:  symbol.Quote,
			Price:        symbol.Price,
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

type llmAnalyzer struct {
	llmSvc llm.Service
}

func NewLLMAnalyzer(llmSvc llm.Service) AbnormalAnalyzer {
	return &llmAnalyzer{
		llmSvc: llmSvc,
	}
}

func (a *llmAnalyzer) Analyze(ctx context.Context, kLines []exchange.Kline) (AbnormalSignal, error) {
	prompt := fmt.Sprintf("这是某交易对最近的5mK线数据: \n"+
		"%+v\n 请判断是否存在异动情况, 异动的大概标准是："+
		"(连续小阳线且量价跟随, 展示出和之前截然不同的走势) 看涨(bullish), (或者是突然大阴线, 或者逐渐放量下跌)看跌(bearish), 你需要判断是否异动(abnormal), "+
		"异动的判断应该更加严谨, 无需震荡请不要认为异动\n"+
		"后续看涨还是看跌(type), 以及你认为异动的原因(reason), "+
		"并且给出一个0-1的置信度(confidence), 请按如下json格式回复我: "+
		`{"abnormal": true | false, "type": "bullish or bearish", "reason": "判断异动的原因", "confidence": 0-1}`, kLines)

	answer, err := a.llmSvc.AskOnce(ctx, llm.Question{Content: prompt})
	if err != nil {
		return AbnormalSignal{}, err
	}

	var signal AbnormalSignal
	if err = a.extractAnswer(answer, &signal); err != nil {
		return AbnormalSignal{}, err
	}
	return signal, nil
}

func (a *llmAnalyzer) extractAnswer(answer llm.Answer, v any) error {
	// 解析JSON
	answer.Content = strings.Trim(answer.Content, "\n")
	lines := strings.Split(answer.Content, "\n")
	if len(lines) < 3 {
		return fmt.Errorf("invalid answer format")
	}
	content := strings.Join(lines[1:len(lines)-1], "\n")
	return json.Unmarshal([]byte(content), v)
}

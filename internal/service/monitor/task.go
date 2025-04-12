package monitor

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/schedule"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/samber/lo"
)

type AbnormalMonitorTask struct {
	symbolSvc    exchange.SymbolService
	abnormalSvc  AbnormalService
	rejectSymbol func(ctx context.Context, symbol exchange.Symbol) bool // if true, reject
}

func NewAbnormalMonitorTask(abSvc AbnormalService, symbolSvc exchange.SymbolService,
	reject ...func(ctx context.Context, symbol exchange.Symbol) bool) schedule.Task {
	task := &AbnormalMonitorTask{
		symbolSvc:   symbolSvc,
		abnormalSvc: abSvc,
		rejectSymbol: func(ctx context.Context, symbol exchange.Symbol) bool {
			return false
		},
	}

	if len(reject) > 0 {
		task.rejectSymbol = reject[0]
	}
	return task
}

func (t *AbnormalMonitorTask) Run(ctx context.Context) error {
	symbols, err := t.symbolSvc.GetAllSymbols(ctx)
	if err != nil {
		return err
	}

	symbols = lo.Reject(symbols, func(item exchange.Symbol, index int) bool {
		return t.rejectSymbol(ctx, item)
	})

	return t.abnormalSvc.Scan(ctx, symbols)
}

func (t *AbnormalMonitorTask) Name() string {
	return "abnormal monitor signal task"
}

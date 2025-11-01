package engine

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/strategy"
)

type Engine interface {
	Run(ctx context.Context) error
	Stop(ctx context.Context) error
	AddStrategy(ctx context.Context, strategy strategy.Strategy) error
}

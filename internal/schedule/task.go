package schedule

import "context"

type Task interface {
	Run(ctx context.Context) error
	Name() string
}

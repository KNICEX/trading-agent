package llm

import (
	"context"
	"io"
)

type Question struct {
	Content string
	Files   []io.Reader
}

type Answer struct {
	Content     string
	InputToken  int
	OutputToken int
}

type Session interface {
	Ask(ctx context.Context, q Question) (Answer, error)
}

type Service interface {
	AskOnce(ctx context.Context, q Question) (Answer, error)
	BeginChat(ctx context.Context) (Session, error)
}

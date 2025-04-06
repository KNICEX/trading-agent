package gemini

import (
	"context"
	"github.com/KNICEX/trading-agent/service/llm"
	"github.com/google/generative-ai-go/genai"
)

type Session struct {
	session *genai.ChatSession
}

type Service struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func (s *Service) AskOnce(ctx context.Context, q llm.Question) (llm.Answer, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Service) BeginChat(ctx context.Context) (llm.Session, error) {
	//TODO implement me
	panic("implement me")
}

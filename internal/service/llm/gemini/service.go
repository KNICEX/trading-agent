package gemini

import (
	"context"
	"github.com/KNICEX/trading-agent/internal/service/llm"
	"github.com/google/generative-ai-go/genai"
	"strings"
)

type Session struct {
	session *genai.ChatSession
}

func (s Session) Ask(ctx context.Context, q llm.Question) (llm.Answer, error) {
	resp, err := s.session.SendMessage(ctx, genai.Text(q.Content))
	if err != nil {
		return llm.Answer{}, err
	}
	res := parseResponse(resp)
	return llm.Answer{
		Content: res,
	}, nil
}

type Service struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewService(client *genai.Client, opts ...Option) llm.Service {
	svc := &Service{
		client: client,
		model:  client.GenerativeModel("gemini-2.0-flash"),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

type Option func(service *Service)

func WithTemperature(temp float32) Option {
	return func(service *Service) {
		service.model.SetTemperature(temp)
	}
}

func WithFlash2() Option {
	return func(service *Service) {
		service.model = service.client.GenerativeModel("gemini-2.0-flash")
	}
}

func (s *Service) AskOnce(ctx context.Context, q llm.Question) (llm.Answer, error) {
	resp, err := s.model.GenerateContent(ctx, genai.Text(q.Content))
	if err != nil {
		return llm.Answer{}, err
	}
	res := parseResponse(resp)
	return llm.Answer{
		Content: res,
	}, nil
}

func (s *Service) BeginChat(ctx context.Context) (llm.Session, error) {
	session := s.model.StartChat()
	return &Session{
		session: session,
	}, nil
}

func parseResponse(resp *genai.GenerateContentResponse) string {
	var resStr strings.Builder
	if resp.Candidates != nil && len(resp.Candidates) > 0 {
		for i, part := range resp.Candidates[0].Content.Parts {
			if part == nil {
				continue
			}
			if text, ok := part.(genai.Text); ok {
				if i > 0 {
					resStr.WriteString("\n")
				}
				resStr.WriteString(string(text))
			} else {
				return ""
			}
		}
	}
	return resStr.String()
}

package notification

import "context"

type EmailService interface {
	SendText(ctx context.Context, to, subject, body string) error
	SendHTML(ctx context.Context, to, subject, body string) error
}

type WebhookService interface {
	Send(ctx context.Context, url string, data map[string]any) error
}

package binance

import "github.com/adshao/go-binance/v2"

type Service struct {
	cli *binance.Client
}

func NewService(apiKey, secretKey string) *Service {
	cli := binance.NewClient(apiKey, secretKey)
	return &Service{cli: cli}
}

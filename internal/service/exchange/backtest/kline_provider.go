package backtest

import (
	"context"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
)

// KlineProvider K线数据提供者接口
// 可以有多种实现：币安API、本地文件、模拟数据等
type KlineProvider interface {
	// GetKlines 获取K线数据
	GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error)
}

// BinanceKlineProvider 币安K线数据提供者
type BinanceKlineProvider struct {
	marketService exchange.MarketService
}

// NewBinanceKlineProvider 创建币安K线提供者
func NewBinanceKlineProvider(marketService exchange.MarketService) KlineProvider {
	return &BinanceKlineProvider{
		marketService: marketService,
	}
}

// GetKlines 从币安API获取K线数据
func (p *BinanceKlineProvider) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	return p.marketService.GetKlines(ctx, req)
}

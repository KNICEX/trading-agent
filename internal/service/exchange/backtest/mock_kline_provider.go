package backtest

import (
	"context"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/shopspring/decimal"
)

// MockKlineProvider 模拟K线数据提供者（用于测试）
type MockKlineProvider struct {
	klines map[string][]exchange.Kline // key: tradingPair_interval
}

// NewMockKlineProvider 创建模拟K线提供者
func NewMockKlineProvider() *MockKlineProvider {
	return &MockKlineProvider{
		klines: make(map[string][]exchange.Kline),
	}
}

// AddKlines 添加模拟K线数据
func (p *MockKlineProvider) AddKlines(tradingPair exchange.TradingPair, interval exchange.Interval, klines []exchange.Kline) {
	key := tradingPair.ToString() + "_" + interval.ToString()
	p.klines[key] = klines
}

// GenerateKlines 生成模拟K线数据
// basePrice: 基础价格
// count: K线数量
// trend: 趋势 ("up"上涨, "down"下跌, "sideways"横盘, "volatile"波动)
func (p *MockKlineProvider) GenerateKlines(
	tradingPair exchange.TradingPair,
	interval exchange.Interval,
	startTime time.Time,
	basePrice float64,
	count int,
	trend string,
) {
	klines := make([]exchange.Kline, count)

	for i := 0; i < count; i++ {
		var price float64

		switch trend {
		case "up":
			// 上涨趋势：每根K线涨0.5%
			price = basePrice * (1 + float64(i)*0.005)
		case "down":
			// 下跌趋势：每根K线跌0.5%
			price = basePrice * (1 - float64(i)*0.005)
		case "volatile":
			// 波动趋势：上下波动
			if i%2 == 0 {
				price = basePrice * (1 + float64(i%10)*0.002)
			} else {
				price = basePrice * (1 - float64(i%10)*0.002)
			}
		default: // sideways
			// 横盘：小幅波动
			price = basePrice * (1 + (float64(i%5)-2)*0.001)
		}

		openTime := startTime.Add(time.Duration(i) * interval.Duration())
		closeTime := openTime.Add(interval.Duration())

		// 生成高低价（在收盘价基础上±0.5%）
		high := price * 1.005
		low := price * 0.995

		klines[i] = exchange.Kline{
			OpenTime:         openTime,
			CloseTime:        closeTime,
			Open:             decimal.NewFromFloat(price * 0.999), // 开盘价略低于收盘价
			Close:            decimal.NewFromFloat(price),
			High:             decimal.NewFromFloat(high),
			Low:              decimal.NewFromFloat(low),
			Volume:           decimal.NewFromFloat(1000 + float64(i)*10),
			QuoteAssetVolume: decimal.NewFromFloat(price * (1000 + float64(i)*10)),
		}
	}

	p.AddKlines(tradingPair, interval, klines)
}

// GetKlines 获取K线数据
func (p *MockKlineProvider) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	key := req.TradingPair.ToString() + "_" + req.Interval.ToString()

	allKlines, exists := p.klines[key]
	if !exists {
		return []exchange.Kline{}, nil
	}

	// 过滤时间范围
	var result []exchange.Kline
	for _, kline := range allKlines {
		if (kline.OpenTime.Equal(req.StartTime) || kline.OpenTime.After(req.StartTime)) &&
			kline.OpenTime.Before(req.EndTime) {
			result = append(result, kline)
		}
	}

	return result, nil
}

// FileKlineProvider 从文件加载K线数据的提供者
// TODO: 实现从CSV/JSON文件加载K线数据
type FileKlineProvider struct {
	// 可以添加文件路径等配置
}

// NewFileKlineProvider 创建文件K线提供者
func NewFileKlineProvider() *FileKlineProvider {
	return &FileKlineProvider{}
}

// GetKlines 从文件获取K线数据
func (p *FileKlineProvider) GetKlines(ctx context.Context, req exchange.GetKlinesReq) ([]exchange.Kline, error) {
	// TODO: 实现从文件读取K线数据
	// 1. 根据tradingPair和interval确定文件路径
	// 2. 读取CSV/JSON文件
	// 3. 解析并过滤时间范围
	// 4. 返回K线数据
	return []exchange.Kline{}, nil
}

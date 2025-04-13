package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/KNICEX/trading-agent/pkg/decimalx"
	"github.com/samber/lo"
	"log/slog"
	"strings"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/KNICEX/trading-agent/internal/service/llm"
	"github.com/shopspring/decimal"
)

type AbnormalSignal struct {
	Abnormal     bool            `json:"abnormal"`
	Symbol       exchange.Symbol `json:"symbol"`
	Reason       string          `json:"reason"`
	Type         AbnormalType    `json:"type"`
	Confidence   float64         `json:"confidence"`
	CurrentPrice decimal.Decimal `json:"current_price"`
	Timestamp    time.Time       `json:"timestamp"`
}

type AbnormalType string

const (
	Bullish AbnormalType = "bullish"
	Bearish AbnormalType = "bearish"
)

type AbnormalAnalyzer interface {
	Analyze(ctx context.Context, input AnalyzeInput) (AbnormalSignal, error)
}

type ruleBasedAnalyzer struct{}

func NewRuleBasedAnalyzer() AbnormalAnalyzer {
	return &ruleBasedAnalyzer{}
}

func (a *ruleBasedAnalyzer) Analyze(ctx context.Context, input AnalyzeInput) (AbnormalSignal, error) {
	subLevel, err := a.analyze15m(input.Klines15Min)
	if err != nil {
		return AbnormalSignal{}, err
	}
	if subLevel.Abnormal {
		return subLevel, nil
	}
	return AbnormalSignal{}, nil
}

func (a *ruleBasedAnalyzer) analyze15m(kLines []exchange.Kline) (AbnormalSignal, error) {
	if len(kLines) < 10 {
		return AbnormalSignal{}, fmt.Errorf("not enough klines to analyze")
	}

	if kLines[len(kLines)-1].CloseTime.Sub(time.Now()).Minutes() > 5 {
		// 距离收盘还有5分钟以上, 裁剪掉这根k线
		kLines = kLines[:len(kLines)-1]
	}

	// 获取最近4根K线
	recentLen := 4
	recentKLines := kLines[len(kLines)-recentLen:]

	follow, slope := a.volumeFollowPrice(recentKLines)
	if !follow {
		// 没有量价跟随
		return AbnormalSignal{Abnormal: false}, nil
	}

	// 计算前面K线的成交量平均值（去除异常值）
	prevKLines := kLines[:len(kLines)-recentLen]

	volumes := make([]decimal.Decimal, 0, len(prevKLines))
	for _, k := range prevKLines {
		volumes = append(volumes, k.Volume)
	}

	// 去除异常值（超过2倍标准差）
	avgVolume, stdDev := a.calculateVolumeStats(volumes)
	filteredVolumes := make([]decimal.Decimal, 0, len(volumes))
	for _, v := range volumes {
		if v.LessThan(avgVolume.Add(decimal.NewFromInt(2)).Mul(stdDev)) {
			filteredVolumes = append(filteredVolumes, v)
		}
	}

	// 计算平滑后的平均成交量
	smoothAvgVolume := a.calculateAverage(filteredVolumes)
	slog.Info("diff volume", "smoothAvgVolume", smoothAvgVolume, "recentVolume", recentKLines[recentLen-1].Volume)

	// 检查最新的一根是否大于平滑后的平均成交量
	if recentKLines[recentLen-1].Volume.GreaterThan(smoothAvgVolume) {

		// 计算成交量差值比率 (当前成交量/平均成交量 - 1)
		volumeRatio := recentKLines[recentLen-1].Volume.Div(smoothAvgVolume).Sub(decimal.NewFromInt(1))

		// 计算置信度 (斜率 * 0.5 + 成交量比率 * 0.5)
		confidence := slope.Mul(decimal.NewFromFloat(0.5)).Add(
			volumeRatio.Mul(decimal.NewFromFloat(0.5)),
		)

		// 确保置信度在0-1范围内
		confidence = decimal.Max(decimal.Zero, decimal.Min(decimal.NewFromFloat(1), confidence))

		return AbnormalSignal{
			Abnormal: true,
			Type:     Bullish,
			Reason:   "连续4根阳线且成交量放大",
			// TODO 根据上升度做计算
			Confidence: confidence.InexactFloat64(), // 固定置信度
			Timestamp:  time.Now(),
		}, nil
	}

	return AbnormalSignal{Abnormal: false}, nil
}

// 量价跟随
func (a *ruleBasedAnalyzer) volumeFollowPrice(kLines []exchange.Kline) (follow bool, slope decimal.Decimal) {
	// 检查是否连续阳线
	isAllBullish := true
	for _, k := range kLines {
		if k.Close.LessThan(k.Open) {
			isAllBullish = false
			break
		}
	}

	if !isAllBullish {
		return false, decimal.Zero
	}

	volumeSlope := decimalx.Slope(lo.Map(kLines, func(item exchange.Kline, index int) decimal.Decimal {
		return item.Volume
	}))

	slog.Info("volume slope", "slope", volumeSlope)
	if volumeSlope.GreaterThan(decimal.NewFromFloat(0.007)) {
		return true, volumeSlope
	}
	return false, volumeSlope
}

// 计算平均值和标准差
func (a *ruleBasedAnalyzer) calculateVolumeStats(volumes []decimal.Decimal) (avg, stdDev decimal.Decimal) {

	avg = a.calculateAverage(volumes)

	// 计算标准差
	var variance decimal.Decimal
	for _, v := range volumes {
		diff := v.Sub(avg)
		variance = variance.Add(diff.Mul(diff))
	}
	variance = variance.Div(decimal.NewFromInt(int64(len(volumes))))
	stdDev = variance.Pow(decimal.NewFromFloat(0.5))

	return avg, stdDev
}

// 计算平均值
func (a *ruleBasedAnalyzer) calculateAverage(values []decimal.Decimal) decimal.Decimal {
	if len(values) == 0 {
		return decimal.Zero
	}

	var sum decimal.Decimal
	for _, v := range values {
		sum = sum.Add(v)
	}
	return sum.Div(decimal.NewFromInt(int64(len(values))))
}

type llmAnalyzer struct {
	llmSvc llm.Service
}

func NewLLMAnalyzer(llmSvc llm.Service) AbnormalAnalyzer {
	return &llmAnalyzer{
		llmSvc: llmSvc,
	}
}

func (a *llmAnalyzer) Analyze(ctx context.Context, input AnalyzeInput) (AbnormalSignal, error) {
	prompt := fmt.Sprintf("这是某交易对最近的15mK线数据: \n"+
		"%+v\n 请判断是否存在异动情况, 异动的大概标准是："+
		"(连续小阳线且量价跟随, 展示出和之前截然不同的走势) 看涨(bullish), (或者是突然大阴线, 或者逐渐放量下跌)看跌(bearish), 你需要判断是否异动(abnormal), "+
		"异动的判断应该更加严谨, 无需震荡请不要认为异动\n"+
		"后续看涨还是看跌(type), 以及你认为异动的原因(reason), "+
		"并且给出一个0-1的置信度(confidence), 请按如下json格式回复我: "+
		`{"abnormal": true | false, "type": "bullish or bearish", "reason": "判断异动的原因", "confidence": 0-1}`, input.Klines15Min)

	answer, err := a.llmSvc.AskOnce(ctx, llm.Question{Content: prompt})
	if err != nil {
		return AbnormalSignal{}, err
	}

	var signal AbnormalSignal
	if err = a.extractAnswer(answer, &signal); err != nil {
		return AbnormalSignal{}, err
	}
	return signal, nil
}

func (a *llmAnalyzer) extractAnswer(answer llm.Answer, v any) error {
	// 解析JSON
	answer.Content = strings.Trim(answer.Content, "\n")
	lines := strings.Split(answer.Content, "\n")
	if len(lines) < 3 {
		return fmt.Errorf("invalid answer format")
	}
	content := strings.Join(lines[1:len(lines)-1], "\n")
	return json.Unmarshal([]byte(content), v)
}

package binance

import (
	"context"
	"fmt"
	"time"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"
)

var _ exchange.AccountService = (*AccountService)(nil)

type AccountService struct {
	cli *futures.Client
}

func NewAccountService(cli *futures.Client) *AccountService {
	return &AccountService{cli: cli}
}

func (s *AccountService) GetAccountInfo(ctx context.Context) (exchange.AccountInfo, error) {
	account, err := s.cli.NewGetAccountService().Do(ctx)
	if err != nil {
		return exchange.AccountInfo{}, err
	}

	// MaxWithdrawAmount 是真正可用于开新仓的资金（已扣除挂单锁定的保证金）
	// AvailableBalance 不准确，不会扣除挂单锁定的资金
	return exchange.AccountInfo{
		TotalBalance:     decimal.RequireFromString(account.TotalWalletBalance),
		AvailableBalance: decimal.RequireFromString(account.MaxWithdrawAmount),
		UnrealizedPnl:    decimal.RequireFromString(account.TotalUnrealizedProfit),
		UsedMargin:       decimal.RequireFromString(account.TotalInitialMargin), // TotalInitialMargin = 持仓保证金 + 挂单保证金
	}, nil
}

func (s *AccountService) GetTransferHistory(ctx context.Context, req exchange.GetTransferHistoryReq) ([]exchange.TransferHistory, error) {
	// 获取所有转账记录（自动分页和分片）
	incomes, err := s.fetchAllIncomes(ctx, "TRANSFER", req.StartTime, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transfer history: %w", err)
	}

	// 转换为 TransferHistory
	var transfers []exchange.TransferHistory
	for _, income := range incomes {
		amount := decimal.RequireFromString(income.Income)

		// 根据金额正负判断方向
		direction := exchange.DirectionIn
		if amount.IsNegative() {
			direction = exchange.DirectionOut
			amount = amount.Abs() // 转为正数
		}

		transfers = append(transfers, exchange.TransferHistory{
			TimeStamp: time.UnixMilli(income.Time),
			Type:      exchange.TransferHistoryType(income.IncomeType),
			Amount:    amount,
			Direction: direction,
			Status:    exchange.TransferStatusSuccess, // Income API 返回的都是已成功的
		})
	}

	return transfers, nil
}

// fetchAllIncomes 获取所有收益记录，自动处理分页和时间分片
// 与 PositionService.fetchAllTrades 逻辑相同
func (s *AccountService) fetchAllIncomes(
	ctx context.Context,
	incomeType string,
	startTime, endTime time.Time,
) ([]*futures.IncomeHistory, error) {
	const (
		maxDays         = 7    // 币安限制最多查询7天
		limitPerRequest = 1000 // 单次请求最多1000条
		dayDuration     = 24 * time.Hour
	)

	var allIncomes []*futures.IncomeHistory

	// 计算时间跨度
	duration := endTime.Sub(startTime)

	// 如果时间跨度超过7天，需要分片
	if duration > maxDays*dayDuration {
		// 按7天分片
		currentStart := startTime
		for currentStart.Before(endTime) {
			currentEnd := currentStart.Add(maxDays * dayDuration)
			if currentEnd.After(endTime) {
				currentEnd = endTime
			}

			// 获取这个时间片的数据
			incomes, err := s.fetchIncomesWithPagination(ctx, incomeType, currentStart, currentEnd)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch incomes from %s to %s: %w",
					currentStart.Format(time.DateOnly), currentEnd.Format(time.DateOnly), err)
			}
			allIncomes = append(allIncomes, incomes...)

			currentStart = currentEnd
		}
	} else {
		// 时间跨度在7天内，直接分页查询
		incomes, err := s.fetchIncomesWithPagination(ctx, incomeType, startTime, endTime)
		if err != nil {
			return nil, err
		}
		allIncomes = incomes
	}

	return allIncomes, nil
}

// fetchIncomesWithPagination 在一个时间范围内分页获取所有收益记录
// 时间范围必须 <= 7天
func (s *AccountService) fetchIncomesWithPagination(
	ctx context.Context,
	incomeType string,
	startTime, endTime time.Time,
) ([]*futures.IncomeHistory, error) {
	const limitPerRequest = 1000

	var allIncomes []*futures.IncomeHistory

	for {
		// 构建请求
		svc := s.cli.NewGetIncomeHistoryService().
			IncomeType(incomeType).
			StartTime(startTime.UnixMilli()).
			EndTime(endTime.UnixMilli()).
			Limit(int64(limitPerRequest))

		// 执行请求
		incomes, err := svc.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch incomes: %w", err)
		}

		// 如果没有数据了，结束
		if len(incomes) == 0 {
			break
		}

		// 添加到结果
		allIncomes = append(allIncomes, incomes...)

		// 如果返回数量少于限制，说明已经是最后一页
		if len(incomes) < limitPerRequest {
			break
		}

		// 更新 startTime 为最后一条记录的时间 + 1ms（继续获取后续数据）
		lastTime := time.UnixMilli(incomes[len(incomes)-1].Time)
		startTime = lastTime.Add(1 * time.Millisecond)

		// 如果新的 startTime 已经超过 endTime，结束
		if startTime.After(endTime) {
			break
		}
	}

	return allIncomes, nil
}

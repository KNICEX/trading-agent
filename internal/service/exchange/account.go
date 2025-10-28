package exchange

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

type AccountInfo struct {
	TotalBalance     decimal.Decimal
	AvailableBalance decimal.Decimal
	UnrealizedPnl    decimal.Decimal
	UsedMargin       decimal.Decimal
}

type TransferHistoryType string

type Direction string

const (
	DirectionIn  Direction = "IN"
	DirectionOut Direction = "OUT"
)

type TransferStatus string

const (
	TransferStatusPending TransferStatus = "PENDING"
	TransferStatusSuccess TransferStatus = "SUCCESS"
	TransferStatusFailed  TransferStatus = "FAILED"
)

type TransferHistory struct {
	TimeStamp time.Time
	Type      TransferHistoryType
	Amount    decimal.Decimal
	Direction Direction
	Status    TransferStatus
}

type GetTransferHistoryReq struct {
	StartTime time.Time
	EndTime   time.Time
}

type AccountService interface {
	GetAccountInfo(ctx context.Context) (AccountInfo, error)
	GetTransferHistory(ctx context.Context, req GetTransferHistoryReq) ([]TransferHistory, error)
}

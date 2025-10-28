package exchange

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestPositionSideGetCloseOrderSide 测试根据持仓方向获取平仓订单方向
func TestPositionSideGetCloseOrderSide(t *testing.T) {
	tests := []struct {
		name         string
		positionSide PositionSide
		want         OrderSide
	}{
		{
			name:         "多头持仓应该用卖单平仓",
			positionSide: PositionSideLong,
			want:         OrderSideSell,
		},
		{
			name:         "空头持仓应该用买单平仓",
			positionSide: PositionSideShort,
			want:         OrderSideBuy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.positionSide.GetCloseOrderSide()
			if got != tt.want {
				t.Errorf("GetCloseOrderSide() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOrderStatusIsFilled 测试判断订单是否已完全成交
func TestOrderStatusIsFilled(t *testing.T) {
	tests := []struct {
		name   string
		status OrderStatus
		want   bool
	}{
		{
			name:   "已成交订单",
			status: OrderStatusFilled,
			want:   true,
		},
		{
			name:   "等待中的订单未成交",
			status: OrderStatusPending,
			want:   false,
		},
		{
			name:   "部分成交的订单未完全成交",
			status: OrderStatusPartiallyFilled,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsFilled()
			if got != tt.want {
				t.Errorf("IsFilled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOrderInfoIsActive 测试判断订单是否处于活跃状态
func TestOrderInfoIsActive(t *testing.T) {
	tests := []struct {
		name  string
		order OrderInfo
		want  bool
	}{
		{
			name: "等待中的订单是活跃的",
			order: OrderInfo{
				Status: OrderStatusPending,
			},
			want: true,
		},
		{
			name: "部分成交的订单是活跃的",
			order: OrderInfo{
				Status: OrderStatusPartiallyFilled,
			},
			want: true,
		},
		{
			name: "已成交的订单不是活跃的",
			order: OrderInfo{
				Status: OrderStatusFilled,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.order.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOrderInfoGetFilledPercentage 测试获取订单成交百分比
func TestOrderInfoGetFilledPercentage(t *testing.T) {
	tests := []struct {
		name  string
		order OrderInfo
		want  string // 使用字符串比较，避免浮点数精度问题
	}{
		{
			name: "完全成交应该是100%",
			order: OrderInfo{
				Quantity:         decimal.NewFromFloat(1.0),
				ExecutedQuantity: decimal.NewFromFloat(1.0),
			},
			want: "100",
		},
		{
			name: "一半成交应该是50%",
			order: OrderInfo{
				Quantity:         decimal.NewFromFloat(1.0),
				ExecutedQuantity: decimal.NewFromFloat(0.5),
			},
			want: "50",
		},
		{
			name: "未成交应该是0%",
			order: OrderInfo{
				Quantity:         decimal.NewFromFloat(1.0),
				ExecutedQuantity: decimal.Zero,
			},
			want: "0",
		},
		{
			name: "数量为0应该返回0",
			order: OrderInfo{
				Quantity:         decimal.Zero,
				ExecutedQuantity: decimal.Zero,
			},
			want: "0",
		},
		{
			name: "部分成交25%",
			order: OrderInfo{
				Quantity:         decimal.NewFromFloat(0.004),
				ExecutedQuantity: decimal.NewFromFloat(0.001),
			},
			want: "25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.order.GetFilledPercentage()
			if got.String() != tt.want {
				t.Errorf("GetFilledPercentage() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

// 示例：如何使用辅助方法
func ExamplePositionSide_GetCloseOrderSide() {
	// 如果持有多头仓位，需要卖出平仓
	longPosition := PositionSideLong
	closeOrderSide := longPosition.GetCloseOrderSide()
	_ = closeOrderSide // OrderSideSell

	// 如果持有空头仓位，需要买入平仓
	shortPosition := PositionSideShort
	closeOrderSide = shortPosition.GetCloseOrderSide()
	_ = closeOrderSide // OrderSideBuy
}

// 示例：如何判断订单状态
func ExampleOrderInfo_IsActive() {
	order := &OrderInfo{
		Id:               "123456",
		Quantity:         decimal.NewFromFloat(0.003),
		ExecutedQuantity: decimal.NewFromFloat(0.001),
		Status:           OrderStatusPartiallyFilled,
	}

	// 判断订单是否还在活跃状态（未完全成交）
	if order.IsActive() {
		// 订单还未完全成交，可以继续监控或取消
		percentage := order.GetFilledPercentage()
		_ = percentage // 33.33%
	}

	// 判断订单是否已经完全成交
	if order.Status.IsFilled() {
		// 订单已完全成交
	}
}

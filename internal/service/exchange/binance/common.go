package binance

import (
	"strings"

	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2"
)

func binanceSide(side exchange.Side) binance.SideType {
	switch side {
	case exchange.Buy:
		return binance.SideTypeBuy
	case exchange.Sell:
		return binance.SideTypeSell
	default:
		return ""
	}
}

func fromBinanceSide(side binance.SideType) exchange.Side {
	switch side {
	case binance.SideTypeBuy:
		return exchange.Buy
	case binance.SideTypeSell:
		return exchange.Sell
	default:
		return exchange.Side(side)
	}
}

func binanceOrderType(typ exchange.OrderType) binance.OrderType {
	switch typ {
	case exchange.OrderTypeLimit:
		return binance.OrderTypeLimit
	case exchange.OrderTypeMarket:
		return binance.OrderTypeMarket
	default:
		return ""
	}
}

func fromBinanceOrderType(typ binance.OrderType) exchange.OrderType {
	switch typ {
	case binance.OrderTypeLimit:
		return exchange.OrderTypeLimit
	case binance.OrderTypeMarket:
		return exchange.OrderTypeMarket
	default:
		return exchange.OrderType(typ)
	}
}

func reverseSide(side binance.SideType) binance.SideType {
	switch side {
	case binance.SideTypeBuy:
		return binance.SideTypeSell
	case binance.SideTypeSell:
		return binance.SideTypeBuy
	default:
		return ""
	}
}

func fromBinanceOrderStatus(status binance.OrderStatusType) exchange.OrderStatus {
	switch status {
	case binance.OrderStatusTypeNew:
		return exchange.OrderStatusCreated
	case binance.OrderStatusTypePartiallyFilled:
		return exchange.OrderStatusPartialFilled
	case binance.OrderStatusTypeFilled:
		return exchange.OrderStatusFilled
	case binance.OrderStatusTypeCanceled:
		return exchange.OrderStatusCancelled
	default:
		return exchange.OrderStatus(status)
	}
}

// fromBinanceSymbol converts a Binance symbol string to an exchange.Symbol.
// 目前仅支持 USDT USDC 交易对
func fromBinanceSymbol(symbol string) exchange.Symbol {
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return exchange.Symbol{
			Base:  base,
			Quote: "USDT",
		}
	}

	if strings.HasSuffix(symbol, "USDC") {
		base := strings.TrimSuffix(symbol, "USDC")
		return exchange.Symbol{
			Base:  base,
			Quote: "USDC",
		}
	}

	panic("unsupported symbol format: " + symbol)
}

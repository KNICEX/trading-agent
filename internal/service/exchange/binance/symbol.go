package binance

import (
	"context"
	"fmt"
	"github.com/KNICEX/trading-agent/internal/service/exchange"
	"github.com/adshao/go-binance/v2"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"log/slog"
	"strings"
)

// 过期/下架币种
var binanceOverdueSymbolBase = []string{
	"BCC", "VEN", "PAX", "BCHABC", "BCHSV", "WAVES", "BTT", "USDS", "XMR", "NANO", "OMG",
	"MITH", "MATIC", "FTM", "USDSB", "GTO", "ERD", "NPXS", "COCOS", "TOMO", "PERL", "MFT",
	"KEY", "STORM", "DOCK", "BUSD", "BEAM", "REN", "HC", "MCO", "VITE", "DREP", "BULL", "BEAR",
	"ETHBULL", "ETHBEAR", "TCT", "WRX", "BTS", "EOSBULL", "EOSBEAR", "XRPBULL", "XRPBEAR", "START", "AION",
	"BNBBULL", "BNBBEAR", "WTC", "XZC", "BTCUP", "BTCDOWN", "GXS", "LEND", "STMX", "REP", "PNT", "BKRW",
	"ETHUP", "ETHDOWN", "ADAUP", "ADADOWN", "LINKUP", "LINKDOWN", "GBP", "DAI", "XTZUP", "XTZDOWN",
	"AUD", "BLZ", "IRIS", "KMD", "JST", "SRM", "ANT", "OCEAN", "WNXM", "BZRX", "YFII", "EOSUP", "EOSDOWN",
	"TRXUP", "TRXDOWN", "DOTUP", "DOTDOWN", "LTCUP", "LTCDOWN", "NBS", "HNT", "UNIUP", "UNIDOWN",
	"ORN", "SXPUP", "SXPDOWN", "FILUP", "FILDOWN", "YFIUP", "YFIDOWN", "BCHUP", "BCHDOWN", "UNFI",
	"XEM", "AAVEUP", "AAVEDOWN", "SUSD", "SUSHIUP", "SUSHIDOWN", "XLMUP", "XLMDOWN", "REEF", "BTCST",
	"LIT", "LINA", "RANP", "EPS", "AUTO", "1INCHUP", "1INCHDOWN", "BTG", "MIR", "BURGER", "MDX",
	"NU", "TORN", "KEEP", "ERN", "KLAY", "CLV", "TVK", "BOND", "FOR", "TRIBE", "POLY", "FRONT", "CVP",
	"DAR", "BNX", "RGT", "KP3R", "VGX", "PLA", "RNDR", "MC", "ANY", "OOKI", "ANC", "NBT", "MULTI",
	"GAL", "EPX", "POLYX", "AGIX", "AMB", "BETH", "LOOM", "OAX", "AERGO", "AST", "COMBO", "GFT",
	"STRAT", "BNBUP", "BNBDOWN", "XRPUP", "XRPDOWN", "AKRO", "DNT", "RAMP", "POLS", "UST", "MOB",
	"NEBL",

	"USDC", "FUSDT", "USDP",
}

type SymbolService struct {
	cli         *binance.Client
	overdueBase map[string]struct{}
}

func NewSymbolService(cli *binance.Client) exchange.SymbolService {
	return &SymbolService{
		cli: cli,
		overdueBase: lo.SliceToMap(binanceOverdueSymbolBase, func(item string) (string, struct{}) {
			return item, struct{}{}
		}),
	}
}
func (svc *SymbolService) GetAllSymbols(ctx context.Context) ([]exchange.Symbol, error) {
	symbols, err := svc.cli.NewListPricesService().Do(ctx)
	if err != nil {
		return nil, err
	}

	symbols = svc.onlyUSDT(symbols)

	res := lo.Map(symbols, func(item *binance.SymbolPrice, index int) exchange.Symbol {
		price, err := decimal.NewFromString(item.Price)
		if err != nil {
			slog.Error("fail to parse price", "price", item.Price, "error", err)
			return exchange.Symbol{}
		}
		return exchange.Symbol{
			Base:  strings.TrimSuffix(item.Symbol, "USDT"),
			Quote: "USDT",
			Price: price,
		}
	})
	return svc.filterOverdue(res), nil
}

// filterOverdue 过滤掉过期的币种
func (svc *SymbolService) filterOverdue(s []exchange.Symbol) []exchange.Symbol {
	return lo.Reject(s, func(item exchange.Symbol, index int) bool {
		if _, ok := svc.overdueBase[item.Base]; ok {
			return true
		}
		return false
	})
}

func (svc *SymbolService) onlyUSDT(s []*binance.SymbolPrice) []*binance.SymbolPrice {
	return lo.Filter(s, func(item *binance.SymbolPrice, index int) bool {
		if strings.HasSuffix(item.Symbol, "USDT") {
			return true
		}
		return false
	})
}

func (svc *SymbolService) GetSymbolPrice(ctx context.Context, symbol exchange.Symbol) (exchange.Symbol, error) {
	s, err := svc.cli.NewListPricesService().Symbol(fmt.Sprintf("%s%s", symbol.Base, symbol.Quote)).Do(ctx)
	if err != nil {
		return exchange.Symbol{}, err
	}
	if len(s) == 0 {
		return exchange.Symbol{}, fmt.Errorf("symbol %s not found", symbol.Base)
	}
	price, err := decimal.NewFromString(s[0].Price)
	if err != nil {
		return exchange.Symbol{}, err
	}
	return exchange.Symbol{
		Base:  strings.TrimSuffix(s[0].Symbol, symbol.Quote),
		Quote: symbol.Quote,
		Price: price,
	}, nil
}

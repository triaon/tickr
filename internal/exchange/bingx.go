package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/etz/tickr/internal/httpx"
	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/tv"
)

type BingX struct {
	BaseURL string
	HTTP    *httpx.Client
	Log     *slog.Logger
}

func NewBingX(baseURL string, http *httpx.Client, log *slog.Logger) *BingX {
	if baseURL == "" {
		baseURL = "https://open-api.bingx.com"
	}
	return &BingX{BaseURL: baseURL, HTTP: http, Log: log}
}

func (b *BingX) Name() string { return "bingx" }

type bingxSpotSymbol struct {
	Symbol      string  `json:"symbol"`
	Status      int     `json:"status"`
	MinQty      float64 `json:"minQty"`
	MaxQty      float64 `json:"maxQty"`
	MinNotional float64 `json:"minNotional"`
	MaxNotional float64 `json:"maxNotional"`
	TickSize    float64 `json:"tickSize"`
	StepSize    float64 `json:"stepSize"`
	ApiStateBuy  bool   `json:"apiStateBuy"`
	ApiStateSell bool   `json:"apiStateSell"`
}

type bingxSpotResponse struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Symbols []bingxSpotSymbol `json:"symbols"`
	} `json:"data"`
}

type bingxSwapContract struct {
	Symbol            string `json:"symbol"`
	ContractID        string `json:"contractId"`
	Asset             string `json:"asset"`
	Currency          string `json:"currency"`
	QuantityPrecision int    `json:"quantityPrecision"`
	PricePrecision    int    `json:"pricePrecision"`
	TradeMinQuantity  float64 `json:"tradeMinQuantity"`
	TradeMinUSDT      float64 `json:"tradeMinUSDT"`
	TickSize          float64 `json:"tickSize"`
	Size              string `json:"size"`
	Status            int    `json:"status"`
	// "true" / "false" as strings. status=1 means the contract is registered;
	// apiStateOpen="true" means it is actually tradable now. Pre-launch and
	// halted contracts can be status=1 with apiStateOpen="false".
	APIStateOpen  string `json:"apiStateOpen"`
	APIStateClose string `json:"apiStateClose"`
	// DisplayName is the trader-facing pair name BingX uses in its UI. For
	// crypto it matches Symbol ("BTC-USDT"). For synthetic stocks/commodities/
	// forex it strips the internal NCSK/NCSI/NCCO encoding — e.g. symbol
	// "NCSKNVDA2USD-USDT" has displayName "NVDA-USDT". We prefer DisplayName
	// when the raw symbol has an internal prefix so the canonical output is
	// human-readable.
	DisplayName string `json:"displayName"`
}

type bingxSwapResponse struct {
	Code int                 `json:"code"`
	Msg  string              `json:"msg"`
	Data []bingxSwapContract `json:"data"`
}

func (b *BingX) Fetch(ctx context.Context, req model.FetchRequest) ([]model.Symbol, []model.Warning, error) {
	var out []model.Symbol
	var warns []model.Warning

	markets := model.MarketTypesFromCategories(req.Categories)
	for _, mt := range markets {
		switch mt {
		case "spot":
			ss, err := b.fetchSpot(ctx, req)
			if err != nil {
				return nil, warns, fmt.Errorf("bingx spot: %w", err)
			}
			out = append(out, ss...)
		case "swap":
			ss, err := b.fetchSwap(ctx, req)
			if err != nil {
				return nil, warns, fmt.Errorf("bingx swap: %w", err)
			}
			out = append(out, ss...)
		}
	}

	if model.HasCategory(req.Categories, model.CatTokenizedStock) ||
		model.HasCategory(req.Categories, model.CatCommodity) ||
		model.HasCategory(req.Categories, model.CatForex) {
		warns = append(warns, model.Warning{
			Level:   "warn",
			Message: "Categories tokenized_stock/commodity/forex are not reliably supported for exchange bingx via public market API",
		})
	}
	return out, warns, nil
}

func (b *BingX) fetchSpot(ctx context.Context, req model.FetchRequest) ([]model.Symbol, error) {
	body, _, err := b.HTTP.GetJSON(ctx, b.BaseURL+"/openApi/spot/v1/common/symbols", nil)
	if err != nil {
		return nil, err
	}
	var resp bingxSpotResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bingx spot code=%d msg=%s", resp.Code, resp.Msg)
	}
	out := make([]model.Symbol, 0, len(resp.Data.Symbols))
	for _, s := range resp.Data.Symbols {
		active := s.Status == 1
		if req.ActiveOnly && !active {
			continue
		}
		base, quote := splitDash(s.Symbol)
		canonical := strings.ReplaceAll(s.Symbol, "-", "")
		sym := model.Symbol{
			Exchange:       b.Name(),
			Symbol:         canonical,
			ExchangeSymbol: s.Symbol,
			BaseAsset:      base,
			QuoteAsset:     quote,
			MarketType:     model.MarketSpot,
			AssetCategory:  classifyBingXAsset(base, canonical),
			Status:         fmt.Sprintf("%d", s.Status),
			IsActive:       active,
			TickSize:       fmtFloat(s.TickSize),
			StepSize:       fmtFloat(s.StepSize),
			MinQty:         fmtFloat(s.MinQty),
			MinNotional:    fmtFloat(s.MinNotional),
		}
		sym.SetFlags()
		sym.TradingViewSymbol = tv.Symbol(sym, ".P")
		if req.IncludeRaw {
			if raw, err := json.Marshal(s); err == nil {
				sym.Raw = raw
			}
		}
		out = append(out, sym)
	}
	return out, nil
}

func (b *BingX) fetchSwap(ctx context.Context, req model.FetchRequest) ([]model.Symbol, error) {
	body, _, err := b.HTTP.GetJSON(ctx, b.BaseURL+"/openApi/swap/v2/quote/contracts", nil)
	if err != nil {
		return nil, err
	}
	var resp bingxSwapResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bingx swap code=%d msg=%s", resp.Code, resp.Msg)
	}
	out := make([]model.Symbol, 0, len(resp.Data))
	for _, c := range resp.Data {
		active := c.Status == 1 && c.APIStateOpen == "true"
		if req.ActiveOnly && !active {
			continue
		}
		// classifyBingXAsset needs to see the raw NCSK/NCSI/NCCO prefix so the
		// asset category is still correct for tokenized stocks etc.
		assetCategory := classifyBingXAsset(c.Asset, strings.ReplaceAll(c.Symbol, "-", ""))

		// Prefer displayName ("NVDA-USDT") over the raw encoded symbol
		// ("NCSKNVDA2USD-USDT") when the contract is a synthetic instrument.
		// For pure crypto the two are identical so this is a no-op.
		nameSource := c.Symbol
		if c.DisplayName != "" && hasBingXSyntheticPrefix(c.Asset) {
			nameSource = c.DisplayName
		}
		base, quote := splitDash(nameSource)
		if base == "" {
			base = c.Asset
		}
		if quote == "" {
			quote = c.Currency
		}
		canonical := strings.ReplaceAll(nameSource, "-", "")
		sym := model.Symbol{
			Exchange:       b.Name(),
			Symbol:         canonical,
			ExchangeSymbol: c.Symbol,
			BaseAsset:      base,
			QuoteAsset:     quote,
			SettleAsset:    quote,
			MarketType:     model.MarketSwap,
			ContractType:   model.ContractPerpetual,
			AssetCategory:  assetCategory,
			Status:         fmt.Sprintf("%d", c.Status),
			IsActive:       active,
			TickSize:       fmtFloat(c.TickSize),
			MinQty:         fmtFloat(c.TradeMinQuantity),
			MinNotional:    fmtFloat(c.TradeMinUSDT),
		}
		if c.PricePrecision > 0 {
			pp := c.PricePrecision
			sym.PricePrecision = &pp
		}
		if c.QuantityPrecision > 0 {
			qp := c.QuantityPrecision
			sym.QuantityPrecision = &qp
		}
		sym.SetFlags()
		sym.TradingViewSymbol = tv.Symbol(sym, ".P")
		if req.IncludeRaw {
			if raw, err := json.Marshal(c); err == nil {
				sym.Raw = raw
			}
		}
		out = append(out, sym)
	}
	return out, nil
}

// hasBingXSyntheticPrefix reports whether the asset code carries one of the
// internal wrappers BingX uses for non-crypto instruments. When true, the
// public symbol is encoded ("NCSKNVDA2USD-USDT") and we should prefer the
// human-readable displayName for naming purposes.
func hasBingXSyntheticPrefix(asset string) bool {
	up := strings.ToUpper(asset)
	return strings.HasPrefix(up, "NCSK") ||
		strings.HasPrefix(up, "NCSI") ||
		strings.HasPrefix(up, "NCCO") ||
		strings.HasPrefix(up, "NCFX")
}

// classifyBingXAsset detects synthetic/non-crypto instruments that BingX lists
// alongside regular USDT crypto perps. BingX wraps third-party data providers
// under a small set of prefixes — each maps to a clear asset_category:
//   NCFX* → forex (New Change FX pairs)
//   NCSK* → tokenized_stock (single-stock perps like AAPL, TSLA)
//   NCSI* → tokenized_stock (stock indexes / ETFs: SPY, QQQ, NASDAQ, DXY)
//   NCCO* → commodity (gold, oil, palladium, natural gas, …)
// A small additional whitelist catches plain index/commodity tickers without
// the NC* prefix. Everything else stays crypto.
func classifyBingXAsset(base, canonical string) string {
	up := strings.ToUpper(base)
	canUp := strings.ToUpper(canonical)

	hasPref := func(p string) bool {
		return strings.HasPrefix(up, p) || strings.HasPrefix(canUp, p)
	}
	switch {
	case hasPref("NCFX"):
		return model.AssetForex
	case hasPref("NCSK"), hasPref("NCSI"):
		return model.AssetTokenizedStock
	case hasPref("NCCO"):
		return model.AssetCommodity
	}
	switch up {
	case "SPX", "NDX", "DJI", "RUT", "NAS100", "US100", "US500", "US30",
		"DAX", "FTSE", "FTSE100", "NIKKEI", "NIKKEI225", "HSI", "CAC40":
		return model.AssetTokenizedStock
	case "XAU", "XAG", "GOLD", "SILVER", "USOIL", "UKOIL", "WTI", "BRENT",
		"NATGAS", "COPPER", "PLATINUM", "PALLADIUM":
		return model.AssetCommodity
	}
	return model.AssetCrypto
}

func splitDash(s string) (string, string) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func fmtFloat(f float64) string {
	if f == 0 {
		return ""
	}
	// Avoid scientific notation for tiny tick sizes.
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.12f", f), "0"), ".")
}

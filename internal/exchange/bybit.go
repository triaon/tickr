package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/etz/tickr/internal/httpx"
	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/tv"
)

type Bybit struct {
	BaseURL string
	HTTP    *httpx.Client
	Log     *slog.Logger
}

func NewBybit(baseURL string, http *httpx.Client, log *slog.Logger) *Bybit {
	if baseURL == "" {
		baseURL = "https://api.bybit.com"
	}
	return &Bybit{BaseURL: baseURL, HTTP: http, Log: log}
}

func (b *Bybit) Name() string { return "bybit" }

type bybitInstrumentsResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Category       string             `json:"category"`
		List           []bybitInstrument  `json:"list"`
		NextPageCursor string             `json:"nextPageCursor"`
	} `json:"result"`
}

type bybitInstrument struct {
	Symbol        string `json:"symbol"`
	ContractType  string `json:"contractType"`
	Status        string `json:"status"`
	BaseCoin      string `json:"baseCoin"`
	QuoteCoin     string `json:"quoteCoin"`
	SettleCoin    string `json:"settleCoin"`
	SymbolType    string `json:"symbolType"`
	PriceFilter   struct {
		TickSize string `json:"tickSize"`
	} `json:"priceFilter"`
	LotSizeFilter struct {
		QtyStep            string `json:"qtyStep"`
		BasePrecision      string `json:"basePrecision"`
		MinOrderQty        string `json:"minOrderQty"`
		MinNotionalValue   string `json:"minNotionalValue"`
		MinOrderAmt        string `json:"minOrderAmt"`
	} `json:"lotSizeFilter"`
}

func (b *Bybit) Fetch(ctx context.Context, req model.FetchRequest) ([]model.Symbol, []model.Warning, error) {
	var out []model.Symbol
	var warns []model.Warning

	markets := model.MarketTypesFromCategories(req.Categories)
	categories := map[string]bool{}
	for _, m := range markets {
		switch m {
		case "spot":
			categories["spot"] = true
		case "swap":
			categories["linear"] = true
			// inverse is reserved per spec; include if explicit market hint requests it
			if strings.EqualFold(req.Market, "inverse") {
				categories["inverse"] = true
			}
		}
	}

	for cat := range categories {
		instruments, err := b.fetchCategory(ctx, cat)
		if err != nil {
			return nil, warns, fmt.Errorf("bybit %s: %w", cat, err)
		}
		for _, ins := range instruments {
			active := ins.Status == "Trading"
			if req.ActiveOnly && !active {
				continue
			}
			marketType := model.MarketSpot
			contractType := ""
			if cat == "linear" || cat == "inverse" {
				if strings.Contains(strings.ToLower(ins.ContractType), "perpetual") {
					marketType = model.MarketSwap
					contractType = model.ContractPerpetual
				} else if ins.ContractType != "" {
					marketType = model.MarketFuture
					contractType = ins.ContractType
					continue // not requested in this version
				} else {
					marketType = model.MarketSwap
				}
			}
			sym := model.Symbol{
				Exchange:       b.Name(),
				Symbol:         ins.Symbol,
				ExchangeSymbol: ins.Symbol,
				BaseAsset:      ins.BaseCoin,
				QuoteAsset:     ins.QuoteCoin,
				SettleAsset:    ins.SettleCoin,
				MarketType:     marketType,
				ContractType:   contractType,
				AssetCategory:  bybitAssetCategory(ins.SymbolType),
				Status:         ins.Status,
				IsActive:       active,
				TickSize:       ins.PriceFilter.TickSize,
				StepSize:       firstNonEmpty(ins.LotSizeFilter.QtyStep, ins.LotSizeFilter.BasePrecision),
				MinQty:         ins.LotSizeFilter.MinOrderQty,
				MinNotional:    firstNonEmpty(ins.LotSizeFilter.MinNotionalValue, ins.LotSizeFilter.MinOrderAmt),
			}
			sym.SetFlags()
			sym.TradingViewSymbol = tv.Symbol(sym, ".P")
			if req.IncludeRaw {
				if raw, err := json.Marshal(ins); err == nil {
					sym.Raw = raw
				}
			}
			out = append(out, sym)
		}
	}
	return out, warns, nil
}

func (b *Bybit) fetchCategory(ctx context.Context, category string) ([]bybitInstrument, error) {
	var all []bybitInstrument
	cursor := ""
	for {
		params := url.Values{}
		params.Set("category", category)
		params.Set("limit", "1000")
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		body, _, err := b.HTTP.GetJSON(ctx, b.BaseURL+"/v5/market/instruments-info", params)
		if err != nil {
			return nil, err
		}
		var resp bybitInstrumentsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		if resp.RetCode != 0 {
			return nil, fmt.Errorf("bybit retCode=%d msg=%s", resp.RetCode, resp.RetMsg)
		}
		all = append(all, resp.Result.List...)
		if resp.Result.NextPageCursor == "" {
			break
		}
		cursor = resp.Result.NextPageCursor
	}
	return all, nil
}

func bybitAssetCategory(symbolType string) string {
	t := strings.ToLower(symbolType)
	switch {
	case strings.Contains(t, "stock"):
		return model.AssetTokenizedStock
	case strings.Contains(t, "commodity"):
		return model.AssetCommodity
	case strings.Contains(t, "forex") || strings.Contains(t, "fx"):
		return model.AssetForex
	default:
		return model.AssetCrypto
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

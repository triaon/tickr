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

type MEXC struct {
	SpotBaseURL    string
	FuturesBaseURL string
	HTTP           *httpx.Client
	Log            *slog.Logger
}

func NewMEXC(spotURL, futURL string, http *httpx.Client, log *slog.Logger) *MEXC {
	if spotURL == "" {
		spotURL = "https://api.mexc.com"
	}
	if futURL == "" {
		futURL = "https://contract.mexc.com"
	}
	return &MEXC{SpotBaseURL: spotURL, FuturesBaseURL: futURL, HTTP: http, Log: log}
}

func (m *MEXC) Name() string { return "mexc" }

type mexcSpotFilter struct {
	FilterType  string `json:"filterType"`
	TickSize    string `json:"tickSize,omitempty"`
	StepSize    string `json:"stepSize,omitempty"`
	MinQty      string `json:"minQty,omitempty"`
	MinNotional string `json:"minNotional,omitempty"`
}

type mexcSpotSymbol struct {
	Symbol               string           `json:"symbol"`
	Status               string           `json:"status"`
	BaseAsset            string           `json:"baseAsset"`
	BaseAssetPrecision   int              `json:"baseAssetPrecision"`
	QuoteAsset           string           `json:"quoteAsset"`
	QuotePrecision       int              `json:"quotePrecision"`
	QuoteAssetPrecision  int              `json:"quoteAssetPrecision"`
	Filters              []mexcSpotFilter `json:"filters"`
}

type mexcSpotExchangeInfo struct {
	Symbols []mexcSpotSymbol `json:"symbols"`
}

type mexcContract struct {
	Symbol      string  `json:"symbol"`
	DisplayName string  `json:"displayName"`
	BaseCoin    string  `json:"baseCoin"`
	QuoteCoin   string  `json:"quoteCoin"`
	SettleCoin  string  `json:"settleCoin"`
	State       int     `json:"state"`
	FutureType  int     `json:"futureType"`
	PriceScale  int     `json:"priceScale"`
	VolScale    int     `json:"volScale"`
	MinVol      float64 `json:"minVol"`
	MaxVol      float64 `json:"maxVol"`
}

type mexcContractResponse struct {
	Success bool           `json:"success"`
	Code    int            `json:"code"`
	Data    []mexcContract `json:"data"`
}

func (m *MEXC) Fetch(ctx context.Context, req model.FetchRequest) ([]model.Symbol, []model.Warning, error) {
	var out []model.Symbol
	var warns []model.Warning

	markets := model.MarketTypesFromCategories(req.Categories)
	for _, mt := range markets {
		switch mt {
		case "spot":
			ss, err := m.fetchSpot(ctx, req)
			if err != nil {
				return nil, warns, fmt.Errorf("mexc spot: %w", err)
			}
			out = append(out, ss...)
		case "swap":
			ss, err := m.fetchSwap(ctx, req)
			if err != nil {
				return nil, warns, fmt.Errorf("mexc swap: %w", err)
			}
			out = append(out, ss...)
		}
	}

	if model.HasCategory(req.Categories, model.CatTokenizedStock) ||
		model.HasCategory(req.Categories, model.CatCommodity) ||
		model.HasCategory(req.Categories, model.CatForex) {
		warns = append(warns, model.Warning{
			Level:   "warn",
			Message: "Categories tokenized_stock/commodity/forex are not reliably supported for exchange mexc via public market API",
		})
	}
	return out, warns, nil
}

// isMexcSpotActive returns whether a spot symbol status represents a tradable instrument.
func isMexcSpotActive(status string) bool {
	switch strings.ToUpper(status) {
	case "ENABLED", "1", "TRADING":
		return true
	default:
		return false
	}
}

func (m *MEXC) fetchSpot(ctx context.Context, req model.FetchRequest) ([]model.Symbol, error) {
	body, _, err := m.HTTP.GetJSON(ctx, m.SpotBaseURL+"/api/v3/exchangeInfo", nil)
	if err != nil {
		return nil, err
	}
	var info mexcSpotExchangeInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	out := make([]model.Symbol, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		active := isMexcSpotActive(s.Status)
		if req.ActiveOnly && !active {
			continue
		}
		sym := model.Symbol{
			Exchange:       m.Name(),
			Symbol:         s.Symbol,
			ExchangeSymbol: s.Symbol,
			BaseAsset:      s.BaseAsset,
			QuoteAsset:     s.QuoteAsset,
			MarketType:     model.MarketSpot,
			AssetCategory:  model.AssetCrypto,
			Status:         s.Status,
			IsActive:       active,
		}
		qp := s.QuoteAssetPrecision
		if qp == 0 {
			qp = s.QuotePrecision
		}
		if qp > 0 {
			sym.PricePrecision = &qp
		}
		if s.BaseAssetPrecision > 0 {
			bp := s.BaseAssetPrecision
			sym.QuantityPrecision = &bp
		}
		for _, f := range s.Filters {
			switch f.FilterType {
			case "PRICE_FILTER":
				sym.TickSize = f.TickSize
			case "LOT_SIZE":
				sym.StepSize = f.StepSize
				sym.MinQty = f.MinQty
			case "MIN_NOTIONAL":
				sym.MinNotional = f.MinNotional
			}
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

func (m *MEXC) fetchSwap(ctx context.Context, req model.FetchRequest) ([]model.Symbol, error) {
	body, _, err := m.HTTP.GetJSON(ctx, m.FuturesBaseURL+"/api/v1/contract/detail", nil)
	if err != nil {
		return nil, err
	}
	var resp mexcContractResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if !resp.Success && resp.Code != 0 {
		return nil, fmt.Errorf("mexc contract detail code=%d", resp.Code)
	}
	out := make([]model.Symbol, 0, len(resp.Data))
	for _, c := range resp.Data {
		// State 0 typically means trading on MEXC contract API.
		active := c.State == 0
		if req.ActiveOnly && !active {
			continue
		}
		// futureType 1 = perpetual on MEXC.
		isPerpetual := c.FutureType == 1
		if !isPerpetual {
			continue
		}
		canonical := strings.ReplaceAll(c.Symbol, "_", "")
		sym := model.Symbol{
			Exchange:       m.Name(),
			Symbol:         canonical,
			ExchangeSymbol: c.Symbol,
			BaseAsset:      c.BaseCoin,
			QuoteAsset:     c.QuoteCoin,
			SettleAsset:    c.SettleCoin,
			MarketType:     model.MarketSwap,
			ContractType:   model.ContractPerpetual,
			AssetCategory:  model.AssetCrypto,
			Status:         fmt.Sprintf("state=%d", c.State),
			IsActive:       active,
		}
		// Backfill quote/settle from suffix when API omits them.
		if sym.QuoteAsset == "" && strings.Contains(c.Symbol, "_") {
			parts := strings.SplitN(c.Symbol, "_", 2)
			if len(parts) == 2 {
				if sym.BaseAsset == "" {
					sym.BaseAsset = parts[0]
				}
				sym.QuoteAsset = parts[1]
			}
		}
		if sym.SettleAsset == "" {
			sym.SettleAsset = sym.QuoteAsset
		}
		if c.PriceScale > 0 {
			ps := c.PriceScale
			sym.PricePrecision = &ps
		}
		if c.VolScale > 0 {
			vs := c.VolScale
			sym.QuantityPrecision = &vs
		}
		if c.MinVol > 0 {
			sym.MinQty = fmt.Sprintf("%g", c.MinVol)
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

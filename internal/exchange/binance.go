package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/etz/tickr/internal/httpx"
	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/tv"
)

type Binance struct {
	SpotBaseURL    string
	FuturesBaseURL string
	HTTP           *httpx.Client
	Log            *slog.Logger
}

func NewBinance(spotURL, futURL string, http *httpx.Client, log *slog.Logger) *Binance {
	if spotURL == "" {
		spotURL = "https://api.binance.com"
	}
	if futURL == "" {
		futURL = "https://fapi.binance.com"
	}
	return &Binance{SpotBaseURL: spotURL, FuturesBaseURL: futURL, HTTP: http, Log: log}
}

func (b *Binance) Name() string { return "binance" }

type binanceFilter struct {
	FilterType  string `json:"filterType"`
	TickSize    string `json:"tickSize,omitempty"`
	StepSize    string `json:"stepSize,omitempty"`
	MinQty      string `json:"minQty,omitempty"`
	MinNotional string `json:"minNotional,omitempty"`
	Notional    string `json:"notional,omitempty"`
}

type binanceSpotSymbol struct {
	Symbol             string          `json:"symbol"`
	Status             string          `json:"status"`
	BaseAsset          string          `json:"baseAsset"`
	BaseAssetPrecision int             `json:"baseAssetPrecision"`
	QuoteAsset         string          `json:"quoteAsset"`
	QuotePrecision     int             `json:"quotePrecision"`
	Filters            []binanceFilter `json:"filters"`
	Permissions        []string        `json:"permissions"`
	IsSpotTradingAllowed bool          `json:"isSpotTradingAllowed"`
}

type binanceSpotExchangeInfo struct {
	Symbols []binanceSpotSymbol `json:"symbols"`
}

type binanceFuturesSymbol struct {
	Symbol             string          `json:"symbol"`
	Pair               string          `json:"pair"`
	Status             string          `json:"status"`
	ContractType       string          `json:"contractType"`
	BaseAsset          string          `json:"baseAsset"`
	QuoteAsset         string          `json:"quoteAsset"`
	MarginAsset        string          `json:"marginAsset"`
	PricePrecision     int             `json:"pricePrecision"`
	QuantityPrecision  int             `json:"quantityPrecision"`
	Filters            []binanceFilter `json:"filters"`
}

type binanceFuturesExchangeInfo struct {
	Symbols []binanceFuturesSymbol `json:"symbols"`
}

func (b *Binance) Fetch(ctx context.Context, req model.FetchRequest) ([]model.Symbol, []model.Warning, error) {
	var out []model.Symbol
	var warns []model.Warning

	markets := model.MarketTypesFromCategories(req.Categories)
	for _, mt := range markets {
		switch mt {
		case "spot":
			ss, err := b.fetchSpot(ctx, req)
			if err != nil {
				return nil, warns, fmt.Errorf("binance spot: %w", err)
			}
			out = append(out, ss...)
		case "swap":
			ss, err := b.fetchSwap(ctx, req)
			if err != nil {
				return nil, warns, fmt.Errorf("binance swap: %w", err)
			}
			out = append(out, ss...)
		}
	}

	if model.HasCategory(req.Categories, model.CatTokenizedStock) ||
		model.HasCategory(req.Categories, model.CatCommodity) ||
		model.HasCategory(req.Categories, model.CatForex) {
		warns = append(warns, model.Warning{
			Level:   "warn",
			Message: "Categories tokenized_stock/commodity/forex are not reliably supported for exchange binance via public market API",
		})
	}
	return out, warns, nil
}

func (b *Binance) fetchSpot(ctx context.Context, req model.FetchRequest) ([]model.Symbol, error) {
	body, _, err := b.HTTP.GetJSON(ctx, b.SpotBaseURL+"/api/v3/exchangeInfo", nil)
	if err != nil {
		return nil, err
	}
	var info binanceSpotExchangeInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	out := make([]model.Symbol, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		active := s.Status == "TRADING"
		if req.ActiveOnly && !active {
			continue
		}
		sym := model.Symbol{
			Exchange:       b.Name(),
			Symbol:         s.Symbol,
			ExchangeSymbol: s.Symbol,
			BaseAsset:      s.BaseAsset,
			QuoteAsset:     s.QuoteAsset,
			MarketType:     model.MarketSpot,
			AssetCategory:  model.AssetCrypto,
			Status:         s.Status,
			IsActive:       active,
		}
		bp, qp := s.BaseAssetPrecision, s.QuotePrecision
		_ = bp
		if qp > 0 {
			qp := qp
			sym.PricePrecision = &qp
		}
		if s.BaseAssetPrecision > 0 {
			bp := s.BaseAssetPrecision
			sym.QuantityPrecision = &bp
		}
		applyBinanceFilters(&sym, s.Filters)
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

func (b *Binance) fetchSwap(ctx context.Context, req model.FetchRequest) ([]model.Symbol, error) {
	body, _, err := b.HTTP.GetJSON(ctx, b.FuturesBaseURL+"/fapi/v1/exchangeInfo", nil)
	if err != nil {
		return nil, err
	}
	var info binanceFuturesExchangeInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	out := make([]model.Symbol, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		active := s.Status == "TRADING"
		if req.ActiveOnly && !active {
			continue
		}
		mt := model.MarketUnknown
		ct := ""
		switch s.ContractType {
		case "PERPETUAL":
			mt = model.MarketSwap
			ct = model.ContractPerpetual
		case "CURRENT_QUARTER", "NEXT_QUARTER", "CURRENT_MONTH", "NEXT_MONTH":
			mt = model.MarketFuture
			ct = s.ContractType
		default:
			mt = model.MarketSwap
			ct = s.ContractType
		}
		// Only swap is requested by spec for this version.
		if mt != model.MarketSwap {
			continue
		}
		sym := model.Symbol{
			Exchange:       b.Name(),
			Symbol:         s.Symbol,
			ExchangeSymbol: s.Symbol,
			BaseAsset:      s.BaseAsset,
			QuoteAsset:     s.QuoteAsset,
			SettleAsset:    s.MarginAsset,
			MarketType:     mt,
			ContractType:   ct,
			AssetCategory:  model.AssetCrypto,
			Status:         s.Status,
			IsActive:       active,
		}
		pp, qp := s.PricePrecision, s.QuantityPrecision
		if pp > 0 {
			pp := pp
			sym.PricePrecision = &pp
		}
		if qp > 0 {
			qp := qp
			sym.QuantityPrecision = &qp
		}
		applyBinanceFilters(&sym, s.Filters)
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

func applyBinanceFilters(sym *model.Symbol, filters []binanceFilter) {
	for _, f := range filters {
		switch f.FilterType {
		case "PRICE_FILTER":
			sym.TickSize = f.TickSize
		case "LOT_SIZE":
			sym.StepSize = f.StepSize
			sym.MinQty = f.MinQty
		case "MIN_NOTIONAL":
			if f.MinNotional != "" {
				sym.MinNotional = f.MinNotional
			}
		case "NOTIONAL":
			if f.MinNotional != "" {
				sym.MinNotional = f.MinNotional
			} else if f.Notional != "" {
				sym.MinNotional = f.Notional
			}
		}
	}
}

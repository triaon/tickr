package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/etz/tickr/internal/config"
	"github.com/etz/tickr/internal/exchange"
	"github.com/etz/tickr/internal/httpx"
	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/output"
	"github.com/etz/tickr/internal/tv"
)

type FetchOptions struct {
	Exchange   string
	Categories []model.Category
	ActiveOnly bool
	IncludeRaw bool
	Quote      string
	Base       string
	Market     string
	TVSuffix   string
	Reverse    bool
}

// BuildRegistry constructs a registry from config.
func BuildRegistry(cfg *config.Config, http *httpx.Client, log *slog.Logger) *exchange.Registry {
	r := exchange.NewRegistry()
	if ex, ok := cfg.Exchanges["binance"]; ok && ex.Enabled {
		r.Register(exchange.NewBinance(ex.SpotBaseURL, ex.FuturesBaseURL, http, log))
	}
	if ex, ok := cfg.Exchanges["bybit"]; ok && ex.Enabled {
		r.Register(exchange.NewBybit(ex.BaseURL, http, log))
	}
	if ex, ok := cfg.Exchanges["mexc"]; ok && ex.Enabled {
		r.Register(exchange.NewMEXC(ex.SpotBaseURL, ex.FuturesBaseURL, http, log))
	}
	if ex, ok := cfg.Exchanges["bingx"]; ok && ex.Enabled {
		r.Register(exchange.NewBingX(ex.BaseURL, http, log))
	}
	return r
}

// Fetch runs the fetch flow for a single exchange and returns the filtered envelope.
func Fetch(ctx context.Context, reg *exchange.Registry, opts FetchOptions, log *slog.Logger) (output.Envelope, error) {
	adapter, err := reg.Get(opts.Exchange)
	if err != nil {
		return output.Envelope{}, err
	}
	req := model.FetchRequest{
		Categories: opts.Categories,
		ActiveOnly: opts.ActiveOnly,
		IncludeRaw: opts.IncludeRaw,
		Quote:      opts.Quote,
		Base:       opts.Base,
		Market:     opts.Market,
	}
	raw, warns, err := adapter.Fetch(ctx, req)
	if err != nil {
		return output.Envelope{}, fmt.Errorf("%s fetch: %w", opts.Exchange, err)
	}
	log.Debug("fetched raw symbols", "exchange", opts.Exchange, "count", len(raw))

	// Re-apply TV suffix in case caller overrode it.
	if opts.TVSuffix != "" && opts.TVSuffix != ".P" {
		for i := range raw {
			raw[i].TradingViewSymbol = tv.Symbol(raw[i], opts.TVSuffix)
		}
	}

	filtered := Apply(raw, FilterOpts{
		Categories: opts.Categories,
		Quote:      opts.Quote,
		Base:       opts.Base,
		Market:     opts.Market,
	})
	log.Debug("after filter", "exchange", opts.Exchange, "count", len(filtered))

	if opts.Reverse {
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}

	// If user requested an asset category that is empty, add a warning.
	if (model.HasCategory(opts.Categories, model.CatTokenizedStock) ||
		model.HasCategory(opts.Categories, model.CatCommodity) ||
		model.HasCategory(opts.Categories, model.CatForex)) && len(filtered) == 0 {
		warns = append(warns, model.Warning{
			Level:   "warn",
			Message: fmt.Sprintf("No symbols matched the requested asset categories on %s", opts.Exchange),
		})
	}

	cats := make([]string, 0, len(opts.Categories))
	for _, c := range opts.Categories {
		cats = append(cats, string(c))
	}

	return output.Envelope{
		Meta: output.Meta{
			Exchange:   opts.Exchange,
			Categories: cats,
			FetchedAt:  time.Now().UTC(),
			Count:      len(filtered),
		},
		Symbols:  filtered,
		Warnings: warns,
	}, nil
}

package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/etz/tickr/internal/model"
)

type Meta struct {
	Exchange   string    `json:"exchange"`
	Categories []string  `json:"categories"`
	FetchedAt  time.Time `json:"fetched_at"`
	Count      int       `json:"count"`
}

type Envelope struct {
	Meta     Meta             `json:"meta"`
	Symbols  []model.Symbol   `json:"symbols"`
	Warnings []model.Warning  `json:"warnings,omitempty"`
}

// Writer abstracts where bytes go (stdout or file).
func Writer(path string) (io.WriteCloser, error) {
	if path == "" || path == "-" {
		return nopCloser{os.Stdout}, nil
	}
	return os.Create(path)
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func WriteJSON(w io.Writer, env Envelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

func WriteCSV(w io.Writer, env Envelope) error {
	cw := csv.NewWriter(w)
	header := []string{
		"exchange", "symbol", "exchange_symbol", "base_asset", "quote_asset",
		"settle_asset", "market_type", "contract_type", "asset_category",
		"status", "is_active", "tradingview_symbol",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, s := range env.Symbols {
		row := []string{
			s.Exchange, s.Symbol, s.ExchangeSymbol, s.BaseAsset, s.QuoteAsset,
			s.SettleAsset, s.MarketType, s.ContractType, s.AssetCategory,
			s.Status, strconv.FormatBool(s.IsActive), s.TradingViewSymbol,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

type TxtOptions struct {
	TradingView bool
	Separator   string // "newline" (default) or "comma"
}

func WriteTXT(w io.Writer, env Envelope, opts TxtOptions) error {
	parts := make([]string, 0, len(env.Symbols))
	for _, s := range env.Symbols {
		if opts.TradingView {
			parts = append(parts, s.TradingViewSymbol)
		} else {
			parts = append(parts, s.Symbol)
		}
	}
	sep := "\n"
	if opts.Separator == "comma" {
		sep = ","
	}
	_, err := fmt.Fprint(w, strings.Join(parts, sep))
	if err != nil {
		return err
	}
	if sep == "\n" && len(parts) > 0 {
		_, err = fmt.Fprintln(w)
	}
	return err
}

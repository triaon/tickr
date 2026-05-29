package tv

import (
	"strings"

	"github.com/etz/tickr/internal/model"
)

// Symbol builds a TradingView-style symbol like BINANCE:BTCUSDT or BYBIT:BTCUSDT.P.
// suffixPerp defaults to ".P" if empty.
func Symbol(s model.Symbol, suffixPerp string) string {
	if suffixPerp == "" {
		suffixPerp = ".P"
	}
	prefix := exchangePrefix(s.Exchange)
	base := s.Symbol
	if s.MarketType == model.MarketSwap {
		return prefix + ":" + base + suffixPerp
	}
	return prefix + ":" + base
}

func exchangePrefix(name string) string {
	switch strings.ToLower(name) {
	case "binance":
		return "BINANCE"
	case "bybit":
		return "BYBIT"
	case "mexc":
		return "MEXC"
	case "bingx":
		return "BINGX"
	default:
		return strings.ToUpper(name)
	}
}

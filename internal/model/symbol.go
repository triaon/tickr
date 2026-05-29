package model

import (
	"encoding/json"
)

const (
	MarketSpot    = "spot"
	MarketSwap    = "swap"
	MarketFuture  = "future"
	MarketOption  = "option"
	MarketUnknown = "unknown"

	ContractPerpetual = "perpetual"

	AssetCrypto         = "crypto"
	AssetTokenizedStock = "tokenized_stock"
	AssetCommodity      = "commodity"
	AssetForex          = "forex"
	AssetUnknown        = "unknown"
)

type Symbol struct {
	Exchange       string `json:"exchange"`
	Symbol         string `json:"symbol"`          // canonical e.g. BTCUSDT
	ExchangeSymbol string `json:"exchange_symbol"` // raw form from API
	BaseAsset      string `json:"base_asset"`
	QuoteAsset     string `json:"quote_asset"`
	SettleAsset    string `json:"settle_asset"`
	MarketType     string `json:"market_type"`
	ContractType   string `json:"contract_type"`
	AssetCategory  string `json:"asset_category"`
	Status         string `json:"status"`
	IsActive       bool   `json:"is_active"`

	IsSpot           bool `json:"is_spot"`
	IsSwap           bool `json:"is_swap"`
	IsUSDT           bool `json:"is_usdt"`
	IsUSDC           bool `json:"is_usdc"`
	IsTokenizedStock bool `json:"is_tokenized_stock"`
	IsCommodity      bool `json:"is_commodity"`
	IsForex          bool `json:"is_forex"`

	PricePrecision    *int   `json:"price_precision"`
	QuantityPrecision *int   `json:"quantity_precision"`
	TickSize          string `json:"tick_size"`
	StepSize          string `json:"step_size"`
	MinQty            string `json:"min_qty"`
	MinNotional       string `json:"min_notional"`

	TradingViewSymbol string          `json:"tradingview_symbol"`
	Raw               json.RawMessage `json:"raw,omitempty"`
}

// Key returns a stable identifier suitable for diff/state by exchange+market+symbol.
func (s Symbol) Key() string {
	return s.Exchange + "|" + s.MarketType + "|" + s.Symbol
}

// SetFlags fills IsSpot/IsSwap/IsUSDT/IsUSDC and asset-category flags from primary fields.
func (s *Symbol) SetFlags() {
	s.IsSpot = s.MarketType == MarketSpot
	s.IsSwap = s.MarketType == MarketSwap
	s.IsUSDT = s.QuoteAsset == "USDT" || s.SettleAsset == "USDT"
	s.IsUSDC = s.QuoteAsset == "USDC" || s.SettleAsset == "USDC"
	s.IsTokenizedStock = s.AssetCategory == AssetTokenizedStock
	s.IsCommodity = s.AssetCategory == AssetCommodity
	s.IsForex = s.AssetCategory == AssetForex
}

type Warning struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type FetchRequest struct {
	Categories []Category
	ActiveOnly bool
	IncludeRaw bool
	Quote      string // optional further filter
	Base       string // optional
	Market     string // optional explicit market hint
}

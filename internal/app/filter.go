package app

import (
	"strings"

	"github.com/etz/tickr/internal/model"
)

// FilterOpts captures all filters applied after raw fetch.
type FilterOpts struct {
	Categories []model.Category
	Quote      string // optional explicit quote filter (e.g. USDT)
	Base       string // optional explicit base filter
	Market     string // optional explicit market_type filter
}

// Apply returns symbols that match all category and explicit filters.
func Apply(syms []model.Symbol, f FilterOpts) []model.Symbol {
	wantSpot := model.HasCategory(f.Categories, model.CatSpot)
	wantSwap := model.HasCategory(f.Categories, model.CatSwap)
	wantUSDT := model.HasCategory(f.Categories, model.CatUSDT)
	wantUSDC := model.HasCategory(f.Categories, model.CatUSDC)
	wantStock := model.HasCategory(f.Categories, model.CatTokenizedStock)
	wantCmd := model.HasCategory(f.Categories, model.CatCommodity)
	wantFx := model.HasCategory(f.Categories, model.CatForex)

	hasMarketCat := wantSpot || wantSwap
	hasAssetCat := wantStock || wantCmd || wantFx

	q := strings.ToUpper(f.Quote)
	b := strings.ToUpper(f.Base)
	m := strings.ToLower(f.Market)

	out := make([]model.Symbol, 0, len(syms))
	for _, s := range syms {
		if hasMarketCat {
			if s.MarketType == model.MarketSpot && !wantSpot {
				continue
			}
			if s.MarketType == model.MarketSwap && !wantSwap {
				continue
			}
			if s.MarketType != model.MarketSpot && s.MarketType != model.MarketSwap {
				continue
			}
		}
		// USDT/USDC act as quote/settle filter. If both are requested it's an OR.
		if wantUSDT || wantUSDC {
			okU := wantUSDT && s.IsUSDT
			okC := wantUSDC && s.IsUSDC
			if !(okU || okC) {
				continue
			}
		}
		if hasAssetCat {
			// Asset categories form an explicit OR — keep symbols whose
			// asset_category matches at least one of the requested types.
			ok := false
			if wantStock && s.IsTokenizedStock {
				ok = true
			}
			if wantCmd && s.IsCommodity {
				ok = true
			}
			if wantFx && s.IsForex {
				ok = true
			}
			if !ok {
				continue
			}
		} else {
			// Default: when user did not ask for forex/stock/commodity,
			// exclude non-crypto instruments (e.g. BingX's NCFX-* synthetic
			// FX perps that masquerade as USDT crypto pairs).
			if s.AssetCategory != model.AssetCrypto && s.AssetCategory != "" && s.AssetCategory != model.AssetUnknown {
				continue
			}
		}
		if q != "" && strings.ToUpper(s.QuoteAsset) != q && strings.ToUpper(s.SettleAsset) != q {
			continue
		}
		if b != "" && strings.ToUpper(s.BaseAsset) != b {
			continue
		}
		if m != "" && strings.ToLower(s.MarketType) != m {
			continue
		}
		out = append(out, s)
	}
	return out
}

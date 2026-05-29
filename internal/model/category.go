package model

import (
	"fmt"
	"strings"
)

type Category string

const (
	CatSpot           Category = "spot"
	CatSwap           Category = "swap"
	CatUSDT           Category = "usdt"
	CatUSDC           Category = "usdc"
	CatTokenizedStock Category = "tokenized_stock"
	CatCommodity      Category = "commodity"
	CatForex          Category = "forex"
)

var allCategories = map[Category]struct{}{
	CatSpot: {}, CatSwap: {}, CatUSDT: {}, CatUSDC: {},
	CatTokenizedStock: {}, CatCommodity: {}, CatForex: {},
}

func ParseCategories(s string) ([]Category, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("categories must not be empty")
	}
	parts := strings.Split(s, ",")
	out := make([]Category, 0, len(parts))
	seen := map[Category]bool{}
	for _, p := range parts {
		c := Category(strings.ToLower(strings.TrimSpace(p)))
		if _, ok := allCategories[c]; !ok {
			return nil, fmt.Errorf("unknown category %q", p)
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out, nil
}

func HasCategory(cats []Category, target Category) bool {
	for _, c := range cats {
		if c == target {
			return true
		}
	}
	return false
}

// MarketTypesFromCategories returns which market_types should be fetched.
// If neither spot nor swap is named, default to both (so quote-only filters work).
func MarketTypesFromCategories(cats []Category) []string {
	wantSpot := HasCategory(cats, CatSpot)
	wantSwap := HasCategory(cats, CatSwap)
	if !wantSpot && !wantSwap {
		// Asset-only filter like tokenized_stock or usdt without market => fetch both.
		return []string{"spot", "swap"}
	}
	out := []string{}
	if wantSpot {
		out = append(out, "spot")
	}
	if wantSwap {
		out = append(out, "swap")
	}
	return out
}

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/output"
)

type ChangedField struct {
	Field string `json:"field"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

type ChangedSymbol struct {
	Key     string          `json:"key"`
	Symbol  string          `json:"symbol"`
	Changes []ChangedField  `json:"changes"`
}

type DiffResult struct {
	Added   []model.Symbol  `json:"added"`
	Removed []model.Symbol  `json:"removed"`
	Changed []ChangedSymbol `json:"changed"`
}

// LoadEnvelope reads a previously-written JSON output file.
func LoadEnvelope(path string) (*output.Envelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var env output.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &env, nil
}

// Diff compares two symbol sets by Key().
func Diff(oldSyms, newSyms []model.Symbol) DiffResult {
	oldIdx := indexByKey(oldSyms)
	newIdx := indexByKey(newSyms)

	var added, removed []model.Symbol
	var changed []ChangedSymbol

	keys := make([]string, 0, len(newIdx))
	for k := range newIdx {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		n := newIdx[k]
		o, ok := oldIdx[k]
		if !ok {
			added = append(added, n)
			continue
		}
		var diffs []ChangedField
		if o.Status != n.Status {
			diffs = append(diffs, ChangedField{"status", o.Status, n.Status})
		}
		if o.QuoteAsset != n.QuoteAsset {
			diffs = append(diffs, ChangedField{"quote_asset", o.QuoteAsset, n.QuoteAsset})
		}
		if o.ContractType != n.ContractType {
			diffs = append(diffs, ChangedField{"contract_type", o.ContractType, n.ContractType})
		}
		if o.TickSize != n.TickSize {
			diffs = append(diffs, ChangedField{"tick_size", o.TickSize, n.TickSize})
		}
		if o.MinQty != n.MinQty {
			diffs = append(diffs, ChangedField{"min_qty", o.MinQty, n.MinQty})
		}
		if len(diffs) > 0 {
			changed = append(changed, ChangedSymbol{Key: k, Symbol: n.Symbol, Changes: diffs})
		}
	}

	oldKeys := make([]string, 0, len(oldIdx))
	for k := range oldIdx {
		oldKeys = append(oldKeys, k)
	}
	sort.Strings(oldKeys)
	for _, k := range oldKeys {
		if _, ok := newIdx[k]; !ok {
			removed = append(removed, oldIdx[k])
		}
	}
	return DiffResult{Added: added, Removed: removed, Changed: changed}
}

func indexByKey(syms []model.Symbol) map[string]model.Symbol {
	out := make(map[string]model.Symbol, len(syms))
	for _, s := range syms {
		out[s.Key()] = s
	}
	return out
}

package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/etz/tickr/internal/exchange"
	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/notify"
	"github.com/etz/tickr/internal/output"
)

type WatchOptions struct {
	Fetch      FetchOptions
	Interval   time.Duration
	StatePath  string
	Notifier   *notify.Telegram
	UseTV      bool
	Categories []model.Category
}

// DefaultStatePath builds ./state/{exchange}_{hash}.json from the categories.
func DefaultStatePath(exch string, cats []model.Category) string {
	parts := make([]string, len(cats))
	for i, c := range cats {
		parts[i] = string(c)
	}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, ",")))
	hash := hex.EncodeToString(sum[:])[:8]
	return filepath.Join("state", fmt.Sprintf("%s_%s.json", exch, hash))
}

func loadState(path string) (*output.Envelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var env output.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

func saveState(path string, env output.Envelope) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Watch executes one tick: fetches, diffs against state, notifies, persists.
func Watch(ctx context.Context, reg *exchange.Registry, opts WatchOptions, log *slog.Logger) (DiffResult, error) {
	env, err := Fetch(ctx, reg, opts.Fetch, log)
	if err != nil {
		return DiffResult{}, err
	}

	statePath := opts.StatePath
	if statePath == "" {
		statePath = DefaultStatePath(opts.Fetch.Exchange, opts.Categories)
	}

	prev, err := loadState(statePath)
	if err != nil {
		log.Warn("failed to load state, starting fresh", "path", statePath, "err", err)
	}

	var diff DiffResult
	if prev != nil {
		diff = Diff(prev.Symbols, env.Symbols)
	} else {
		// First run: treat everything as a baseline, no "added" alert.
		diff = DiffResult{}
	}

	if err := saveState(statePath, env); err != nil {
		return diff, fmt.Errorf("save state: %w", err)
	}

	if len(diff.Added) > 0 && opts.Notifier != nil {
		msg := buildTelegramMessage(opts.Fetch.Exchange, opts.Categories, diff.Added, opts.UseTV)
		if err := opts.Notifier.Send(ctx, msg); err != nil {
			log.Warn("telegram send failed", "err", err)
		}
	}
	return diff, nil
}

// RunWatch loops at the requested interval until ctx is cancelled.
func RunWatch(ctx context.Context, reg *exchange.Registry, opts WatchOptions, log *slog.Logger) error {
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()
	// Run once immediately.
	if _, err := Watch(ctx, reg, opts, log); err != nil {
		log.Error("watch tick failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			diff, err := Watch(ctx, reg, opts, log)
			if err != nil {
				log.Error("watch tick failed", "err", err)
				continue
			}
			log.Info("watch tick",
				"added", len(diff.Added),
				"removed", len(diff.Removed),
				"changed", len(diff.Changed))
		}
	}
}

func buildTelegramMessage(exch string, cats []model.Category, added []model.Symbol, useTV bool) string {
	parts := make([]string, len(cats))
	for i, c := range cats {
		parts[i] = string(c)
	}
	header := fmt.Sprintf("New symbols on %s %s:\n\n", capitalize(exch), strings.Join(parts, " "))
	body := make([]string, 0, len(added))
	for _, s := range added {
		if useTV {
			body = append(body, s.TradingViewSymbol)
		} else {
			body = append(body, s.Symbol)
		}
	}
	footer := fmt.Sprintf("\n\nCount: %d\nFetched at: %s", len(added), time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	return header + strings.Join(body, "\n") + footer
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

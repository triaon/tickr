package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/etz/tickr/internal/app"
	"github.com/etz/tickr/internal/config"
	"github.com/etz/tickr/internal/httpx"
	"github.com/etz/tickr/internal/model"
	"github.com/etz/tickr/internal/notify"
	"github.com/etz/tickr/internal/output"
)

func main() {
	if len(os.Args) < 2 {
		// No subcommand → drop into the interactive wizard.
		if err := runInteractive(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "fetch":
		err = runFetch(args)
	case "diff":
		err = runDiff(args)
	case "watch":
		err = runWatch(args)
	case "interactive", "i", "wizard":
		err = runInteractive()
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `tickr - fetch normalized trading symbols from CEX APIs

Usage:
  tickr                                    # interactive wizard
  tickr interactive | i                    # same, explicit
  tickr fetch  --exchange <name> --categories <list> [flags]
  tickr diff   --old <file> --new <file> [--out <file>]
  tickr watch  --exchange <name> --categories <list> --interval <dur> [flags]

Run "tickr <command> -h" for command-specific flags.`)
}

func newLogger(level string, debug bool) *slog.Logger {
	lvl := slog.LevelInfo
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	if debug {
		lvl = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

// ---- fetch ----

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	var exch, cats, format, out, tvSep, quote, base, market, cfgPath, logLevel string
	var useTV, activeOnly, includeRaw, debug, reverse bool
	activeOnly = true
	strVar(fs, &exch, "", "exchange: binance|mexc|bingx|bybit (required)", "exchange", "e")
	strVar(fs, &cats, "", "comma list: spot,swap,usdt,usdc,tokenized_stock,commodity,forex (required)", "categories", "c")
	strVar(fs, &format, "", "output format: json|csv|txt (default json)", "format", "f")
	strVar(fs, &out, "", "output file (default stdout)", "out", "o")
	boolVar(fs, &useTV, false, "emit TradingView-style symbols", "tv")
	strVar(fs, &tvSep, "newline", "TXT separator when --tv: newline|comma", "tv-separator")
	boolVar(fs, &activeOnly, true, "only active/trading symbols", "active-only", "a")
	boolVar(fs, &includeRaw, false, "include raw response objects", "include-raw", "r")
	strVar(fs, &quote, "", "filter quoteAsset (e.g. USDT)", "quote", "q")
	strVar(fs, &base, "", "filter baseAsset", "base", "b")
	strVar(fs, &market, "", "explicit market type: spot|swap|linear|inverse|futures", "market", "m")
	strVar(fs, &cfgPath, "config.yaml", "path to config.yaml", "config")
	strVar(fs, &logLevel, "info", "log level: debug|info|warn|error", "log-level", "l")
	boolVar(fs, &debug, false, "shortcut for --log-level debug", "debug", "d")
	boolVar(fs, &reverse, false, "reverse the final symbol list", "reverse")
	_ = fs.Parse(args)

	if exch == "" || cats == "" {
		return errors.New("--exchange/-e and --categories/-c are required")
	}
	categories, err := model.ParseCategories(cats)
	if err != nil {
		return err
	}
	log := newLogger(logLevel, debug)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if format == "" {
		format = cfg.Output.DefaultFormat
	}
	if format == "" {
		format = "json"
	}
	if !includeRaw {
		includeRaw = cfg.Output.IncludeRaw
	}

	http := httpx.New(log)
	reg := app.BuildRegistry(cfg, http, log)
	if _, err := reg.Get(exch); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	env, err := app.Fetch(ctx, reg, app.FetchOptions{
		Exchange:   exch,
		Categories: categories,
		ActiveOnly: activeOnly,
		IncludeRaw: includeRaw,
		Quote:      quote,
		Base:       base,
		Market:     market,
		TVSuffix:   cfg.TradingView.SuffixPerp,
		Reverse:    reverse,
	}, log)
	if err != nil {
		return err
	}

	for _, w := range env.Warnings {
		log.Warn(w.Message, "level", w.Level)
	}

	w, err := output.Writer(out)
	if err != nil {
		return err
	}
	defer w.Close()

	switch strings.ToLower(format) {
	case "json":
		return output.WriteJSON(w, env)
	case "csv":
		return output.WriteCSV(w, env)
	case "txt":
		return output.WriteTXT(w, env, output.TxtOptions{TradingView: useTV, Separator: tvSep})
	default:
		return fmt.Errorf("unknown --format %q", format)
	}
}

// ---- diff ----

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	var oldPath, newPath, out string
	strVar(fs, &oldPath, "", "old JSON file (required)", "old")
	strVar(fs, &newPath, "", "new JSON file (required)", "new", "n")
	strVar(fs, &out, "", "output file (default stdout)", "out", "o")
	_ = fs.Parse(args)

	if oldPath == "" || newPath == "" {
		return errors.New("--old and --new/-n are required")
	}
	oldEnv, err := app.LoadEnvelope(oldPath)
	if err != nil {
		return err
	}
	newEnv, err := app.LoadEnvelope(newPath)
	if err != nil {
		return err
	}
	diff := app.Diff(oldEnv.Symbols, newEnv.Symbols)

	w, err := output.Writer(out)
	if err != nil {
		return err
	}
	defer w.Close()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(diff)
}

// ---- watch ----

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	var exch, cats, intervalStr, statePath, notifyMode, tgToken, tgChat, cfgPath, logLevel, quote string
	var useTV, activeOnly, debug bool
	activeOnly = true
	strVar(fs, &exch, "", "exchange (required)", "exchange", "e")
	strVar(fs, &cats, "", "categories (required)", "categories", "c")
	strVar(fs, &intervalStr, "10m", "poll interval, e.g. 30s, 5m, 1h", "interval", "i")
	strVar(fs, &statePath, "", "state file path (default state/<exchange>_<hash>.json)", "state", "s")
	strVar(fs, &quote, "", "filter quoteAsset (e.g. USDT)", "quote", "q")
	strVar(fs, &notifyMode, "", "notify channel: telegram or empty", "notify")
	strVar(fs, &tgToken, "", "override telegram bot token", "telegram-token")
	strVar(fs, &tgChat, "", "override telegram chat id", "telegram-chat-id")
	boolVar(fs, &useTV, false, "emit TradingView-style symbols in notifications", "tv")
	boolVar(fs, &activeOnly, true, "only active symbols", "active-only", "a")
	strVar(fs, &cfgPath, "config.yaml", "path to config.yaml", "config")
	strVar(fs, &logLevel, "info", "log level", "log-level", "l")
	boolVar(fs, &debug, false, "log-level=debug", "debug", "d")
	_ = fs.Parse(args)

	if exch == "" || cats == "" {
		return errors.New("--exchange/-e and --categories/-c are required")
	}
	categories, err := model.ParseCategories(cats)
	if err != nil {
		return err
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return fmt.Errorf("invalid --interval: %w", err)
	}
	log := newLogger(logLevel, debug)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	http := httpx.New(log)
	reg := app.BuildRegistry(cfg, http, log)
	if _, err := reg.Get(exch); err != nil {
		return err
	}

	var notifier *notify.Telegram
	if strings.EqualFold(notifyMode, "telegram") {
		token := tgToken
		chat := tgChat
		if token == "" {
			token = cfg.Telegram.BotToken
		}
		if chat == "" {
			chat = cfg.Telegram.ChatID
		}
		if token == "" || chat == "" {
			return errors.New("telegram notify requires bot token and chat id (flags or config)")
		}
		notifier = notify.NewTelegram(token, chat)
	}

	ctx, cancel := signalContext()
	defer cancel()

	opts := app.WatchOptions{
		Fetch: app.FetchOptions{
			Exchange:   exch,
			Categories: categories,
			ActiveOnly: activeOnly,
			Quote:      quote,
			TVSuffix:   cfg.TradingView.SuffixPerp,
		},
		Interval:   interval,
		StatePath:  statePath,
		Notifier:   notifier,
		UseTV:      useTV,
		Categories: categories,
	}
	err = app.RunWatch(ctx, reg, opts, log)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// strVar registers a string flag under one or more names that share the same destination.
// First name is the canonical long form; additional names are aliases (e.g. short -e).
func strVar(fs *flag.FlagSet, p *string, def, usage string, names ...string) {
	*p = def
	for i, n := range names {
		help := usage
		if i > 0 {
			help = "alias for --" + names[0]
		}
		fs.StringVar(p, n, def, help)
	}
}

// boolVar registers a bool flag under one or more names that share the same destination.
func boolVar(fs *flag.FlagSet, p *bool, def bool, usage string, names ...string) {
	*p = def
	for i, n := range names {
		help := usage
		if i > 0 {
			help = "alias for --" + names[0]
		}
		fs.BoolVar(p, n, def, help)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	return ctx, cancel
}

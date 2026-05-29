package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/etz/tickr/internal/notify"
)

// prompter wraps a buffered reader with a small set of helpers that show a
// short question with an optional default value in brackets.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

func newPrompter() *prompter {
	return &prompter{in: bufio.NewReader(os.Stdin), out: os.Stderr}
}

func (p *prompter) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if err == io.EOF && line == "" {
		return "", io.EOF
	}
	return line, nil
}

func (p *prompter) ask(question, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", question, def)
	} else {
		fmt.Fprintf(p.out, "%s: ", question)
	}
	line, err := p.readLine()
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func (p *prompter) askRequired(question string) (string, error) {
	for {
		v, err := p.ask(question, "")
		if err != nil {
			return "", err
		}
		if v != "" {
			return v, nil
		}
		fmt.Fprintln(p.out, "  required, please enter a value")
	}
}

func (p *prompter) askBool(question string, def bool) (bool, error) {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	v, err := p.ask(question+" ["+hint+"]", "")
	if err != nil {
		return def, err
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return def, nil
	case "y", "yes", "1", "true":
		return true, nil
	case "n", "no", "0", "false":
		return false, nil
	default:
		fmt.Fprintln(p.out, "  please answer y or n")
		return p.askBool(question, def)
	}
}

func (p *prompter) askChoice(title string, options []string, def int) (int, string, error) {
	fmt.Fprintln(p.out, title)
	for i, o := range options {
		fmt.Fprintf(p.out, "  %d) %s\n", i+1, o)
	}
	for {
		v, err := p.ask("Choice", strconv.Itoa(def+1))
		if err != nil {
			return 0, "", err
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 || n > len(options) {
			fmt.Fprintf(p.out, "  enter a number between 1 and %d\n", len(options))
			continue
		}
		return n - 1, options[n-1], nil
	}
}

// toggleOption is one row in askToggle.
//   - Label: human text shown next to the number
//   - Exclusive: when toggled on, clears every other selected option and is
//     itself cleared by toggling any non-exclusive option. Use for "any" /
//     "custom…" rows that don't make sense alongside others.
//   - Custom: when toggled on, immediately prompts for a free-form string;
//     the string is returned alongside the option label so the caller can
//     route it (e.g. quote=BTC → --quote BTC instead of --categories).
type toggleOption struct {
	Label     string
	Exclusive bool
	Custom    bool
}

// askToggle renders a numbered list with [x]/[ ] markers and lets the user
// flip exactly one item per input. Empty Enter confirms. This avoids the
// "1,3" / "13" parsing ambiguity entirely — every input is a single number.
//
// Returns the selected labels in option order, plus an optional custom-value
// map keyed by label (only populated for options marked Custom).
func (p *prompter) askToggle(title string, options []toggleOption, defaultSelected []int) ([]string, map[string]string, error) {
	selected := make([]bool, len(options))
	for _, i := range defaultSelected {
		if i >= 0 && i < len(options) {
			selected[i] = true
		}
	}
	customVals := map[string]string{}
	render := func() {
		fmt.Fprintln(p.out, title)
		for i, o := range options {
			mark := " "
			if selected[i] {
				mark = "x"
			}
			extra := ""
			if v, ok := customVals[o.Label]; ok && v != "" {
				extra = " = " + v
			}
			fmt.Fprintf(p.out, "  [%s] %d) %s%s\n", mark, i+1, o.Label, extra)
		}
	}
	for {
		render()
		v, err := p.ask("Toggle item number (Enter = confirm)", "")
		if err != nil {
			return nil, nil, err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			// confirm
			out := make([]string, 0, len(options))
			for i, o := range options {
				if selected[i] {
					out = append(out, o.Label)
				}
			}
			if len(out) == 0 {
				fmt.Fprintln(p.out, "  pick at least one item")
				continue
			}
			return out, customVals, nil
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > len(options) {
			fmt.Fprintf(p.out, "  enter a number between 1 and %d, or empty Enter to confirm\n", len(options))
			continue
		}
		idx := n - 1
		opt := options[idx]
		if selected[idx] {
			// turning off
			selected[idx] = false
			delete(customVals, opt.Label)
			continue
		}
		// turning on
		if opt.Exclusive {
			for i := range selected {
				selected[i] = false
				delete(customVals, options[i].Label)
			}
			selected[idx] = true
		} else {
			// turning on a normal option also clears any exclusive ones
			for i, o := range options {
				if o.Exclusive {
					selected[i] = false
					delete(customVals, o.Label)
				}
			}
			selected[idx] = true
		}
		if opt.Custom {
			cv, err := p.ask("  custom value (empty cancels)", "")
			if err != nil {
				return nil, nil, err
			}
			cv = strings.TrimSpace(cv)
			if cv == "" {
				selected[idx] = false
			} else {
				customVals[opt.Label] = cv
			}
		}
	}
}

// suggestFilename builds a default like "bingx_spot_usdt.json".
func suggestFilename(exchange string, cats []string, format string) string {
	parts := append([]string{exchange}, cats...)
	return strings.Join(parts, "_") + "." + format
}

// ensureExt appends ".<format>" if the path has no extension already.
func ensureExt(path, format string) string {
	// Don't touch dotfiles (".env") or paths that already have an extension.
	base := path
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		base = path[i+1:]
	}
	if strings.Contains(base, ".") {
		return path
	}
	return path + "." + format
}

// pickWithCustom shows a numbered list. Convention:
//   - first option "no filter" returns "".
//   - last option "custom…" prompts for a free-form value.
//   - any other option is returned as-is.
func pickWithCustom(p *prompter, title string, options []string) (string, error) {
	_, sel, err := p.askChoice(title, options, 0)
	if err != nil {
		return "", err
	}
	switch sel {
	case "no filter":
		return "", nil
	case "custom…":
		v, err := p.ask("  custom value (empty = skip)", "")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(v), nil
	default:
		return sel, nil
	}
}

// runInteractive walks the user through a command (fetch/diff/watch) and then
// dispatches to the same handlers used by the non-interactive entry points.
func runInteractive() error {
	p := newPrompter()
	fmt.Fprintln(p.out, "=== tickr: interactive mode (Ctrl-C to quit) ===")

	commands := []string{"fetch — get current symbols", "diff — compare two saved files", "watch — poll and notify"}
	idx, _, err := p.askChoice("\nWhat do you want to do?", commands, 0)
	if err != nil {
		return err
	}
	switch idx {
	case 0:
		return interactiveFetch(p)
	case 1:
		return interactiveDiff(p)
	case 2:
		return interactiveWatch(p)
	}
	return errors.New("nothing selected")
}

func interactiveFetch(p *prompter) error {
	exchanges := []string{"binance", "mexc", "bingx", "bybit"}
	_, exch, err := p.askChoice("\nExchange:", exchanges, 2)
	if err != nil {
		return err
	}

	cats, quoteOverride, labelParts, err := pickSelection(p)
	if err != nil {
		return err
	}

	formats := []string{"txt (tradingview)", "json", "csv"}
	fmtIdx, fmtChoice, err := p.askChoice("\nOutput format:", formats, 0)
	if err != nil {
		return err
	}
	fmtSel := "txt"
	switch fmtChoice {
	case "json":
		fmtSel = "json"
	case "csv":
		fmtSel = "csv"
	}

	useTV := false
	tvSep := "newline"
	if fmtIdx == 0 { // txt
		useTV, err = p.askBool("Use TradingView format (EX:SYMBOL.P)?", true)
		if err != nil {
			return err
		}
		if useTV {
			sepIdx, _, err := p.askChoice("Separator:", []string{"newline", "comma"}, 0)
			if err != nil {
				return err
			}
			if sepIdx == 1 {
				tvSep = "comma"
			}
		}
	}

	suggested := suggestFilename(exch, labelParts, fmtSel)
	out, err := p.ask("\nOutput file (press Enter for suggestion, '-' for stdout)", suggested)
	if err != nil {
		return err
	}
	if out == "-" {
		out = ""
	} else if out != "" {
		out = ensureExt(out, fmtSel)
	}

	activeOnly, err := p.askBool("\nOnly active/trading symbols?", true)
	if err != nil {
		return err
	}

	base := ""
	addBase, err := p.askBool("\nAdd a base-asset filter (e.g. only BTC-based pairs)?", false)
	if err != nil {
		return err
	}
	if addBase {
		baseOpts := []string{"no filter", "BTC", "ETH", "SOL", "custom…"}
		base, err = pickWithCustom(p, "Base filter:", baseOpts)
		if err != nil {
			return err
		}
	}

	includeRaw, err := p.askBool("Include raw exchange objects in JSON?", false)
	if err != nil {
		return err
	}

	reverse, err := p.askBool("Reverse the final symbol list?", false)
	if err != nil {
		return err
	}

	args := []string{
		"--exchange", exch,
		"--categories", strings.Join(cats, ","),
		"--format", fmtSel,
		"--active-only=" + strconv.FormatBool(activeOnly),
	}
	if useTV {
		args = append(args, "--tv", "--tv-separator", tvSep)
	}
	if out != "" {
		args = append(args, "--out", out)
	}
	if quoteOverride != "" {
		args = append(args, "--quote", quoteOverride)
	}
	if base != "" {
		args = append(args, "--base", base)
	}
	if includeRaw {
		args = append(args, "--include-raw")
	}
	if reverse {
		args = append(args, "--reverse")
	}

	fmt.Fprintf(p.out, "\n→ tickr fetch %s\n\n", strings.Join(args, " "))
	if err := runFetch(args); err != nil {
		return err
	}
	fmt.Fprintln(p.out, "\ndone")
	return nil
}

// pickSelection walks the user through three independent axes:
//   1. Market type     — single-select: spot | swap | both
//   2. Quote currency  — toggle multi-select: USDT, USDC, any, custom…
//   3. Asset class     — toggle multi-select: crypto, tokenized_stock, commodity, forex
//
// Returns:
//   - cats: the legacy []model.Category-style tokens to feed --categories
//   - quoteOverride: free-form quote (when user picked "custom…") for --quote
//   - labelParts: tokens used to suggest a filename (e.g. ["swap","usdt"])
func pickSelection(p *prompter) (cats []string, quoteOverride string, labelParts []string, err error) {
	// Axis 1 — market type
	_, market, err := p.askChoice("\nMarket type:", []string{"spot", "swap (perpetual)", "both"}, 1)
	if err != nil {
		return nil, "", nil, err
	}
	switch market {
	case "spot":
		cats = append(cats, "spot")
		labelParts = append(labelParts, "spot")
	case "swap (perpetual)":
		cats = append(cats, "swap")
		labelParts = append(labelParts, "swap")
	case "both":
		cats = append(cats, "spot", "swap")
		labelParts = append(labelParts, "spot", "swap")
	}

	// Axis 2 — quote / settle
	quoteOpts := []toggleOption{
		{Label: "USDT"},
		{Label: "USDC"},
		{Label: "any", Exclusive: true},
		{Label: "custom…", Exclusive: true, Custom: true},
	}
	qSel, qCustom, err := p.askToggle("\nQuote / settle currency:", quoteOpts, []int{0})
	if err != nil {
		return nil, "", nil, err
	}
	for _, l := range qSel {
		switch l {
		case "USDT":
			cats = append(cats, "usdt")
			labelParts = append(labelParts, "usdt")
		case "USDC":
			cats = append(cats, "usdc")
			labelParts = append(labelParts, "usdc")
		case "any":
			// no filter
		case "custom…":
			quoteOverride = qCustom["custom…"]
			if quoteOverride != "" {
				labelParts = append(labelParts, strings.ToLower(quoteOverride))
			}
		}
	}

	// Axis 3 — asset class. `crypto` is marked Exclusive so toggling a
	// synthetic class auto-clears crypto (and vice-versa). The backend
	// can't express "crypto + synthetic" today — Apply() either filters to
	// non-crypto or defaults to crypto-only — so the toggle model enforces
	// that invariant for the user.
	assetOpts := []toggleOption{
		{Label: "crypto", Exclusive: true},
		{Label: "tokenized_stock"},
		{Label: "commodity"},
		{Label: "forex"},
	}
	aSel, _, err := p.askToggle("\nAsset class:", assetOpts, []int{0})
	if err != nil {
		return nil, "", nil, err
	}
	for _, l := range aSel {
		if l == "crypto" {
			continue // default behaviour, no category needed
		}
		cats = append(cats, l)
		labelParts = append(labelParts, l)
	}
	return cats, quoteOverride, labelParts, nil
}

func interactiveDiff(p *prompter) error {
	oldPath, err := p.askRequired("\nOld JSON file")
	if err != nil {
		return err
	}
	newPath, err := p.askRequired("New JSON file")
	if err != nil {
		return err
	}
	out, err := p.ask("Output file (empty = stdout)", "")
	if err != nil {
		return err
	}
	args := []string{"--old", oldPath, "--new", newPath}
	if out != "" {
		args = append(args, "--out", out)
	}
	fmt.Fprintf(p.out, "\n→ tickr diff %s\n\n", strings.Join(args, " "))
	return runDiff(args)
}

func interactiveWatch(p *prompter) error {
	exchanges := []string{"binance", "mexc", "bingx", "bybit"}
	_, exch, err := p.askChoice("\nExchange:", exchanges, 2)
	if err != nil {
		return err
	}
	cats, quoteOverride, _, err := pickSelection(p)
	if err != nil {
		return err
	}
	interval, err := p.ask("Poll interval (e.g. 30s, 5m, 1h)", "10m")
	if err != nil {
		return err
	}
	state, err := p.ask("State file path (empty = default state/<exch>_<hash>.json)", "")
	if err != nil {
		return err
	}
	wantTG, err := p.askBool("Send Telegram notifications on new listings?", false)
	if err != nil {
		return err
	}
	useTV, err := p.askBool("Emit TradingView symbols in notifications?", false)
	if err != nil {
		return err
	}

	args := []string{
		"--exchange", exch,
		"--categories", strings.Join(cats, ","),
		"--interval", interval,
	}
	if state != "" {
		args = append(args, "--state", state)
	}
	if quoteOverride != "" {
		args = append(args, "--quote", quoteOverride)
	}
	if useTV {
		args = append(args, "--tv")
	}
	if wantTG {
		args = append(args, "--notify", "telegram")
		token, err := p.ask("Telegram bot token (empty = use config.yaml)", "")
		if err != nil {
			return err
		}
		if token != "" {
			args = append(args, "--telegram-token", token)
		}
		chat, err := discoverOrAskChatID(p, token)
		if err != nil {
			return err
		}
		if chat != "" {
			args = append(args, "--telegram-chat-id", chat)
		}
	}

	fmt.Fprintf(p.out, "\n→ tickr watch %s\n", strings.Join(args, " "))
	fmt.Fprintln(p.out, "  (Ctrl-C to stop)")
	fmt.Fprintln(p.out)
	return runWatch(args)
}

// discoverOrAskChatID lets the user pick a chat id. If a token was supplied we
// offer to auto-discover it via getUpdates: the user opens t.me/<botname>,
// sends /start, presses Enter, and we read the chat id from the latest update.
//
// Returns "" when the user wants to fall back to config.yaml.
func discoverOrAskChatID(p *prompter, token string) (string, error) {
	if token == "" {
		// No token in hand → can't auto-discover. Just ask.
		return p.ask("Telegram chat id (empty = use config.yaml)", "")
	}
	auto, err := p.askBool("Auto-discover chat id via /start?", true)
	if err != nil {
		return "", err
	}
	if !auto {
		return p.ask("Telegram chat id (empty = use config.yaml)", "")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	username, err := notify.BotInfo(ctx, token)
	cancel()
	if err != nil {
		fmt.Fprintf(p.out, "  could not query bot info: %v\n", err)
		return p.ask("Telegram chat id (empty = use config.yaml)", "")
	}
	fmt.Fprintf(p.out, "\n  1. Open https://t.me/%s\n", username)
	fmt.Fprintln(p.out, "  2. Send /start (or any message) to the bot")
	fmt.Fprintln(p.out, "  3. Come back here and press Enter")
	for attempt := 1; attempt <= 5; attempt++ {
		v, err := p.ask("  Enter = poll bot, or type chat id manually", "")
		if err != nil {
			return "", err
		}
		if v != "" {
			return v, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		chatID, err := notify.DiscoverChatID(ctx, token)
		cancel()
		if err != nil {
			fmt.Fprintf(p.out, "  getUpdates failed: %v\n", err)
			continue
		}
		if chatID != "" {
			fmt.Fprintf(p.out, "  ✓ found chat id: %s\n", chatID)
			return chatID, nil
		}
		fmt.Fprintf(p.out, "  no messages yet (attempt %d/5) — send /start to the bot and try again\n", attempt)
	}
	fmt.Fprintln(p.out, "  giving up auto-discovery")
	return p.ask("Telegram chat id (empty = use config.yaml)", "")
}

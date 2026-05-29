# tickr

CLI **and desktop UI** to fetch normalized trading instruments from Binance, MEXC, BingX, and Bybit. Designed for building TradingView watchlists, monitoring new listings, comparing exchange coverage, and filtering USDT/USDC spot/swap pairs.

Three ways to use it:

- **Interactive wizard** — `./tickr` with no args walks you through every option.
- **Direct CLI** — `tickr fetch ...` / `diff` / `watch` for scripts and automation.
- **Desktop app** — Electron + React UI under `ui/` with dark/light theme, see [UI](#desktop-ui).

## Build

```
go build ./cmd/tickr
```

Pre-built binaries for macOS (Apple Silicon + Intel), Linux (x64 + ARM), and Windows (x64 + ARM) are produced by `./scripts/build-all.sh` into `dist/`.

## Commands

### fetch

```
tickr fetch --exchange <name> --categories <list> [flags]
```

Required:
- `--exchange` — `binance | mexc | bingx | bybit`
- `--categories` — comma list of `spot, swap, usdt, usdc, tokenized_stock, commodity, forex`

Optional:
- `--format` — `json | csv | txt` (default `json`)
- `--out` — output file (default stdout)
- `--tv` — emit TradingView-style symbols (used by TXT)
- `--tv-separator` — `newline` (default) or `comma`
- `--active-only` — only tradable symbols (default true)
- `--reverse` — reverse the final symbol list (newest listings first)
- `--include-raw` — embed raw exchange response in each symbol
- `--quote`, `--base` — extra asset filters
- `--market` — explicit market type
- `--config` — path to `config.yaml`
- `--log-level`, `--debug`

Examples:

```
tickr fetch --exchange bingx --categories spot,usdt --format json --out bingx_spot_usdt.json
tickr fetch --exchange bybit --categories swap,usdt --format txt --tv --out bybit_usdt_perps_tv.txt
tickr fetch --exchange binance --categories spot,usdt --format csv --out binance_spot_usdt.csv
tickr fetch --exchange mexc --categories spot,swap,usdt --format json --out mexc_all_usdt.json
```

### diff

```
tickr diff --old old.json --new new.json [--out diff.json]
```

Compares two saved fetch envelopes by `exchange|market_type|symbol` and reports added/removed/changed. Watched fields: `status`, `quote_asset`, `contract_type`, `tick_size`, `min_qty`.

### watch

```
tickr watch --exchange <name> --categories <list> --interval 10m \
  [--state ./state/...] [--notify telegram --telegram-token X --telegram-chat-id Y] [--tv]
```

Polls on the given interval, persists the last snapshot to `./state/{exchange}_{categories_hash}.json` by default, and prints diffs. When Telegram is configured and new symbols appear, sends a message.

The interactive wizard can also **auto-discover your Telegram chat id**: enter the bot token, open `t.me/<botname>`, send `/start`, and the wizard reads the chat id from `getUpdates` for you.

## Desktop UI

Minimalist Electron + React + TypeScript app under `ui/`. Dark theme by default (toggle in the sidebar), warm monochrome palette, JetBrains Mono for symbol cells, Space Grotesk for headings.

```
cd ui
npm install
npm run dev           # launch dev (Vite + Electron with hot reload)
npm run package:mac   # produce .dmg + .zip
npm run package:win   # produce .exe + .zip
npm run package:linux # produce .AppImage + .deb
```

The desktop app shells out to the Go binary in `dist/` (dev) or from `resources/bin/tickr` (packaged). Build the binary first via `./scripts/build-all.sh`.

## Configuration

Copy `config.example.yaml` to `config.yaml` (the latter is gitignored) and tweak as needed. The defaults shipped in code take over when no `config.yaml` is present:

```yaml
exchanges:
  binance:
    enabled: true
    spot_base_url: "https://api.binance.com"
    futures_base_url: "https://fapi.binance.com"
  bybit:
    enabled: true
    base_url: "https://api.bybit.com"
  mexc:
    enabled: true
    spot_base_url: "https://api.mexc.com"
    futures_base_url: "https://contract.mexc.com"
  bingx:
    enabled: true
    base_url: "https://open-api.bingx.com"

output:
  default_format: "json"
  include_raw: false

tradingview:
  suffix_perp: ".P"

telegram:
  enabled: false
  bot_token: ""
  chat_id: ""
```

## Architecture

```
cmd/tickr/main.go      # CLI entry, subcommands, flags
internal/app/                   # fetch/diff/watch + filter pipeline
internal/exchange/              # adapters (binance/bybit/mexc/bingx)
internal/model/                 # Symbol, Category, FetchRequest, Warning
internal/output/                # JSON / CSV / TXT writers
internal/tv/                    # TradingView symbol formatter
internal/notify/                # Telegram client
internal/httpx/                 # retrying HTTP client
internal/config/                # YAML config loader
```

Adding an exchange: implement `exchange.Adapter`, register it in `app.BuildRegistry`.

## Notes & caveats

- Public market endpoints only — no API keys required, none ever logged.
- HTTP: 10s timeout, 3 retries, backoff 500ms/1s/2s.
- `tokenized_stock`, `commodity`, `forex` are emitted only if the exchange's response carries a clear type signal (e.g. Bybit `symbolType`). Otherwise the fetch returns an empty list with a `warn` entry — it never crashes.
- Bybit and Binance are geo-blocked from some regions; that surfaces as an HTTP 4xx error from the adapter.
- BingX swap `--active-only` now requires `apiStateOpen == "true"` on top of `status == 1` — this filters out pre-launch and halted contracts that BingX still returns as "registered".
- For BingX tokenized stocks / commodities / forex, the human-readable `displayName` field is used to derive the canonical symbol (e.g. `NCSKNVDA2USD-USDT` → `NVDAUSDT` / `BINGX:NVDAUSDT.P`).

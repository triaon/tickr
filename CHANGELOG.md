# Changelog

## v0.1.2 — 2026-06-01

### Changed

- **Desktop UI rewritten on Tauri 2 (Rust).** Replaces the Electron 33
  bundle. The macOS DMG drops from ~103 MB to ~7.4 MB. Same React
  frontend, same features, same look — just a Rust shell calling the
  bundled Go CLI as a sidecar.

### Notes

- v0.1.2 ships the macOS arm64 DMG only. Windows / Linux Tauri builds
  need the CI workflow updated; tracked for the next release.
- Plain CLI binaries are unchanged from v0.1.1.

## v0.1.1 — 2026-05-31

### Added

- **Cross-exchange intersection** via `--intersect-with <exchange>` on
  `fetch`. Filters the result down to symbols whose base asset is also
  listed (same categories) on a second exchange. Example: Binance perp
  USDT tickers in TradingView format, but only for assets that also
  trade on BingX. Mirrored in the wizard and as an "Intersect with"
  segmented control in the desktop UI.

## v0.1.0 — 2026-05-29

First tagged release. The CLI is feature-complete for spot/swap fetches
across Binance, MEXC, BingX, and Bybit, plus a new desktop UI.

### Added

- **Interactive wizard** (`tickr` with no args) with three independent
  axes — market type, quote currency, asset class — so every selection is one
  number per prompt and ambiguity is gone.
- **Desktop UI** under `ui/` — Electron + React + TypeScript, dark theme by
  default with a sun/moon toggle, warm monochrome palette, JetBrains Mono for
  symbol cells, Space Grotesk for headings. Shells out to the Go binary.
- **`--reverse` flag** on `fetch` to flip the final symbol list (newest first).
  Also a switch in the wizard and the desktop UI.
- **Telegram chat-id auto-discovery** in the `watch` wizard: enter the bot
  token, send `/start` to the bot, the wizard reads the chat id from
  `getUpdates` and wires it into `--telegram-chat-id`.
- **Cross-platform build script** `scripts/build-all.sh` produces six binaries
  in `dist/` with plain-English names: `mac-m`, `mac-intel`, `linux`,
  `linux-arm`, `windows.exe`, `windows-arm.exe`.

### Changed

- Wizard's default output format is now `txt (tradingview)` instead of `json`.
- Wizard's `Use TradingView format` prompt now defaults to `Y`.
- Wizard prints `done` on successful completion of a fetch.

### Fixed

- **BingX swap active filter** now requires `apiStateOpen == "true"` in
  addition to `status == 1`. Previously the fetcher returned ~46 pre-launch
  or halted contracts (QAIT, CITREA, DOPHIN, etc.) marked as "active".
- **BingX tokenized-stock naming** — instead of emitting
  `NCSKNVDA2USDUSDT.P` (the internal encoded contract code), the canonical
  symbol is now derived from BingX's `displayName` for synthetic prefixes
  (NCSK/NCSI/NCCO/NCFX), yielding `NVDAUSDT` / `BINGX:NVDAUSDT.P`.
- Watch mode accepts a `--quote` flag, so a custom quote chosen in the
  interactive wizard actually reaches the fetch pipeline.

### Notes

Pre-built binaries for macOS (ARM + Intel), Linux (x64 + ARM), and Windows
(x64 + ARM) are attached to the GitHub release. The desktop app needs to be
built locally — `cd ui && npm install && npm run package:<platform>`.

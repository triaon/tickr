import { useEffect, useMemo, useState } from "react";
import type { FetchArgs } from "./types";

type Theme = "dark" | "light";
const THEME_KEY = "tickr.theme";

type Market = "spot" | "swap" | "both";

const EXCHANGES = ["binance", "mexc", "bingx", "bybit"] as const;
type Exchange = (typeof EXCHANGES)[number];

const QUOTES = ["USDT", "USDC", "any", "custom"] as const;
type Quote = (typeof QUOTES)[number];

const ASSET_CLASSES = ["crypto", "tokenized_stock", "commodity", "forex"] as const;
type AssetClass = (typeof ASSET_CLASSES)[number];

export function App() {
  const [exchange, setExchange] = useState<Exchange>("bingx");
  const [market, setMarket] = useState<Market>("swap");
  const [quote, setQuote] = useState<Quote>("USDT");
  const [customQuote, setCustomQuote] = useState("");
  const [assetClass, setAssetClass] = useState<AssetClass>("crypto");
  const [activeOnly, setActiveOnly] = useState(true);
  const [tradingView, setTradingView] = useState(true);
  const [reverse, setReverse] = useState(false);
  const [intersectWith, setIntersectWith] = useState<Exchange | "">("");

  const [loading, setLoading] = useState(false);
  const [symbols, setSymbols] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [lastQuery, setLastQuery] = useState<string | null>(null);

  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem(THEME_KEY);
    return saved === "light" ? "light" : "dark";
  });
  useEffect(() => {
    if (intersectWith === exchange) setIntersectWith("");
  }, [exchange, intersectWith]);

  useEffect(() => {
    document.documentElement.classList.toggle("theme-light", theme === "light");
    document.documentElement.classList.toggle("theme-dark", theme === "dark");
    localStorage.setItem(THEME_KEY, theme);
  }, [theme]);

  const tags = useMemo(() => {
    const t: { label: string; color: "blue" | "green" | "yellow" | "red" }[] = [];
    t.push({ label: exchange, color: "blue" });
    t.push({ label: market, color: "green" });
    t.push({ label: quote === "custom" ? (customQuote || "custom") : quote, color: "yellow" });
    if (assetClass !== "crypto") t.push({ label: assetClass, color: "red" });
    return t;
  }, [exchange, market, quote, customQuote, assetClass]);

  function buildArgs(): FetchArgs {
    const categories: string[] = [];
    if (market === "spot") categories.push("spot");
    else if (market === "swap") categories.push("swap");
    else categories.push("spot", "swap");

    let quoteOverride: string | undefined;
    if (quote === "USDT") categories.push("usdt");
    else if (quote === "USDC") categories.push("usdc");
    else if (quote === "custom" && customQuote.trim()) quoteOverride = customQuote.trim();
    // "any" → no quote category

    if (assetClass !== "crypto") categories.push(assetClass);

    return {
      exchange,
      categories,
      quote: quoteOverride,
      activeOnly,
      tradingView,
      reverse,
      intersectWith: intersectWith || undefined,
    };
  }

  async function onFetch() {
    setLoading(true);
    setError(null);
    const args = buildArgs();
    try {
      const res = await window.api.fetchSymbols(args);
      if (!res.ok) {
        setError(res.error || "unknown error");
        setSymbols([]);
      } else {
        setSymbols(res.symbols);
        setLastQuery(`${exchange} · ${market} · ${args.categories.join(", ") || "—"}`);
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }

  async function onCopy() {
    if (!symbols.length) return;
    await navigator.clipboard.writeText(symbols.join("\n"));
  }

  async function onSave() {
    if (!symbols.length) return;
    const name = `${exchange}_${market}_${quote.toLowerCase()}.txt`;
    await window.api.saveText(name, symbols.join("\n") + "\n");
  }

  function onReverseLocal() {
    setSymbols((prev) => [...prev].reverse());
  }

  return (
    <div className="app">
      <div className="titlebar-drag" />
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-text">
            <BotLogo />
            <h1>tickr</h1>
          </div>
          <button
            className="theme-btn"
            title={theme === "dark" ? "Switch to light" : "Switch to dark"}
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          >
            {theme === "dark" ? (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="4" />
                <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
              </svg>
            ) : (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </svg>
            )}
          </button>
        </div>
        <div className="form">
          <div className="field">
            <span className="label">Exchange</span>
            <div className="segmented" style={{ flexWrap: "wrap" }}>
              {EXCHANGES.map((e) => (
                <button
                  key={e}
                  className={exchange === e ? "active" : ""}
                  onClick={() => setExchange(e)}
                >
                  {e}
                </button>
              ))}
            </div>
          </div>

          <div className="field">
            <span className="label">Market</span>
            <div className="segmented">
              {(["spot", "swap", "both"] as Market[]).map((m) => (
                <button key={m} className={market === m ? "active" : ""} onClick={() => setMarket(m)}>
                  {m}
                </button>
              ))}
            </div>
          </div>

          <div className="field">
            <span className="label">Quote</span>
            <div className="segmented">
              {QUOTES.map((q) => (
                <button key={q} className={quote === q ? "active" : ""} onClick={() => setQuote(q)}>
                  {q}
                </button>
              ))}
            </div>
            {quote === "custom" && (
              <input
                className="text-input"
                style={{ marginTop: 8 }}
                placeholder="e.g. BTC"
                value={customQuote}
                onChange={(e) => setCustomQuote(e.target.value)}
              />
            )}
          </div>

          <div className="field">
            <span className="label">Asset class</span>
            <div className="toggles">
              {ASSET_CLASSES.map((a) => (
                <button
                  key={a}
                  className={`toggle ${assetClass === a ? "on" : ""}`}
                  onClick={() => setAssetClass(a)}
                >
                  <span className="box" />
                  <span>{a.replace("_", " ")}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="field">
            <span className="label">Intersect with</span>
            <div className="segmented" style={{ flexWrap: "wrap" }}>
              <button
                className={intersectWith === "" ? "active" : ""}
                onClick={() => setIntersectWith("")}
              >
                none
              </button>
              {EXCHANGES.filter((e) => e !== exchange).map((e) => (
                <button
                  key={e}
                  className={intersectWith === e ? "active" : ""}
                  onClick={() => setIntersectWith(e)}
                >
                  {e}
                </button>
              ))}
            </div>
          </div>

          <div className="field flag-row">
            <span className="label">Options</span>
            <div className="flag">
              <span>Active only</span>
              <button className={`switch ${activeOnly ? "on" : ""}`} onClick={() => setActiveOnly(!activeOnly)} />
            </div>
            <div className="flag">
              <span>TradingView format</span>
              <button className={`switch ${tradingView ? "on" : ""}`} onClick={() => setTradingView(!tradingView)} />
            </div>
            <div className="flag">
              <span>Reverse list</span>
              <button className={`switch ${reverse ? "on" : ""}`} onClick={() => setReverse(!reverse)} />
            </div>
          </div>

          <button className="fetch-btn" disabled={loading} onClick={onFetch}>
            {loading ? (
              <>
                <span className="spinner" />
                <span>Fetching</span>
              </>
            ) : (
              <span>Fetch symbols</span>
            )}
          </button>
        </div>
      </aside>

      <main className="main">
        <header className="main-header">
          <div className="summary">
            <span className="count">{symbols.length}</span>
            <span className="summary-text">{lastQuery ?? "no query yet"}</span>
          </div>
          <div className="actions">
            <button className="action" disabled={!symbols.length} onClick={onReverseLocal}>Reverse</button>
            <button className="action" disabled={!symbols.length} onClick={onCopy}>Copy</button>
            <button className="action" disabled={!symbols.length} onClick={onSave}>Save…</button>
          </div>
        </header>

        <section className="results">
          {error && <div className="error">{error}</div>}

          {!error && symbols.length === 0 && !loading && (
            <div className="empty">
              <div className="big">Ready when you are.</div>
              <div className="hint">configure the run on the left, press <span className="kbd">Fetch symbols</span></div>
            </div>
          )}

          {symbols.length > 0 && (
            <div className="fade-in">
              <div className="tags">
                {tags.map((t, i) => (
                  <span key={i} className={`tag ${t.color}`}>{t.label}</span>
                ))}
                {tradingView && <span className="tag green">tv</span>}
                {reverse && <span className="tag yellow">reversed</span>}
                {intersectWith && <span className="tag red">∩ {intersectWith}</span>}
              </div>
              <div className="symbol-list">
                {symbols.map((s, i) => (
                  <div key={`${s}-${i}`} className="symbol-cell">
                    <span>{s}</span>
                    <span className="idx">{String(i + 1).padStart(3, "0")}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </section>
      </main>
    </div>
  );
}

function BotLogo() {
  return (
    <svg
      className="bot-logo"
      width="44"
      height="44"
      viewBox="0 0 48 48"
      fill="none"
      aria-hidden="true"
    >
      {/* antenna */}
      <line x1="24" y1="2" x2="24" y2="6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <circle cx="24" cy="2" r="2" fill="currentColor" />
      {/* head */}
      <rect x="10" y="7" width="28" height="20" rx="5" stroke="currentColor" strokeWidth="2.2" fill="none" />
      {/* eyes */}
      <circle cx="18" cy="16" r="2.3" fill="currentColor" />
      <circle cx="30" cy="16" r="2.3" fill="currentColor" />
      {/* mouth slit */}
      <rect x="20" y="21" width="8" height="1.8" rx="0.9" fill="currentColor" />
      {/* arms reaching down to hold the coin */}
      <path d="M 13 27 L 17 36" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" />
      <path d="M 35 27 L 31 36" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" />
      {/* hands gripping the sides of the coin */}
      <circle cx="17" cy="38" r="1.6" fill="currentColor" />
      <circle cx="31" cy="38" r="1.6" fill="currentColor" />
      {/* bitcoin coin held between the hands */}
      <circle cx="24" cy="39" r="6.2" stroke="currentColor" strokeWidth="2.2" fill="none" />
      <text
        x="24"
        y="42.3"
        textAnchor="middle"
        fontFamily="Space Grotesk, system-ui, sans-serif"
        fontWeight="700"
        fontSize="8"
        fill="currentColor"
      >
        ₿
      </text>
    </svg>
  );
}

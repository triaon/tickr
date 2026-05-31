export type FetchArgs = {
  exchange: string;
  categories: string[];
  quote?: string;
  base?: string;
  activeOnly: boolean;
  tradingView: boolean;
  reverse: boolean;
  intersectWith?: string;
};

export type FetchResult = {
  ok: boolean;
  symbols: string[];
  error?: string;
};

declare global {
  interface Window {
    api: {
      fetchSymbols(args: FetchArgs): Promise<FetchResult>;
      saveText(defaultName: string, content: string): Promise<{ ok: boolean; path?: string }>;
      openExternal(url: string): Promise<void>;
      platform: string;
    };
  }
}

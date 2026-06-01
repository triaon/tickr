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


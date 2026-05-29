import { contextBridge, ipcRenderer } from "electron";

export type FetchArgs = {
  exchange: string;
  categories: string[];
  quote?: string;
  base?: string;
  activeOnly: boolean;
  tradingView: boolean;
  reverse: boolean;
};

contextBridge.exposeInMainWorld("api", {
  fetchSymbols: (args: FetchArgs) => ipcRenderer.invoke("fetch-symbols", args),
  saveText: (defaultName: string, content: string) => ipcRenderer.invoke("save-text", defaultName, content),
  openExternal: (url: string) => ipcRenderer.invoke("open-external", url),
  platform: process.platform,
});

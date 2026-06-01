import { invoke } from "@tauri-apps/api/core";
import { save } from "@tauri-apps/plugin-dialog";
import { writeTextFile } from "@tauri-apps/plugin-fs";
import { open } from "@tauri-apps/plugin-shell";
import type { FetchArgs, FetchResult } from "./types";

export const api = {
  async fetchSymbols(args: FetchArgs): Promise<FetchResult> {
    return await invoke<FetchResult>("fetch_symbols", { args });
  },
  async saveText(defaultName: string, content: string): Promise<{ ok: boolean; path?: string }> {
    const path = await save({ defaultPath: defaultName, filters: [{ name: "Text", extensions: ["txt"] }] });
    if (!path) return { ok: false };
    await writeTextFile(path, content);
    return { ok: true, path };
  },
  async openExternal(url: string): Promise<void> {
    await open(url);
  },
};

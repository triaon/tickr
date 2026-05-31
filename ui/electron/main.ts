import { app, BrowserWindow, ipcMain, dialog, shell } from "electron";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { writeFile } from "node:fs/promises";
import * as path from "node:path";

const isDev = process.env.NODE_ENV === "development";

// Resolve the bundled Go CLI binary.
// Dev: walk up from ui/ → repo root → dist/tickr-<platform>.
// Prod: process.resourcesPath/bin/tickr-<arch>(.exe). extraResources in
// package.json bundles both x64 and arm64 binaries per platform; we pick
// the right one at runtime based on process.arch.
function resolveBinary(): string {
  if (isDev) {
    const repoRoot = path.resolve(__dirname, "..", "..");
    const candidates = [
      "dist/tickr-mac-m",
      "dist/tickr-mac-intel",
      "dist/tickr-linux",
      "dist/tickr-linux-arm",
      "dist/tickr-windows.exe",
      "dist/tickr-windows-arm.exe",
    ];
    for (const rel of candidates) {
      const p = path.join(repoRoot, rel);
      if (existsSync(p)) return p;
    }
  }
  const ext = process.platform === "win32" ? ".exe" : "";
  const arch = process.arch === "arm64" ? "arm64" : "x64";
  return path.join(process.resourcesPath, "bin", `tickr-${arch}${ext}`);
}

type FetchArgs = {
  exchange: string;
  categories: string[];
  quote?: string;
  base?: string;
  activeOnly: boolean;
  tradingView: boolean;
  reverse: boolean;
  intersectWith?: string;
};

ipcMain.handle("fetch-symbols", async (_evt, args: FetchArgs): Promise<{ ok: boolean; symbols: string[]; error?: string }> => {
  const bin = resolveBinary();
  if (!existsSync(bin)) {
    return { ok: false, symbols: [], error: `Go binary not found at ${bin}` };
  }
  const cliArgs = [
    "fetch",
    "--exchange", args.exchange,
    "--categories", args.categories.join(","),
    "--format", "txt",
    `--active-only=${args.activeOnly}`,
  ];
  if (args.tradingView) cliArgs.push("--tv");
  if (args.reverse) cliArgs.push("--reverse");
  if (args.quote) cliArgs.push("--quote", args.quote);
  if (args.base) cliArgs.push("--base", args.base);
  if (args.intersectWith) cliArgs.push("--intersect-with", args.intersectWith);

  return new Promise((resolve) => {
    const proc = spawn(bin, cliArgs);
    let stdout = "";
    let stderr = "";
    proc.stdout.on("data", (b) => (stdout += b.toString()));
    proc.stderr.on("data", (b) => (stderr += b.toString()));
    proc.on("error", (err) => resolve({ ok: false, symbols: [], error: err.message }));
    proc.on("close", (code) => {
      if (code !== 0) {
        resolve({ ok: false, symbols: [], error: stderr.trim() || `exit ${code}` });
        return;
      }
      const symbols = stdout.split("\n").map((s) => s.trim()).filter(Boolean);
      resolve({ ok: true, symbols });
    });
  });
});

ipcMain.handle("save-text", async (_evt, defaultName: string, content: string) => {
  const win = BrowserWindow.getFocusedWindow();
  const result = await dialog.showSaveDialog(win!, {
    defaultPath: defaultName,
    filters: [{ name: "Text", extensions: ["txt"] }, { name: "All Files", extensions: ["*"] }],
  });
  if (result.canceled || !result.filePath) return { ok: false };
  await writeFile(result.filePath, content, "utf8");
  return { ok: true, path: result.filePath };
});

ipcMain.handle("open-external", async (_evt, url: string) => {
  await shell.openExternal(url);
});

function createWindow(): void {
  const win = new BrowserWindow({
    width: 980,
    height: 720,
    minWidth: 780,
    minHeight: 560,
    backgroundColor: "#F7F6F3",
    titleBarStyle: "hiddenInset",
    show: false,
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  win.once("ready-to-show", () => win.show());

  if (isDev) {
    win.loadURL("http://localhost:5173");
  } else {
    win.loadFile(path.join(__dirname, "..", "dist", "index.html"));
  }
}

app.whenReady().then(() => {
  createWindow();
  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});

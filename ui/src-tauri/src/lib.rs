use serde::{Deserialize, Serialize};
use tauri_plugin_shell::process::CommandEvent;
use tauri_plugin_shell::ShellExt;

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct FetchArgs {
    exchange: String,
    categories: Vec<String>,
    quote: Option<String>,
    base: Option<String>,
    active_only: bool,
    trading_view: bool,
    reverse: bool,
    intersect_with: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct FetchResult {
    ok: bool,
    symbols: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

#[tauri::command]
async fn fetch_symbols(app: tauri::AppHandle, args: FetchArgs) -> Result<FetchResult, String> {
    let mut cli_args: Vec<String> = vec![
        "fetch".into(),
        "--exchange".into(),
        args.exchange,
        "--categories".into(),
        args.categories.join(","),
        "--format".into(),
        "txt".into(),
        format!("--active-only={}", args.active_only),
    ];
    if args.trading_view {
        cli_args.push("--tv".into());
    }
    if args.reverse {
        cli_args.push("--reverse".into());
    }
    if let Some(q) = args.quote {
        if !q.is_empty() {
            cli_args.push("--quote".into());
            cli_args.push(q);
        }
    }
    if let Some(b) = args.base {
        if !b.is_empty() {
            cli_args.push("--base".into());
            cli_args.push(b);
        }
    }
    if let Some(iw) = args.intersect_with {
        if !iw.is_empty() {
            cli_args.push("--intersect-with".into());
            cli_args.push(iw);
        }
    }

    let sidecar = app
        .shell()
        .sidecar("tickr-cli")
        .map_err(|e| format!("sidecar lookup failed: {e}"))?
        .args(cli_args);

    let (mut rx, _child) = sidecar
        .spawn()
        .map_err(|e| format!("spawn failed: {e}"))?;

    let mut stdout = String::new();
    let mut stderr = String::new();
    let mut code: Option<i32> = None;
    while let Some(ev) = rx.recv().await {
        match ev {
            CommandEvent::Stdout(b) => stdout.push_str(&String::from_utf8_lossy(&b)),
            CommandEvent::Stderr(b) => stderr.push_str(&String::from_utf8_lossy(&b)),
            CommandEvent::Terminated(t) => code = t.code,
            _ => {}
        }
    }

    if code.unwrap_or(-1) != 0 {
        let msg = stderr.trim().to_string();
        return Ok(FetchResult {
            ok: false,
            symbols: vec![],
            error: Some(if msg.is_empty() {
                format!("exit {}", code.unwrap_or(-1))
            } else {
                msg
            }),
        });
    }
    let symbols: Vec<String> = stdout
        .split('\n')
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
        .collect();
    Ok(FetchResult {
        ok: true,
        symbols,
        error: None,
    })
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_fs::init())
        .invoke_handler(tauri::generate_handler![fetch_symbols])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

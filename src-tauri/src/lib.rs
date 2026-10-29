#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
  tauri::Builder::default()
    .plugin(tauri_plugin_shell::init())
    .plugin(tauri_plugin_http::init())
    .setup(|app| {
      if cfg!(debug_assertions) {
        app.handle().plugin(
          tauri_plugin_log::Builder::default()
            .level(log::LevelFilter::Info)
            .build(),
        )?;
      }

      let handle = app.handle().clone();
      tauri::async_runtime::spawn(async move {
        if let Err(e) = start_sidecar(&handle).await {
          log::error!("Failed to start sidecar: {}", e);
        }
      });

      Ok(())
    })
    .invoke_handler(tauri::generate_handler![
      get_metrics,
      get_history,
      get_geoip_stats,
      sidecar_health
    ])
    .run(tauri::generate_context!())
    .expect("error while building tauri application");
}

async fn start_sidecar(handle: &tauri::AppHandle) -> Result<(), Box<dyn std::error::Error>> {
  use tauri_plugin_shell::ShellExt;
  
  let shell = handle.shell();
  let sidecar = shell.sidecar("bandwidth-tracker-sidecar")?;
  
  let (mut rx, _child) = sidecar.spawn()?;
  
  tauri::async_runtime::spawn(async move {
    while let Some(event) = rx.recv().await {
      match event {
        tauri_plugin_shell::process::CommandEvent::Stdout(line) => {
          log::info!("[sidecar stdout] {}", String::from_utf8_lossy(&line));
        }
        tauri_plugin_shell::process::CommandEvent::Stderr(line) => {
          log::error!("[sidecar stderr] {}", String::from_utf8_lossy(&line));
        }
        tauri_plugin_shell::process::CommandEvent::Error(err) => {
          log::error!("[sidecar error] {}", err);
        }
        tauri_plugin_shell::process::CommandEvent::Terminated(payload) => {
          log::info!("[sidecar terminated] {:?}", payload);
        }
        _ => {}
      }
    }
  });

  // Wait for sidecar to be ready
  tokio::time::sleep(tokio::time::Duration::from_secs(2)).await;
  
  log::info!("Sidecar started");
  Ok(())
}

#[tauri::command]
async fn sidecar_health() -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  match client.get("http://127.0.0.1:8080/health").send().await {
    Ok(resp) => {
      let status = resp.status();
      let body = resp.json::<serde_json::Value>().await.unwrap_or(serde_json::json!({}));
      Ok(serde_json::json!({"status": status.as_u16(), "body": body}))
    }
    Err(e) => Ok(serde_json::json!({"error": e.to_string()})),
  }
}

#[tauri::command]
async fn get_metrics() -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  match client.get("http://127.0.0.1:8080/api/metrics").send().await {
    Ok(resp) => {
      let body = resp.json::<serde_json::Value>().await.map_err(|e| e.to_string())?;
      Ok(body)
    }
    Err(e) => Err(e.to_string()),
  }
}

#[tauri::command]
async fn get_history(range: Option<String>) -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  let range = range.unwrap_or_else(|| "24h".to_string());
  let url = format!("http://127.0.0.1:8080/api/history?range={}", range);
  match client.get(&url).send().await {
    Ok(resp) => {
      let body = resp.json::<serde_json::Value>().await.map_err(|e| e.to_string())?;
      Ok(body)
    }
    Err(e) => Err(e.to_string()),
  }
}

#[tauri::command]
async fn get_geoip_stats(range: Option<String>) -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  let range = range.unwrap_or_else(|| "24h".to_string());
  let url = format!("http://127.0.0.1:8080/api/geoip/stats?range={}", range);
  match client.get(&url).send().await {
    Ok(resp) => {
      let body = resp.json::<serde_json::Value>().await.map_err(|e| e.to_string())?;
      Ok(body)
    }
    Err(e) => Err(e.to_string()),
  }
}
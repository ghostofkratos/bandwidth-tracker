use tauri::Manager;

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

      let port = reserve_loopback_port()?;
      app.manage(BackendPort(port));

      let handle = app.handle().clone();
      tauri::async_runtime::spawn(async move {
        if let Err(e) = start_sidecar(&handle, port).await {
          log::error!("Failed to start sidecar: {}", e);
        }
      });

      Ok(())
    })
    .invoke_handler(tauri::generate_handler![
      get_metrics,
      get_history,
      get_geoip_stats,
      sidecar_health,
      wait_for_backend
    ])
    .run(tauri::generate_context!())
    .expect("error while building tauri application");
}

#[derive(Clone, Copy)]
struct BackendPort(u16);

fn reserve_loopback_port() -> Result<u16, std::io::Error> {
  let listener = std::net::TcpListener::bind(("127.0.0.1", 0))?;
  Ok(listener.local_addr()?.port())
}

async fn start_sidecar(handle: &tauri::AppHandle, port: u16) -> Result<(), Box<dyn std::error::Error>> {
  use tauri_plugin_shell::ShellExt;
  
  let shell = handle.shell();
  let sidecar = shell
    .sidecar("bandwidth-tracker-sidecar")?
    .env("PORT", port.to_string())
    .kill_on_drop(true);
  
  let (mut rx, child) = sidecar.spawn()?;
  
  tauri::async_runtime::spawn(async move {
    let _child = child;
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

  log::info!("Sidecar process spawned on 127.0.0.1:{}", port);
  Ok(())
}

#[tauri::command]
async fn sidecar_health(state: tauri::State<'_, BackendPort>) -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  let response = client
    .get(format!("http://127.0.0.1:{}/health", state.0))
    .send()
    .await
    .map_err(|e| e.to_string())?;
  let status = response.status();
  let body = response
    .error_for_status()
    .map_err(|e| e.to_string())?
    .json::<serde_json::Value>()
    .await
    .map_err(|e| e.to_string())?;
  Ok(serde_json::json!({"status": status.as_u16(), "body": body}))
}

#[tauri::command]
async fn wait_for_backend(state: tauri::State<'_, BackendPort>) -> Result<(), String> {
  let client = reqwest::Client::builder()
    .timeout(tokio::time::Duration::from_secs(1))
    .build()
    .map_err(|e| e.to_string())?;
  let url = format!("http://127.0.0.1:{}/health", state.0);
  let deadline = tokio::time::Instant::now() + tokio::time::Duration::from_secs(20);

  loop {
    if let Ok(response) = client.get(&url).send().await {
      if response.status().is_success() {
        return Ok(());
      }
    }
    if tokio::time::Instant::now() >= deadline {
      return Err(format!("Backend did not become ready on 127.0.0.1:{}", state.0));
    }
    tokio::time::sleep(tokio::time::Duration::from_millis(250)).await;
  }
}

#[tauri::command]
async fn get_metrics(state: tauri::State<'_, BackendPort>) -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  let response = client
    .get(format!("http://127.0.0.1:{}/api/metrics", state.0))
    .send()
    .await
    .map_err(|e| e.to_string())?
    .error_for_status()
    .map_err(|e| e.to_string())?;
  response.json::<serde_json::Value>().await.map_err(|e| e.to_string())
}

#[tauri::command]
async fn get_history(range: Option<String>, state: tauri::State<'_, BackendPort>) -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  let range = range.unwrap_or_else(|| "24h".to_string());
  let response = client
    .get(format!("http://127.0.0.1:{}/api/history", state.0))
    .query(&[("range", range)])
    .send()
    .await
    .map_err(|e| e.to_string())?
    .error_for_status()
    .map_err(|e| e.to_string())?;
  response.json::<serde_json::Value>().await.map_err(|e| e.to_string())
}

#[tauri::command]
async fn get_geoip_stats(range: Option<String>, state: tauri::State<'_, BackendPort>) -> Result<serde_json::Value, String> {
  let client = reqwest::Client::new();
  let range = range.unwrap_or_else(|| "24h".to_string());
  let response = client
    .get(format!("http://127.0.0.1:{}/api/geoip/stats", state.0))
    .query(&[("range", range)])
    .send()
    .await
    .map_err(|e| e.to_string())?
    .error_for_status()
    .map_err(|e| e.to_string())?;
  response.json::<serde_json::Value>().await.map_err(|e| e.to_string())
}

# Bandwidth Tracker

A cross-platform network bandwidth monitoring application with a Go backend and Tauri/React frontend. Tracks per-process network usage with GeoIP enrichment and automatic data aggregation.

## Features

- **Real-time network monitoring** - Tracks TCP/UDP connections per process using Windows IP Helper API
- **GeoIP enrichment** - MaxMind GeoLite2 Country lookup for remote IPs with 24h in-memory cache
- **Automatic aggregation** - Raw 5s samples → hourly summaries (7d retention) → daily summaries (1y retention)
- **Tauri desktop app** - Native Windows app with sidecar Go backend
- **REST API** - `/api/metrics`, `/api/history`, `/api/geoip/stats` endpoints

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Frontend       │────▶│  Tauri/Rust      │────▶│  Go Sidecar     │
│  (TypeScript)   │     │  (Commands)      │     │  (HTTP :8080)   │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                              ┌───────────────────────────┘
                              ▼
                    ┌──────────────────┐
                    │  SQLite Database │
                    │  (%APPDATA%/     │
                    │  bandwidth-      │
                    │  tracker/)       │
                    └──────────────────┘
```

## Prerequisites

- **Go** 1.21+ (tested with 1.27.3)
- **Rust** 1.90+ (MSVC toolchain on Windows)
- **Node.js** 20+ with **pnpm** 12+
- **MaxMind GeoLite2-Country.mmdb** - Download from [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) and place at `%APPDATA%\bandwidth-tracker\GeoLite2-Country.mmdb` or set `GEOIP_DB_PATH` env var

## Project Structure

```
bandwidth-tracker/
├── backend/                    # Go backend
│   ├── main.go                 # Entry point, HTTP server, workers
│   ├── go.mod / go.sum
│   ├── bin/                    # Compiled sidecar binary (gitignored)
│   └── internal/
│       ├── db/                 # Database layer
│       │   ├── schema.go       # SQL schema definitions
│       │   ├── db.go           # Connection + migrations
│       │   ├── process_repo.go # Process CRUD
│       │   └── traffic_repo.go # Traffic samples, aggregates, history
│       ├── collector/          # Windows network collection
│       │   ├── windows_connections.go # IP Helper API calls
│       │   ├── correlator.go   # Connection → sample correlation
│       │   └── worker.go       # 5s polling worker
│       ├── geoip/              # MaxMind GeoIP lookup
│       │   └── geoip.go        # Cached Lookup(ip) -> countryCode
│       └── aggregator/         # Data aggregation
│           ├── aggregator.go   # Hourly/daily rollups + cleanup
│           └── worker.go       # Hourly aggregation worker
├── src/                        # Frontend (TypeScript + Vite)
│   ├── main.ts                 # Dashboard UI + Tauri commands
│   └── style.css
├── src-tauri/                  # Tauri/Rust app
│   ├── Cargo.toml
│   ├── tauri.conf.json         # Sidecar config (externalBin)
│   ├── capabilities/default.json # Permissions
│   └── src/
│       ├── lib.rs              # Sidecar spawn + Tauri commands
│       └── main.rs
└── dist/                       # Built frontend (gitignored)
```

## Quick Start

### 1. Backend Development

```bash
cd backend
go mod tidy
go run main.go
# Server starts on http://localhost:8080
# Test: curl http://localhost:8080/health
# Test: curl http://localhost:8080/api/metrics
```

### 2. Frontend Development

```bash
# Terminal 1: Start backend
cd backend && go run main.go

# Terminal 2: Start Vite dev server
pnpm dev
# Frontend at http://localhost:5173 (proxied to backend)
```

### 3. Tauri Desktop App

```bash
# Build Go sidecar (required before tauri build)
cd backend
go build -o bin/bandwidth-tracker-sidecar-x86_64-pc-windows-msvc.exe main.go

# Build frontend
cd ..
pnpm build

# Run Tauri dev (starts sidecar automatically)
cd src-tauri
cargo tauri dev

# Or build release
cargo tauri build
```

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `DB_PATH` | `%APPDATA%\bandwidth-tracker` | SQLite database directory |
| `POLL_INTERVAL_SECONDS` | `5` | Collector polling interval |
| `AGGREGATION_WINDOW_SECONDS` | `60` | `/api/metrics` time window |
| `AGGREGATION_INTERVAL_MINUTES` | `60` | Aggregation worker interval |
| `RAW_RETENTION_DAYS` | `7` | Raw samples retention |
| `HOURLY_RETENTION_DAYS` | `365` | Hourly aggregates retention |
| `GEOIP_DB_PATH` | `%APPDATA%\bandwidth-tracker\GeoLite2-Country.mmdb` | MaxMind DB path |

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /api/metrics` | Current aggregated metrics (last 60s by default) |
| `GET /api/history?range=1h\|24h\|7d\|30d` | Time-series + per-app breakdown |
| `GET /api/geoip/stats?range=24h` | Top countries by traffic |

### Response Examples

**`/api/metrics`**
```json
{
  "download": 1048576,
  "upload": 524288,
  "apps": [
    {"pid": 1234, "name": "chrome.exe", "download": 500000, "upload": 200000, "connections": 15, "country": "US"}
  ]
}
```

**`/api/history?range=24h`**
```json
{
  "range": "24h",
  "granularity": "hour",
  "series": [
    {"timestamp": 1700000000000, "download": 1024000, "upload": 512000}
  ],
  "apps": [
    {"pid": 1234, "name": "chrome.exe", "download": 500000, "upload": 200000, "country": "US"}
  ]
}
```

## Database Schema

```sql
-- Processes
CREATE TABLE processes (
  id INTEGER PRIMARY KEY,
  pid INTEGER NOT NULL,
  exe_name TEXT NOT NULL,
  exe_path TEXT,
  icon_path TEXT,
  first_seen INTEGER NOT NULL,
  last_seen INTEGER NOT NULL,
  UNIQUE(pid, exe_path)
);

-- Raw samples (5s interval, 7d retention)
CREATE TABLE traffic_samples (
  id INTEGER PRIMARY KEY,
  timestamp INTEGER NOT NULL,
  process_id INTEGER NOT NULL,
  bytes_in INTEGER NOT NULL DEFAULT 0,
  bytes_out INTEGER NOT NULL DEFAULT 0,
  remote_ip TEXT,
  remote_port INTEGER,
  local_ip TEXT,
  local_port INTEGER,
  protocol TEXT NOT NULL,
  estimated BOOLEAN NOT NULL DEFAULT 1,
  country_code TEXT DEFAULT 'XX',
  FOREIGN KEY(process_id) REFERENCES processes(id)
);

-- Hourly aggregates (1h granularity, 365d retention)
CREATE TABLE hourly_aggregates (
  id INTEGER PRIMARY KEY,
  hour_start INTEGER NOT NULL,
  process_id INTEGER NOT NULL,
  country_code TEXT,
  bytes_in INTEGER NOT NULL,
  bytes_out INTEGER NOT NULL,
  connection_count INTEGER NOT NULL,
  FOREIGN KEY(process_id) REFERENCES processes(id),
  UNIQUE(hour_start, process_id, country_code)
);

-- Daily aggregates (1d granularity, 1y retention)
CREATE TABLE daily_aggregates (
  id INTEGER PRIMARY KEY,
  day_start INTEGER NOT NULL,
  process_id INTEGER NOT NULL,
  country_code TEXT,
  bytes_in INTEGER NOT NULL,
  bytes_out INTEGER NOT NULL,
  connection_count INTEGER NOT NULL,
  FOREIGN KEY(process_id) REFERENCES processes(id),
  UNIQUE(day_start, process_id, country_code)
);
```

## Testing

```bash
# Backend tests
cd backend
go test ./...
go vet ./...

# Frontend lint
pnpm lint

# Rust checks
cd src-tauri
cargo clippy -- -D warnings
cargo test
```

## Building for Distribution

```bash
# 1. Build Go sidecar
cd backend
go build -ldflags="-s -w" -o bin/bandwidth-tracker-sidecar-x86_64-pc-windows-msvc.exe main.go

# 2. Build frontend
cd ..
pnpm build

# 3. Build Tauri app
cd src-tauri
cargo tauri build
# Output: src-tauri/target/release/bundle/
```

## Known Limitations

- **Windows only** - Uses IP Helper API (`iphlpapi.dll`) for connection enumeration
- **Estimated bytes** - Uses 1500 MTU × 10 packets heuristic (no ETW/kernel counters yet)
- **No Linux/macOS collectors** - Platform-specific implementations needed
- **MaxMind DB manual** - Auto-update not implemented; replace `.mmdb` file manually

## License

ISC
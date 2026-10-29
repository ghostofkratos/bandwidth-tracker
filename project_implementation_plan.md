# Project Plan: Lightweight Open-Source Bandwidth & Telemetry Tracker

## 1. Project Overview & Scope
A lightweight, cross-platform desktop application inspired by GlassWire's visual telemetry, focused strictly on **data usage tracking, application-level bandwidth attribution, time-series history, and GeoIP visualization**, without heavy firewall management complexity.

### Tech Stack
* **Backend Agent:** Go (High performance, low memory footprint, excellent OS socket/network querying capabilities).
* **Frontend & Shell:** Tauri v2 (Rust + Web frontend framework like React, Vue, or vanilla HTML/TS for a sleek, lightweight UI).
* **Data Storage:** SQLite (Embedded, zero-config time-series storage with custom aggregation tables).
* **GeoIP Lookup:** MaxMind GeoLite2 Offline Database (`.mmdb` file) via a Go GeoIP reader library.

---

## 2. Prerequisites & Development Toolchain
Since this compiles into a native desktop application running a Go backend alongside a Tauri web frontend, you will need the following tools installed on your development machine (VS Code):

### Core Runtimes & Languages
* **Go (>= 1.22):** For compiling the backend data-collection agent.
* **Node.js (>= 18) & npm/pnpm:** For managing frontend dependencies and bundling web assets.
* **Rust Toolchain (`rustc`, `cargo`):** Required by Tauri to build the native system window and shell. Install via [rustup.rs](https://rustup.rs/).

### Platform-Specific Build Tools (Required for Tauri native compilation)
* **Windows:** 
  * Microsoft Visual Studio C++ Build Tools (or Visual Studio Community with "Desktop development with C++" workload).
  * WebView2 runtime (usually pre-installed on modern Windows 10/11).
* **macOS:** 
  * Xcode Command Line Tools (`xcode-select --install`).
* **Linux (Ubuntu/Debian example):** 
  * `sudo apt update && sudo apt install libwebkit2gtk-4.1-dev build-essential curl wget file libssl-dev libayatana-appindicator3-dev librsvg2-dev`

### Testing & Quality Assurance Tools
* **Go Testing:** Built-in `go test` package for unit-testing metric collection and database queries.
* **Frontend Testing:** Vitest or Playwright for UI component and end-to-end integration tests.
* **Linting:** `golangci-lint` (Go) and ESLint/Prettier (Frontend).

---

## 3. Architecture & Data Flow

```text
┌─────────────────────────────────────────────────────────────┐
│                       Tauri App Shell                       │
│  ┌───────────────────────┐         ┌─────────────────────┐  │
│  │    Frontend (UI)      │ <-----> │  Rust Core (Tauri)  │  │
│  │ (Charts, Tables, Map) │  IPC    │  (System Tray/Host) │  │
│  └───────────────────────┘         └──────────┬──────────┘  │
└───────────────────────────────────────────────┼─────────────┘
                                                │ Sidecar Process / IPC
┌───────────────────────────────────────────────▼─────────────┐
│                 Go Backend Agent (Daemon)                   │
│  ┌──────────────────┐  ┌─────────────────┐  ┌────────────┐  │
│  │ OS Kernel Stats  │  │ MaxMind GeoIP   │  │ SQLite DB  │  │
│  │ (Socket/Proc/ETW)│  │ (IP Lookup)     │  │ (TimeSeries)│ │
│  └──────────────────┘  └─────────────────┘  └────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. Development Stages

### Stage 1: Environment Setup & Project Initialization
* [x] Initialize a Tauri project with a frontend framework (e.g., React + TypeScript or Vanilla Vite).
* [x] Initialize a Go module (`go mod init bandwidth-tracker`) in a backend subdirectory.
* [ ] Verify local compilation of both systems using `cargo tauri dev` and `go build`.

### Stage 2: Go Backend & OS Network Telemetry Core
* [ ] **Network Polling/Capture:** 
  * Implement OS-specific handlers in Go to periodically query active network connections and byte counters mapped to Process IDs (PIDs).
  * *Linux:* Read `/proc/net/tcp` and cross-reference with `/proc/[pid]/fd` or use `gopacket`/eBPF stubs.
  * *Windows:* Query Windows networking performance counters or ETW network socket telemetry.
* [ ] **Database Schema Design:** Create SQLite tables for:
  * `processes` (id, executable_name, icon_path)
  * `traffic_samples` (timestamp, process_id, bytes_in, bytes_out, remote_ip)
  * `daily_aggregates` (date, process_id, total_bytes_in, total_bytes_out)

### Stage 3: GeoIP Integration & Aggregation Layer
* [ ] Download MaxMind GeoLite2 City/Country `.mmdb` database.
* [ ] Integrate a Go GeoIP library (e.g., `github.com/oschwald/maxminddb-golang`) into the backend.
* [ ] Write an enrichment worker that tags outgoing connection destination IPs with country codes and coordinates.
* [ ] Implement an automatic downsampling/cleanup routine in SQLite to collapse raw 1-minute samples older than 7 days into hourly summaries.

### Stage 4: Tauri Sidecar & IPC Bridge Integration
* [ ] Configure the Go backend to run as a **Tauri Sidecar** (spawned automatically when the Tauri app launches, running locally on a random ephemeral port or local Unix socket/named pipe).
* [ ] Set up Tauri Commands / Event listeners so the frontend can query real-time bandwidth metrics and historical time-series data on demand.

### Stage 5: Frontend Dashboard & Charting UI
* [ ] Design the layout: Top navigation, live system summary cards (Total Download/Upload speed), and time-range selectors (Last Hour, Today, This Week).
* [ ] Implement smooth area charts using a performant charting library (e.g., **Apache ECharts** or **Chart.js** via React wrappers) to mirror GlassWire's visual style.
* [ ] Build the **App Breakdown List**: A sortable table/list showing each application's total data usage, percentage share, and country flags for destinations.

### Stage 6: Testing, Packaging, and Distribution
* [ ] **Unit Tests:** Write Go unit tests for database insertions and metric calculations.
* [ ] **Integration Tests:** Test IPC communication between Tauri Rust core and the Go sidecar daemon.
* [ ] **Native Bundling:** Use Tauri's build pipeline (`cargo tauri build`) to generate native installers (`.msi`/`.exe` for Windows, `.dmg` for macOS, `.deb`/`.AppImage` for Linux).
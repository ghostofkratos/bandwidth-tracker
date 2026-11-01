import './style.css';

interface Metrics {
  download: number;
  upload: number;
  apps: AppMetrics[];
  estimated: boolean;
}

interface AppMetrics {
  pid: number;
  name: string;
  download: number;
  upload: number;
  connections: number;
  country: string;
}

interface HistoryResponse {
  range: string;
  granularity: string;
  series: HistoryPoint[];
  apps: AppHistoryPoint[];
  estimated: boolean;
}

interface HistoryPoint {
  timestamp: number;
  download: number;
  upload: number;
}

interface AppHistoryPoint {
  pid: number;
  name: string;
  download: number;
  upload: number;
  country: string;
}

interface GeoIPStats {
  countries: CountryStats[];
  estimated: boolean;
}

interface CountryStats {
  country_code: string;
  download: number;
  upload: number;
  total: number;
}

async function invoke<T>(command: string, args?: Record<string, unknown>): Promise<T> {
  const { invoke } = await import('@tauri-apps/api/core');
  return invoke<T>(command, args);
}

async function loadMetrics(): Promise<boolean> {
  try {
    const metrics = await invoke<Metrics>('get_metrics');
    updateMetricsDisplay(metrics);
    return true;
  } catch (error) {
    console.error('Failed to load metrics:', error);
    updateStatus(`Error: ${error}`);
    return false;
  }
}

async function loadHistory(range: string = '24h'): Promise<boolean> {
  try {
    const history = await invoke<HistoryResponse>('get_history', { range });
    updateHistoryDisplay(history);
    return true;
  } catch (error) {
    console.error('Failed to load history:', error);
    updateStatus(`History unavailable: ${error}`);
    return false;
  }
}

async function loadGeoIPStats(range: string = '24h'): Promise<boolean> {
  try {
    const stats = await invoke<GeoIPStats>('get_geoip_stats', { range });
    updateGeoIPDisplay(stats);
    return true;
  } catch (error) {
    console.error('Failed to load GeoIP stats:', error);
    updateStatus(`GeoIP totals unavailable: ${error}`);
    return false;
  }
}

function updateMetricsDisplay(metrics: Metrics) {
  const downloadEl = document.getElementById('download');
  const uploadEl = document.getElementById('upload');
  const appsEl = document.getElementById('apps');
  const qualityEl = document.getElementById('data-quality');

  if (qualityEl) {
    qualityEl.textContent = metrics.estimated
      ? 'Traffic values are estimates from active connections, not measured byte counters.'
      : 'No traffic samples have been collected yet.';
  }

  if (downloadEl) downloadEl.textContent = formatBytes(metrics.download) + '/s';
  if (uploadEl) uploadEl.textContent = formatBytes(metrics.upload) + '/s';

  if (appsEl) {
    appsEl.innerHTML = (metrics.apps ?? [])
      .sort((a, b) => (b.download + b.upload) - (a.download + a.upload))
      .slice(0, 10)
      .map(app => `
        <tr>
          <td>${app.pid}</td>
          <td>${app.name}</td>
          <td>${formatBytes(app.download)}/s</td>
          <td>${formatBytes(app.upload)}/s</td>
          <td>${app.connections}</td>
          <td>${app.country}</td>
        </tr>
      `).join('');
  }
}

function updateHistoryDisplay(history: HistoryResponse) {
  const seriesEl = document.getElementById('history-series');
  if (seriesEl) {
    seriesEl.innerHTML = (history.series ?? [])
      .map(point => `
        <tr>
          <td>${new Date(point.timestamp).toLocaleTimeString()}</td>
          <td>${formatBytes(point.download)}</td>
          <td>${formatBytes(point.upload)}</td>
        </tr>
      `).join('');
  }
  const freshnessEl = document.getElementById('history-freshness');
  if (freshnessEl) {
    freshnessEl.textContent = history.range === '30d'
      ? 'Daily totals are aggregated once per day; the current day appears after its daily rollup.'
      : history.range === '1h'
        ? 'This range uses recent samples.'
        : 'Hourly totals are aggregated after each hour completes; the current hour appears in the next rollup.';
  }
}

function updateGeoIPDisplay(stats: GeoIPStats) {
  const geoipEl = document.getElementById('geoip-stats');
  if (geoipEl) {
    geoipEl.innerHTML = (stats.countries ?? [])
      .map(c => `
        <tr>
          <td>${c.country_code}</td>
          <td>${formatBytes(c.download)}</td>
          <td>${formatBytes(c.upload)}</td>
          <td>${formatBytes(c.total)}</td>
        </tr>
      `).join('');
  }
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.max(0, Math.min(sizes.length - 1, Math.floor(Math.log(bytes) / Math.log(k))));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function updateStatus(message: string) {
  const statusEl = document.getElementById('status');
  if (statusEl) statusEl.textContent = message;
}

const app = document.getElementById('app');
if (app) {
  app.innerHTML = `
    <div style="padding: 2rem; font-family: system-ui, sans-serif; max-width: 1000px; margin: 0 auto;">
      <h1>Bandwidth Tracker</h1>
      <p id="data-quality" style="margin-bottom: 1rem; color: #aab;">
        Traffic values are estimates from active connections, not measured byte counters.
      </p>
      
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 2rem;">
        <div style="background: #f5f5f5; padding: 1rem; border-radius: 8px;">
          <h3>Download</h3>
          <p id="download" style="font-size: 1.5rem; font-weight: bold;">0 B/s</p>
        </div>
        <div style="background: #f5f5f5; padding: 1rem; border-radius: 8px;">
          <h3>Upload</h3>
          <p id="upload" style="font-size: 1.5rem; font-weight: bold;">0 B/s</p>
        </div>
      </div>

      <div style="margin-bottom: 2rem;">
        <label for="range-select" style="margin-right: 0.5rem;">Time Range:</label>
        <select id="range-select" onchange="window.loadSelectedRange()">
          <option value="1h">Last Hour</option>
          <option value="24h" selected>Last 24 Hours</option>
          <option value="7d">Last 7 Days</option>
          <option value="30d">Last 30 Days</option>
        </select>
        <button onclick="window.refreshData()" style="margin-left: 1rem; padding: 0.5rem 1rem;">Refresh</button>
      </div>

      <div style="margin-bottom: 2rem;">
        <h2>Top Applications</h2>
        <table style="width: 100%; border-collapse: collapse;">
          <thead>
            <tr style="background: #f5f5f5;">
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">PID</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Name</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Download</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Upload</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Connections</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Country</th>
            </tr>
          </thead>
          <tbody id="apps" style="border-bottom: 1px solid #ddd;"></tbody>
        </table>
      </div>

      <div style="margin-bottom: 2rem;">
        <h2>History (<span id="range-display">24h</span>)</h2>
        <p id="history-freshness" style="margin: 0.5rem 0; color: #aab;"></p>
        <table style="width: 100%; border-collapse: collapse;">
          <thead>
            <tr style="background: #f5f5f5;">
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Time</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Download</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Upload</th>
            </tr>
          </thead>
          <tbody id="history-series" style="border-bottom: 1px solid #ddd;"></tbody>
        </table>
      </div>

      <div>
        <h2>Top Countries by Traffic</h2>
        <table style="width: 100%; border-collapse: collapse;">
          <thead>
            <tr style="background: #f5f5f5;">
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Country</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Download</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Upload</th>
              <th style="padding: 0.5rem; text-align: left; border-bottom: 1px solid #ddd;">Total</th>
            </tr>
          </thead>
          <tbody id="geoip-stats" style="border-bottom: 1px solid #ddd;"></tbody>
        </table>
      </div>

      <p id="status" style="margin-top: 2rem; color: #666;">Connecting to backend...</p>
    </div>
  `;
}

async function refreshData(range?: string) {
  const select = document.getElementById('range-select') as HTMLSelectElement;
  const selectedRange = range ?? select.value;
  const results = await Promise.all([
    loadMetrics(),
    loadHistory(selectedRange),
    loadGeoIPStats(selectedRange),
  ]);
  const display = document.getElementById('range-display');
  if (display) display.textContent = selectedRange;
  if (results.every(Boolean)) updateStatus('Connected to backend');
  return results.every(Boolean);
}

let metricsPollTimer: number | undefined;

(window as any).loadSelectedRange = async () => {
  const select = document.getElementById('range-select') as HTMLSelectElement;
  await refreshData(select.value);
};

(window as any).refreshData = async () => {
  await refreshData();
};

async function connectAndLoad() {
  updateStatus('Waiting for backend...');
  try {
    await invoke<void>('wait_for_backend');
    const loaded = await refreshData();
    if (!loaded) {
      window.setTimeout(connectAndLoad, 2000);
      return;
    }
    if (metricsPollTimer === undefined) {
      metricsPollTimer = window.setInterval(() => {
        void loadMetrics().then((loadedMetrics) => {
          if (!loadedMetrics) void connectAndLoad();
        });
      }, 5000);
    }
  } catch (error) {
    console.error('Backend did not become ready:', error);
    updateStatus(`Backend unavailable: ${error}. Retrying...`);
    window.setTimeout(connectAndLoad, 2000);
  }
}

void connectAndLoad();

console.log('Frontend loaded');

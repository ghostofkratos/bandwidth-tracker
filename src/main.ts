import './style.css';

const app = document.getElementById('app');
if (app) {
  app.innerHTML = `
    <div style="padding: 2rem; font-family: system-ui, sans-serif;">
      <h1>Bandwidth Tracker</h1>
      <p>Tauri + Vanilla TypeScript frontend initialized successfully.</p>
      <p id="status">Ready to connect to Go backend...</p>
    </div>
  `;
}

console.log('Frontend loaded');

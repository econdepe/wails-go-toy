<script>
  import { onMount } from 'svelte';
  import { GetServiceStatus, InstallService, InstallSystemService, UninstallService, StartService, StopService, ReadLog } from '../wailsjs/go/app/App';

  let status = 'Loading...';
  let message = '';
  let log = '';
  let loading = false;

  async function refreshStatus() {
    try {
      status = await GetServiceStatus();
    } catch (e) {
      status = 'Error: ' + e;
    }
  }

  async function refreshLog() {
    try {
      log = await ReadLog();
    } catch (e) {
      log = 'Error reading log: ' + e;
    }
  }

  async function handleInstall() {
    loading = true;
    message = await InstallService();
    await refreshStatus();
    loading = false;
  }

  async function handleInstallSystem() {
    loading = true;
    message = await InstallSystemService();
    await refreshStatus();
    loading = false;
  }

  async function handleUninstall() {
    loading = true;
    message = await UninstallService();
    await refreshStatus();
    loading = false;
  }

  async function handleStart() {
    loading = true;
    message = await StartService();
    await refreshStatus();
    setTimeout(refreshLog, 1000);
    loading = false;
  }

  async function handleStop() {
    loading = true;
    message = await StopService();
    await refreshStatus();
    loading = false;
  }

  onMount(() => {
    refreshStatus();
    refreshLog();
    
    // Auto-refresh status and log every 5 seconds
    const interval = setInterval(() => {
      refreshStatus();
      refreshLog();
    }, 5000);

    return () => clearInterval(interval);
  });
</script>

<main>
  <div class="container">
    <h1>🔄 Service Manager</h1>
    
    <div class="status-box">
      <h2>Service Status</h2>
      <div class="status {status.toLowerCase().includes('running') ? 'running' : status.toLowerCase().includes('not') ? 'not-installed' : 'stopped'}">
        {status}
      </div>
    </div>

    <div class="controls">
      <button on:click={handleInstall} disabled={loading}>Install (user)</button>
      <button on:click={handleInstallSystem} disabled={loading}>Install (system)</button>
      <button on:click={handleStart} disabled={loading}>Start Service</button>
      <button on:click={handleStop} disabled={loading}>Stop Service</button>
      <button on:click={handleUninstall} disabled={loading}>Uninstall Service</button>
    </div>

    {#if message}
      <div class="message">
        {message}
      </div>
    {/if}

    <div class="log-box">
      <h2>Service Log</h2>
      <div class="log">
        {log || 'No log data available'}
      </div>
      <button on:click={refreshLog} disabled={loading}>Refresh Log</button>
    </div>
  </div>
</main>

<style>
  main {
    height: 100vh;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    justify-content: center;
    align-items: center;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  }

  .container {
    background: white;
    border-radius: 20px;
    padding: 40px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    max-width: 700px;
    width: 90%;
  }

  h1 {
    text-align: center;
    color: #333;
    margin-bottom: 30px;
  }

  h2 {
    color: #555;
    font-size: 18px;
    margin-bottom: 15px;
  }

  .status-box {
    background: #f8f9fa;
    padding: 20px;
    border-radius: 10px;
    margin-bottom: 25px;
  }

  .status {
    font-size: 24px;
    font-weight: bold;
    padding: 15px;
    border-radius: 8px;
    text-align: center;
  }

  .status.running {
    background: #d4edda;
    color: #155724;
  }

  .status.stopped {
    background: #fff3cd;
    color: #856404;
  }

  .status.not-installed {
    background: #f8d7da;
    color: #721c24;
  }

  .controls {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 15px;
    margin-bottom: 25px;
  }

  button {
    background: #667eea;
    color: white;
    border: none;
    padding: 12px 20px;
    border-radius: 8px;
    font-size: 16px;
    cursor: pointer;
    transition: all 0.3s;
  }

  button:hover:not(:disabled) {
    background: #5568d3;
    transform: translateY(-2px);
    box-shadow: 0 5px 15px rgba(102, 126, 234, 0.4);
  }

  button:disabled {
    background: #ccc;
    cursor: not-allowed;
  }

  .message {
    background: #e7f3ff;
    border-left: 4px solid #2196F3;
    padding: 15px;
    margin-bottom: 25px;
    border-radius: 5px;
    color: #0c5460;
  }

  .log-box {
    background: #f8f9fa;
    padding: 20px;
    border-radius: 10px;
  }

  .log {
    background: #1e1e1e;
    color: #d4d4d4;
    padding: 15px;
    border-radius: 8px;
    font-family: 'Courier New', monospace;
    font-size: 13px;
    max-height: 200px;
    overflow-y: auto;
    margin-bottom: 15px;
    white-space: pre-wrap;
    word-wrap: break-word;
  }

  .log-box button {
    width: 100%;
  }
</style>
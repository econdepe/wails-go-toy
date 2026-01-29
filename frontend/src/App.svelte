<script>
  import { onMount } from 'svelte';
  import { GetServiceStatus, InstallService, InstallSystemService, UninstallService, StartService, StopService, ReadLog } from '../wailsjs/go/app/App';
  import { buildLogForDisplay } from './helpers/log';

  let status = 'Loading...';
  let message = '';
  let log = '';
  let loading = false;
  let logElement;
  let displayLog = '';

  $: displayLog = buildLogForDisplay(log);

  const refreshStatus = async () => {
    try {
      status = await GetServiceStatus();
    } catch (e) {
      status = 'Error: ' + e;
    }
  };

  const scrollLogToTop = () => {
    logElement?.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const scrollLogToBottom = () => {
    if (!logElement) return;
    logElement.scrollTo({ top: logElement.scrollHeight, behavior: 'smooth' });
  };

  const refreshLog = async () => {
    try {
      loading = true;
      log = await ReadLog();
    } catch (e) {
      log = 'Error reading log: ' + e;
    } finally {
      loading = false;
    }
  };

  const handleInstall = async () => {
    loading = true;
    message = await InstallService();
    await refreshStatus();
    loading = false;
  };

  const handleInstallSystem = async () => {
    loading = true;
    message = await InstallSystemService();
    await refreshStatus();
    loading = false;
  };

  const handleUninstall = async () => {
    loading = true;
    message = await UninstallService();
    await refreshStatus();
    loading = false;
  };

  const handleStart = async () => {
    loading = true;
    message = await StartService();
    await refreshStatus();
    setTimeout(refreshLog, 1000);
    loading = false;
  };

  const handleStop = async () => {
    loading = true;
    message = await StopService();
    await refreshStatus();
    loading = false;
  };

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
    <h1>🧸 GO-TOY - A Toy Wails App</h1>
    
    <div class="status-box">
      <h2>Service Status</h2>
      <div class="status {status.toLowerCase().includes('running') ? 'running' : status.toLowerCase().includes('not') ? 'not-installed' : 'stopped'}">
        {status}
      </div>
    </div>

    <div class="controls">
      <button class="span-2" on:click={handleInstall} disabled={loading}>Install (user)</button>
      <button class="span-2" on:click={handleInstallSystem} disabled={loading}>Install (system)</button>
      <button class="span-2" on:click={handleUninstall} disabled={loading}>Uninstall Service</button>
      <button class="span-3" on:click={handleStart} disabled={loading}>Start Service</button>
      <button class="span-3" on:click={handleStop} disabled={loading}>Stop Service</button>
    </div>

    {#if message}
      <div class="message">
        {message}
      </div>
    {/if}

    <div class="log-box">
      <h2>Service Log</h2>
      <div class="log" bind:this={logElement}>
        {displayLog || 'No log data available'}
      </div>
      <div class="log-controls">
        <button on:click={scrollLogToTop} disabled={loading}>Scroll to top</button>
        <button on:click={scrollLogToBottom} disabled={loading}>Scroll to bottom</button>
      </div>
    </div>
  </div>
</main>

<style>
  main {
    height: 100vh;
    height: 100dvh;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    justify-content: center;
    align-items: flex-start;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 24px;
    box-sizing: border-box;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  }

  .container {
    background: white;
    border-radius: 20px;
    padding: 40px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    max-width: 700px;
    width: 90%;
    box-sizing: border-box;
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
    grid-template-columns: repeat(6, 1fr);
    gap: 15px;
    margin-bottom: 25px;
  }

  .controls button {
    width: 100%;
  }

  .controls .span-2 {
    grid-column: span 2;
  }

  .controls .span-3 {
    grid-column: span 3;
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

  .log-controls {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 15px;
  }

  .log-box button {
    width: 100%;
  }
</style>
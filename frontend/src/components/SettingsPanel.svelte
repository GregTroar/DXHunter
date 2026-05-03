<script>
  import { createEventDispatcher, onMount } from 'svelte';
  const dispatch = createEventDispatcher();

  let cfg = null;
  let loading = true;
  let saving = false;
  let saveError = '';
  let saveOk = false;

  let openSection = 'general';

  onMount(async () => {
    try {
      const res  = await fetch('/api/config');
      const data = await res.json();
      cfg = data.data;
    } catch (e) {
      saveError = e.message;
    } finally {
      loading = false;
    }
  });

  async function save() {
    if (saving) return;
    saving    = true;
    saveError = '';
    saveOk    = false;
    try {
      const res  = await fetch('/api/config', {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify(cfg),
      });
      const data = await res.json();
      if (!data.success) { saveError = data.error || 'Save failed'; return; }
      saveOk = true;
      setTimeout(() => { saveOk = false; dispatch('saved'); }, 1200);
    } catch (e) {
      saveError = e.message;
    } finally {
      saving = false;
    }
  }

  function toggle(section) {
    openSection = openSection === section ? '' : section;
  }

  const LOG_LEVELS = ['DEBUG', 'INFO', 'WARN'];
  const INPUT = 'px-2 py-1 rounded bg-slate-800 border border-slate-600 text-slate-200 text-xs focus:outline-none focus:border-blue-500/70 w-full';
  const RESTART_BADGE = 'ml-1 px-1 py-0 rounded text-[9px] font-bold bg-amber-500/20 text-amber-400 border border-amber-500/40';

  // ── QRZ test ─────────────────────────────────────────────────────────────────
  let qrzTesting = false;
  let qrzTestResult = null; // null | { ok: bool, msg: string }

  // ── Cluster test ──────────────────────────────────────────────────────────────
  let clusterTesting = [];   // bool per index
  let clusterTestResult = []; // null | { ok: bool, msg: string } per index

  async function testCluster(i) {
    if (clusterTesting[i]) return;
    clusterTesting[i]    = true;
    clusterTestResult[i] = null;
    clusterTesting = [...clusterTesting];
    clusterTestResult = [...clusterTestResult];
    try {
      const cl = cfg.clusters[i];
      const res = await fetch('/api/config/test-cluster', {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify({ server: cl.server, port: cl.port }),
      });
      const data = await res.json();
      clusterTestResult[i] = { ok: data.success, msg: data.success ? data.message : (data.error || 'Failed') };
    } catch (e) {
      clusterTestResult[i] = { ok: false, msg: e.message };
    } finally {
      clusterTesting[i] = false;
      clusterTesting = [...clusterTesting];
      clusterTestResult = [...clusterTestResult];
    }
  }

  async function testQRZ() {
    if (qrzTesting) return;
    qrzTesting    = true;
    qrzTestResult = null;
    try {
      const res  = await fetch('/api/config/test-qrz', {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify({ username: cfg.qrz.username, password: cfg.qrz.password }),
      });
      const data = await res.json();
      qrzTestResult = { ok: data.success, msg: data.success ? 'Connection successful' : (data.error || 'Failed') };
    } catch (e) {
      qrzTestResult = { ok: false, msg: e.message };
    } finally {
      qrzTesting = false;
    }
  }
</script>

<!-- Overlay -->
<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
     role="dialog" aria-modal="true"
     on:click|self={() => dispatch('close')}
     on:keydown={(e) => e.key === 'Escape' && dispatch('close')}>

  <div class="w-full max-w-2xl max-h-[90vh] flex flex-col bg-slate-900 border border-slate-700/80 rounded-xl shadow-2xl overflow-hidden">

    <!-- Header -->
    <div class="flex items-center justify-between px-4 py-3 border-b border-slate-700/60 flex-shrink-0">
      <span class="text-sm font-bold text-slate-200 flex items-center gap-2">
        <svg class="w-4 h-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        Configuration
      </span>
      <button on:click={() => dispatch('close')}
        class="text-slate-500 hover:text-white transition-colors text-lg leading-none">✕</button>
    </div>

    <!-- Body -->
    <div class="flex-1 overflow-y-auto px-4 py-3 space-y-2 text-xs">

      {#if loading}
        <div class="flex items-center justify-center h-32 text-slate-500">Loading…</div>

      {:else if cfg}

        <!-- ── Général ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('general')}>
            <span class="font-semibold text-slate-300">🔧 General</span>
            <span class="text-slate-500">{openSection === 'general' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'general'}
            <div class="px-3 py-3 space-y-3 bg-slate-900/40">
              <div class="grid grid-cols-2 gap-3">
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">Callsign</span>
                  <input bind:value={cfg.general.callsign} class="{INPUT} uppercase" />
                </label>
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">Grid (Maidenhead)</span>
                  <input bind:value={cfg.general.grid} class="{INPUT} uppercase" maxlength="6" />
                </label>
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">Log level</span>
                  <select bind:value={cfg.general.logLevel} class="{INPUT}">
                    {#each LOG_LEVELS as l}<option value={l}>{l}</option>{/each}
                  </select>
                </label>
                <div class="flex flex-col gap-2 pt-3">
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" bind:checked={cfg.general.flexRadioSpot} class="accent-blue-500" />
                    <span class="text-slate-400">FlexRadio spot</span>
                  </label>
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" bind:checked={cfg.general.sendFreqModeToLog} class="accent-blue-500" />
                    <span class="text-slate-400">Send freq/mode to Log4OM</span>
                  </label>
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" bind:checked={cfg.general.telnetServer} class="accent-blue-500" />
                    <span class="text-slate-400">Telnet server</span>
                  </label>
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" bind:checked={cfg.general.deleteLogFileAtStart} class="accent-blue-500" />
                    <span class="text-slate-400">Delete log file at start</span>
                  </label>
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" bind:checked={cfg.general.logToFile} class="accent-blue-500" />
                    <span class="text-slate-400">Log to file <span class={RESTART_BADGE}>⚠ restart</span></span>
                  </label>
                </div>
              </div>

              <!-- Contest -->
              <div class="border-t border-slate-700/40 pt-3 space-y-2">
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" bind:checked={cfg.general.contestMode} class="accent-amber-500" />
                  <span class="text-slate-300 font-semibold">Contest mode</span>
                </label>
                {#if cfg.general.contestMode}
                  <label class="flex flex-col gap-1">
                    <span class="text-slate-500">Contest prefix</span>
                    <input bind:value={cfg.general.contestPrefix} class="{INPUT} uppercase font-mono" placeholder="e.g. CQ-WW" />
                  </label>
                {/if}
              </div>
            </div>
          {/if}
        </div>

        <!-- ── Database ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('database')}>
            <span class="font-semibold text-slate-300">🗄️ Database / Logbook</span>
            <span class="text-slate-500">{openSection === 'database' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'database'}
            <div class="px-3 py-3 space-y-3 bg-slate-900/40">
              <div class="flex items-center gap-4">
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="radio" name="db-type" value={false} bind:group={cfg.database.mysql} class="accent-blue-500" />
                  <span class="text-slate-300">SQLite</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="radio" name="db-type" value={true} bind:group={cfg.database.mysql} class="accent-blue-500" />
                  <span class="text-slate-300">MySQL</span>
                </label>
                <span class={RESTART_BADGE}>⚠ restart</span>
              </div>

              {#if !cfg.database.mysql}
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">SQLite path <span class={RESTART_BADGE}>⚠ restart</span></span>
                  <input bind:value={cfg.database.sqlitePath} class="{INPUT} font-mono" placeholder="/path/to/log4om.db" />
                </label>
              {:else}
                <div class="grid grid-cols-2 gap-3">
                  <label class="flex flex-col gap-1">
                    <span class="text-slate-500">Host <span class={RESTART_BADGE}>⚠ restart</span></span>
                    <input bind:value={cfg.database.mysqlHost} class="{INPUT} font-mono" placeholder="localhost" />
                  </label>
                  <label class="flex flex-col gap-1">
                    <span class="text-slate-500">Port <span class={RESTART_BADGE}>⚠ restart</span></span>
                    <input bind:value={cfg.database.mysqlPort} class="{INPUT} font-mono" placeholder="3306" />
                  </label>
                  <label class="flex flex-col gap-1">
                    <span class="text-slate-500">Database name <span class={RESTART_BADGE}>⚠ restart</span></span>
                    <input bind:value={cfg.database.mysqlDbName} class="{INPUT} font-mono" />
                  </label>
                  <label class="flex flex-col gap-1">
                    <span class="text-slate-500">User <span class={RESTART_BADGE}>⚠ restart</span></span>
                    <input bind:value={cfg.database.mysqlUser} class="{INPUT} font-mono" />
                  </label>
                  <label class="flex flex-col gap-1 col-span-2">
                    <span class="text-slate-500">Password <span class={RESTART_BADGE}>⚠ restart</span></span>
                    <input type="password" bind:value={cfg.database.mysqlPassword} class="{INPUT} font-mono" autocomplete="new-password" />
                  </label>
                </div>
              {/if}

              <label class="flex flex-col gap-1">
                <span class="text-slate-500">Logbook type <span class={RESTART_BADGE}>⚠ restart</span></span>
                <select bind:value={cfg.database.logbookType} class="{INPUT}">
                  <option value="log4om">Log4OM</option>
                  <option value="hrd">HRD</option>
                </select>
              </label>
            </div>
          {/if}
        </div>

        <!-- ── FTx ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('ftx')}>
            <span class="font-semibold text-slate-300">📡 FTx (WSJT-X / JTDX / MSHV)</span>
            <span class="text-slate-500">{openSection === 'ftx' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'ftx'}
            <div class="px-3 py-3 grid grid-cols-2 gap-3 bg-slate-900/40">
              <label class="flex flex-col gap-1">
                <span class="text-slate-500">Multicast IP <span class={RESTART_BADGE}>⚠ restart</span></span>
                <input bind:value={cfg.ftx.multicastIp} class="{INPUT} font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-slate-500">Port <span class={RESTART_BADGE}>⚠ restart</span></span>
                <input type="number" bind:value={cfg.ftx.port} class="{INPUT} font-mono" />
              </label>
              <div class="flex flex-col gap-2 pt-1">
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" bind:checked={cfg.ftx.enabled} class="accent-purple-500" />
                  <span class="text-slate-400">Enabled <span class={RESTART_BADGE}>⚠ restart</span></span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" bind:checked={cfg.ftx.multicast} class="accent-purple-500" />
                  <span class="text-slate-400">Multicast <span class={RESTART_BADGE}>⚠ restart</span></span>
                </label>
              </div>
            </div>
          {/if}
        </div>

        <!-- ── FlexRadio ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('flex')}>
            <span class="font-semibold text-slate-300">📻 FlexRadio</span>
            <span class="text-slate-500">{openSection === 'flex' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'flex'}
            <div class="px-3 py-3 grid grid-cols-2 gap-3 bg-slate-900/40">
              <label class="flex flex-col gap-1">
                <span class="text-slate-500">IP Address <span class={RESTART_BADGE}>⚠ restart</span></span>
                <input bind:value={cfg.flex.ip} class="{INPUT} font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-slate-500">Spot life (seconds)</span>
                <input bind:value={cfg.flex.spotLife} class="{INPUT} font-mono" />
              </label>
              <label class="flex items-center gap-2 cursor-pointer pt-1">
                <input type="checkbox" bind:checked={cfg.flex.discovery} class="accent-blue-500" />
                <span class="text-slate-400">Discovery (same network) <span class={RESTART_BADGE}>⚠ restart</span></span>
              </label>
            </div>
          {/if}
        </div>

        <!-- ── QRZ ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('qrz')}>
            <span class="font-semibold text-slate-300">🔍 QRZ.com</span>
            <span class="text-slate-500">{openSection === 'qrz' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'qrz'}
            <div class="px-3 py-3 space-y-2 bg-slate-900/40">
              <div class="grid grid-cols-2 gap-3">
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">Username</span>
                  <input bind:value={cfg.qrz.username} class="{INPUT}" autocomplete="off" />
                </label>
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">Password</span>
                  <input type="password" bind:value={cfg.qrz.password} class="{INPUT}" autocomplete="new-password" />
                </label>
              </div>
              <div class="flex items-center gap-3 pt-1">
                <button on:click={testQRZ} disabled={qrzTesting || !cfg.qrz.username}
                  class="px-3 py-1 rounded text-xs font-semibold border transition-colors
                    {qrzTesting ? 'bg-slate-700 text-slate-400 border-slate-600 cursor-wait'
                                : 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40 hover:bg-cyan-500/35 disabled:opacity-40'}">
                  {qrzTesting ? 'Test…' : 'Test Connection'}
                </button>
                {#if qrzTestResult}
                  <span class="text-xs font-semibold {qrzTestResult.ok ? 'text-green-400' : 'text-red-400'}">
                    {qrzTestResult.ok ? '✓' : '✗'} {qrzTestResult.msg}
                  </span>
                {/if}
              </div>
            </div>
          {/if}
        </div>

        <!-- ── Gotify ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('gotify')}>
            <span class="font-semibold text-slate-300">🔔 Gotify Notifications</span>
            <span class="text-slate-500">{openSection === 'gotify' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'gotify'}
            <div class="px-3 py-3 space-y-3 bg-slate-900/40">
              <div class="grid grid-cols-2 gap-3">
                <label class="flex flex-col gap-1 col-span-2">
                  <span class="text-slate-500">URL</span>
                  <input bind:value={cfg.gotify.url} class="{INPUT} font-mono" />
                </label>
                <label class="flex flex-col gap-1">
                  <span class="text-slate-500">Token</span>
                  <input bind:value={cfg.gotify.token} class="{INPUT} font-mono" />
                </label>
                <div class="flex items-center gap-2 pt-3">
                  <input type="checkbox" bind:checked={cfg.gotify.enable} class="accent-yellow-500" id="gotify-enable" />
                  <label for="gotify-enable" class="text-slate-400 cursor-pointer">Enabled</label>
                </div>
              </div>
              <div class="grid grid-cols-3 gap-2 pt-1 border-t border-slate-700/40">
                {#each [
                  { key: 'newDXCC',        label: 'New DXCC' },
                  { key: 'newBand',        label: 'New Band' },
                  { key: 'newMode',        label: 'New Mode' },
                  { key: 'newBandAndMode', label: 'Band + Mode' },
                  { key: 'watchlist',      label: 'Watchlist' },
                  { key: 'windowsNotify',  label: 'Windows Notify' },
                ] as n}
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" bind:checked={cfg.gotify[n.key]} class="accent-yellow-500" />
                    <span class="text-slate-400">{n.label}</span>
                  </label>
                {/each}
              </div>
            </div>
          {/if}
        </div>

        <!-- ── Clusters ── -->
        <div class="border border-slate-700/50 rounded-lg overflow-hidden">
          <button class="w-full flex items-center justify-between px-3 py-2 bg-slate-800/60 hover:bg-slate-800 text-left transition-colors"
            on:click={() => toggle('clusters')}>
            <span class="font-semibold text-slate-300">🌐 Clusters</span>
            <span class="text-slate-500">{openSection === 'clusters' ? '▲' : '▼'}</span>
          </button>
          {#if openSection === 'clusters'}
            <div class="bg-slate-900/40 divide-y divide-slate-700/30">
              {#each cfg.clusters as cl, i}
                <div class="px-3 py-2.5 space-y-2">
                  <div class="flex items-center gap-2">
                    <input bind:value={cl.name} placeholder="Display name"
                      class="{INPUT} font-mono font-semibold flex-1" />
                    <label class="flex items-center gap-1.5 cursor-pointer whitespace-nowrap">
                      <input type="checkbox" bind:checked={cl.enabled} class="accent-green-500" />
                      <span class="text-slate-400">Enabled</span>
                    </label>
                    <label class="flex items-center gap-1.5 cursor-pointer whitespace-nowrap">
                      <input type="radio" name="cfg-master" checked={cl.master}
                        on:change={() => { cfg.clusters = cfg.clusters.map((c, j) => ({ ...c, master: j === i })); }}
                        class="accent-blue-500" />
                      <span class="text-slate-400">Master</span>
                    </label>
                    <button on:click={() => { cfg.clusters = cfg.clusters.filter((_, j) => j !== i); }}
                      disabled={cfg.clusters.length <= 1}
                      class="text-red-400 hover:text-red-300 disabled:opacity-30 disabled:cursor-not-allowed transition-colors text-xs px-1">
                      ✕
                    </button>
                  </div>
                  <div class="grid grid-cols-2 gap-2">
                    <label class="flex flex-col gap-0.5">
                      <span class="text-slate-600">Server <span class={RESTART_BADGE}>⚠ restart</span></span>
                      <input bind:value={cl.server} class="{INPUT} font-mono text-[10px]" />
                    </label>
                    <label class="flex flex-col gap-0.5">
                      <span class="text-slate-600">Port <span class={RESTART_BADGE}>⚠ restart</span></span>
                      <input bind:value={cl.port} class="{INPUT} font-mono text-[10px]" />
                    </label>
                    <label class="flex flex-col gap-0.5">
                      <span class="text-slate-600">Login <span class={RESTART_BADGE}>⚠ restart</span></span>
                      <input bind:value={cl.login} class="{INPUT} font-mono text-[10px]" />
                    </label>
                    <label class="flex flex-col gap-0.5">
                      <span class="text-slate-600">Password</span>
                      <input bind:value={cl.password} type="password" class="{INPUT} font-mono text-[10px]" autocomplete="new-password" />
                    </label>
                    <label class="flex flex-col gap-0.5 col-span-2">
                      <span class="text-slate-600">Filter / Command</span>
                      <input bind:value={cl.command} class="{INPUT} font-mono text-[10px]" />
                    </label>
                  </div>
                  <div class="flex gap-3 pt-0.5">
                    {#each [
                      { key: 'skimmer', label: 'Skimmer' },
                      { key: 'ft8',     label: 'FT8' },
                      { key: 'ft4',     label: 'FT4' },
                      { key: 'beacon',  label: 'Beacon' },
                    ] as f}
                      <label class="flex items-center gap-1 cursor-pointer">
                        <input type="checkbox" bind:checked={cl[f.key]} class="accent-cyan-500" />
                        <span class="text-slate-500">{f.label}</span>
                      </label>
                    {/each}
                  </div>
                  <div class="flex items-center gap-3 pt-1 border-t border-slate-700/30">
                    <button on:click={() => testCluster(i)} disabled={clusterTesting[i] || !cl.server}
                      class="px-3 py-1 rounded text-xs font-semibold border transition-colors
                        {clusterTesting[i] ? 'bg-slate-700 text-slate-400 border-slate-600 cursor-wait'
                                           : 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40 hover:bg-cyan-500/35 disabled:opacity-40'}">
                      {clusterTesting[i] ? 'Testing…' : 'Test Connection'}
                    </button>
                    {#if clusterTestResult[i]}
                      <span class="text-xs font-semibold {clusterTestResult[i].ok ? 'text-green-400' : 'text-red-400'}">
                        {clusterTestResult[i].ok ? '✓' : '✗'} {clusterTestResult[i].msg}
                      </span>
                    {/if}
                  </div>
                </div>
              {/each}
              <!-- Add cluster -->
              <div class="px-3 py-2">
                <button on:click={() => {
                    cfg.clusters = [...cfg.clusters, {
                      name: '', server: '', port: '7300', login: cfg?.general?.callsign ?? '',
                      password: '', enabled: true, master: false,
                      skimmer: false, ft8: false, ft4: false, beacon: false,
                      command: '', loginPrompt: 'login:', clusterType: ''
                    }];
                  }}
                  class="flex items-center gap-1.5 text-xs text-blue-400 hover:text-blue-300 transition-colors">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                  </svg>
                  Add cluster
                </button>
              </div>
            </div>
          {/if}
        </div>

      {/if}
    </div>

    <!-- Footer -->
    <div class="flex items-center justify-between px-4 py-3 border-t border-slate-700/60 flex-shrink-0 bg-slate-900/60">
      <span class="text-[10px] text-slate-600">Written to config.yml — fields marked <span class="text-amber-400/80">⚠ restart</span> require a restart</span>
      <div class="flex items-center gap-2">
        {#if saveError}
          <span class="text-xs text-red-400">{saveError}</span>
        {/if}
        <button on:click={() => dispatch('close')}
          class="px-3 py-1 rounded text-xs text-slate-400 border border-slate-600 hover:border-slate-500 transition-colors">
          Fermer
        </button>
        <button on:click={save} disabled={saving || !cfg}
          class="px-4 py-1 rounded text-xs font-semibold transition-colors border
            {saveOk ? 'bg-green-500/20 text-green-400 border-green-500/50'
                    : 'bg-blue-500/20 text-blue-300 border-blue-500/40 hover:bg-blue-500/35 disabled:opacity-40'}">
          {saving ? 'Saving…' : saveOk ? '✓ Saved' : 'Enregistrer'}
        </button>
      </div>
    </div>

  </div>
</div>


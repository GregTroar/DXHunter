<script>
  let step = 1;
  const totalSteps = 4;

  let callsign = '';
  let grid = '';
  let flexIP = '';
  let sqlitePath = '';

  let clusters = [
    { name: 'F4BPO Cluster', server: 'cluster.f4bpo.com', port: '7300', enabled: true, master: true },
    { name: 'POTA Cluster', server: 'pota-cluster.iz2lsc.eu', port: '7373', enabled: true, master: false },
  ];

  let saving = false;
  let saveError = '';
  let starting = false;

  function nextStep() {
    if (step < totalSteps) step++;
  }

  function prevStep() {
    if (step > 1) step--;
  }

  function addCluster() {
    clusters = [...clusters, { name: '', server: '', port: '7300', enabled: true, master: false }];
  }

  function removeCluster(i) {
    clusters = clusters.filter((_, idx) => idx !== i);
  }

  function setMaster(i) {
    clusters = clusters.map((c, idx) => ({ ...c, master: idx === i }));
  }

  async function submit() {
    saving = true;
    saveError = '';
    try {
      const resp = await fetch('/api/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          callsign: callsign.toUpperCase().trim(),
          grid: grid.toUpperCase().trim(),
          flexIP: flexIP.trim(),
          sqlitePath: sqlitePath.trim(),
          clusters: clusters
            .filter(c => c.server.trim() !== '')
            .map(c => ({ ...c, server: c.server.trim(), port: c.port.trim() || '7300' })),
        }),
      });
      const data = await resp.json();
      if (!data.success) {
        saveError = data.error || 'Failed to save configuration';
        saving = false;
        return;
      }
      starting = true;
      pollUntilReady();
    } catch (e) {
      saveError = e.message;
      saving = false;
    }
  }

  async function pollUntilReady() {
    for (let i = 0; i < 90; i++) {
      await new Promise(r => setTimeout(r, 1000));
      try {
        const resp = await fetch('/api/setup-required');
        const data = await resp.json();
        if (!data.required) {
          window.location.reload();
          return;
        }
      } catch (_) {
        // Server restarting — keep polling
      }
    }
    saveError = 'Services took too long to start. Please restart the application manually.';
    starting = false;
    saving = false;
  }
</script>

<!-- Full-screen overlay -->
<div class="fixed inset-0 z-50 bg-slate-900/95 backdrop-blur-sm flex items-center justify-center p-4">
  <div class="bg-slate-800 border border-slate-700 rounded-xl shadow-2xl w-full max-w-lg">

    <!-- Header -->
    <div class="p-6 border-b border-slate-700">
      <div class="flex items-center gap-3 mb-1">
        <svg class="w-7 h-7 text-blue-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M9.348 14.651a3.75 3.75 0 010-5.303m5.304 0a3.75 3.75 0 010 5.303m-7.425 2.122a6.75 6.75 0 010-9.546m9.546 0a6.75 6.75 0 010 9.546M12 12h.008v.007H12V12z" />
        </svg>
        <h2 class="text-xl font-bold bg-gradient-to-r from-blue-400 to-purple-400 bg-clip-text text-transparent">
          FlexDXCluster — First Run Setup
        </h2>
      </div>
      <p class="text-xs text-slate-400 ml-10">No configuration file found. Let's get you set up.</p>

      <!-- Step indicators -->
      <div class="flex gap-2 mt-4 ml-10">
        {#each Array(totalSteps) as _, i}
          <div class="flex items-center gap-1">
            <div class="w-6 h-6 rounded-full text-xs font-bold flex items-center justify-center transition-colors
              {i + 1 < step ? 'bg-green-500 text-white' : i + 1 === step ? 'bg-blue-500 text-white' : 'bg-slate-700 text-slate-400'}">
              {#if i + 1 < step}
                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
                </svg>
              {:else}
                {i + 1}
              {/if}
            </div>
            {#if i < totalSteps - 1}
              <div class="w-8 h-0.5 {i + 1 < step ? 'bg-green-500' : 'bg-slate-700'}"></div>
            {/if}
          </div>
        {/each}
      </div>
    </div>

    <!-- Body -->
    <div class="p-6 min-h-[260px]">

      {#if starting}
        <div class="flex flex-col items-center justify-center h-48 gap-4">
          <svg class="w-12 h-12 text-blue-400 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/>
          </svg>
          <p class="text-slate-300 font-semibold">Starting services...</p>
          <p class="text-xs text-slate-400">The application will reload automatically.</p>
        </div>

      {:else if step === 1}
        <h3 class="text-sm font-semibold text-slate-200 mb-4">Station Identity</h3>
        <div class="space-y-4">
          <div>
            <label for="setup-callsign" class="block text-xs text-slate-400 mb-1">Your Callsign <span class="text-red-400">*</span></label>
            <input
              id="setup-callsign"
              type="text"
              placeholder="e.g. F4BPO"
              bind:value={callsign}
              on:input={() => callsign = callsign.toUpperCase()}
              class="w-full bg-slate-700 border border-slate-600 rounded px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 uppercase"
            />
          </div>
          <div>
            <label for="setup-grid" class="block text-xs text-slate-400 mb-1">Maidenhead Grid Locator <span class="text-red-400">*</span></label>
            <input
              id="setup-grid"
              type="text"
              placeholder="e.g. JN03"
              bind:value={grid}
              on:input={() => grid = grid.toUpperCase()}
              maxlength="6"
              class="w-full bg-slate-700 border border-slate-600 rounded px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500 uppercase"
            />
            <p class="text-xs text-slate-500 mt-1">Used for greyline and distance calculations.</p>
          </div>
        </div>

      {:else if step === 2}
        <h3 class="text-sm font-semibold text-slate-200 mb-4">FlexRadio Settings</h3>
        <div class="space-y-4">
          <div>
            <label for="setup-flex-ip" class="block text-xs text-slate-400 mb-1">FlexRadio IP Address</label>
            <input
              id="setup-flex-ip"
              type="text"
              placeholder="e.g. 192.168.1.100  (leave empty for auto-discovery)"
              bind:value={flexIP}
              class="w-full bg-slate-700 border border-slate-600 rounded px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500"
            />
            <p class="text-xs text-slate-500 mt-1">
              Leave empty to enable SmartSDR auto-discovery on the local network.
            </p>
          </div>
        </div>

      {:else if step === 3}
        <h3 class="text-sm font-semibold text-slate-200 mb-4">Log4OM Database</h3>
        <div class="space-y-4">
          <div>
            <label for="setup-sqlite" class="block text-xs text-slate-400 mb-1">Log4OM SQLite Database Path</label>
            <input
              id="setup-sqlite"
              type="text"
              placeholder="e.g. C:\Users\You\Documents\Log4OM2\Log4OM2.sqlite"
              bind:value={sqlitePath}
              class="w-full bg-slate-700 border border-slate-600 rounded px-3 py-2 text-sm text-white placeholder-slate-500 focus:outline-none focus:border-blue-500"
            />
            <p class="text-xs text-slate-500 mt-1">
              Leave empty if you don't use Log4OM. Can be configured later in Settings.
            </p>
          </div>
        </div>

      {:else if step === 4}
        <h3 class="text-sm font-semibold text-slate-200 mb-3">DX Clusters</h3>
        <div class="space-y-2 max-h-48 overflow-y-auto pr-1">
          {#each clusters as cl, i}
            <div class="bg-slate-700/60 rounded-lg p-3 border border-slate-600/50">
              <div class="grid grid-cols-[1fr_80px] gap-2 mb-2">
                <input
                  type="text"
                  placeholder="Server hostname"
                  bind:value={cl.server}
                  class="bg-slate-600 border border-slate-500 rounded px-2 py-1 text-xs text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
                />
                <input
                  type="text"
                  placeholder="Port"
                  bind:value={cl.port}
                  class="bg-slate-600 border border-slate-500 rounded px-2 py-1 text-xs text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
                />
              </div>
              <input
                type="text"
                placeholder="Display name (optional)"
                bind:value={cl.name}
                class="w-full bg-slate-600 border border-slate-500 rounded px-2 py-1 text-xs text-white placeholder-slate-400 focus:outline-none focus:border-blue-500 mb-2"
              />
              <div class="flex items-center justify-between">
                <div class="flex gap-3">
                  <label class="flex items-center gap-1 text-xs text-slate-400 cursor-pointer">
                    <input type="checkbox" bind:checked={cl.enabled} class="accent-blue-500" />
                    Enabled
                  </label>
                  <label class="flex items-center gap-1 text-xs text-amber-400 cursor-pointer">
                    <input type="radio" name="master" checked={cl.master} on:change={() => setMaster(i)} class="accent-amber-500" />
                    Master
                  </label>
                </div>
                <button
                  on:click={() => removeCluster(i)}
                  class="text-xs text-red-400 hover:text-red-300 transition-colors"
                  disabled={clusters.length === 1}>
                  Remove
                </button>
              </div>
            </div>
          {/each}
        </div>
        <button
          on:click={addCluster}
          class="mt-2 text-xs text-blue-400 hover:text-blue-300 transition-colors flex items-center gap-1">
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          Add cluster
        </button>
      {/if}

      {#if saveError}
        <div class="mt-4 p-3 bg-red-500/20 border border-red-500/50 rounded text-xs text-red-400">
          {saveError}
        </div>
      {/if}
    </div>

    <!-- Footer -->
    <div class="px-6 pb-6 flex items-center justify-between">
      <button
        on:click={prevStep}
        disabled={step === 1 || starting}
        class="px-4 py-2 text-xs bg-slate-700 hover:bg-slate-600 border border-slate-600 rounded transition-colors disabled:opacity-40 disabled:cursor-not-allowed">
        Back
      </button>

      <span class="text-xs text-slate-500">Step {step} of {totalSteps}</span>

      {#if step < totalSteps}
        <button
          on:click={nextStep}
          disabled={step === 1 ? (callsign.trim().length < 3 || grid.trim().length < 4) : false}
          class="px-4 py-2 text-xs bg-blue-600 hover:bg-blue-500 rounded transition-colors disabled:opacity-40 disabled:cursor-not-allowed font-semibold">
          Next
        </button>
      {:else}
        <button
          on:click={submit}
          disabled={saving || !clusters.some(c => c.enabled && c.server.trim() !== '')}
          class="px-4 py-2 text-xs bg-green-600 hover:bg-green-500 rounded transition-colors disabled:opacity-40 disabled:cursor-not-allowed font-semibold flex items-center gap-1.5">
          {#if saving}
            <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/>
            </svg>
          {/if}
          Save & Start
        </button>
      {/if}
    </div>

  </div>
</div>

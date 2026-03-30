<script>
  export let selectedCallsign = '';

  const BANDS = ['160M', '80M', '60M', '40M', '30M', '20M', '17M', '15M', '12M', '10M', '6M'];

  const MODE_COLS = [
    { label: 'Phone', key: 'phone', modes: ['USB', 'LSB', 'SSB', 'AM', 'FM', 'PHONE'] },
    { label: 'CW',    key: 'cw',    modes: ['CW'] },
    { label: 'FT8',   key: 'ft8',   modes: ['FT8'] },
    { label: 'FT4',   key: 'ft4',   modes: ['FT4'] },
    { label: 'RTTY',  key: 'rtty',  modes: ['RTTY', 'PSK', 'PSK31', 'PSK63', 'FSK', 'DIGI'] },
  ];

  let info = null;
  let recentSpots = [];
  let loading = false;
  let error = '';
  let inputCallsign = '';

  // Grid: band -> colKey -> count
  let grid = {};

  function buildGrid(bandModes) {
    const g = {};
    for (const band of BANDS) {
      g[band] = {};
      for (const col of MODE_COLS) {
        g[band][col.key] = 0;
      }
    }
    for (const bm of bandModes || []) {
      const bandUp = bm.band?.toUpperCase();
      const modeUp = bm.mode?.toUpperCase();
      if (!g[bandUp]) continue;
      for (const col of MODE_COLS) {
        if (col.modes.includes(modeUp)) {
          g[bandUp][col.key] += bm.count;
          break;
        }
      }
    }
    return g;
  }

  async function fetchInfo(call) {
    if (!call) return;
    loading = true;
    error = '';
    info = null;
    try {
      const [bmRes, spotsRes] = await Promise.all([
        fetch(`/api/callsign/band-modes?call=${encodeURIComponent(call.toUpperCase())}`),
        fetch(`/api/callsign/spots?call=${encodeURIComponent(call.toUpperCase())}`),
      ]);
      const bmJson = await bmRes.json();
      const spotsJson = await spotsRes.json();
      if (bmJson.success) {
        info = bmJson.data;
        grid = buildGrid(info.bandModes);
      } else {
        error = bmJson.error || 'Error fetching data';
      }
      recentSpots = spotsJson.success ? (spotsJson.data || []) : [];
    } catch (e) {
      error = `Connection error: ${e.message}`;
    } finally {
      loading = false;
    }
  }

  // Auto-fetch when selectedCallsign changes (set from spot click)
  $: if (selectedCallsign && selectedCallsign !== inputCallsign) {
    inputCallsign = selectedCallsign;
    fetchInfo(selectedCallsign);
  }

  function handleSearch() {
    const call = inputCallsign.trim().toUpperCase();
    if (call) fetchInfo(call);
  }

  function handleKey(e) {
    if (e.key === 'Enter') handleSearch();
  }

  function colTotal(colKey) {
    return BANDS.reduce((sum, b) => sum + (grid[b]?.[colKey] ?? 0), 0);
  }

  function formatDate(d) {
    if (!d) return '—';
    return d.slice(0, 10);
  }
</script>

<div class="flex flex-col h-full p-3 gap-3 overflow-y-auto">

  <!-- Search bar -->
  <div class="flex gap-2">
    <input
      type="text"
      bind:value={inputCallsign}
      on:keydown={handleKey}
      placeholder="Enter callsign (e.g. T31TTT)"
      class="flex-1 bg-slate-800 border border-slate-600 rounded px-3 py-1.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500"
    />
    <button
      on:click={handleSearch}
      class="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded font-semibold transition-colors"
    >
      Search
    </button>
  </div>

  {#if loading}
    <div class="text-slate-400 text-sm text-center py-4">Loading…</div>

  {:else if error}
    <div class="text-red-400 text-sm text-center py-4">{error}</div>

  {:else if info}
    <!-- Header info -->
    <div class="bg-slate-800/60 rounded-lg border border-slate-700/50 p-3 flex flex-wrap gap-4 items-center">
      <div>
        <span class="text-xl font-bold text-white">{info.callsign}</span>
        {#if info.country}
          <span class="ml-2 text-slate-400 text-sm">{info.country}</span>
        {/if}
        {#if info.dxcc}
          <span class="ml-1 text-xs text-slate-500">({info.dxcc})</span>
        {/if}
      </div>
      <div class="flex gap-4 text-sm ml-auto">
        <div class="text-center">
          <div class="text-blue-400 font-bold text-lg">{info.totalQSOs}</div>
          <div class="text-slate-500 text-xs">Total QSOs</div>
        </div>
        {#if info.firstQSO}
          <div class="text-center">
            <div class="text-slate-300 font-semibold">{formatDate(info.firstQSO)}</div>
            <div class="text-slate-500 text-xs">First QSO</div>
          </div>
        {/if}
        {#if info.lastQSO}
          <div class="text-center">
            <div class="text-slate-300 font-semibold">{formatDate(info.lastQSO)}</div>
            <div class="text-slate-500 text-xs">Last QSO</div>
          </div>
        {/if}
      </div>
    </div>

    <!-- Band/Mode grid -->
    <div class="bg-slate-800/60 rounded-lg border border-slate-700/50 overflow-hidden">
      <table class="w-full text-xs">
        <thead>
          <tr class="bg-slate-900/60 border-b border-slate-700/50">
            <th class="py-2 px-3 text-left text-slate-400 font-semibold w-14">Band</th>
            {#each MODE_COLS as col}
              <th class="py-2 px-2 text-center text-slate-300 font-bold">{col.label}</th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each BANDS as band}
            {@const hasAny = MODE_COLS.some(c => (grid[band]?.[c.key] ?? 0) > 0)}
            <tr class="border-b border-slate-700/30 {hasAny ? '' : 'opacity-40'}">
              <td class="py-1.5 px-3 font-semibold text-slate-300">{band}</td>
              {#each MODE_COLS as col}
                {@const count = grid[band]?.[col.key] ?? 0}
                <td class="py-1.5 px-2 text-center">
                  {#if count > 0}
                    <div class="inline-flex items-center justify-center rounded font-bold
                      {col.key === 'phone' ? 'bg-blue-500/30 text-blue-300' :
                       col.key === 'cw'    ? 'bg-orange-500/30 text-orange-300' :
                       col.key === 'ft8'   ? 'bg-purple-500/30 text-purple-300' :
                       col.key === 'ft4'   ? 'bg-pink-500/30 text-pink-300' :
                                             'bg-yellow-500/30 text-yellow-300'}
                      min-w-[2rem] px-1.5 py-0.5 text-xs">
                      {count}
                    </div>
                  {:else}
                    <span class="text-slate-700">·</span>
                  {/if}
                </td>
              {/each}
            </tr>
          {/each}
        </tbody>
        <tfoot>
          <tr class="bg-slate-900/40 border-t border-slate-600/50">
            <td class="py-1.5 px-3 text-slate-400 font-semibold text-xs">Total</td>
            {#each MODE_COLS as col}
              {@const t = colTotal(col.key)}
              <td class="py-1.5 px-2 text-center">
                {#if t > 0}
                  <span class="text-slate-200 font-bold">{t}</span>
                {:else}
                  <span class="text-slate-600">0</span>
                {/if}
              </td>
            {/each}
          </tr>
        </tfoot>
      </table>
    </div>

    <!-- Derniers spots -->
    {#if recentSpots.length > 0}
      <div class="bg-slate-800/60 rounded-lg border border-slate-700/50 overflow-hidden">
        <div class="px-3 py-2 border-b border-slate-700/50 bg-slate-900/40">
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wide">Last {recentSpots.length} spots</span>
        </div>
        <table class="w-full text-xs">
          <thead>
            <tr class="border-b border-slate-700/30 text-slate-500">
              <th class="py-1.5 px-3 text-left">Time</th>
              <th class="py-1.5 px-2 text-left">Freq</th>
              <th class="py-1.5 px-2 text-left">Band</th>
              <th class="py-1.5 px-2 text-left">Mode</th>
              <th class="py-1.5 px-2 text-left">Spotter</th>
              <th class="py-1.5 px-2 text-left">Comment</th>
              <th class="py-1.5 px-2 text-left">Cluster</th>
            </tr>
          </thead>
          <tbody>
            {#each recentSpots as s}
              <tr class="border-b border-slate-700/20 hover:bg-slate-700/20">
                <td class="py-1.5 px-3 text-slate-400 font-mono">{s.UTCTime}</td>
                <td class="py-1.5 px-2 font-mono text-slate-200">{parseFloat(s.FrequencyMhz).toFixed(3)}</td>
                <td class="py-1.5 px-2">
                  <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-slate-300">{s.Band}</span>
                </td>
                <td class="py-1.5 px-2">
                  <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded">{s.Mode}</span>
                </td>
                <td class="py-1.5 px-2 text-slate-400">{s.SpotterCallsign}</td>
                <td class="py-1.5 px-2 text-slate-500 truncate max-w-[120px]" title={s.OriginalComment}>{s.OriginalComment || '—'}</td>
                <td class="py-1.5 px-2 text-slate-600 text-xs">{s.ClusterName || '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  {:else}
    <div class="flex flex-col items-center justify-center py-12 text-slate-500 gap-2">
      <svg class="w-10 h-10 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
          d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
      <p class="text-sm">Click a spot or enter a callsign to see band/mode stats</p>
    </div>
  {/if}
</div>

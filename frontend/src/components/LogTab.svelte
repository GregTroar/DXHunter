<script>
  export let recentQSOs;
  export let logStats;
  export let dxccProgress;
  export let huntStatus = [];

  let view = 'qsos'; // 'qsos' | 'hunt'
  let huntSearch = '';
  let huntSort = 'country'; // 'country' | 'needs'

  // Standard HF/VHF band order for grid columns
  const BAND_ORDER = ['160M','80M','60M','40M','30M','20M','17M','15M','12M','10M','6M','2M'];

  // Derive the bands actually present in the data, sorted by BAND_ORDER
  $: activeBands = (() => {
    const seen = new Set();
    for (const e of huntStatus) {
      for (const b of e.bands) seen.add(b.band);
    }
    const ordered = BAND_ORDER.filter(b => seen.has(b));
    // Append any unknown bands at the end
    for (const b of seen) {
      if (!BAND_ORDER.includes(b)) ordered.push(b);
    }
    return ordered;
  })();

  // Build lookup: dxcc → band → modes[]
  $: lookup = (() => {
    const m = {};
    for (const e of huntStatus) {
      m[e.dxcc] = {};
      for (const b of e.bands) m[e.dxcc][b.band] = b.modes;
    }
    return m;
  })();

  $: filtered = (() => {
    let list = huntStatus;
    if (huntSearch.trim()) {
      const q = huntSearch.trim().toLowerCase();
      list = list.filter(e => e.country.toLowerCase().includes(q) || e.dxcc.toLowerCase().includes(q));
    }
    if (huntSort === 'needs') {
      list = [...list].sort((a, b) => a.bands.length - b.bands.length);
    }
    return list;
  })();

  // Mode → colour class
  function modeColor(mode) {
    if (mode === 'FT8')  return 'bg-purple-500/80 text-white';
    if (mode === 'FT4')  return 'bg-pink-500/80 text-white';
    if (mode === 'CW')   return 'bg-blue-500/80 text-white';
    if (mode === 'SSB')  return 'bg-green-500/80 text-white';
    if (mode === 'RTTY') return 'bg-orange-500/80 text-white';
    return 'bg-slate-500/80 text-white';
  }

  $: totalSlots = huntStatus.reduce((acc, e) => acc + e.bands.length, 0);
</script>

<div class="h-full flex flex-col overflow-hidden">

  <!-- ── Header + stats ──────────────────────────────────────────────────────── -->
  <div class="p-3 border-b border-slate-700/50 flex-shrink-0">
    <div class="flex items-center justify-between mb-3">
      <h2 class="text-lg font-bold">Station Log</h2>
      <!-- View switcher -->
      <div class="flex rounded-lg overflow-hidden border border-slate-700/50 text-xs">
        <button
          class="px-3 py-1.5 transition-colors {view === 'qsos' ? 'bg-blue-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
          on:click={() => view = 'qsos'}>QSOs</button>
        <button
          class="px-3 py-1.5 transition-colors {view === 'hunt' ? 'bg-blue-600 text-white' : 'text-slate-400 hover:text-slate-200'}"
          on:click={() => view = 'hunt'}>🏆 Hunt</button>
      </div>
    </div>

    <div class="grid grid-cols-4 gap-2 mb-3">
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">Today</div>
        <div class="text-xl font-bold text-blue-400">{logStats.today || 0}</div>
      </div>
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">This Week</div>
        <div class="text-xl font-bold text-green-400">{logStats.thisWeek || 0}</div>
      </div>
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">This Month</div>
        <div class="text-xl font-bold text-purple-400">{logStats.thisMonth || 0}</div>
      </div>
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">Total</div>
        <div class="text-xl font-bold text-orange-400">{logStats.total || 0}</div>
      </div>
    </div>

    <!-- DXCC Progress Bar -->
    <div class="bg-slate-900/50 rounded p-3">
      <div class="flex items-center justify-between mb-2">
        <span class="text-sm font-semibold text-slate-300">DXCC Progress</span>
        <span class="text-sm font-bold text-green-400">{dxccProgress.worked || 0} / {dxccProgress.total || 340}</span>
      </div>
      <div class="w-full bg-slate-700/30 rounded-full h-3 overflow-hidden">
        <div
          class="h-full rounded-full bg-gradient-to-r from-green-500 to-emerald-500 transition-all duration-500"
          style="width: {dxccProgress.percentage || 0}%">
        </div>
      </div>
      <div class="text-xs text-slate-400 text-right mt-1">{(dxccProgress.percentage || 0).toFixed(1)}% Complete</div>
    </div>
  </div>

  <!-- ── QSOs view ────────────────────────────────────────────────────────────── -->
  {#if view === 'qsos'}
  <div class="flex-1 overflow-y-auto">
    <div class="p-3">
      <h3 class="text-sm font-bold text-slate-400 mb-2">Recent QSOs</h3>

      {#if recentQSOs.length === 0}
        <div class="text-center py-8 text-slate-400">
          <svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <p class="text-sm">No QSOs in log</p>
        </div>
      {:else}
        <table class="w-full text-xs">
          <thead class="bg-slate-900/50 sticky top-0">
            <tr class="text-left text-xs text-slate-400">
              <th class="p-2 font-semibold">Date</th>
              <th class="p-2 font-semibold">Time</th>
              <th class="p-2 font-semibold">Callsign</th>
              <th class="p-2 font-semibold">Band</th>
              <th class="p-2 font-semibold">Mode</th>
              <th class="p-2 font-semibold">RST S/R</th>
              <th class="p-2 font-semibold">Country</th>
            </tr>
          </thead>
          <tbody>
            {#each recentQSOs as qso}
              {@const date = qso.date ? new Date(qso.date.replace(' ', 'T')) : null}
              {@const qsoDate = date ? date.toISOString().split('T')[0] : 'N/A'}
              {@const qsoTime = date ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false }) : 'N/A'}
              <tr class="border-b border-slate-700/30 hover:bg-slate-700/30 transition-colors">
                <td class="p-2 text-slate-300">{qsoDate}</td>
                <td class="p-2 text-slate-300">{qsoTime}</td>
                <td class="p-2"><span class="font-bold text-blue-400">{qso.callsign}</span></td>
                <td class="p-2"><span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-xs">{qso.band || 'N/A'}</span></td>
                <td class="p-2"><span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs">{qso.mode || 'N/A'}</span></td>
                <td class="p-2 font-mono text-xs text-slate-400">{qso.rstSent || '---'} / {qso.rstRcvd || '---'}</td>
                <td class="p-2 text-slate-400">{qso.country || 'N/A'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </div>

  <!-- ── Hunt view ─────────────────────────────────────────────────────────────── -->
  {:else}
  <div class="flex-1 flex flex-col overflow-hidden">

    <!-- Hunt toolbar -->
    <div class="px-3 pt-2 pb-2 flex items-center gap-2 flex-shrink-0 border-b border-slate-700/30">
      <input
        type="text"
        placeholder="Search country / DXCC…"
        bind:value={huntSearch}
        class="flex-1 bg-slate-900/60 border border-slate-700/50 rounded px-2 py-1 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500/60"
      />
      <select
        bind:value={huntSort}
        class="bg-slate-900/60 border border-slate-700/50 rounded px-2 py-1 text-xs text-slate-300 focus:outline-none">
        <option value="country">A–Z</option>
        <option value="needs">Least worked</option>
      </select>
    </div>

    <!-- Summary row -->
    <div class="px-3 py-1.5 flex-shrink-0 flex items-center gap-3 text-[10px] text-slate-500 border-b border-slate-700/20">
      <span><span class="text-slate-300 font-semibold">{huntStatus.length}</span> DXCCs worked</span>
      <span><span class="text-slate-300 font-semibold">{totalSlots}</span> band slots</span>
      {#if huntSearch}<span class="text-blue-400">{filtered.length} matches</span>{/if}
    </div>

    <!-- Scrollable grid -->
    <div class="flex-1 overflow-auto">
      {#if huntStatus.length === 0}
        <div class="flex items-center justify-center h-32 text-slate-500 text-sm">No log data available</div>
      {:else}
        <table class="w-full text-xs border-collapse" style="min-width: max-content;">
          <!-- Fixed header -->
          <thead class="sticky top-0 z-10 bg-slate-900">
            <tr>
              <th class="p-2 text-left text-slate-400 font-semibold whitespace-nowrap sticky left-0 bg-slate-900 border-b border-slate-700/50" style="min-width:130px;">Country</th>
              {#each activeBands as band}
                <th class="p-2 text-center text-slate-400 font-semibold whitespace-nowrap border-b border-slate-700/50" style="min-width:52px;">{band}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each filtered as entry (entry.dxcc)}
              <tr class="border-b border-slate-700/20 hover:bg-slate-700/20 transition-colors">
                <!-- Country name (sticky left) -->
                <td class="p-2 sticky left-0 bg-slate-800/80 backdrop-blur whitespace-nowrap font-medium text-slate-200" style="min-width:130px;">
                  {entry.country || entry.dxcc}
                  <span class="text-[9px] text-slate-500 ml-1">#{entry.total}</span>
                </td>
                <!-- Band cells -->
                {#each activeBands as band}
                  {@const modes = lookup[entry.dxcc]?.[band]}
                  <td class="p-1 text-center align-middle" style="min-width:52px;">
                    {#if modes && modes.length > 0}
                      <div class="flex flex-wrap gap-0.5 justify-center">
                        {#each modes as mode}
                          <span class="px-1 py-px rounded text-[9px] font-bold leading-none {modeColor(mode)}">{mode}</span>
                        {/each}
                      </div>
                    {:else}
                      <span class="text-slate-700">·</span>
                    {/if}
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </div>
  {/if}

</div>

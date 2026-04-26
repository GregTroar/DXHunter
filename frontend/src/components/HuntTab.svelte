<script>
  export let huntStatus = [];

  let search = '';
  let sortBy = 'country'; // 'country' | 'needs'

  const BAND_ORDER = ['160M','80M','60M','40M','30M','20M','17M','15M','12M','10M','6M','2M'];

  $: activeBands = (() => {
    const seen = new Set();
    for (const e of huntStatus) for (const b of e.bands) seen.add(b.band);
    const ordered = BAND_ORDER.filter(b => seen.has(b));
    for (const b of seen) if (!BAND_ORDER.includes(b)) ordered.push(b);
    return ordered;
  })();

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
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      list = list.filter(e => e.country.toLowerCase().includes(q) || e.dxcc.toLowerCase().includes(q));
    }
    if (sortBy === 'needs') list = [...list].sort((a, b) => a.bands.length - b.bands.length);
    return list;
  })();

  $: totalSlots = huntStatus.reduce((acc, e) => acc + e.bands.length, 0);

  function modeColor(mode) {
    if (mode === 'FT8')                               return 'bg-purple-500 text-white';
    if (mode === 'FT4')                               return 'bg-pink-500 text-white';
    if (mode === 'CW')                                return 'bg-blue-500 text-white';
    if (mode === 'SSB' || mode === 'USB' || mode === 'LSB') return 'bg-green-600 text-white';
    if (mode === 'RTTY' || mode === 'DIGI')           return 'bg-orange-500 text-white';
    return 'bg-slate-500 text-white';
  }
</script>

<div class="h-full flex flex-col overflow-hidden">

  <!-- Toolbar -->
  <div class="flex items-center gap-3 px-3 py-2 border-b border-slate-700/50 flex-shrink-0">
    <div class="flex items-center gap-2 text-xs text-slate-400">
      <span class="text-slate-200 font-semibold">{huntStatus.length}</span> DXCCs worked ·
      <span class="text-slate-200 font-semibold">{totalSlots}</span> band slots
    </div>
    <div class="flex-1"></div>
    <input
      type="text"
      placeholder="Search country / DXCC…"
      bind:value={search}
      class="w-52 bg-slate-900/60 border border-slate-700/50 rounded px-2 py-1 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500/60"
    />
    <select
      bind:value={sortBy}
      class="bg-slate-900/60 border border-slate-700/50 rounded px-2 py-1 text-xs text-slate-300 focus:outline-none">
      <option value="country">A–Z</option>
      <option value="needs">Least worked</option>
    </select>
  </div>

  <!-- Grid -->
  <div class="flex-1 overflow-auto">
    {#if huntStatus.length === 0}
      <div class="flex items-center justify-center h-full text-slate-500 text-sm">No log data</div>
    {:else}
      <table class="text-xs border-collapse" style="min-width: max-content; width: 100%;">
        <thead class="sticky top-0 z-10 bg-slate-900">
          <tr>
            <th class="px-3 py-2 text-left text-slate-400 font-semibold whitespace-nowrap sticky left-0 bg-slate-900 border-b border-r border-slate-700/50" style="min-width:180px;">Country</th>
            {#each activeBands as band}
              <th class="px-2 py-2 text-center text-slate-400 font-semibold whitespace-nowrap border-b border-slate-700/50" style="min-width:60px;">{band}</th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each filtered as entry (entry.dxcc)}
            <tr class="border-b border-slate-700/20 hover:bg-slate-700/20 transition-colors">
              <td class="px-3 py-1.5 sticky left-0 bg-slate-800/90 backdrop-blur whitespace-nowrap border-r border-slate-700/30" style="min-width:180px;">
                <span class="font-medium text-slate-200">{entry.country || entry.dxcc}</span>
                <span class="text-[9px] text-slate-500 ml-1.5">({entry.bands.length} band{entry.bands.length > 1 ? 's' : ''})</span>
              </td>
              {#each activeBands as band}
                {@const modes = lookup[entry.dxcc]?.[band]}
                <td class="px-1 py-1 text-center" style="min-width:60px;">
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

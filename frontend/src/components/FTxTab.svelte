<script>
  export let ftxEnabled = false;
  export let ftxDecodes = [];   // maintained by App.svelte — persists across tab switches

  let paused = false;
  let filterCQOnly = false;
  let filterMyCall = false;

  let snapshot = [];
  // Wall-clock ms when Clear was last pressed; hide any decode received before this.
  let clearedAt = 0;

  function togglePause() {
    if (!paused) snapshot = [...ftxDecodes];
    paused = !paused;
  }

  function clearDisplay() {
    clearedAt = Date.now();
    paused = false;
    snapshot = [];
  }

  async function toggleEnabled() {
    await fetch('/api/ftx/toggle', { method: 'POST' });
    // ftxEnabled will update via the stats WS message
  }

  // Grid columns — no Mode column, added LoTW column
  const cols = '50px 34px 36px 52px 48px minmax(0,1fr) 140px 130px 46px';

  $: visibleDecodes = clearedAt > 0
    ? ftxDecodes.filter(d => d.receivedAt > clearedAt)
    : ftxDecodes;

  $: displayed = (paused ? snapshot : visibleDecodes).filter(d => {
    if (filterCQOnly && !d.isCQ) return false;
    if (filterMyCall && !d.myCall) return false;
    return true;
  });

  // Group displayed decodes by period (same time = same period), newest group first
  $: groupedDecodes = (() => {
    const groups = [];
    let current = null;
    for (const d of displayed) {
      if (!current || d.time !== current.time) {
        current = { time: d.time, decodes: [] };
        groups.push(current);
      }
      current.decodes.push(d);
    }
    return groups;
  })();

  // Session total (since last clear, ignoring CQ/MyCall filters)
  $: sessionCount = visibleDecodes.length;

  // Count of decodes in the most recent period
  $: lastPeriodCount = groupedDecodes.length > 0 ? groupedDecodes[0].decodes.length : 0;

  async function sendReply(decode) {
    try {
      const res = await fetch('/api/ftx/reply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decode, clientId: 'MSHV' })
      });
      const data = await res.json();
      if (!data.success) console.error('FTx reply failed:', data.message);
    } catch (err) {
      console.error('FTx reply error:', err);
    }
  }

  function rowBg(decode) {
    if (decode.myCall)  return 'bg-cyan-500/25 border-l-2 border-cyan-400';
    if (decode.newDXCC) return 'bg-green-500/15 border-l-2 border-green-500';
    if (decode.newBand || decode.newMode || decode.newSlot)
                        return 'bg-yellow-500/10 border-l-2 border-yellow-500';
    return 'hover:bg-slate-700/40';
  }

  function snrClass(snr) {
    if (snr >= 0)   return 'text-green-400 font-semibold';
    if (snr >= -10) return 'text-yellow-400';
    return 'text-slate-400';
  }

  function formatTime(t) {
    if (t.length !== 6) return t;
    return `${t.slice(0,2)}:${t.slice(2,4)}:${t.slice(4,6)}`;
  }
</script>

<div class="flex flex-col h-full overflow-hidden text-xs">
  <!-- Toolbar -->
  <div class="flex items-center gap-2 px-2 py-1.5 bg-slate-900/50 border-b border-slate-700/50 flex-shrink-0 flex-wrap">

    <!-- Enable toggle -->
    <button
      on:click={toggleEnabled}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors {ftxEnabled ? 'bg-green-500/20 text-green-400 border border-green-500/40' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:border-slate-500'}">
      {ftxEnabled ? '● ON' : '○ OFF'}
    </button>

    <span class="text-slate-500">|</span>

    <label class="flex items-center gap-1 cursor-pointer select-none text-slate-300">
      <input type="checkbox" bind:checked={filterCQOnly} class="accent-blue-500" /> CQ
    </label>

    <label class="flex items-center gap-1 cursor-pointer select-none text-slate-300">
      <input type="checkbox" bind:checked={filterMyCall} class="accent-cyan-500" /> My Call
    </label>

    <button
      on:click={togglePause}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors {paused ? 'bg-orange-500/30 text-orange-300 border border-orange-500/50' : 'bg-slate-700 text-slate-300 border border-slate-600 hover:border-slate-500'}">
      {paused ? '▶ Resume' : '⏸ Pause'}
    </button>

    <button
      on:click={clearDisplay}
      class="px-2 py-0.5 rounded text-xs font-semibold bg-slate-700 text-slate-300 border border-slate-600 hover:border-red-500/50 hover:text-red-400 transition-colors">
      Clear
    </button>

    <span class="ml-auto text-slate-500">
      last: <span class="text-slate-400">{lastPeriodCount}</span>
      <span class="mx-1">·</span>
      session: <span class="text-slate-400">{sessionCount}</span>
    </span>
  </div>

  {#if !ftxEnabled}
    <div class="flex-1 flex items-center justify-center text-slate-500 text-sm">
      FTx monitoring is disabled
    </div>
  {:else}
    <!-- Header -->
    <div class="grid text-slate-500 font-semibold uppercase tracking-wide bg-slate-900/50 border-b border-slate-700/50 flex-shrink-0"
         style="grid-template-columns: {cols};">
      <div class="px-1 py-1 text-center">Time</div>
      <div class="px-1 py-1 text-center">SNR</div>
      <div class="px-1 py-1 text-center">DT</div>
      <div class="px-1 py-1 text-center">Freq</div>
      <div class="px-1 py-1 text-center">Band</div>
      <div class="px-1 py-1 text-center">Message</div>
      <div class="px-1 py-1 text-center">Country</div>
      <div class="px-1 py-1 text-center">Status</div>
      <div class="px-1 py-1 text-center">LoTW</div>
    </div>

    <!-- Rows grouped by period -->
    <div class="flex-1 overflow-y-auto">
      {#if displayed.length === 0}
        <div class="flex items-center justify-center h-20 text-slate-600">
          Waiting for FT8/FT4/FT2 decodes…
        </div>
      {/if}

      {#each groupedDecodes as group, gi (group.time)}
        <!-- Period separator row (not before the first/newest group) -->
        {#if gi > 0}
          <div class="flex items-center gap-2 px-2 py-0.5 bg-slate-950 border-t border-b border-slate-700/60 text-[10px] font-mono select-none">
            <span class="text-slate-400 font-semibold">{formatTime(group.time)}</span>
            <span class="flex-1 border-t border-slate-700/50 mx-1"></span>
            <span class="text-slate-600">{group.decodes.length} decodes</span>
          </div>
        {/if}

        {#each group.decodes as decode (decode.time + decode.message + decode.df)}
          <div
            class="grid items-center cursor-pointer transition-colors hover:brightness-125 {rowBg(decode)}"
            style="grid-template-columns: {cols};"
            role="button"
            tabindex="0"
            on:click={() => sendReply(decode)}
            on:keydown={(e) => e.key === 'Enter' && sendReply(decode)}
            title="Click to reply — {decode.dxCall || decode.message}">

            <div class="px-1 py-0.5 font-mono text-slate-400 text-center">{decode.time}</div>

            <div class="px-1 py-0.5 text-center font-mono {snrClass(decode.snr)}">
              {decode.snr > 0 ? '+' : ''}{decode.snr}
            </div>

            <div class="px-1 py-0.5 text-center font-mono text-slate-400">
              {decode.dt?.toFixed(1)}
            </div>

            <div class="px-1 py-0.5 text-center font-mono text-slate-300">
              {decode.df}
            </div>

            <div class="px-1 py-0.5 text-center">
              {#if decode.band}
                <span class="px-1 rounded bg-blue-500/20 text-blue-300">{decode.band}</span>
              {/if}
            </div>

            <div class="px-1 py-0.5 font-mono truncate" title={decode.message}>
              {#if decode.myCall}
                <span class="text-cyan-300 font-bold">{decode.message}</span>
              {:else if decode.isCQ}
                <span class="text-green-300">{decode.message}</span>
              {:else}
                <span class="text-slate-200">{decode.message}</span>
              {/if}
            </div>

            <div class="pl-3 pr-1 py-0.5 text-slate-400 truncate" title={decode.countryName}>
              {decode.countryName || ''}
            </div>

            <div class="px-1 py-0.5 flex gap-1 items-center justify-center flex-wrap">
              {#if decode.newDXCC}
                <span class="px-1.5 py-0.5 rounded text-[11px] font-semibold bg-green-500/30 text-green-300 border border-green-500/50 whitespace-nowrap">DXCC</span>
              {/if}
              {#if decode.newBand && !decode.newDXCC}
                <span class="px-1.5 py-0.5 rounded text-[11px] font-semibold bg-yellow-500/30 text-yellow-300 border border-yellow-500/50 whitespace-nowrap">Band</span>
              {/if}
              {#if decode.newMode && !decode.newDXCC}
                <span class="px-1.5 py-0.5 rounded text-[11px] font-semibold bg-orange-500/30 text-orange-300 border border-orange-500/50 whitespace-nowrap">Mode</span>
              {/if}
              {#if decode.newSlot && !decode.newDXCC}
                <span class="px-1.5 py-0.5 rounded text-[11px] font-semibold bg-cyan-500/30 text-cyan-300 border border-cyan-500/50 whitespace-nowrap">Slot</span>
              {/if}
              {#if decode.worked && !decode.newDXCC && !decode.newBand && !decode.newMode && !decode.newSlot}
                <span class="px-1.5 py-0.5 rounded text-[11px] bg-slate-500/30 text-slate-400 border border-slate-500/40 whitespace-nowrap">Wkd</span>
              {/if}
              {#if decode.lowConfidence}
                <span class="px-1.5 py-0.5 rounded text-[11px] bg-red-500/20 text-red-400 border border-red-500/30">?</span>
              {/if}
            </div>

            <!-- LoTW -->
            <div class="px-1 py-0.5 text-center">
              {#if decode.lotwUser}
                <span class="px-1.5 py-0.5 rounded text-[11px] font-semibold bg-blue-500/30 text-blue-300 border border-blue-500/50">LoTW</span>
              {/if}
            </div>
          </div>
        {/each}
      {/each}
    </div>
  {/if}
</div>

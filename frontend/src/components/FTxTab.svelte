<script>
  export let ftxEnabled = false;
  export let ftxDecodes = [];   // maintained by App.svelte — persists across tab switches

  let paused = false;
  let filterCQOnly = false;
  let filterMyCall = false;

  let snapshot = [];
  // How many entries were in ftxDecodes when Clear was last pressed.
  // We slice off that many from the end (oldest entries) to hide pre-clear decodes.
  let clearedCount = 0;

  function togglePause() {
    if (!paused) snapshot = [...ftxDecodes];
    paused = !paused;
  }

  function clearDisplay() {
    clearedCount = ftxDecodes.length;
    paused = false;
    snapshot = [];
  }

  // Grid columns — no Mode column
  const cols = '50px 34px 36px 52px 48px minmax(0,1fr) 140px 140px';

  // Direct dependency on ftxDecodes so Svelte tracks prop changes reliably.
  // ftxDecodes is newest-first; slice off the clearedCount oldest entries.
  $: visibleDecodes = clearedCount > 0
    ? ftxDecodes.slice(0, ftxDecodes.length - clearedCount)
    : ftxDecodes;

  $: displayed = (paused ? snapshot : visibleDecodes).filter(d => {
    if (filterCQOnly && !d.isCQ) return false;
    if (filterMyCall && !d.myCall) return false;
    return true;
  });

  // Alternating background per decode period (same time string = same period)
  $: periodParity = (() => {
    const map = {};
    let parity = 0;
    let lastTime = null;
    for (const d of displayed) {
      if (lastTime !== null && d.time !== lastTime) parity = 1 - parity;
      map[d.time + d.message + d.df] = parity;
      lastTime = d.time;
    }
    return map;
  })();

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

  function rowBg(decode, parity) {
    if (decode.myCall)  return 'bg-cyan-500/25 border-l-2 border-cyan-400';
    if (decode.newDXCC) return 'bg-green-500/15 border-l-2 border-green-500';
    if (decode.newBand || decode.newMode || decode.newSlot)
                        return 'bg-yellow-500/10 border-l-2 border-yellow-500';
    return parity === 0 ? 'bg-slate-800/50' : 'bg-slate-900/60';
  }

  function snrClass(snr) {
    if (snr >= 0)   return 'text-green-400 font-semibold';
    if (snr >= -10) return 'text-yellow-400';
    return 'text-slate-400';
  }
</script>

<div class="flex flex-col h-full overflow-hidden text-xs">
  <!-- Toolbar -->
  <div class="flex items-center gap-2 px-2 py-1.5 bg-slate-900/50 border-b border-slate-700/50 flex-shrink-0 flex-wrap">
    <span class="text-slate-400 font-semibold">FT8/FT4</span>
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

    <span class="ml-auto text-slate-500">{displayed.length} decodes</span>
  </div>

  {#if !ftxEnabled}
    <div class="flex-1 flex items-center justify-center text-slate-500 text-sm">
      FTx monitoring is disabled — enable it in config.yml
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
    </div>

    <!-- Rows -->
    <div class="flex-1 overflow-y-auto">
      {#if displayed.length === 0}
        <div class="flex items-center justify-center h-20 text-slate-600">
          Waiting for FT8/FT4 decodes…
        </div>
      {/if}

      {#each displayed as decode (decode.time + decode.message + decode.df)}
        {@const key = decode.time + decode.message + decode.df}
        {@const parity = periodParity[key] ?? 0}
        <div
          class="grid items-center cursor-pointer transition-colors hover:brightness-125 {rowBg(decode, parity)}"
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
        </div>
      {/each}
    </div>
  {/if}
</div>

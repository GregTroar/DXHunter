<script>
  export let callsign = '';   // auto-populated by parent (autoCallTarget)

  let manualSearch = '';
  let activeCall   = '';      // call currently displayed
  let info         = null;
  let loading      = false;
  let error        = '';

  // Auto-lookup when callsign prop changes
  $: if (callsign && callsign !== activeCall) lookup(callsign);

  async function lookup(call) {
    call = call?.trim().toUpperCase();
    if (!call) return;
    activeCall   = call;
    manualSearch = call;
    info         = null;
    error        = '';
    loading      = true;
    try {
      const res  = await fetch(`/api/qrz/${encodeURIComponent(call)}`);
      const data = await res.json();
      if (!res.ok || !data.success) {
        error = data.error || 'Lookup failed';
      } else {
        info = data.data;
        if (info?.error) { error = info.error; info = null; }
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function onSearch(e) {
    if (e.key === 'Enter') lookup(manualSearch);
  }

  function fullName(i) {
    return [i.fname, i.name].filter(Boolean).join(' ') || '—';
  }

  function location(i) {
    return [i.addr2, i.state, i.country].filter(Boolean).join(', ') || i.land || '—';
  }
</script>

<div class="flex flex-col h-full overflow-hidden bg-slate-900/40 border-l border-slate-700/50">

  <!-- Search bar -->
  <div class="flex items-center gap-1.5 px-2 py-1.5 border-b border-slate-700/50 flex-shrink-0">
    <span class="text-slate-500 text-xs font-semibold">QRZ</span>
    <input
      bind:value={manualSearch}
      on:keydown={onSearch}
      placeholder="Callsign…"
      class="flex-1 px-2 py-0.5 text-xs font-mono bg-slate-800 border border-slate-600 rounded text-slate-200 placeholder-slate-600 focus:outline-none focus:border-blue-500/70 uppercase"
      style="text-transform:uppercase"
    />
    <button
      on:click={() => lookup(manualSearch)}
      disabled={loading}
      class="px-2 py-0.5 rounded text-xs font-semibold bg-blue-500/20 text-blue-300 border border-blue-500/40 hover:bg-blue-500/35 transition-colors disabled:opacity-40">
      {loading ? '…' : '⌕'}
    </button>
  </div>

  <!-- Content -->
  <div class="flex-1 overflow-y-auto px-2 py-2 text-xs">

    {#if loading}
      <div class="flex items-center justify-center h-20 text-slate-500">Looking up {activeCall}…</div>

    {:else if error}
      <div class="flex flex-col items-center justify-center gap-1 h-20 text-center">
        <span class="text-red-400 font-semibold">{activeCall}</span>
        <span class="text-slate-500">{error}</span>
      </div>

    {:else if info}
      <!-- Photo -->
      {#if info.image}
        <div class="flex justify-center mb-2">
          <img
            src={info.image}
            alt={info.call}
            class="max-h-28 rounded-lg object-cover border border-slate-700/50"
            on:error={(e) => e.target.style.display='none'}
          />
        </div>
      {/if}

      <!-- Callsign + name -->
      <div class="text-center mb-3">
        <a href="https://www.qrz.com/db/{info.call}" target="_blank" rel="noreferrer"
          class="text-xl font-bold font-mono text-blue-300 hover:text-blue-200 transition-colors">
          {info.call}
        </a>
        <div class="text-slate-300 font-medium mt-0.5">{fullName(info)}</div>
        {#if info.class}
          <span class="inline-block mt-1 px-2 py-0 rounded bg-slate-700/60 text-slate-400 text-[10px] font-semibold">{info.class}</span>
        {/if}
      </div>

      <!-- Fields -->
      <div class="space-y-1">
        {#each [
          { label: 'Location', value: location(info) },
          { label: 'Grid',     value: info.grid },
          { label: 'Country',  value: info.land || info.country },
          { label: 'QSL',      value: info.qslmgr },
          { label: 'Email',    value: info.email },
          { label: 'Born',     value: info.born },
          { label: 'Aliases',  value: info.aliases },
        ] as row}
          {#if row.value}
            <div class="flex gap-1.5">
              <span class="text-slate-500 w-14 flex-shrink-0">{row.label}</span>
              <span class="text-slate-300 break-all">{row.value}</span>
            </div>
          {/if}
        {/each}

        {#if info.url}
          <div class="flex gap-1.5">
            <span class="text-slate-500 w-14 flex-shrink-0">Web</span>
            <a href={info.url} target="_blank" rel="noreferrer"
              class="text-blue-400 hover:text-blue-300 truncate transition-colors">{info.url}</a>
          </div>
        {/if}
      </div>

    {:else if !activeCall}
      <div class="flex items-center justify-center h-20 text-slate-600 text-center">
        Enter a callsign<br/>or start auto call
      </div>
    {/if}
  </div>

  <!-- Footer -->
  <div class="px-2 py-1 border-t border-slate-700/50 flex-shrink-0">
    <a href="https://www.qrz.com" target="_blank" rel="noreferrer"
      class="text-[10px] text-slate-600 hover:text-slate-400 transition-colors">qrz.com</a>
  </div>
</div>

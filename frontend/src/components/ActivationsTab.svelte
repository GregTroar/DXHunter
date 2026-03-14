<script>
  import { createEventDispatcher } from 'svelte';
  export let activations = [];
  export let watchlist = [];

  const dispatch = createEventDispatcher();

  let showActive = true;
  let showUpcoming = true;
  let search = '';

  // Set local mis à jour immédiatement après ajout (sans attendre le WS)
  let localAdded = new Set();

  $: watchlistCallsigns = new Set([
    ...(watchlist || []).map(w => w.callsign?.toUpperCase()),
    ...localAdded
  ]);

  $: filtered = activations.filter(a => {
    if (a.status === 'active' && !showActive) return false;
    if (a.status === 'upcoming' && !showUpcoming) return false;
    if (search) {
      const s = search.toUpperCase();
      return a.callsign?.toUpperCase().includes(s) || a.dxcc?.toUpperCase().includes(s);
    }
    return true;
  }).sort((a, b) => {
    if (a.status === 'active' && b.status !== 'active') return -1;
    if (b.status === 'active' && a.status !== 'active') return 1;
    return a.startDate.localeCompare(b.startDate);
  });

  $: activeCount   = activations.filter(a => a.status === 'active').length;
  $: upcomingCount = activations.filter(a => a.status === 'upcoming').length;

  // Retourne les callsigns pas encore en watchlist
  function getCallsignsToAdd(callsign) {
    return callsign.split(',')
      .map(c => c.trim().toUpperCase())
      .filter(c => c && !watchlistCallsigns.has(c));
  }

  function isInWatchlist(callsign) {
    return callsign.split(',').map(c => c.trim().toUpperCase())
      .every(c => watchlistCallsigns.has(c));
  }

  function isPartiallyInWatchlist(callsign) {
    const calls = callsign.split(',').map(c => c.trim().toUpperCase());
    const inWl = calls.filter(c => watchlistCallsigns.has(c));
    return inWl.length > 0 && inWl.length < calls.length;
  }

  async function addToWatchlist(callsign) {
    const toAdd = getCallsignsToAdd(callsign);
    if (toAdd.length === 0) {
      dispatch('toast', { message: 'Already in watchlist', type: 'warning' });
      return;
    }

    let added = 0;
    for (const call of toAdd) {
      try {
        const response = await fetch('/api/watchlist/add', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ callsign: call })
        });
        const data = await response.json();
        if (response.ok) {
          localAdded.add(call);
          localAdded = localAdded; // force Svelte reactivity
          added++;
        } else {
          dispatch('toast', { message: data.error || `Failed to add ${call}`, type: 'error' });
        }
      } catch (e) {
        dispatch('toast', { message: `Error: ${e.message}`, type: 'error' });
      }
    }

    if (added > 0) {
      const names = toAdd.slice(0, added).join(', ');
      dispatch('toast', { message: `✅ ${names} added to watchlist`, type: 'success' });
    }
  }
</script>

<div class="flex flex-col h-full overflow-hidden text-sm">

  <!-- Toolbar -->
  <div class="flex items-center gap-2 px-3 py-2 border-b border-slate-700/50 flex-shrink-0">
    <button
      on:click={() => showActive = !showActive}
      class="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold transition-colors
        {showActive ? 'bg-green-500/20 text-green-400 border border-green-500/40' : 'bg-slate-700/40 text-slate-500 border border-slate-700'}">
      <span class="w-1.5 h-1.5 rounded-full {showActive ? 'bg-green-400 animate-pulse' : 'bg-slate-600'}"></span>
      Active {activeCount}
    </button>
    <button
      on:click={() => showUpcoming = !showUpcoming}
      class="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold transition-colors
        {showUpcoming ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/40' : 'bg-slate-700/40 text-slate-500 border border-slate-700'}">
      <span class="w-1.5 h-1.5 rounded-full {showUpcoming ? 'bg-yellow-400' : 'bg-slate-600'}"></span>
      Upcoming {upcomingCount}
    </button>
    <input
      bind:value={search}
      placeholder="Search…"
      class="ml-auto w-28 px-2 py-0.5 text-xs bg-slate-900/60 border border-slate-700 rounded text-slate-300 placeholder-slate-600 focus:outline-none focus:border-slate-500"
    />
  </div>

  <!-- List -->
  <div class="flex-1 overflow-y-auto px-2 py-2 space-y-2">
    {#if filtered.length === 0}
      <div class="flex items-center justify-center h-20 text-slate-500 text-xs">No activations</div>
    {:else}
      {#each filtered as a}
        <div class="rounded-lg overflow-hidden border
          {a.status === 'active' ? 'border-green-500/30' : 'border-slate-600/40'}">

          <!-- Header coloré -->
          <div class="flex items-center gap-2 px-3 py-1.5
            {a.status === 'active' ? 'bg-green-500/10' : 'bg-slate-700/30'}">

            <span class="w-2 h-2 rounded-full flex-shrink-0
              {a.status === 'active' ? 'bg-green-400 animate-pulse' : 'bg-yellow-400'}">
            </span>

            <!-- Callsign en gros -->
            {#if a.link}
              <a href={a.link} target="_blank" rel="noreferrer"
                class="font-bold text-base {a.status === 'active' ? 'text-green-300 hover:text-green-200' : 'text-yellow-300 hover:text-yellow-200'} transition-colors leading-none">
                {a.callsign}
              </a>
            {:else}
              <span class="font-bold text-base {a.status === 'active' ? 'text-green-300' : 'text-yellow-300'} leading-none">
                {a.callsign}
              </span>
            {/if}

            <!-- DXCC bien visible -->
            <span class="text-white font-medium truncate flex-1 min-w-0">{a.dxcc}</span>

            <!-- Bouton watchlist -->
            {#if isInWatchlist(a.callsign)}
              <span class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs bg-pink-500/20 text-pink-400 border border-pink-500/30" title="In watchlist">
                ★
              </span>
            {:else if isPartiallyInWatchlist(a.callsign)}
              <button
                on:click={() => addToWatchlist(a.callsign)}
                class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs bg-pink-500/10 text-pink-300 border border-pink-500/20 hover:bg-pink-500/20 transition-colors"
                title="Add remaining calls to watchlist">
                ★½
              </button>
            {:else}
              <button
                on:click={() => addToWatchlist(a.callsign)}
                class="flex-shrink-0 px-1.5 py-0.5 rounded text-xs bg-slate-700/50 text-slate-400 border border-slate-600 hover:bg-pink-500/20 hover:text-pink-400 hover:border-pink-500/30 transition-colors"
                title="Add to watchlist">
                ☆
              </button>
            {/if}

            <!-- QSL -->
            {#if a.qsl}
              <span class="text-slate-400 text-xs flex-shrink-0">QSL: {a.qsl}</span>
            {/if}
          </div>

          <!-- Body -->
          <div class="px-3 py-1.5 bg-slate-900/40 space-y-1">

            <!-- Dates -->
            <div class="flex items-center gap-1.5 flex-wrap">
              <span class="text-slate-300 text-xs">📅 {a.startDate} → {a.endDate}</span>
            </div>

            <!-- Bandes + Modes -->
            {#if (a.bands || []).length > 0 || (a.modes || []).length > 0}
              <div class="flex items-center gap-1 flex-wrap">
                {#each (a.bands || []) as band}
                  <span class="px-1.5 py-0 bg-blue-500/20 text-blue-300 border border-blue-500/30 rounded text-xs">{band}</span>
                {/each}
                {#each (a.modes || []) as mode}
                  <span class="px-1.5 py-0 bg-purple-500/20 text-purple-300 border border-purple-500/30 rounded text-xs">{mode}</span>
                {/each}
              </div>
            {/if}

            <!-- Opérateurs -->
            {#if a.operators}
              <div class="text-xs text-slate-400 truncate" title={a.operators}>
                👤 {a.operators}
              </div>
            {/if}

          </div>
        </div>
      {/each}
    {/if}
  </div>

  <!-- Footer -->
  <div class="px-3 py-1.5 border-t border-slate-700/50 flex-shrink-0 flex items-center justify-between">
    <span class="text-xs text-slate-500">{filtered.length} activations · updated hourly</span>
    <a href="https://www.ng3k.com/misc/adxo.html" target="_blank" rel="noreferrer"
      class="text-xs text-slate-600 hover:text-slate-400 transition-colors">ng3k.com</a>
  </div>
</div>
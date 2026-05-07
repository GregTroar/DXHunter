<script>
  import { createEventDispatcher } from 'svelte';

  export let stats;
  export let solarData;
  export let wsStatus;

  const dispatch = createEventDispatcher();

  async function clearFlexSpots() {
    try {
      await fetch('/api/flex/clear-spots', { method: 'POST' });
    } catch (e) {
      console.error('Clear flex spots failed:', e);
    }
  }

  function getSFIColor(sfi) {
    const value = parseInt(sfi);
    if (isNaN(value)) return 'text-slate-500';
    if (value >= 150) return 'text-green-400 font-bold';
    if (value >= 100) return 'text-yellow-400';
    return 'text-red-400';
  }

  function getSunspotsColor(sunspots) {
    const value = parseInt(sunspots);
    if (isNaN(value)) return 'text-slate-500';
    if (value >= 100) return 'text-green-400 font-bold';
    if (value >= 50) return 'text-yellow-400';
    return 'text-orange-400';
  }

  function getAIndexColor(aIndex) {
    const value = parseInt(aIndex);
    if (isNaN(value)) return 'text-slate-500';
    if (value <= 7) return 'text-green-400 font-bold';
    if (value <= 15) return 'text-yellow-400';
    return 'text-red-400';
  }

  function getKIndexColor(kIndex) {
    const value = parseInt(kIndex);
    if (isNaN(value)) return 'text-slate-500';
    if (value <= 2) return 'text-green-400 font-bold';
    if (value <= 4) return 'text-yellow-400';
    return 'text-red-400';
  }
</script>

<div class="flex items-center justify-between mb-3">
  <div class="flex items-center gap-3">
    <svg class="w-8 h-8 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.348 14.651a3.75 3.75 0 010-5.303m5.304 0a3.75 3.75 0 010 5.303m-7.425 2.122a6.75 6.75 0 010-9.546m9.546 0a6.75 6.75 0 010 9.546M5.106 18.894c-3.808-3.808-3.808-9.98 0-13.789m13.788 0c3.808 3.808 3.808 9.981 0 13.79M12 12h.008v.007H12V12zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z" />
    </svg>
    <div>
      <h1 class="text-2xl font-bold bg-gradient-to-r from-blue-400 to-purple-400 bg-clip-text text-transparent">
        DXHunter
      </h1>
      <div class="flex items-center gap-2 text-xs text-slate-400">
        <span class="font-semibold text-blue-400">{stats.myCallsign || 'N/A'}</span>
        <span class="text-slate-600">•</span>
        <span>{stats.totalContacts} QSOs</span>
        <span class="text-slate-600">•</span>
        <span class="text-pink-400 font-semibold">{stats.totalSpots || 0}</span>
        <span>spots</span>
        <span class="text-slate-600">|</span>
        <span class="flex items-center gap-0.5">
          <span class="text-amber-400">SFI</span>
          <span class={getSFIColor(solarData.sfi)}>{solarData.sfi}</span>
        </span>
        <span class="flex items-center gap-0.5">
          <span class="text-yellow-400">SSN</span>
          <span class={getSunspotsColor(solarData.sunspots)}>{solarData.sunspots}</span>
        </span>
        <span class="flex items-center gap-0.5">
          <span class="text-red-400">A</span>
          <span class={getAIndexColor(solarData.aIndex)}>{solarData.aIndex}</span>
        </span>
        <span class="flex items-center gap-0.5">
          <span class="text-purple-400">K</span>
          <span class={getKIndexColor(solarData.kIndex)}>{solarData.kIndex}</span>
        </span>
      </div>
    </div>
  </div>

  <div class="flex items-center gap-2">
    {#if wsStatus === 'connected'}
      <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-green-500/20 text-green-400">
        <span class="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse"></span>
        WS
      </span>
    {:else if wsStatus === 'connecting'}
      <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-orange-500/20 text-orange-400">
        <span class="w-1.5 h-1.5 bg-orange-500 rounded-full animate-pulse"></span>
        WS
      </span>
    {:else}
      <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-red-500/20 text-red-400">
        <span class="w-1.5 h-1.5 bg-red-500 rounded-full"></span>
        WS
      </span>
    {/if}

    <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold {stats.clusterStatus === 'connected' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}">
      <span class="w-1.5 h-1.5 {stats.clusterStatus === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-red-500'} rounded-full"></span>
      Cluster
    </span>

    <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold {stats.flexStatus === 'connected' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}">
      <span class="w-1.5 h-1.5 {stats.flexStatus === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-red-500'} rounded-full"></span>
      Flex
    </span>
    {#if stats.flexStatus === 'connected'}
      <button
        on:click={clearFlexSpots}
        class="px-2 py-1 text-xs bg-slate-700 hover:bg-amber-600/30 border border-slate-600 hover:border-amber-500/50 text-slate-400 hover:text-amber-300 rounded transition-colors flex items-center gap-1"
        title="Clear all FlexRadio panadapter spots">
        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
        <span class="hidden lg:inline">Clear Pan</span>
      </button>
    {/if}

    <button
      on:click={() => dispatch('settings')}
      class="px-2.5 py-1 text-xs bg-slate-700 hover:bg-slate-600 border border-slate-600 hover:border-slate-500 rounded transition-colors flex items-center gap-1"
      title="Configuration">
      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
          d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    </button>

    <button
      on:click={() => dispatch('shutdown')}
      class="px-2.5 py-1 text-xs bg-red-600 hover:bg-red-700 rounded transition-colors flex items-center gap-1">
      <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
      <span class="hidden lg:inline">Shutdown</span>
    </button>
  </div>
</div>

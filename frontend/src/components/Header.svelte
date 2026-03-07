<script>
  import { createEventDispatcher, onMount } from 'svelte';
  
  export let stats;
  export let solarData;
  export let wsStatus;
  export let cacheLoaded = false;

  const dispatch = createEventDispatcher();

  let ctyUpdating = false;

  async function updateCty() {
    ctyUpdating = true;
    try {
      const res = await fetch('/api/cty/update', { method: 'POST' });
      const data = await res.json();
      dispatch('ctyUpdate', { success: data.success, message: data.message || data.error });
    } catch (e) {
      dispatch('ctyUpdate', { success: false, message: 'Network error' });
    } finally {
      ctyUpdating = false;
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
        FlexDXCluster
      </h1>
      <div class="flex items-center gap-2 text-xs text-slate-400">
        <!-- Callsign & Contacts -->
        <span class="font-semibold text-blue-400">{stats.myCallsign || 'N/A'}</span>
        <span class="text-slate-600">•</span>
        <span>{stats.totalContacts} QSOs</span>
        
        <!-- Spots count -->
        <span class="text-slate-600">•</span>
        <span class="text-pink-400 font-semibold">{stats.totalSpots || 0}</span>
        <span>spots</span>
        
        <span class="text-slate-600">|</span>
        
        <!-- Solar data compacts -->
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
<!-- Dans Header.svelte -->
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

    {#if cacheLoaded}
      <span class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-purple-500/20 text-purple-400" title="Data loaded from cache">
        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"></path>
        </svg>
      </span>
    {/if}

    <button 
      on:click={updateCty}
      disabled={ctyUpdating}
      title="Update cty.plist from country-files.com"
      class="px-2.5 py-1 text-xs bg-blue-600/30 hover:bg-blue-600/50 text-blue-300 rounded transition-colors flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed">
      {#if ctyUpdating}
        <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"/>
        </svg>
      {:else}
        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
      {/if}
      <span class="hidden lg:inline">CTY</span>
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
<script>
  import { createEventDispatcher, onMount } from 'svelte';
  
  export let stats;
  export let solarData;
  export let wsStatus;
  export let cacheLoaded = false;
  export let soundManager;

  let soundEnabled = false; // ✅ Initialisé à false
  
  const dispatch = createEventDispatcher();

  // ✅ Synchroniser avec le soundManager au montage
  onMount(() => {
    soundEnabled = soundManager.isEnabled();
  });

  function toggleSound() {
    soundEnabled = !soundEnabled;
    soundManager.setEnabled(soundEnabled);
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
    {#if stats.contestMode}
      <div 
        role="button"
        tabindex="0"
        aria-label="Switch to Normal Mode"
        class="px-3 py-1 bg-yellow-600/20 text-yellow-400 rounded text-xs font-semibold flex items-center gap-1 cursor-pointer hover:bg-yellow-600/30 transition-colors focus:outline-none focus:ring-2 focus:ring-yellow-500 focus:ring-opacity-50"
        on:click={() => {
          const event = new CustomEvent('toggleContestMode');
          window.dispatchEvent(event);
        }}
        on:keydown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            const event = new CustomEvent('toggleContestMode');
            window.dispatchEvent(event);
          }
        }}>
        <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/>
        </svg>
        Contest Mode
      </div>
    {/if}
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
      on:click={toggleSound}
      title="{soundEnabled ? 'Disable' : 'Enable'} sound alerts"
      class="px-2.5 py-1 rounded transition-colors {soundEnabled ? 'bg-blue-600 hover:bg-blue-700' : 'bg-slate-700 hover:bg-slate-600'} flex items-center gap-1.5">
      {#if soundEnabled}
        <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM14.657 2.929a1 1 0 011.414 0A9.972 9.972 0 0119 10a9.972 9.972 0 01-2.929 7.071 1 1 0 01-1.414-1.414A7.971 7.971 0 0017 10c0-2.21-.894-4.208-2.343-5.657a1 1 0 010-1.414zm-2.829 2.828a1 1 0 011.415 0A5.983 5.983 0 0115 10a5.984 5.984 0 01-1.757 4.243 1 1 0 01-1.415-1.415A3.984 3.984 0 0013 10a3.983 3.983 0 00-1.172-2.828 1 1 0 010-1.415z" clip-rule="evenodd"></path>
        </svg>
      {:else}
        <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M9.383 3.076A1 1 0 0110 4v12a1 1 0 01-1.707.707L4.586 13H2a1 1 0 01-1-1V8a1 1 0 011-1h2.586l3.707-3.707a1 1 0 011.09-.217zM12.293 7.293a1 1 0 011.414 0L15 8.586l1.293-1.293a1 1 0 111.414 1.414L16.414 10l1.293 1.293a1 1 0 01-1.414 1.414L15 11.414l-1.293 1.293a1 1 0 01-1.414-1.414L13.586 10l-1.293-1.293a1 1 0 010-1.414z" clip-rule="evenodd"></path>
        </svg>
      {/if}
      <span class="text-xs hidden lg:inline">{soundEnabled ? 'ON' : 'OFF'}</span>
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
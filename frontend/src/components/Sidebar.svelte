<script>
  import { createEventDispatcher } from 'svelte';
  import StatsTab from './StatsTab.svelte';
  import WatchlistTab from './WatchlistTab.svelte';
  import LogTab from './LogTab.svelte';
  
  export let activeTab;
  export let topSpotters;
  export let spots;
  export let watchlist;
  export let recentQSOs;
  export let logStats;
  export let dxccProgress;
  export let showOnlyActive = false; // ✅ Export pour persister l'état
  
  const dispatch = createEventDispatcher();
  
  // ✅ Propagation des évènements vers le parent
  function handleToast(event) {
    dispatch('toast', event.detail);
  }
</script>

<div class="bg-slate-800/50 backdrop-blur rounded-lg border border-slate-700/50 flex flex-col h-full" style="height: 100%; max-height: 100%;">
  <!-- Tabs Header -->
  <div class="flex border-b border-slate-700/50 bg-slate-900/30 flex-shrink-0">
    <button 
      class="px-4 py-2 text-sm font-semibold transition-colors {activeTab === 'stats' ? 'bg-blue-500/20 text-blue-400 border-b-2 border-blue-500' : 'text-slate-400 hover:text-slate-300'}"
      on:click={() => activeTab = 'stats'}>
      <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
      Stats
    </button>
    
    <button 
      class="px-4 py-2 text-sm font-semibold transition-colors {activeTab === 'watchlist' ? 'bg-blue-500/20 text-blue-400 border-b-2 border-blue-500' : 'text-slate-400 hover:text-slate-300'}"
      on:click={() => activeTab = 'watchlist'}>
      <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
      </svg>
      Watchlist
    </button>
    
    <button 
      class="px-4 py-2 text-sm font-semibold transition-colors {activeTab === 'log' ? 'bg-blue-500/20 text-blue-400 border-b-2 border-blue-500' : 'text-slate-400 hover:text-slate-300'}"
      on:click={() => activeTab = 'log'}>
      <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      Log
    </button>
  </div>
  
  <!-- Tab Content -->
  <div class="flex-1 overflow-hidden" style="min-height: 0;">
    {#if activeTab === 'stats'}
      <StatsTab {topSpotters} {spots} />
    {:else if activeTab === 'watchlist'}  
      <WatchlistTab
        {watchlist}
        {spots}
        bind:showOnlyActive
        on:toast={handleToast}
      />
    {:else if activeTab === 'log'}
      <LogTab 
        {recentQSOs} 
        {logStats} 
        {dxccProgress}
      />
    {/if}
  </div>
</div>
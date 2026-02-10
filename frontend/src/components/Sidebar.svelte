<script>
  import { createEventDispatcher } from 'svelte';
  import StatsTab from './StatsTab.svelte';
  import WatchlistTab from './WatchlistTab.svelte';
  import LogTab from './LogTab.svelte';
  import LogsTab from './LogsTab.svelte';
  import ConsoleTab from './ConsoleTab.svelte';
  
  export let activeTab;
  export let topSpotters;
  export let spots;
  export let watchlist;
  export let recentQSOs;
  export let logStats;
  export let dxccProgress;
  export let showOnlyActive = true;
  export let showOnlyNotWorked = true;
  export let logs = [];
  export let contestMode = false;
  export let contestPrefix = "";
  export let contestCallsigns = [];
  export let wsStatus = 'disconnected';
  export let clusterType = 'unknown';
  
  const dispatch = createEventDispatcher();
  
  function handleToast(event) {
    dispatch('toast', event.detail);
  }
  
  function handleClearLogs() {
    dispatch('clearLogs');
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
      Log4OM
    </button>

    <button 
      class="px-4 py-2 text-sm font-semibold transition-colors {activeTab === 'logs' ? 'bg-blue-500/20 text-blue-400 border-b-2 border-blue-500' : 'text-slate-400 hover:text-slate-300'}"
      on:click={() => activeTab = 'logs'}>
      <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
      </svg>
      AppLogs
    </button>

    <button 
      class="px-4 py-2 text-sm font-semibold transition-colors {activeTab === 'console' ? 'bg-blue-500/20 text-blue-400 border-b-2 border-blue-500' : 'text-slate-400 hover:text-slate-300'}"
      on:click={() => activeTab = 'console'}>
      <svg class="w-4 h-4 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
      </svg>
      Console
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
        bind:showOnlyNotWorked
        {contestMode}
        {contestPrefix}
        {contestCallsigns}
        on:toast={handleToast}
      />
    {:else if activeTab === 'log'}
      <LogTab 
        {recentQSOs} 
        {logStats} 
        {dxccProgress}
      />
    {:else if activeTab === 'logs'}
      <LogsTab 
        {logs} 
        on:clearLogs={handleClearLogs}
      />
    {:else if activeTab === 'console'}
      <ConsoleTab 
        {wsStatus}
        {clusterType}
        on:sendCommand={(e) => dispatch('sendCommand', e.detail)}
      />
    {/if}
  </div>
</div>
<script>
  import { createEventDispatcher } from 'svelte';
  
  export let logs = [];
  
  const dispatch = createEventDispatcher();
  
  let autoScroll = true;
  let container;
  let selectedLevels = {
    debug: false,
    info: true,
    warning: true,
    error: true
  };
  
  // ✅ Filtrer les logs par niveau sélectionné
  $: filteredLogs = logs.filter(log => {
    const level = log.level.toLowerCase();
    return selectedLevels[level] || false;
  });
  
  // ✅ Auto-scroll UNIQUEMENT si activé
  $: if (autoScroll && container && filteredLogs.length > 0) {
    setTimeout(() => {
      if (autoScroll) {
        container.scrollTop = container.scrollHeight;
      }
    }, 10);
  }
  
  function getLevelColor(level) {
    switch(level.toLowerCase()) {
      case 'error': return 'text-red-400';
      case 'warning':
      case 'warn': return 'text-yellow-400';
      case 'info': return 'text-blue-400';
      case 'debug': return 'text-slate-400';
      default: return 'text-slate-300';
    }
  }
  
  function getLevelBadge(level) {
    switch(level.toLowerCase()) {
      case 'error': return 'bg-red-500/20 text-red-400 border-red-500/50';
      case 'warning':
      case 'warn': return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50';
      case 'info': return 'bg-blue-500/20 text-blue-400 border-blue-500/50';
      case 'debug': return 'bg-slate-500/20 text-slate-400 border-slate-500/50';
      default: return 'bg-slate-500/20 text-slate-300 border-slate-500/50';
    }
  }
  
  // ✅ Dispatcher l'événement au parent
  function clearLogs() {
    dispatch('clearLogs');
  }
  
  function toggleLevel(level) {
    selectedLevels[level] = !selectedLevels[level];
  }
</script>

<div class="h-full flex flex-col bg-slate-900/50 rounded-lg border border-slate-700/50">
  <!-- Header -->
  <div class="p-3 border-b border-slate-700/50 flex-shrink-0">
    <div class="flex items-center justify-between mb-3">
      <h2 class="text-lg font-bold">Application Logs</h2>
      <div class="flex items-center gap-3">
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input 
            type="checkbox" 
            bind:checked={autoScroll} 
            class="rounded cursor-pointer">
          <span>Auto-scroll</span>
        </label>
        <button 
          on:click={clearLogs}
          class="px-3 py-1.5 bg-red-600/20 hover:bg-red-600/40 text-red-400 rounded text-sm transition-colors">
          Clear Logs
        </button>
      </div>
    </div>
    
    <!-- ✅ Filtres par niveau -->
    <div class="flex items-center gap-2 text-xs">
      <span class="text-slate-400 mr-2">Show:</span>
      
      <button 
        on:click={() => toggleLevel('debug')}
        class="px-2 py-1 rounded border transition-colors {selectedLevels.debug ? 'bg-slate-500/20 text-slate-400 border-slate-500/50' : 'bg-slate-800/50 text-slate-600 border-slate-700/30'}">
        DEBUG
      </button>
      
      <button 
        on:click={() => toggleLevel('info')}
        class="px-2 py-1 rounded border transition-colors {selectedLevels.info ? 'bg-blue-500/20 text-blue-400 border-blue-500/50' : 'bg-slate-800/50 text-slate-600 border-slate-700/30'}">
        INFO
      </button>
      
      <button 
        on:click={() => toggleLevel('warning')}
        class="px-2 py-1 rounded border transition-colors {selectedLevels.warning ? 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50' : 'bg-slate-800/50 text-slate-600 border-slate-700/30'}">
        WARN
      </button>
      
      <button 
        on:click={() => toggleLevel('error')}
        class="px-2 py-1 rounded border transition-colors {selectedLevels.error ? 'bg-red-500/20 text-red-400 border-red-500/50' : 'bg-slate-800/50 text-slate-600 border-slate-700/30'}">
        ERROR
      </button>
      
      <span class="ml-auto text-slate-500">{filteredLogs.length} / {logs.length} logs</span>
    </div>
  </div>
  
  <!-- Logs container -->
  <div 
    bind:this={container}
    class="flex-1 overflow-y-auto p-3 font-mono text-xs"
    style="min-height: 0;">
    
    {#if filteredLogs.length === 0}
      <div class="text-center py-8 text-slate-400">
        <svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        <p class="text-sm">
          {logs.length === 0 ? 'No logs yet' : 'No logs matching selected levels'}
        </p>
      </div>
    {:else}
      {#each filteredLogs as log (log.timestamp + log.message)}
        <div class="flex gap-3 py-1 hover:bg-slate-800/30 px-2 rounded">
          <span class="text-slate-500 flex-shrink-0">{log.timestamp}</span>
          <span class="px-2 py-0.5 rounded border text-xs font-semibold flex-shrink-0 {getLevelBadge(log.level)}">
            {log.level.toUpperCase()}
          </span>
          <span class="{getLevelColor(log.level)} flex-1 break-all">{log.message}</span>
        </div>
      {/each}
    {/if}
  </div>
</div>
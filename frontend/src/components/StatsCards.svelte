<script>
  import { createEventDispatcher } from 'svelte';
  
  export let stats;
  
  const dispatch = createEventDispatcher();
  
  function handleFilterChange(filterName, checked) {
    dispatch('filterChange', { name: filterName, value: checked });
  }
</script>

<div class="grid grid-cols-[repeat(4,1fr)_auto] gap-3 mb-3 items-center">
  <!-- Total Spots -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
      </svg>
      <div class="text-xl font-bold text-blue-400">{stats.totalSpots}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Total Spots</p>
  </div>
  
  <!-- New DXCC -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <div class="text-xl font-bold text-green-400">{stats.newDXCC}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">New DXCC</p>
  </div>
  
  <!-- Telnet Clients -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
      </svg>
      <div class="text-xl font-bold text-orange-400">{stats.connectedClients}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Clients</p>
  </div>
  
  <!-- Total Contacts -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      <div class="text-xl font-bold text-purple-400">{stats.totalContacts.toLocaleString()}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">QSOs</p>
  </div>
  
  <!-- Cluster Filters -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50 h-full">
    <div class="flex items-center justify-center gap-4 h-full">
      <span class="text-xs text-slate-400 font-semibold whitespace-nowrap">Cluster Filters:</span>
      
      <label class="flex items-center gap-1.5 cursor-pointer hover:bg-slate-700/30 px-2 py-1 rounded transition-colors">
        <input 
          type="checkbox" 
          checked={stats.filters.skimmer}
          on:change={(e) => handleFilterChange('skimmer', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-9 h-5 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-xs font-medium whitespace-nowrap">Skimmer</span>
      </label>
      
      <label class="flex items-center gap-1.5 cursor-pointer hover:bg-slate-700/30 px-2 py-1 rounded transition-colors">
        <input 
          type="checkbox" 
          checked={stats.filters.ft8}
          on:change={(e) => handleFilterChange('ft8', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-9 h-5 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-xs font-medium">FT8</span>
      </label>
      
      <label class="flex items-center gap-1.5 cursor-pointer hover:bg-slate-700/30 px-2 py-1 rounded transition-colors">
        <input 
          type="checkbox" 
          checked={stats.filters.ft4}
          on:change={(e) => handleFilterChange('ft4', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-9 h-5 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-xs font-medium">FT4</span>
      </label>
      
      <label class="flex items-center gap-1.5 cursor-pointer hover:bg-slate-700/30 px-2 py-1 rounded transition-colors">
        <input 
          type="checkbox" 
          checked={stats.filters.beacon}
          on:change={(e) => handleFilterChange('beacon', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-9 h-5 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-xs font-medium">Beacon</span>
      </label>
    </div>
  </div>
</div>
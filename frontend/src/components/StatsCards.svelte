<script>
  import { createEventDispatcher } from 'svelte';
  
  export let stats;
  
  const dispatch = createEventDispatcher();
  
  function handleFilterChange(filterName, checked) {
    dispatch('filterChange', { name: filterName, value: checked });
  }
</script>

<div class="grid grid-cols-7 gap-3 mb-3">
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
      </svg>
      <div class="text-xl font-bold text-blue-400">{stats.totalSpots}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Total Spots</p>
  </div>
  
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <div class="text-xl font-bold text-green-400">{stats.newDXCC}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">New DXCC</p>
  </div>
  
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
      </svg>
      <div class="text-xl font-bold text-purple-400">{stats.activeSpotters}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Spotters</p>
  </div>
  
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0" />
      </svg>
      <div class="text-xl font-bold text-orange-400">{stats.connectedClients}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Clients</p>
  </div>
  
    <div class="col-span-3 bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-center gap-6 h-full">
      <label class="flex items-center gap-2 cursor-pointer">
        <input 
          type="checkbox" 
          checked={stats.filters.skimmer}
          on:change={(e) => handleFilterChange('skimmer', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-11 h-6 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-sm font-medium">CW</span>
      </label>
      
      <label class="flex items-center gap-2 cursor-pointer">
        <input 
          type="checkbox" 
          checked={stats.filters.ft8}
          on:change={(e) => handleFilterChange('ft8', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-11 h-6 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-sm font-medium">FT8</span>
      </label>
      
      <label class="flex items-center gap-2 cursor-pointer">
        <input 
          type="checkbox" 
          checked={stats.filters.ft4}
          on:change={(e) => handleFilterChange('ft4', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-11 h-6 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-sm font-medium">FT4</span>
      </label>
      
      <!-- ✅ AJOUTER ce switch Beacon -->
      <label class="flex items-center gap-2 cursor-pointer">
        <input 
          type="checkbox" 
          checked={stats.filters.beacon}
          on:change={(e) => handleFilterChange('beacon', e.target.checked)}
          class="sr-only peer"
        />
        <div class="relative w-11 h-6 bg-slate-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
        <span class="text-sm font-medium">Beacon</span>
      </label>
    </div>
  </div>
</div>
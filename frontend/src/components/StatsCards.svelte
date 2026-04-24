<script>
  export let stats;
  export let spots = [];

  $: newDXCCCount = spots.length
    ? new Set(spots.filter(s => s.NewDXCC).map(s => s.DXCC || s.CountryName)).size
    : (stats.newDXCC || 0);
</script>

<div class="grid grid-cols-7 gap-3 mb-3 items-start">
  <!-- Spots Received -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-cyan-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
      </svg>
      <div class="text-xl font-bold text-cyan-400">{stats.spotsReceived || 0}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Received</p>
  </div>

  <!-- Spots Processed -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <div class="text-xl font-bold text-green-400">{stats.spotsProcessed || 0}</div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Processed</p>
  </div>

  <!-- Success Rate -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 {stats.spotSuccessRate >= 95 ? 'text-green-400' : stats.spotSuccessRate >= 80 ? 'text-yellow-400' : 'text-red-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
      <div class="text-xl font-bold {stats.spotSuccessRate >= 95 ? 'text-green-400' : stats.spotSuccessRate >= 80 ? 'text-yellow-400' : 'text-red-400'}">
        {stats.spotSuccessRate ? stats.spotSuccessRate.toFixed(1) : '0.0'}%
      </div>
    </div>
    <p class="text-xs text-slate-400 mt-1">Success Rate</p>
  </div>

  <!-- New DXCC -->
  <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50">
    <div class="flex items-center justify-between">
      <svg class="w-6 h-6 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <div class="text-xl font-bold text-emerald-400">{newDXCCCount}</div>
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

  <!-- Cluster Status -->
  {#if true}
    {@const clusters = stats.clusters || []}
    {@const col1 = clusters.slice(0, 3)}
    {@const col2 = clusters.slice(3, 6)}
    <div class="bg-slate-800/50 backdrop-blur rounded-lg p-3 border border-slate-700/50 flex items-center gap-3">
      <!-- col 1: icon + label -->
      <div class="flex flex-col items-center gap-1 flex-shrink-0">
        <svg class="w-6 h-6 text-sky-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
        </svg>
        <span class="text-xs text-slate-400">Clusters</span>
      </div>
      <!-- col 2: first 3 clusters -->
      <div class="flex flex-col gap-0.5 flex-1 min-w-0 pl-2 border-l border-slate-700/50">
        {#each col1 as c}
          <div class="flex items-center gap-1 min-w-0">
            <div class="w-1.5 h-1.5 rounded-full flex-shrink-0 {c.status === 'connected' ? 'bg-green-400 shadow-[0_0_3px_#4ade80]' : 'bg-red-500'}"></div>
            <span class="text-[10px] text-slate-300 truncate leading-tight">{c.name}</span>
          </div>
        {/each}
      </div>
      <!-- col 3: next 3 clusters (hidden if empty) -->
      {#if col2.length > 0}
        <div class="flex flex-col gap-0.5 flex-1 min-w-0">
          {#each col2 as c}
            <div class="flex items-center gap-1 min-w-0">
              <div class="w-1.5 h-1.5 rounded-full flex-shrink-0 {c.status === 'connected' ? 'bg-green-400 shadow-[0_0_3px_#4ade80]' : 'bg-red-500'}"></div>
              <span class="text-[10px] text-slate-300 truncate leading-tight">{c.name}</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

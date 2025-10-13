<script>
  export let recentQSOs;
  export let logStats;
  export let dxccProgress;
</script>

<div class="h-full flex flex-col overflow-hidden">
  <div class="p-3 border-b border-slate-700/50 flex-shrink-0">
    <h2 class="text-lg font-bold mb-3">Station Log</h2>
    
    <div class="grid grid-cols-4 gap-2 mb-3">
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">Today</div>
        <div class="text-xl font-bold text-blue-400">{logStats.today || 0}</div>
      </div>
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">This Week</div>
        <div class="text-xl font-bold text-green-400">{logStats.thisWeek || 0}</div>
      </div>
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">This Month</div>
        <div class="text-xl font-bold text-purple-400">{logStats.thisMonth || 0}</div>
      </div>
      <div class="bg-slate-900/50 rounded p-2">
        <div class="text-xs text-slate-400">Total</div>
        <div class="text-xl font-bold text-orange-400">{logStats.total || 0}</div>
      </div>
    </div>
    
    <!-- DXCC Progress Bar -->
    <div class="bg-slate-900/50 rounded p-3">
      <div class="flex items-center justify-between mb-2">
        <span class="text-sm font-semibold text-slate-300">DXCC Progress</span>
        <span class="text-sm font-bold text-green-400">{dxccProgress.worked || 0} / {dxccProgress.total || 340}</span>
      </div>
      <div class="w-full bg-slate-700/30 rounded-full h-3 overflow-hidden">
        <div 
          class="h-full rounded-full bg-gradient-to-r from-green-500 to-emerald-500 transition-all duration-500" 
          style="width: {dxccProgress.percentage || 0}%">
        </div>
      </div>
      <div class="text-xs text-slate-400 text-right mt-1">{(dxccProgress.percentage || 0).toFixed(1)}% Complete</div>
    </div>
  </div>
  
  <!-- Recent QSOs Table -->
  <div class="flex-1 overflow-y-auto">
    <div class="p-3">
      <h3 class="text-sm font-bold text-slate-400 mb-2">Recent QSOs</h3>
      
      {#if recentQSOs.length === 0}
        <div class="text-center py-8 text-slate-400">
          <svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <p class="text-sm">No QSOs in log</p>
        </div>
      {:else}
        <table class="w-full text-xs">
          <thead class="bg-slate-900/50 sticky top-0">
            <tr class="text-left text-xs text-slate-400">
              <th class="p-2 font-semibold">Date</th>
              <th class="p-2 font-semibold">Time</th>
              <th class="p-2 font-semibold">Callsign</th>
              <th class="p-2 font-semibold">Band</th>
              <th class="p-2 font-semibold">Mode</th>
              <th class="p-2 font-semibold">RST S/R</th>
              <th class="p-2 font-semibold">Country</th>
            </tr>
          </thead>
          <tbody>
            {#each recentQSOs as qso}
              {@const date = qso.date ? new Date(qso.date.replace(' ', 'T')) : null}
              {@const qsoDate = date ? date.toISOString().split('T')[0] : 'N/A'}
              {@const qsoTime = date ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false }) : 'N/A'}
              
              <tr class="border-b border-slate-700/30 hover:bg-slate-700/30 transition-colors">
                <td class="p-2 text-slate-300">{qsoDate}</td>
                <td class="p-2 text-slate-300">{qsoTime}</td>
                <td class="p-2">
                  <span class="font-bold text-blue-400">{qso.callsign}</span>
                </td>
                <td class="p-2">
                  <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-xs">{qso.band || 'N/A'}</span>
                </td>
                <td class="p-2">
                  <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs">{qso.mode || 'N/A'}</span>
                </td>
                <td class="p-2 font-mono text-xs text-slate-400">{qso.rstSent || '---'} / {qso.rstRcvd || '---'}</td>
                <td class="p-2 text-slate-400">{qso.country || 'N/A'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  </div>
</div>
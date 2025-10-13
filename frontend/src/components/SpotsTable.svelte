<script>
  import { createEventDispatcher } from 'svelte';
  
  export let spots;
  export let watchlist;
  export let myCallsign;
  
  const dispatch = createEventDispatcher();
  
  function handleSpotClick(spot) {
    dispatch('clickSpot', {
      callsign: spot.DX,
      frequency: spot.FrequencyMhz,
      mode: spot.Mode
    });
  }
  
  function getPriorityColor(spot) {
    const inWatchlist = watchlist.some(pattern => 
      spot.DX === pattern || spot.DX.startsWith(pattern)
    );
    
    if (inWatchlist) return 'bg-pink-500/20 text-pink-400 border-pink-500/50';
    if (spot.DX === myCallsign) return 'bg-red-500/20 text-red-400 border-red-500/50';
    if (spot.NewDXCC) return 'bg-green-500/20 text-green-400 border-green-500/50';
    if (spot.NewBand && spot.NewMode) return 'bg-purple-500/20 text-purple-400 border-purple-500/50';
    if (spot.NewBand) return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50';
    if (spot.NewMode) return 'bg-orange-500/20 text-orange-400 border-orange-500/50';
    if (spot.Worked) return 'bg-cyan-500/20 text-cyan-400 border-cyan-500/50';
    return 'bg-gray-500/20 text-gray-400 border-gray-500/50';
  }
  
  function getStatusLabel(spot) {
    const inWatchlist = watchlist.some(pattern => 
      spot.DX === pattern || spot.DX.startsWith(pattern)
    );
    
    if (inWatchlist) return 'Watchlist';
    if (spot.DX === myCallsign) return 'My Call';
    if (spot.NewDXCC) return 'New DXCC';
    if (spot.NewBand && spot.NewMode) return 'New B&M';
    if (spot.NewBand) return 'New Band';
    if (spot.NewMode) return 'New Mode';
    if (spot.NewSlot) return 'New Slot';
    if (spot.Worked) return 'Worked';
    return '';
  }
</script>

<div class="bg-slate-800/50 backdrop-blur rounded-lg border border-slate-700/50 flex flex-col overflow-hidden h-full">
  <div class="p-3 border-b border-slate-700/50 flex items-center justify-between flex-shrink-0">
    <h2 class="text-lg font-bold">Recent Spots (<span>{spots.length}</span>)</h2>
  </div>
  
  <div class="overflow-y-auto flex-1">
    <table class="w-full text-sm">
      <thead class="bg-slate-900/50 sticky top-0">
        <tr class="text-left text-xs text-slate-400">
          <th class="p-2 font-semibold">DX</th>
          <th class="p-2 font-semibold">Freq</th>
          <th class="p-2 font-semibold">Band</th>
          <th class="p-2 font-semibold">Mode</th>
          <th class="p-2 font-semibold">Spotter</th>
          <th class="p-2 font-semibold">Time</th>
          <th class="p-2 font-semibold">Country</th>
          <th class="p-2 font-semibold">Status</th>
        </tr>
      </thead>
      <tbody>
        {#each spots as spot (spot.ID)}
          <tr class="border-b border-slate-700/30 hover:bg-slate-700/30 transition-colors text-sm">
            <td class="p-2">
              <button
                class="font-bold text-blue-400 dx-callsign"
                on:click={() => handleSpotClick(spot)}
                title="Click to send to Log4OM and tune radio">
                {spot.DX}
              </button>
            </td>
            <td class="p-2 font-mono text-xs">{spot.FrequencyMhz}</td>
            <td class="p-2">
              <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-xs">{spot.Band}</span>
            </td>
            <td class="p-2">
              <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs">{spot.Mode}</span>
            </td>
            <td class="p-2 text-slate-300 text-xs">{spot.SpotterCallsign}</td>
            <td class="p-2 text-slate-400 text-xs">{spot.UTCTime}</td>
            <td class="p-2 text-slate-400 text-xs">{spot.CountryName || 'N/A'}</td>
            <td class="p-2">
              {#if getStatusLabel(spot)}
                <span class="px-1.5 py-0.5 rounded text-xs font-semibold border {getPriorityColor(spot)}">
                  {getStatusLabel(spot)}
                </span>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
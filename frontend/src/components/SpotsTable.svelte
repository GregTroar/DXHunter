<script>
  import { createEventDispatcher } from 'svelte';
  import VirtualList from 'svelte-virtual-list';
  
  export let spots;
  export let watchlist;
  export let myCallsign;
  
  const dispatch = createEventDispatcher();
  
  let container;
  const itemHeight = 43;
  
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
  
  <!-- Header fixe -->
  <div class="bg-slate-900/50 flex-shrink-0">
    <div class="flex text-left text-xs text-slate-400 font-semibold">
      <div class="p-2" style="width: 12%;">DX</div>
      <div class="p-2" style="width: 12%;">Freq</div>
      <div class="p-2" style="width: 8%;">Band</div>
      <div class="p-2" style="width: 8%;">Mode</div>
      <div class="p-2" style="width: 12%;">Spotter</div>
      <div class="p-2" style="width: 8%;">Time</div>
      <div class="p-2" style="width: 25%;">Country</div>
      <div class="p-2" style="width: 15%;">Status</div>
    </div>
  </div>
  
  <!-- Liste virtualisée -->
  <div class="flex-1 overflow-hidden" bind:this={container}>
    <VirtualList items={spots} {itemHeight} let:item>
      <div class="flex border-b border-slate-700/30 hover:bg-slate-700/30 transition-colors text-sm" style="height: {itemHeight}px;">
        <div class="p-2 flex items-center" style="width: 12%;">
          <button
            class="font-bold text-blue-400 hover:text-blue-300 transition-colors truncate w-full text-left"
            on:click={() => handleSpotClick(item)}
            title="Click to send to Log4OM and tune radio">
            {item.DX}
          </button>
        </div>
        <div class="p-2 flex items-center font-mono text-xs" style="width: 12%;">{item.FrequencyMhz}</div>
        <div class="p-2 flex items-center" style="width: 8%;">
          <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-xs">{item.Band}</span>
        </div>
        <div class="p-2 flex items-center" style="width: 8%;">
          <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs">{item.Mode}</span>
        </div>
        <div class="p-2 flex items-center text-slate-300 text-xs truncate" style="width: 12%;" title={item.SpotterCallsign}>
          {item.SpotterCallsign}
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs" style="width: 8%;">{item.UTCTime}</div>
        <div class="p-2 flex items-center text-slate-400 text-xs truncate" style="width: 25%;" title={item.CountryName || 'N/A'}>
          {item.CountryName || 'N/A'}
        </div>
        <div class="p-2 flex items-center" style="width: 15%;">
          {#if getStatusLabel(item)}
            <span class="px-1.5 py-0.5 rounded text-xs font-semibold border {getPriorityColor(item)} truncate">
              {getStatusLabel(item)}
            </span>
          {/if}
        </div>
      </div>
    </VirtualList>
  </div>
</div>
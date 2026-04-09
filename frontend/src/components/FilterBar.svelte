<script>
  import { createEventDispatcher } from 'svelte';
  
  export let spotFilters;
  export let spots;
  export let watchlist;
  export let contestMode = false; 
  export let contestPrefix = "";     
  export let contestCallsigns = [];  
  
  const dispatch = createEventDispatcher();
  
  $: watchlistCount = spots && watchlist ? countWatchlistSpots(spots, watchlist) : 0;
  $: newDXCCCount = spots ? countSpotsByType(spots, 'newDXCC') : 0;
  $: newBandModeCount = spots ? countSpotsByType(spots, 'newBandMode') : 0;
  $: newBandCount = spots ? countSpotsByType(spots, 'newBand') : 0;
  $: newModeCount = spots ? countSpotsByType(spots, 'newMode') : 0;
  $: newSlotCount = spots ? countSpotsByType(spots, 'newSlot') : 0;
  $: workedCount = spots ? countSpotsByType(spots, 'worked') : 0;
  
  $: ft8Count  = spots ? countSpotsByMode(spots, 'ft8') : 0;
  $: ft4Count  = spots ? countSpotsByMode(spots, 'ft4') : 0;
  $: rttyCount = spots ? countSpotsByMode(spots, 'rtty') : 0;
  $: ssbCount = spots ? countSpotsByMode(spots, 'ssb') : 0;
  $: cwCount = spots ? countSpotsByMode(spots, 'cw') : 0;
  
  $: band160MCount = spots ? countSpotsByType(spots, '160M') : 0;
  $: band80MCount = spots ? countSpotsByType(spots, '80M') : 0;
  $: band60MCount = spots ? countSpotsByType(spots, '60M') : 0;
  $: band40MCount = spots ? countSpotsByType(spots, '40M') : 0;
  $: band30MCount = spots ? countSpotsByType(spots, '30M') : 0;
  $: band20MCount = spots ? countSpotsByType(spots, '20M') : 0;
  $: band17MCount = spots ? countSpotsByType(spots, '17M') : 0;
  $: band15MCount = spots ? countSpotsByType(spots, '15M') : 0;
  $: band12MCount = spots ? countSpotsByType(spots, '12M') : 0;
  $: band10MCount = spots ? countSpotsByType(spots, '10M') : 0;
  $: band6MCount = spots ? countSpotsByType(spots, '6M') : 0;
  
  function countSpotsByType(spotsList, type) {
    if (!spotsList || !Array.isArray(spotsList)) return 0;
    
    switch(type) {
      case 'newDXCC':
        return new Set(spotsList.filter(s => s.NewDXCC).map(s => s.DXCC || s.CountryName)).size;
      case 'newBandMode': 
        return spotsList.filter(s => s.NewBand && s.NewMode && !s.NewDXCC).length;
      case 'newBand': 
        return spotsList.filter(s => s.NewBand && !s.NewMode && !s.NewDXCC).length;
      case 'newMode': 
        return spotsList.filter(s => s.NewMode && !s.NewBand && !s.NewDXCC).length;
      case 'newSlot': 
        return spotsList.filter(s => s.NewSlot && !s.NewDXCC && !s.NewBand && !s.NewMode).length;
      case 'worked': 
        return spotsList.filter(s => s.Worked).length;
      default:
        return spotsList.filter(s => s.Band === type).length;
    }
  }
  
  function countSpotsByMode(spotsList, mode) {
    if (!spotsList || !Array.isArray(spotsList)) return 0;
    
    switch(mode) {
      case 'ft8':
        return spotsList.filter(s => s.Mode === 'FT8').length;
      case 'ft4':
        return spotsList.filter(s => s.Mode === 'FT4').length;
      case 'rtty':
        return spotsList.filter(s => s.Mode === 'RTTY').length;
      case 'ssb':
        return spotsList.filter(s => ['SSB', 'USB', 'LSB'].includes(s.Mode)).length;
      case 'cw':
        return spotsList.filter(s => s.Mode === 'CW').length;
      default:
        return 0;
    }
  }
  
  function countWatchlistSpots(spotsList, wl) {
    if (!spotsList || !Array.isArray(spotsList) || !wl || !Array.isArray(wl)) return 0;
    const patterns = wl.map(e => typeof e === 'string' ? e : e.callsign).filter(Boolean);
    return spotsList.filter(spot => 
      patterns.some(pattern => spot.DX === pattern || spot.DX.startsWith(pattern))
    ).length;
  }
</script>

    <div class="bg-slate-800/50 backdrop-blur rounded-lg p-2 border border-slate-700/50 mb-3">
      <div class="flex items-center gap-1 flex-wrap">
        
        <span class="text-xs font-bold text-slate-400 mr-2">TYPE:</span>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showAll')}
      class={spotFilters.showAll ? 'px-2 py-0.5 text-xs rounded transition-colors bg-blue-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      All ({spots.length})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showWatchlist')}
      class={spotFilters.showWatchlist ? 'px-2 py-0.5 text-xs rounded transition-colors bg-pink-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      Watch ({watchlistCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showNewDXCC')}
      class={spotFilters.showNewDXCC ? 'px-2 py-0.5 text-xs rounded transition-colors bg-green-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      DXCC ({newDXCCCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showNewBandMode')}
      class={spotFilters.showNewBandMode ? 'px-2 py-0.5 text-xs rounded transition-colors bg-purple-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      B&M ({newBandModeCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showNewBand')}
      class={spotFilters.showNewBand ? 'px-2 py-0.5 text-xs rounded transition-colors bg-yellow-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      Band ({newBandCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showNewMode')}
      class={spotFilters.showNewMode ? 'px-2 py-0.5 text-xs rounded transition-colors bg-orange-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      Mode ({newModeCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showNewSlot')}
      class={spotFilters.showNewSlot ? 'px-2 py-0.5 text-xs rounded transition-colors bg-sky-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      Slot ({newSlotCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showWorked')}
      class={spotFilters.showWorked ? 'px-2 py-0.5 text-xs rounded transition-colors bg-cyan-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      Wkd ({workedCount})
    </button>
    
    <span class="text-slate-600 mx-1">|</span>
    <span class="text-xs font-bold text-slate-400 mr-2">MODE:</span>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showFT8')}
      class={spotFilters.showFT8 ? 'px-2 py-0.5 text-xs rounded transition-colors bg-teal-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      FT8 ({ft8Count})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showFT4')}
      class={spotFilters.showFT4 ? 'px-2 py-0.5 text-xs rounded transition-colors bg-teal-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      FT4 ({ft4Count})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showRTTY')}
      class={spotFilters.showRTTY ? 'px-2 py-0.5 text-xs rounded transition-colors bg-teal-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      RTTY ({rttyCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showSSB')}
      class={spotFilters.showSSB ? 'px-2 py-0.5 text-xs rounded transition-colors bg-amber-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      SSB ({ssbCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'showCW')}
      class={spotFilters.showCW ? 'px-2 py-0.5 text-xs rounded transition-colors bg-rose-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      CW ({cwCount})
    </button>

    <button 
      on:click={() => dispatch('toggleFilter', 'showPOTA')}
      class={spotFilters.showPOTA ? 'px-2 py-0.5 text-xs rounded transition-colors bg-emerald-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      🏕 POTA ({spots.filter(s => s.POTARef).length})
    </button>

    <button 
      on:click={() => dispatch('toggleFilter', 'showSOTA')}
      class={spotFilters.showSOTA ? 'px-2 py-0.5 text-xs rounded transition-colors bg-amber-600 text-white' : 'px-2 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      ⛰ SOTA ({spots.filter(s => s.SOTARef).length})
    </button>
    
    <span class="text-slate-600 mx-1">|</span>
    <span class="text-xs font-bold text-slate-400 mr-2">BAND:</span>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band160M')}
      class={spotFilters.band160M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      160 ({band160MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band80M')}
      class={spotFilters.band80M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      80 ({band80MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band60M')}
      class={spotFilters.band60M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      60 ({band60MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band40M')}
      class={spotFilters.band40M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      40 ({band40MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band30M')}
      class={spotFilters.band30M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      30 ({band30MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band20M')}
      class={spotFilters.band20M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      20 ({band20MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band17M')}
      class={spotFilters.band17M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      17 ({band17MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band15M')}
      class={spotFilters.band15M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      15 ({band15MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band12M')}
      class={spotFilters.band12M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      12 ({band12MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band10M')}
      class={spotFilters.band10M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      10 ({band10MCount})
    </button>
    
    <button 
      on:click={() => dispatch('toggleFilter', 'band6M')}
      class={spotFilters.band6M ? 'px-1.5 py-0.5 text-xs rounded transition-colors bg-indigo-600 text-white' : 'px-1.5 py-0.5 text-xs rounded transition-colors bg-slate-700/50 text-slate-300 hover:bg-slate-700'}>
      6 ({band6MCount})
    </button>
    {#if contestMode && (contestPrefix || (contestCallsigns && contestCallsigns.length > 0))}
        {@const contestSpots = spots.filter(s => {
          if (contestPrefix && s.DX.includes(contestPrefix)) return true;
          if (contestCallsigns && contestCallsigns.length > 0) {
            return contestCallsigns.some(cc => s.DX === cc || s.DX.startsWith(cc + '/'));
          }
          return false;
        })}
        <button 
          on:click={() => dispatch('toggleFilter', 'showContest')} 
          class="px-3 py-1.5 text-xs rounded transition-colors flex items-center gap-1 {spotFilters.showContest ? 'bg-yellow-600 text-white' : 'bg-yellow-600/20 text-yellow-400 hover:bg-yellow-600/30'}">
          <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
            <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/>
          </svg>
          Contest ({contestSpots.length})
        </button>
      {/if}
  </div>
</div>
<script>
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import { soundManager } from '../lib/soundManager.js';
  
  export let watchlist;
  export let spots;
  export let showOnlyActive = false;
  export let contestMode = false;
  export let contestPrefix = "";
  export let contestCallsigns = [];
  
  const dispatch = createEventDispatcher();
  
  let newCallsign = '';
  let watchlistSpots = [];
  let refreshInterval;
  let selectedBand = 'ALL'; // ✅ NOUVEAU : Filtre de bande
 
  $: if (!contestMode && selectedBand !== 'ALL') {
  selectedBand = 'ALL';
}
  let showOnlyNotWorked = false; // ✅ NOUVEAU : Filtre "Not Worked Only"

  // ✅ Liste des bandes disponibles
  const bands = ['ALL', '80M', '40M', '30M', '20M', 
               '17M', '15M', '12M', '10M'];
  
  // ✅ SIMPLIFIÉ : Utiliser directement la watchlist du backend (qui filtre déjà selon contest mode)
  $: displayList = getDisplayList(watchlist, watchlistSpots, showOnlyActive);
  $: matchingSpots = watchlistSpots.length;
  $: filteredSpots = selectedBand === 'ALL' 
    ? matchingSpots 
    : countWatchlistSpotsByBand(watchlistSpots, selectedBand, showOnlyNotWorked);

  $: if (watchlist.length > 0) {
    fetchWatchlistSpots();
  }
  
  $: if (spots.length > 0 && watchlist.length > 0) {
    fetchWatchlistSpots();
  }
  
  onMount(() => {
    refreshInterval = setInterval(() => {
      if (watchlist.length > 0) {
        fetchWatchlistSpots();
      }
    }, 10000);
    
    window.addEventListener('watchlistAlert', handleWatchlistAlert);
    
    // ✅ NOUVEAU : Écouter les changements de bande du FlexRadio
    window.addEventListener('flexBandChange', handleFlexBandChange);
  });
  
  onDestroy(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
    window.removeEventListener('watchlistAlert', handleWatchlistAlert);
    window.removeEventListener('flexBandChange', handleFlexBandChange);
  });
  
  function handleWatchlistAlert(event) {
    const { callsign, playSound } = event.detail;
    
    if (playSound) {
      soundManager.playWatchlistAlert('medium');
    }
    
    dispatch('toast', { 
      message: `🎯 ${callsign} spotted!`, 
      type: 'success'
    });
  }
  
  // ✅ NOUVEAU : Gérer les changements de bande du FlexRadio
  function handleFlexBandChange(event) {
    const { band, frequency } = event.detail;
    
    if (band && band !== 'ALL') {
      selectedBand = band;
      console.log(`FlexRadio band changed to ${band} (${frequency} MHz) - watchlist filter updated`);
    }
  }
  
  function getDisplayList(wl, wlSpots, activeOnly) {
    let list = wl;
    
    // ✅ NOUVEAU : Si une bande spécifique est sélectionnée, ne montrer que les callsigns avec des spots sur cette bande
    if (selectedBand !== 'ALL') {
      list = wl.filter(entry => {
        const spots = wlSpots.filter(s => 
          (s.dx === entry.callsign || s.dx.startsWith(entry.callsign)) &&
          s.band === selectedBand
        );
        return spots.length > 0;
      });
    } else if (activeOnly) {
      // Sinon, appliquer le filtre "Active Only" normal
      list = wl.filter(entry => {
        const spots = wlSpots.filter(s => s.dx === entry.callsign || s.dx.startsWith(entry.callsign));
        return spots.length > 0;
      });
    }
    
    // ✅ NOUVEAU : Filtre "Not Worked Only" - ne montrer que les callsigns avec au moins un spot non contacté
    if (showOnlyNotWorked) {
      list = list.filter(entry => {
        let spots = wlSpots.filter(s => s.dx === entry.callsign || s.dx.startsWith(entry.callsign));
        
        // Appliquer le filtre de bande si nécessaire
        if (selectedBand !== 'ALL') {
          spots = spots.filter(s => s.band === selectedBand);
        }
        
        // Vérifier s'il y a au moins un spot non contacté (workedBandMode = false)
        return spots.some(s => !s.workedBandMode);
      });
    }
    
    return [...list].sort((a, b) => a.callsign.localeCompare(b.callsign, 'en', { numeric: true }));
  }
  
  async function fetchWatchlistSpots() {
    try {
      const response = await fetch('/api/watchlist/spots');
      const json = await response.json();
      
      if (json.success) {
        watchlistSpots = json.data || [];
      }
    } catch (error) {
      console.error('Error fetching watchlist spots:', error);
    }
  }
  
function countWatchlistSpotsByBand(wlSpots, band, onlyNotWorked) {
  let spots = wlSpots.filter(s => s.band === band);
  
  // ✅ Si "Not Worked" activé, ne compter que les non contactés
  if (onlyNotWorked) {
    spots = spots.filter(s => !s.workedBandMode);
  }
  
  return spots.length;
}

  function getMatchingSpotsForCallsign(callsign) {
    let spots = watchlistSpots.filter(s => s.dx === callsign || s.dx.startsWith(callsign));
    
    // ✅ NOUVEAU : Filtrer par bande sélectionnée
    if (selectedBand !== 'ALL') {
      spots = spots.filter(s => s.band === selectedBand);
    }
    
    const bandOrder = { '160M': 0, '80M': 1, '60M': 2, '40M': 3, '30M': 4, '20M': 5, '17M': 6, '15M': 7, '12M': 8, '10M': 9, '6M': 10 };
    const modeOrder = { 'CW': 0, 'SSB': 1, 'USB': 1, 'LSB': 1, 'RTTY': 2, 'FT4': 3, 'FT8': 4, 'FM': 5 };
    
    return spots.sort((a, b) => {
      const bandA = bandOrder[a.band] ?? 99;
      const bandB = bandOrder[b.band] ?? 99;
      if (bandA !== bandB) return bandA - bandB;
      
      const modeA = modeOrder[a.mode] ?? 99;
      const modeB = modeOrder[b.mode] ?? 99;
      if (modeA !== modeB) return modeA - modeB;
      
      const freqA = parseFloat(a.frequencyMhz) || 0;
      const freqB = parseFloat(b.frequencyMhz) || 0;
      if (freqA !== freqB) return freqA - freqB;
      
      const timeA = a.utcTime || "00:00";
      const timeB = b.utcTime || "00:00";
      return timeB.localeCompare(timeA);
    });
  }
  
  async function addToWatchlist() {
    const callsign = newCallsign.trim().toUpperCase();
    
    if (!callsign) {
      dispatch('toast', { message: 'Please enter a callsign', type: 'warning' });
      return;
    }
    
    try {
      const response = await fetch('/api/watchlist/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ callsign })
      });
      
      const data = await response.json();
      if (data.success) {
        newCallsign = '';
        dispatch('toast', { message: `${callsign} added to watchlist`, type: 'success' });
        await fetchWatchlistSpots();
      } else {
        dispatch('toast', { message: data.error || 'Failed to add callsign', type: 'error' });
      }
    } catch (error) {
      console.error('Error adding to watchlist:', error);
      dispatch('toast', { message: `Error: ${error.message}`, type: 'error' });
    }
  }
  
  async function removeFromWatchlist(callsign) {
    try {
      const response = await fetch('/api/watchlist/remove', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ callsign })
      });
      
      const data = await response.json();
      if (data.success) {
        dispatch('toast', { message: `${callsign} removed from watchlist`, type: 'success' });
        await fetchWatchlistSpots();
      } else {
        dispatch('toast', { message: 'Failed to remove callsign', type: 'error' });
      }
    } catch (error) {
      console.error('Error removing from watchlist:', error);
      dispatch('toast', { message: `Error: ${error.message}`, type: 'error' });
    }
  }
  
  async function updateSound(callsign, playSound) {
    try {
      const response = await fetch('/api/watchlist/update-sound', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ callsign, playSound })
      });
      
      const data = await response.json();
      if (data.success) {
        dispatch('toast', { message: `Sound ${playSound ? 'enabled' : 'disabled'}`, type: 'success' });
      }
    } catch (error) {
      console.error('Error updating sound:', error);
    }
  }
  
  function handleKeyPress(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      addToWatchlist();
    }
  }
  
  function sendSpot(spot) {
    const event = new CustomEvent('sendSpot', {
      detail: {
        callsign: spot.dx,
        frequency: spot.frequencyMhz,
        mode: spot.mode
      }
    });
    window.dispatchEvent(event);
  }
  
  function isContestCallsign(callsign) {
    if (!contestMode) return false;
    
    if (contestPrefix && callsign.includes(contestPrefix)) {
      return true;
    }
    
    if (contestCallsigns && contestCallsigns.length > 0) {
      for (const contestCall of contestCallsigns) {
        if (callsign === contestCall || callsign.startsWith(contestCall + '/')) {
          return true;
        }
      }
    }
    
    return false;
  }
  
  function toggleActiveOnly() {
    showOnlyActive = !showOnlyActive;
  }
  
  function toggleNotWorked() {
    showOnlyNotWorked = !showOnlyNotWorked;
  }
</script>

<div class="h-full flex flex-col" style="height: 100%; max-height: 100%;">
  <div class="p-3 border-b border-slate-700/50 flex-shrink-0">
    <div class="flex items-center justify-between mb-2">
      <h2 class="text-lg font-bold">Watchlist</h2>
      <div class="flex gap-2">
        {#if contestMode && (contestPrefix || (contestCallsigns && contestCallsigns.length > 0))}
          <span class="px-2 py-1 bg-yellow-600/20 text-yellow-400 rounded text-xs font-semibold flex items-center gap-1">
            <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
              <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/>
            </svg>
            {contestPrefix || 'Contest'}
          </span>
        {/if}
        
        <button 
          on:click={toggleActiveOnly}
          class="px-3 py-1.5 text-xs rounded transition-colors flex items-center gap-2 {showOnlyActive ? 'bg-blue-600 text-white' : 'bg-slate-700/50 text-slate-300 hover:bg-slate-700'}">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"></path>
          </svg>
          {showOnlyActive ? 'Show All' : 'Active Only'}
        </button>
        
        <!-- ✅ NOUVEAU : Bouton Not Worked Only -->
        <button 
          on:click={toggleNotWorked}
          class="px-3 py-1.5 text-xs rounded transition-colors flex items-center gap-2 {showOnlyNotWorked ? 'bg-orange-600 text-white' : 'bg-slate-700/50 text-slate-300 hover:bg-slate-700'}">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
          </svg>
          {showOnlyNotWorked ? 'Show All' : 'Not Worked'}
        </button>
      </div>
    </div>
    <div class="flex items-center gap-2 mb-3">
      <div class="flex items-center gap-1.5 text-xs">
        <span class="text-slate-400">Spots:</span>
        <span class="px-2 py-0.5 bg-slate-700/50 text-white rounded font-semibold">
          {matchingSpots}
        </span>
        {#if selectedBand !== 'ALL' && filteredSpots !== matchingSpots}
          <span class="text-slate-500">•</span>
          <span class="text-slate-400">{selectedBand}:</span>
          <span class="px-2 py-0.5 bg-blue-600/20 text-blue-400 rounded font-semibold border border-blue-500/30">
            {filteredSpots}
        </span>
        {/if}
      </div>
    </div>
    
    {#if contestMode}
      <div class="mb-3">
        <label for="band-filter" class="text-xs text-slate-400 mb-1 block">Filter by Band</label>
        <select 
          id="band-filter"
          bind:value={selectedBand}
          class="w-full px-3 py-2 bg-slate-700/50 border border-slate-600 rounded text-sm text-white focus:outline-none focus:border-blue-500">
          {#each bands as band}
            <option value={band}>{band === 'ALL' ? 'All Bands' : band}</option>
          {/each}
        </select>
      </div>
    {/if}
    
    <div class="flex gap-2">
      <input 
        type="text" 
        bind:value={newCallsign}
        on:keypress={handleKeyPress}
        placeholder="Enter callsign or prefix (e.g., VK9)"
        class="flex-1 px-3 py-2 bg-slate-700/50 border border-slate-600 rounded text-sm text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
      />
      <button 
        on:click={addToWatchlist}
        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm font-medium transition-colors">
        Add
      </button>
    </div>
  </div>
  
  <div class="flex-1 p-3 overflow-y-auto" style="overflow-y: auto; min-height: 0; flex: 1 1 0;">
    {#if displayList.length === 0}
      <div class="text-center py-8 text-slate-400">
        <svg class="w-12 h-12 mx-auto mb-3 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
        </svg>
        <p class="text-sm">{showOnlyActive ? 'No active spots for watchlist callsigns' : 'No callsigns in watchlist'}</p>
        <p class="text-xs mt-1">{showOnlyActive ? 'Click "Active Only" to see all entries' : 'Add callsigns or prefixes to monitor'}</p>
      </div>
    {:else}
      {#each displayList as entry}
        {@const matchingSpots = getMatchingSpotsForCallsign(entry.callsign)}
        {@const count = matchingSpots.length}
        {@const neededCount = matchingSpots.filter(s => !s.workedBandMode).length}
        {@const borderClass = neededCount > 0 ? 'border-orange-500/30' : 'border-slate-700/50'}
        {@const isContest = entry.isContest || false}
        
        <div class="mb-3 p-3 bg-slate-900/30 rounded hover:bg-slate-700/30 transition-colors border {borderClass}">
          <div class="flex items-center justify-between mb-2">
            <div class="flex-1">
              <div class="flex items-center gap-2 flex-wrap">
                <div class="font-bold text-pink-400 text-lg">{entry.callsign}</div>
                
                {#if isContest}
                  <span class="px-1.5 py-0.5 bg-yellow-600/20 text-yellow-400 rounded text-xs font-semibold" title="Contest station - daily contacts allowed">
                    🏆 Contest
                  </span>
                {/if}
                
                {#if entry.playSound}
                  <span class="text-xs" title="Sound enabled">🔊</span>
                {/if}
                
                {#if count > 0}
                  <span class="text-xs text-slate-400">{count} active spot{count !== 1 ? 's' : ''}</span>
                  {#if neededCount > 0}
                    <span class="px-1.5 py-0.5 bg-orange-500/20 text-orange-400 rounded text-xs font-semibold">
                      {isContest ? `${neededCount} today` : `${neededCount} needed`}
                    </span>
                  {:else}
                    <span class="px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded text-xs font-semibold">
                      {isContest ? 'Worked today' : 'All worked'}
                    </span>
                  {/if}
                {:else}
                  <span class="text-xs text-slate-500">No active spots</span>
                {/if}
                
                {#if entry.lastSeenStr && entry.lastSeenStr !== 'Never'}
                  <span class="text-xs text-slate-500">• {entry.lastSeenStr}</span>
                {/if}
                
                {#if entry.spotCount > 0}
                  <span class="text-xs text-slate-600">• {entry.spotCount} total spot{entry.spotCount !== 1 ? 's' : ''}</span>
                {/if}
              </div>
            </div>
            
            <div class="flex gap-1">
              <button
                on:click={() => updateSound(entry.callsign, !entry.playSound)}
                class="px-2 py-1 text-xs rounded transition-colors {entry.playSound ? 'bg-blue-600/20 text-blue-400' : 'bg-slate-700/50 text-slate-400'}"
                title="{entry.playSound ? 'Disable' : 'Enable'} sound">
                {entry.playSound ? '🔊' : '🔇'}
              </button>
              
              <button 
                on:click={() => removeFromWatchlist(entry.callsign)}
                title="Remove from watchlist"
                class="px-2 py-1 text-xs bg-red-600/20 hover:bg-red-600/40 text-red-400 rounded transition-colors">
                Remove
              </button>
            </div>
          </div>
          
          {#if count > 0}
            <div class="mt-2 space-y-1 max-h-48 overflow-y-auto" style="overflow-y: auto;">
              {#each matchingSpots.slice(0, 10) as spot}
                <button 
                  on:click={() => sendSpot(spot)}
                  class="w-full flex items-center justify-between p-2 bg-slate-800/50 rounded text-xs hover:bg-slate-700/50 transition-colors {!spot.workedBandMode ? 'border-l-2 border-orange-500' : ''}"
                  title="Click to send to Log4OM and tune radio">
                  <div class="flex items-center gap-2 flex-1 min-w-0">
                    {#if spot.workedBandMode}
                      <svg class="w-4 h-4 text-green-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"></path>
                      </svg>
                    {:else}
                      <svg class="w-4 h-4 text-orange-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>
                      </svg>
                    {/if}
                    <span class="font-bold text-blue-400">{spot.dx}</span>
                    <span class="text-slate-400 text-xs truncate" style="max-width: 120px;" title="{spot.countryName || 'Unknown'}">{spot.countryName || 'Unknown'}</span>
                    <span class="px-1.5 py-0.5 bg-slate-700/50 rounded flex-shrink-0">{spot.band}</span>
                    <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded flex-shrink-0">{spot.mode}</span>
                    <span class="text-slate-400 font-mono truncate">{spot.frequencyMhz}</span>
                  </div>
                  <div class="flex items-center gap-2 flex-shrink-0 ml-2">
                    {#if spot.workedBandMode}
                      <span class="px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded text-xs font-semibold">
                        {isContest ? 'Today ✓' : 'Worked'}
                      </span>
                    {:else if spot.newDXCC}
                      <span class="px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded text-xs font-semibold">New DXCC!</span>
                    {:else if spot.newBand && spot.newMode}
                      <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs font-semibold">New B&M!</span>
                    {:else if spot.newBand}
                      <span class="px-1.5 py-0.5 bg-yellow-500/20 text-yellow-400 rounded text-xs font-semibold">New Band!</span>
                    {:else if spot.newMode}
                      <span class="px-1.5 py-0.5 bg-orange-500/20 text-orange-400 rounded text-xs font-semibold">New Mode!</span>
                    {:else}
                      <span class="px-1.5 py-0.5 bg-orange-500/20 text-orange-400 rounded text-xs font-semibold">
                        {isContest ? 'Work Today!' : 'Needed!'}
                      </span>
                    {/if}
                    <span class="text-slate-500">{spot.utcTime}</span>
                  </div>
                </button>
              {/each}
            </div>
          {:else}
            <div class="mt-2 text-xs text-slate-500 text-center py-2 bg-slate-800/30 rounded">No active spots</div>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>
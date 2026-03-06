<script>
  import { createEventDispatcher } from 'svelte';
  import VirtualList from 'svelte-virtual-list';
  
  export let spots;
  export let watchlist;
  export let myCallsign;
  
  const dispatch = createEventDispatcher();
  
  let container;
  const itemHeight = 43;

  // Tri
  let sortCol = null;   // 'dx' | 'country' | 'mode' | null
  let sortDir = 'asc';  // 'asc' | 'desc'

  function toggleSort(col) {
    if (sortCol === col) {
      if (sortDir === 'asc') {
        sortDir = 'desc';
      } else {
        // 3ème clic : reset
        sortCol = null;
        sortDir = 'asc';
      }
    } else {
      sortCol = col;
      sortDir = 'asc';
    }
  }

  $: sortedSpots = (() => {
    if (!sortCol) return spots;
    return [...spots].sort((a, b) => {
      let valA, valB;
      if (sortCol === 'dx')      { valA = a.DX || '';          valB = b.DX || ''; }
      if (sortCol === 'country') { valA = a.CountryName || ''; valB = b.CountryName || ''; }
      if (sortCol === 'mode')    { valA = a.Mode || '';        valB = b.Mode || ''; }
      const cmp = valA.localeCompare(valB);
      return sortDir === 'asc' ? cmp : -cmp;
    });
  })();
  
  function handleSpotClick(spot) {
    dispatch('clickSpot', {
      callsign: spot.DX,
      frequency: spot.FrequencyMhz,
      mode: spot.Mode
    });
  }
  
  // Retourne un tableau de { label, classes } pour afficher plusieurs badges
  function getStatusBadges(spot) {
    const badges = [];

    const inWatchlist = watchlist.some(entry =>
      spot.DX === entry.callsign || spot.DX.startsWith(entry.callsign)
    );

    if (spot.DX === myCallsign) {
      badges.push({ label: 'My Call', classes: 'bg-red-500/20 text-red-400 border-red-500/50' });
      return badges;
    }

    if (inWatchlist) {
      badges.push({ label: 'Watchlist', classes: 'bg-pink-500/20 text-pink-400 border-pink-500/50' });
    }

    if (spot.NewDXCC) {
      badges.push({ label: 'New DXCC', classes: 'bg-green-500/20 text-green-400 border-green-500/50' });
    } else if (spot.NewBand && spot.NewMode) {
      badges.push({ label: 'New B&M', classes: 'bg-purple-500/20 text-purple-400 border-purple-500/50' });
    } else if (spot.NewBand) {
      badges.push({ label: 'New Band', classes: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50' });
    } else if (spot.NewMode) {
      badges.push({ label: 'New Mode', classes: 'bg-orange-500/20 text-orange-400 border-orange-500/50' });
    } else if (spot.NewSlot) {
      badges.push({ label: 'New Slot', classes: 'bg-sky-500/20 text-sky-400 border-sky-500/50' });
    } else if (spot.Worked && !inWatchlist) {
      badges.push({ label: 'Worked', classes: 'bg-cyan-500/20 text-cyan-400 border-cyan-500/50' });
    }

    return badges;
  }
  
  function getCleanComment(spot) {
    if (!spot.OriginalComment) return '';
    return spot.OriginalComment.trim();
  }
</script>

<div class="bg-slate-800/50 backdrop-blur rounded-lg border border-slate-700/50 flex flex-col overflow-hidden h-full">
  <div class="p-3 border-b border-slate-700/50 flex items-center justify-between flex-shrink-0">
    <h2 class="text-lg font-bold">Recent Spots (<span>{spots.length}</span>)</h2>
  </div>
  
  <!-- Header fixe -->
  <div class="bg-slate-900/50 flex-shrink-0">
    <div class="flex text-left text-xs text-slate-400 font-semibold">
      <button class="p-2 flex items-center gap-1 hover:text-white transition-colors" style="width: 10%;" on:click={() => toggleSort('dx')}>
        DX
        {#if sortCol === 'dx'}
          <span class="text-blue-400">{sortDir === 'asc' ? '↑' : '↓'}</span>
        {:else}
          <span class="text-slate-600">↕</span>
        {/if}
      </button>
      <button class="p-2 flex items-center gap-1 hover:text-white transition-colors" style="width: 18%;" on:click={() => toggleSort('country')}>
        Country
        {#if sortCol === 'country'}
          <span class="text-blue-400">{sortDir === 'asc' ? '↑' : '↓'}</span>
        {:else}
          <span class="text-slate-600">↕</span>
        {/if}
      </button>
      <div class="p-2" style="width: 7%;">Time</div>
      <div class="p-2" style="width: 10%;">Freq</div>
      <div class="p-2" style="width: 7%;">Band</div>
      <button class="p-2 flex items-center gap-1 hover:text-white transition-colors" style="width: 7%;" on:click={() => toggleSort('mode')}>
        Mode
        {#if sortCol === 'mode'}
          <span class="text-blue-400">{sortDir === 'asc' ? '↑' : '↓'}</span>
        {:else}
          <span class="text-slate-600">↕</span>
        {/if}
      </button>
      <div class="p-2" style="width: 10%;">Spotter</div>
      <div class="p-2" style="width: 18%;">Comment</div>
      <div class="p-2" style="width: 13%;">Status</div>
    </div>
  </div>
  
  <!-- Liste virtualisée -->
  <div class="flex-1 overflow-hidden" bind:this={container}>
    <VirtualList items={sortedSpots} {itemHeight} let:item>
      <div class="flex border-b border-slate-700/30 hover:bg-slate-700/30 transition-colors text-sm" style="height: {itemHeight}px;">
        <div class="p-2 flex items-center" style="width: 10%;">
          <button
            class="font-bold text-blue-400 hover:text-blue-300 transition-colors truncate w-full text-left"
            on:click={() => handleSpotClick(item)}
            title="Click to send to Log4OM and tune radio">
            {item.DX}
          </button>
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs truncate" style="width: 18%;" title={item.CountryName || 'N/A'}>
          {item.CountryName || 'N/A'}
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs" style="width: 7%;">{item.UTCTime}</div>
        <div class="p-2 flex items-center font-mono text-xs" style="width: 10%;">{item.FrequencyMhz}</div>
        <div class="p-2 flex items-center" style="width: 7%;">
          <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-xs">{item.Band}</span>
        </div>
        <div class="p-2 flex items-center" style="width: 7%;">
          <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs">{item.Mode}</span>
        </div>
        <div class="p-2 flex items-center text-slate-300 text-xs truncate" style="width: 10%;" title={item.SpotterCallsign}>
          {item.SpotterCallsign}
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs truncate" style="width: 18%;" title={getCleanComment(item)}>
          {getCleanComment(item)}
        </div>
        <div class="p-2 flex items-center gap-1 flex-wrap" style="width: 13%;">
          {#each getStatusBadges(item) as badge}
            <span class="px-1.5 py-0.5 rounded text-xs font-semibold border {badge.classes} whitespace-nowrap">
              {badge.label}
            </span>
          {/each}
        </div>
      </div>
    </VirtualList>
  </div>
</div>
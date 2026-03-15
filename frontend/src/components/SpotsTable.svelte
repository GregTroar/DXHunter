<script>
  import { createEventDispatcher } from 'svelte';
  import VirtualList from 'svelte-virtual-list';
  
  export let spots;
  export let watchlist;
  export let myCallsign;
  
  const dispatch = createEventDispatcher();
  
  let container;
  const itemHeight = 43;

  // ─── Tri colonnes ───────────────────────────────────────────
  let sortCol = '';   // 'DX' | 'Country' | 'Mode' | ''
  let sortDir = 0;    // 0 = none, 1 = asc, -1 = desc

  function toggleSort(col) {
    if (sortCol !== col) {
      sortCol = col;
      sortDir = 1;
    } else if (sortDir === 1) {
      sortDir = -1;
    } else {
      sortCol = '';
      sortDir = 0;
    }
  }

  $: indDX      = (sortCol === 'DX'      && sortDir !== 0) ? (sortDir === 1 ? '↑' : '↓') : '↕';
  $: indCountry = (sortCol === 'Country' && sortDir !== 0) ? (sortDir === 1 ? '↑' : '↓') : '↕';
  $: indFreq    = (sortCol === 'Freq'    && sortDir !== 0) ? (sortDir === 1 ? '↑' : '↓') : '↕';
  $: indBand    = (sortCol === 'Band'    && sortDir !== 0) ? (sortDir === 1 ? '↑' : '↓') : '↕';
  $: indMode    = (sortCol === 'Mode'    && sortDir !== 0) ? (sortDir === 1 ? '↑' : '↓') : '↕';

  $: sortedSpots = (() => {
    if (!sortCol || sortDir === 0) return spots;
    return [...spots].sort((a, b) => {
      let va = '', vb = '';
      if (sortCol === 'DX')      { va = a.DX || ''; vb = b.DX || ''; }
      if (sortCol === 'Country') { va = a.CountryName || ''; vb = b.CountryName || ''; }
      if (sortCol === 'Mode')    { va = a.Mode || ''; vb = b.Mode || ''; }
      if (sortCol === 'Band')    { va = a.Band || ''; vb = b.Band || ''; }
      if (sortCol === 'Freq') {
        return sortDir * ((parseFloat(a.FrequencyMhz) || 0) - (parseFloat(b.FrequencyMhz) || 0));
      }
      return sortDir * va.localeCompare(vb);
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

    if (spot.POTARef) {
      badges.push({ label: `🏕 ${spot.POTARef}`, classes: 'bg-emerald-500/20 text-emerald-400 border-emerald-500/50' });
    }

    if (spot.SOTARef) {
      badges.push({ label: `⛰ ${spot.SOTARef}`, classes: 'bg-amber-500/20 text-amber-400 border-amber-500/50' });
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
      <button class="p-2 select-none text-left text-slate-400 hover:text-slate-200" style="width: 10%;" on:click={() => toggleSort('DX')}>DX <span class={sortCol === 'DX' ? 'text-white' : 'text-slate-500'}>{indDX}</span></button>
      <button class="p-2 select-none text-left text-slate-400 hover:text-slate-200" style="width: 10%;" on:click={() => toggleSort('Country')}>Country <span class={sortCol === 'Country' ? 'text-white' : 'text-slate-500'}>{indCountry}</span></button>
      <div class="p-2" style="width: 7%;">Time</div>
      <button class="p-2 select-none text-left text-slate-400 hover:text-slate-200" style="width: 8%;" on:click={() => toggleSort('Freq')}>Freq <span class={sortCol === 'Freq' ? 'text-white' : 'text-slate-500'}>{indFreq}</span></button>
      <button class="p-2 select-none text-left text-slate-400 hover:text-slate-200" style="width: 7%;" on:click={() => toggleSort('Band')}>Band <span class={sortCol === 'Band' ? 'text-white' : 'text-slate-500'}>{indBand}</span></button>
      <button class="p-2 select-none text-left text-slate-400 hover:text-slate-200" style="width: 7%;" on:click={() => toggleSort('Mode')}>Mode <span class={sortCol === 'Mode' ? 'text-white' : 'text-slate-500'}>{indMode}</span></button>
      <div class="p-2" style="width: 8%;">Spotter</div>
      <div class="p-2" style="width: 22%;">Comment</div>
      <div class="p-2" style="width: 13%;">Status</div>
      <div class="p-2" style="width: 8%;">Cluster</div>
    </div>
  </div>
  
  <!-- Liste virtualisée -->
  <div class="flex-1 overflow-hidden" bind:this={container}>
    <VirtualList items={sortedSpots} {itemHeight} let:item>
      <div class="flex border-b border-slate-700/30 hover:bg-slate-700/30 transition-colors text-sm" style="height: {itemHeight}px;">
        <div class="p-2 flex items-center" style="width: 10%;">
          <button
            class="font-bold text-slate-200 hover:text-white transition-colors truncate w-full text-left"
            on:click={() => handleSpotClick(item)}
            title="Click to send to Log4OM and tune radio">
            {item.DX}
          </button>
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs truncate" style="width: 10%;" title={item.CountryName || 'N/A'}>
          {item.CountryName || 'N/A'}
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs" style="width: 7%;">{item.UTCTime}</div>
        <div class="p-2 flex items-center font-mono text-xs" style="width: 8%;">{item.FrequencyMhz}</div>
        <div class="p-2 flex items-center" style="width: 7%;">
          <span class="px-1.5 py-0.5 bg-slate-700/50 rounded text-xs">{item.Band}</span>
        </div>
        <div class="p-2 flex items-center" style="width: 7%;">
          <span class="px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded text-xs">{item.Mode}</span>
        </div>
        <div class="p-2 flex items-center text-slate-300 text-xs truncate" style="width: 8%;" title={item.SpotterCallsign}>
          {item.SpotterCallsign}
        </div>
        <div class="p-2 flex items-center text-slate-400 text-xs truncate" style="width: 22%;" title={getCleanComment(item)}>
          {getCleanComment(item)}
        </div>
        <div class="p-2 flex items-center gap-1 flex-wrap" style="width: 13%;">
          {#each getStatusBadges(item) as badge}
            <span class="px-1.5 py-0.5 rounded text-xs font-semibold border {badge.classes} whitespace-nowrap">
              {badge.label}
            </span>
          {/each}
        </div>
        <div class="p-2 flex items-center" style="width: 8%;">
          <span class="px-1.5 py-0.5 bg-cyan-500/15 text-cyan-400 border border-cyan-500/30 rounded text-xs truncate" title={item.ClusterName}>
            {item.ClusterName || ''}
          </span>
        </div>
      </div>
    </VirtualList>
  </div>
</div>
<script>
  export let spots;
  
  const BANDS = ['160M', '80M', '60M', '40M', '30M', '20M', '17M', '15M', '12M', '10M', '6M'];
  const BAND_COLORS = {
    '160M': '#ef4444',
    '80M': '#f97316',
    '60M': '#f59e0b',
    '40M': '#eab308',
    '30M': '#84cc16',
    '20M': '#22c55e',
    '17M': '#10b981',
    '15M': '#14b8a6',
    '12M': '#06b6d4',
    '10M': '#0ea5e9',
    '6M': '#3b82f6'
  };
  
  $: bandStats = getBandStats(spots);
  $: maxSpots = Math.max(...Object.values(bandStats), 1);
  
  function getBandStats(allSpots) {
    const stats = {};
    BANDS.forEach(band => {
      stats[band] = allSpots.filter(s => s.Band === band).length;
    });
    return stats;
  }
</script>

<div class="h-full overflow-y-auto">
 
  <div class="p-3 border-t border-slate-700/50">
    <h2 class="text-lg font-bold mb-3">Band Propagation</h2>
  </div>
  
  <div class="px-3 pb-3 space-y-2">
    {#each BANDS as band}
      {@const count = bandStats[band]}
      {@const percentage = maxSpots > 0 ? (count / maxSpots) * 100 : 0}
      {@const color = BAND_COLORS[band]}
      
      <div class="cursor-pointer hover:opacity-80 transition-opacity">
        <div class="flex items-center justify-between mb-1">
          <span class="text-xs font-semibold" style="color: {color}">{band}</span>
          <span class="text-xs text-slate-400">{count} spots</span>
        </div>
        <div class="w-full bg-slate-700/30 rounded-full h-2 overflow-hidden">
          <div 
            class="h-full rounded-full transition-all duration-500" 
            style="width: {percentage}%; background-color: {color}">
          </div>
        </div>
      </div>
    {/each}
  </div>
</div>
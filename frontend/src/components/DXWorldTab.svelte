<script>
  export let news = [];

  let search = '';

  $: filtered = news.filter(n => {
    if (!search) return true;
    const s = search.toUpperCase();
    return n.title?.toUpperCase().includes(s) || n.excerpt?.toUpperCase().includes(s);
  });

  function formatDate(d) { return d || ''; }

  function tagStyle(tag) {
    if (!tag) return null;
    if (tag.includes('NEW ACTIVITY') || tag.includes('ACTIVITY'))
      return { cls: 'bg-green-500/20 text-green-300 border-green-500/40', label: 'NEW ACTIVITY' };
    if (tag.includes('UPDATE'))
      return { cls: 'bg-amber-500/20 text-amber-300 border-amber-500/40', label: 'UPDATE' };
    if (tag.includes('NEWS'))
      return { cls: 'bg-blue-500/20 text-blue-300 border-blue-500/40', label: 'NEWS' };
    return { cls: 'bg-slate-600/40 text-slate-400 border-slate-500/40', label: tag };
  }
</script>

<div class="flex flex-col h-full overflow-hidden text-sm">

  <!-- Toolbar -->
  <div class="flex items-center gap-2 px-3 py-2 border-b border-slate-700/50 flex-shrink-0">
    <span class="text-slate-400 font-semibold text-xs">DX-World News</span>
    <input
      bind:value={search}
      placeholder="Search…"
      class="ml-auto w-28 px-2 py-0.5 text-xs bg-slate-900/60 border border-slate-700 rounded text-slate-300 placeholder-slate-600 focus:outline-none focus:border-slate-500"
    />
  </div>

  <!-- News list -->
  <div class="flex-1 overflow-y-auto px-2 py-2 space-y-2">
    {#if filtered.length === 0}
      <div class="flex items-center justify-center h-20 text-slate-500 text-xs">
        {news.length === 0 ? 'Loading news…' : 'No results'}
      </div>
    {:else}
      {#each filtered as item}
        <div class="rounded-lg overflow-hidden border border-slate-700/50 border-l-2 border-l-blue-500/50 bg-slate-800/40 hover:bg-slate-800/70 transition-colors">

          <div class="flex items-start gap-2 px-3 py-2">
            <!-- Thumbnail -->
            {#if item.imageUrl}
              <img
                src={item.imageUrl}
                alt=""
                class="w-14 h-14 object-cover rounded flex-shrink-0 mt-0.5"
                loading="lazy"
                on:error={(e) => e.target.style.display='none'}
              />
            {/if}

            <div class="flex-1 min-w-0">
              <!-- Title -->
              <a
                href={item.link}
                target="_blank"
                rel="noreferrer"
                class="font-semibold text-blue-300 hover:text-blue-200 transition-colors leading-snug block">
                {item.title}
              </a>

              <!-- Meta -->
              <div class="flex items-center gap-2 mt-0.5 text-[10px] text-slate-500 flex-wrap">
                {#if item.tag}
                  {@const ts = tagStyle(item.tag)}
                  <span class="px-1.5 py-0 rounded border text-[10px] font-semibold {ts.cls}">{ts.label}</span>
                {/if}
                <span>📅 {formatDate(item.pubDate)}</span>
                {#if item.creator}
                  <span>· {item.creator}</span>
                {/if}
              </div>

              <!-- Excerpt -->
              {#if item.excerpt}
                <p class="mt-1 text-xs text-slate-400 leading-relaxed line-clamp-2">{item.excerpt}</p>
              {/if}
            </div>
          </div>

        </div>
      {/each}
    {/if}
  </div>

  <!-- Footer -->
  <div class="px-3 py-1.5 border-t border-slate-700/50 flex-shrink-0 flex items-center justify-between">
    <span class="text-xs text-slate-500">{filtered.length} articles · updated every 30 min</span>
    <a href="https://dx-world.net" target="_blank" rel="noreferrer"
      class="text-xs text-slate-600 hover:text-slate-400 transition-colors">dx-world.net</a>
  </div>
</div>

<script>

  export let ftxEnabled = false;
  export let ftxDecodes = [];   // maintained by App.svelte — persists across tab switches
  export let watchlist = [];
  export let spots = [];

  let filterCQOnly = false;
  let filterMyCall = false;
  let showSlotAdvisor = false;
  let isDXpedition = true;

  let clearedAt = 0;

  function clearDisplay() {
    clearedAt = Date.now();
  }

  async function toggleEnabled() {
    await fetch('/api/ftx/toggle', { method: 'POST' });
  }

  let haltBusy = false;
  let haltOk = false;

  async function haltTX() {
    haltBusy = true;
    haltOk = false;
    try {
      await fetch('/api/ftx/halttx', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ clientId: 'MSHV', autoOnly: false })
      });
      haltOk = true;
      setTimeout(() => haltOk = false, 1500);
    } finally {
      haltBusy = false;
    }
  }

  const cols = '50px 34px 36px 52px 48px 36px minmax(0,2fr) minmax(0,1.5fr) 130px';

  const SIG_WIDTH = 60;  // Hz — practical FT8 signal width

  // Passband: DXpeditions listen 1000-3000 Hz, normal QSO 0-3000 Hz
  $: DF_MIN   = isDXpedition ? 1000 : 0;
  $: DF_MAX   = 3000;
  $: DF_RANGE = DF_MAX - DF_MIN;

  $: visibleDecodes = clearedAt > 0
    ? ftxDecodes.filter(d => d.receivedAt > clearedAt)
    : ftxDecodes;

  $: displayed = visibleDecodes.filter(d => {
    if (filterCQOnly && !d.isCQ) return false;
    if (filterMyCall && !d.myCall) return false;
    return true;
  });

  // Group by period (same time = same period), newest first
  $: groupedDecodes = (() => {
    const groups = [];
    let current = null;
    for (const d of displayed) {
      if (!current || d.time !== current.time) {
        current = { time: d.time, decodes: [] };
        groups.push(current);
      }
      current.decodes.push(d);
    }
    return groups;
  })();

  $: lastPeriodCount = groupedDecodes.length > 0 ? groupedDecodes[0].decodes.length : 0;

  // ── TX Slot Advisor ─────────────────────────────────────────────────────────
  // Cherche les espaces entre signaux consécutifs ≥ SIG_WIDTH Hz.
  // Calculé uniquement si le panel est ouvert — pas de ressource gaspillée sinon.
  $: slotAnalysis = (() => {
    if (!showSlotAdvisor) return null;

    // Uniquement la dernière période reçue (index 0).
    const recent = groupedDecodes[0]?.decodes || [];
    if (recent.length === 0) return null;

    // DFs triés dans le passband utile
    const dfs = [...new Set(
      recent.map(d => d.df).filter(df => df >= DF_MIN && df <= DF_MAX)
    )].sort((a, b) => a - b);

    if (dfs.length === 0) return null;

    // Tous les gaps entre signaux consécutifs (+ bords du passband), sans filtre minimum.
    // Sur bande saturée on veut quand même proposer quelque chose.
    const allEdges = [DF_MIN, ...dfs, DF_MAX];
    const gaps = [];
    for (let i = 0; i < allEdges.length - 1; i++) {
      const lo   = allEdges[i];
      const hi   = allEdges[i + 1];
      const size = hi - lo;
      if (size > 0) {
        // ok: gap suffisant pour un signal FT8 (≥ SIG_WIDTH)
        // tight: gap serré mais utilisable (≥ 30 Hz)
        // busy: bande vraiment saturée, risque de collision
        const quality = size >= SIG_WIDTH ? 'ok' : size >= 30 ? 'tight' : 'busy';
        gaps.push({ lo, hi, size, df: Math.round((lo + hi) / 2), quality });
      }
    }

    // Le DX décode tout 1000-3000 Hz : plus grand gap = moins de QRM = meilleur décodage.
    gaps.sort((a, b) => b.size - a.size);

    return { dfs, gaps, best: gaps[0] || null };
  })();

  // Convert Hz to % position in the spectrum bar
  function dfPct(hz) { return ((hz - DF_MIN) / DF_RANGE) * 100; }

  // ── Watchlist active+not-worked match ───────────────────────────────────────
  // spots are TelnetSpot (no json tags) → fields are DX/NewDXCC/NewBand/etc. (PascalCase).
  // workedBandMode doesn't exist on TelnetSpot; use NewXxx flags + CallsignWorked instead.
  $: activeWatchlistCalls = (() => {
    const wlCalls = new Set(watchlist.map(w => w.callsign?.toUpperCase()).filter(Boolean));
    if (wlCalls.size === 0 || spots.length === 0) return new Set();
    const active = new Set();
    for (const s of spots) {
      const dx = (s.DX || s.dx || '').toUpperCase();
      if (!wlCalls.has(dx)) continue;
      // Needed = new on at least one dimension, or never worked at all
      if (s.NewDXCC || s.NewBand || s.NewMode || s.NewSlot || !s.CallsignWorked) {
        active.add(dx);
      }
    }
    return active;
  })();

  function isActiveWatchlist(decode) {
    if (activeWatchlistCalls.size === 0) return false;
    const call = (decode.dxCall || '').toUpperCase();
    return call !== '' && activeWatchlistCalls.has(call);
  }

  // ── Helpers ─────────────────────────────────────────────────────────────────
  async function sendReply(decode) {
    try {
      const res = await fetch('/api/ftx/reply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ decode, clientId: 'MSHV' })
      });
      const data = await res.json();
      if (!data.success) console.error('FTx reply failed:', data.message);
    } catch (err) {
      console.error('FTx reply error:', err);
    }
  }

  function rowBg(decode) {
    if (decode.myCall)         return 'bg-cyan-500/25 border-l-2 border-cyan-400';
    if (autoCallTarget && (decode.dxCall || '').toUpperCase() === (autoCallTarget.dxCall || '').toUpperCase())
                               return 'bg-emerald-500/20 border-l-2 border-emerald-400';
    if (isActiveWatchlist(decode)) return 'bg-orange-500/15 border-l-2 border-orange-400';
    if (decode.newDXCC)        return 'bg-green-500/15 border-l-2 border-green-500';
    if (decode.newBand || decode.newMode || decode.newSlot)
                               return 'bg-yellow-500/10 border-l-2 border-yellow-500';
    return 'hover:bg-slate-700/40';
  }

  function snrClass(snr) {
    if (snr >= 0)   return 'text-green-400 font-semibold';
    if (snr >= -10) return 'text-yellow-400';
    return 'text-slate-400';
  }

  function formatTime(t) {
    if (t.length !== 6) return t;
    return `${t.slice(0,2)}:${t.slice(2,4)}:${t.slice(4,6)}`;
  }

  // ── Auto Call ────────────────────────────────────────────────────────────────
  let autoCallEnabled  = false;
  let autoCallTarget   = null;   // decode currently being called
  let autoCallAttempts = 0;      // RX periods: target decoded but no reply
  let autoCallMissed   = 0;      // RX periods: target not decoded at all
  let autoCallBusy     = false;  // prevent concurrent handlers
  let autoCallStopped  = false;  // hit attempt/miss limit (watch call: need manual restart)
  let priorityCall     = '';     // watch call field — only call this station when set

  const AUTO_MISS_MAX    = 3;
  const AUTO_ATTEMPT_MAX = 7;

  // Priority: DXCC > Band+Mode > Band > Mode > Slot > nothing > not enriched
  function acPriority(d) {
    if (typeof d.newDXCC === 'undefined') return -1;
    if (d.newDXCC)              return 5;
    if (d.newBand && d.newMode) return 4;
    if (d.newBand)              return 3;
    if (d.newMode)              return 2;
    if (d.newSlot)              return 1;
    return 0;
  }

  function acPriorityLabel(d) {
    if (!d) return '';
    const p = acPriority(d);
    if (p === 5) return 'DXCC';
    if (p === 4) return 'B+M';
    if (p === 3) return 'Band';
    if (p === 2) return 'Mode';
    if (p === 1) return 'Slot';
    return '';
  }

  // Best candidate: highest priority → watchlist first → highest SNR
  function acBestCandidate(decodes) {
    const wlUC = new Set(watchlist.map(w => w.callsign?.toUpperCase()).filter(Boolean));
    const eligible = decodes.filter(d => acPriority(d) > 0);
    if (!eligible.length) return null;
    eligible.sort((a, b) => {
      const pd = acPriority(b) - acPriority(a);
      if (pd !== 0) return pd;
      const aWL = wlUC.has((a.dxCall || '').toUpperCase()) ? 1 : 0;
      const bWL = wlUC.has((b.dxCall || '').toUpperCase()) ? 1 : 0;
      if (aWL !== bWL) return bWL - aWL;
      return b.snr - a.snr;
    });
    return eligible[0];
  }

  function acQSOComplete(decodes, targetCall) {
    const uc = targetCall.toUpperCase();
    return decodes.some(d => {
      if ((d.dxCall || '').toUpperCase() !== uc) return false;
      if (!d.myCall) return false;
      const msg = (d.message || '').toUpperCase();
      return msg.endsWith(' RR73') || msg.endsWith(' RRR') || msg.endsWith(' 73');
    });
  }

  // Manual restart of watch call after stop
  function acRestartWatchCall() {
    autoCallStopped  = false;
    autoCallTarget   = null;
    autoCallAttempts = 0;
    autoCallMissed   = 0;
  }

  // Manual row click: send reply and, if auto is on, adopt clicked station as target
  function handleRowClick(decode) {
    sendReply(decode);
    if (autoCallEnabled) {
      autoCallTarget   = decode;
      autoCallAttempts = 0;
      autoCallMissed   = 0;
      autoCallStopped  = false;
    }
  }

  // Disable autocall: halt and reset everything
  $: if (!autoCallEnabled) {
    if (autoCallTarget) haltTX();
    autoCallTarget   = null;
    autoCallAttempts = 0;
    autoCallMissed   = 0;
    autoCallStopped  = false;
    _acLastPeriod    = '';
  }

  // Trigger: fire as soon as DB enrichment lands for the latest period.
  // Flow: ftxBatch arrives → new period detected → start 3s fallback timer.
  //       ftxEnrich arrives → enrichment detected → cancel timer, fire immediately.
  // TX periods produce no decodes → reactive doesn't fire → not counted as miss (correct).
  let _acLastPeriod  = '';
  let _acEnrichTimer = null;

  $: if (autoCallEnabled && groupedDecodes.length > 0) {
    const g = groupedDecodes[0];

    if (g.time !== _acLastPeriod) {
      // New period — schedule fallback in case enrichment is slow or absent
      _acLastPeriod = g.time;
      if (_acEnrichTimer) { clearTimeout(_acEnrichTimer); _acEnrichTimer = null; }
      _acEnrichTimer = setTimeout(() => {
        _acEnrichTimer = null;
        _handleAutoCallPeriod(g.time);
      }, 3000);
    } else if (_acEnrichTimer) {
      // Same period but groupedDecodes changed → enrichment just landed
      const hasEnrichment = g.decodes.some(d => typeof d.newDXCC !== 'undefined');
      if (hasEnrichment) {
        clearTimeout(_acEnrichTimer);
        _acEnrichTimer = null;
        _handleAutoCallPeriod(g.time);
      }
    }
  }

  async function _handleAutoCallPeriod(periodTime) {
    if (!autoCallEnabled || autoCallBusy) return;
    autoCallBusy = true;
    let action = null;
    try {
      const group   = groupedDecodes.find(g => g.time === periodTime);
      const decodes = group ? group.decodes : [];

      if (priorityCall) {
        // ── Watch call mode: only ever call this station ──────────────────────
        if (autoCallStopped) return; // waiting for manual restart via button

        const prioUC     = priorityCall.toUpperCase();
        const prioDecode = decodes.find(d => (d.dxCall || '').toUpperCase() === prioUC);
        const isTarget   = (autoCallTarget?.dxCall || '').toUpperCase() === prioUC;

        if (prioDecode) {
          // Station decoded this RX period
          autoCallMissed = 0;
          if (!isTarget) {
            // First time we see it — start calling
            autoCallTarget   = prioDecode;
            autoCallAttempts = 0;
            action = { type: 'reply', decode: prioDecode };
          } else if (acQSOComplete(decodes, prioUC)) {
            autoCallTarget   = null;
            autoCallAttempts = 0;
          } else {
            const replied = decodes.some(d =>
              (d.dxCall || '').toUpperCase() === prioUC && d.myCall
            );
            if (replied) {
              autoCallAttempts = 0; // in QSO
            } else {
              autoCallAttempts++;
              if (autoCallAttempts >= AUTO_ATTEMPT_MAX) {
                autoCallStopped  = true;
                autoCallTarget   = null;
                autoCallAttempts = 0;
                action = { type: 'halt' };
              } else {
                action = { type: 'reply', decode: autoCallTarget };
              }
            }
          }
        } else if (isTarget) {
          // Already targeting it but not decoded this RX period → miss
          autoCallMissed++;
          if (autoCallMissed >= AUTO_MISS_MAX) {
            autoCallStopped  = true;
            autoCallTarget   = null;
            autoCallMissed   = 0;
            autoCallAttempts = 0;
            action = { type: 'halt' };
          }
          // else: MSHV keeps TX-ing automatically, wait for next period
        }
        // Not yet targeting and not decoded → wait silently

      } else {
        // ── Normal mode: auto-pick best candidate ─────────────────────────────
        if (autoCallTarget) {
          const targetUC    = (autoCallTarget.dxCall || '').toUpperCase();
          const targetSeen  = decodes.find(d => (d.dxCall || '').toUpperCase() === targetUC);

          if (targetSeen) {
            autoCallMissed = 0;
            if (acQSOComplete(decodes, targetUC)) {
              autoCallTarget   = null;
              autoCallAttempts = 0;
            } else {
              const replied = decodes.some(d =>
                (d.dxCall || '').toUpperCase() === targetUC && d.myCall
              );
              if (replied) {
                autoCallAttempts = 0;
              } else {
                autoCallAttempts++;
                if (autoCallAttempts >= AUTO_ATTEMPT_MAX) {
                  autoCallTarget   = null;
                  autoCallAttempts = 0;
                  autoCallMissed   = 0;
                  action = { type: 'halt' };
                  // Next period will pick a new candidate
                } else {
                  action = { type: 'reply', decode: autoCallTarget };
                }
              }
            }
          } else {
            // Target not decoded this RX period → miss
            autoCallMissed++;
            if (autoCallMissed >= AUTO_MISS_MAX) {
              autoCallTarget   = null;
              autoCallMissed   = 0;
              autoCallAttempts = 0;
              action = { type: 'halt' };
            }
          }
        } else {
          // No target → pick best candidate from this period
          const candidate = acBestCandidate(decodes);
          if (candidate) {
            autoCallTarget   = candidate;
            autoCallAttempts = 0;
            autoCallMissed   = 0;
            action = { type: 'reply', decode: candidate };
          }
        }
      }
    } finally {
      autoCallBusy = false;
    }

    // Network calls outside the busy lock so slow responses never block the next period
    if (action?.type === 'reply') sendReply(action.decode).catch(e => console.error('FTx reply:', e));
    if (action?.type === 'halt')  haltTX().catch(e => console.error('FTx halt:', e));
  }
</script>

<div class="flex flex-col h-full overflow-hidden text-xs">

  <!-- Toolbar -->
  <div class="flex items-center gap-2 px-2 py-1.5 bg-slate-900/50 border-b border-slate-700/50 flex-shrink-0 flex-wrap">

    <button
      on:click={toggleEnabled}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors {ftxEnabled ? 'bg-green-500/20 text-green-400 border border-green-500/40' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:border-slate-500'}">
      {ftxEnabled ? '● ON' : '○ OFF'}
    </button>

    <span class="text-slate-500">|</span>

    <button
      on:click={() => showSlotAdvisor = !showSlotAdvisor}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors {showSlotAdvisor ? 'bg-violet-500/25 text-violet-300 border border-violet-500/50' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:border-slate-500'}">
      📡 Slots
    </button>

    <span class="text-slate-500">|</span>

    <label class="flex items-center gap-1 cursor-pointer select-none text-slate-300">
      <input type="checkbox" bind:checked={filterCQOnly} class="accent-blue-500" /> CQ
    </label>

    <label class="flex items-center gap-1 cursor-pointer select-none text-slate-300">
      <input type="checkbox" bind:checked={filterMyCall} class="accent-cyan-500" /> My Call
    </label>

    <button
      on:click={clearDisplay}
      class="px-2 py-0.5 rounded text-xs font-semibold bg-slate-700 text-slate-300 border border-slate-600 hover:border-red-500/50 hover:text-red-400 transition-colors">
      Clear
    </button>

    <span class="text-slate-500">|</span>

    <!-- Halt TX -->
    <button
      on:click={haltTX}
      disabled={haltBusy}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors border {haltOk ? 'bg-green-500/20 text-green-400 border-green-500/50' : haltBusy ? 'bg-red-600/10 text-red-400/50 border-red-600/20 cursor-wait' : 'bg-red-600/20 text-red-400 border-red-600/40 hover:bg-red-600/40 hover:border-red-500'}"
      title="Stop TX immediately in WSJT-X/JTDX/MSHV">
      {haltBusy ? '…' : haltOk ? '✓ Halted' : '⛔ Halt TX'}
    </button>

    <span class="text-slate-500">|</span>

    <!-- Auto Call -->
    <button
      on:click={() => autoCallEnabled = !autoCallEnabled}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors border {autoCallEnabled ? 'bg-emerald-500/25 text-emerald-300 border-emerald-500/50' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:border-slate-500'}"
      title="Auto call: picks the best new station each period (DXCC > Band+Mode > Band > Mode > Slot)">
      {autoCallEnabled ? '▶ Auto' : '▷ Auto'}
    </button>

    {#if autoCallEnabled}
      {#if autoCallStopped && priorityCall}
        <span class="text-[10px] text-red-400 font-semibold">{priorityCall} stopped</span>
        <button
          on:click={acRestartWatchCall}
          class="px-2 py-0.5 rounded text-xs font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 hover:bg-amber-500/35 transition-colors">
          Restart
        </button>
      {:else if autoCallTarget}
        <span class="font-mono text-emerald-300 font-semibold">{autoCallTarget.dxCall}</span>
        <span class="text-[10px] text-emerald-500/80 font-semibold">{acPriorityLabel(autoCallTarget)}</span>
        {#if autoCallAttempts > 0}
          <span class="text-[10px] text-orange-400">Call:{autoCallAttempts}/{AUTO_ATTEMPT_MAX}</span>
        {/if}
        {#if autoCallMissed > 0}
          <span class="text-[10px] text-red-400">Miss:{autoCallMissed}/{AUTO_MISS_MAX}</span>
        {/if}
      {:else}
        <span class="text-[10px] text-slate-500 italic">waiting…</span>
      {/if}

      <span class="text-slate-500">|</span>
      <input
        type="text"
        bind:value={priorityCall}
        placeholder="Watch call…"
        maxlength="12"
        on:input={(e) => { priorityCall = e.target.value.toUpperCase(); autoCallStopped = false; autoCallTarget = null; autoCallAttempts = 0; autoCallMissed = 0; }}
        class="w-24 px-1.5 py-0.5 rounded text-xs font-mono bg-slate-800 border {priorityCall ? 'border-amber-500/60 text-amber-300' : 'border-slate-600 text-slate-400'} placeholder-slate-600 focus:outline-none focus:border-amber-500/80"
        title="Watch call: only call this station when decoded (Auto must be ON)" />
    {/if}

    <span class="ml-auto text-slate-500">
      Last: <span class="text-slate-400">{lastPeriodCount}</span>
    </span>

  </div>

  <!-- TX Slot Advisor Panel -->
  {#if showSlotAdvisor}
    <div class="px-3 py-2 bg-slate-950 border-b border-violet-500/30 flex-shrink-0">
      <div class="flex items-center gap-3 mb-1.5">
        <span class="text-violet-400 font-semibold uppercase tracking-wide text-[10px]">TX Slot Advisor</span>
        <span class="text-slate-600 text-[10px]">{slotAnalysis?.dfs?.length ?? 0} signals · {DF_MIN}-{DF_MAX} Hz</span>
        <div class="flex items-center gap-1.5 ml-auto cursor-pointer select-none">
          <span class="text-slate-500 text-[10px]">DXpedition</span>
          <div
            role="switch"
            aria-checked={isDXpedition}
            tabindex="0"
            on:click={() => isDXpedition = !isDXpedition}
            on:keydown={(e) => e.key === 'Enter' && (isDXpedition = !isDXpedition)}
            class="relative w-8 h-4 rounded-full transition-colors cursor-pointer {isDXpedition ? 'bg-violet-500/60' : 'bg-slate-600'}">
            <div class="absolute top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform {isDXpedition ? 'translate-x-4' : 'translate-x-0.5'}"></div>
          </div>
        </div>
      </div>

      {#if !slotAnalysis || slotAnalysis.dfs.length === 0}
        <div class="text-slate-600 text-[10px]">Pas encore de décodes à analyser.</div>
      {:else}
        <!-- Best suggestion -->
        {#if slotAnalysis.best}
          {@const q = slotAnalysis.best.quality}
          <div class="flex items-center gap-3 mb-2">
            <div class="flex items-center gap-1.5">
              <span class="text-slate-500 text-[10px]">Meilleur créneau :</span>
              <span class="font-bold text-sm font-mono {q === 'ok' ? 'text-green-300' : q === 'tight' ? 'text-yellow-300' : 'text-red-400'}">{slotAnalysis.best.df} Hz</span>
              <span class="text-slate-600 text-[10px]">({slotAnalysis.best.size} Hz)</span>
              {#if q === 'ok'}
                <span class="text-[10px] text-green-500">✓ libre</span>
              {:else if q === 'tight'}
                <span class="text-[10px] text-yellow-500">⚠ serré</span>
              {:else}
                <span class="text-[10px] text-red-500">✗ saturé</span>
              {/if}
            </div>
            {#if slotAnalysis.gaps.length > 1}
              <span class="text-slate-600">|</span>
              <div class="flex gap-2">
                {#each slotAnalysis.gaps.slice(1, 4) as g}
                  <span class="font-mono text-[11px] {g.quality === 'ok' ? 'text-blue-300' : g.quality === 'tight' ? 'text-yellow-400/70' : 'text-red-400/60'}">{g.df} <span class="text-slate-600">({g.size})</span></span>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <!-- Spectrum bar -->
        <div class="relative h-5 rounded overflow-hidden bg-slate-800 mb-1" style="width:100%">

          <!-- Free gap highlights (top 3) -->
          {#each slotAnalysis.gaps.slice(0, 3) as gap, i}
            <div
              class="absolute top-0 h-full {i === 0 ? 'bg-green-500/25' : 'bg-blue-500/12'}"
              style="left:{dfPct(gap.lo)}%; width:{dfPct(gap.hi) - dfPct(gap.lo)}%">
            </div>
          {/each}

          <!-- Signal marks -->
          {#each slotAnalysis.dfs as df}
            <div
              class="absolute top-0 h-full w-px bg-red-500/70"
              style="left:{dfPct(df)}%">
            </div>
          {/each}

          <!-- Best suggestion marker -->
          {#if slotAnalysis.best}
            <div
              class="absolute top-0 h-full w-0.5 bg-green-400"
              style="left:{dfPct(slotAnalysis.best.df)}%">
            </div>
          {/if}

          <!-- Alt suggestions markers -->
          {#each slotAnalysis.gaps.slice(1, 4) as gap}
            <div
              class="absolute top-0 h-full w-0.5 bg-blue-400/70"
              style="left:{dfPct(gap.df)}%">
            </div>
          {/each}
        </div>

        <!-- Frequency axis labels -->
        <div class="relative h-3 text-[9px] text-slate-600 font-mono select-none">
          {#each (isDXpedition ? [1000, 1250, 1500, 1750, 2000, 2250, 2500, 2750, 3000] : [0, 250, 500, 750, 1000, 1250, 1500, 1750, 2000, 2250, 2500, 2750, 3000]) as f}
            <span class="absolute -translate-x-1/2" style="left:{dfPct(f)}%">{f}</span>
          {/each}
        </div>

        <!-- Legend -->
        <div class="flex gap-3 mt-1 text-[9px] text-slate-600">
          <span><span class="text-red-400">▌</span> Signal occupé</span>
          <span><span class="text-green-400">▌</span> Meilleur créneau</span>
          <span><span class="text-blue-400">▌</span> Alternatives</span>
        </div>
      {/if}
    </div>
  {/if}

  {#if !ftxEnabled}
    <div class="flex-1 flex items-center justify-center text-slate-500 text-sm">
      FTx monitoring is disabled
    </div>
  {:else}
    <!-- Column header -->
    <div class="grid text-slate-500 font-semibold tracking-wide bg-slate-900/50 border-b border-slate-700/50 flex-shrink-0"
         style="grid-template-columns: {cols};">
      <div class="px-1 py-1 text-center">Time</div>
      <div class="px-1 py-1 text-center">SNR</div>
      <div class="px-1 py-1 text-center">DT</div>
      <div class="px-1 py-1 text-center">Freq</div>
      <div class="px-1 py-1 text-center">Band</div>
      <div class="px-1 py-1 text-center">Mode</div>
      <div class="px-1 py-1 text-center">Message</div>
      <div class="px-1 py-1 text-center">Country</div>
      <div class="px-1 py-1 text-center">Status</div>
    </div>

    <!-- Rows grouped by period -->
    <div class="flex-1 overflow-y-auto">
      {#if displayed.length === 0}
        <div class="flex items-center justify-center h-20 text-slate-600">
          Waiting for FT8/FT4/FT2 decodes…
        </div>
      {/if}

      {#each groupedDecodes as group, gi (group.time)}

        <!-- Period separator (between groups, not before the first) -->
        {#if gi > 0}
          <div class="flex items-center gap-2 px-2 py-0.5 bg-slate-950 border-t border-b border-slate-700/60 text-[10px] font-mono select-none">
            <span class="text-slate-400 font-semibold">{formatTime(group.time)}</span>
            <span class="flex-1 border-t border-slate-700/50 mx-1"></span>
            <span class="text-slate-600">{group.decodes.length} decodes</span>
          </div>
        {/if}

        {#each group.decodes as decode (decode.time + decode.message + decode.df)}
          <div
            class="grid items-center cursor-pointer transition-colors hover:brightness-125 {rowBg(decode)}"
            style="grid-template-columns: {cols};"
            role="button"
            tabindex="0"
            on:click={() => handleRowClick(decode)}
            on:keydown={(e) => e.key === 'Enter' && handleRowClick(decode)}
            title="Click to reply — {decode.dxCall || decode.message}">

            <div class="px-1 py-0.5 font-mono text-slate-400 text-center">{decode.time}</div>

            <div class="px-1 py-0.5 text-center font-mono {snrClass(decode.snr)}">
              {decode.snr > 0 ? '+' : ''}{decode.snr}
            </div>

            <div class="px-1 py-0.5 text-center font-mono text-slate-400">
              {decode.dt?.toFixed(1)}
            </div>

            <div class="px-1 py-0.5 text-center font-mono text-slate-300">
              {decode.df}
            </div>

            <div class="px-1 py-0.5 text-center">
              {#if decode.band}
                <span class="px-1 rounded bg-blue-500/20 text-blue-300">{decode.band}</span>
              {/if}
            </div>

            <div class="px-1 py-0.5 text-center font-mono text-[10px] {decode.mode === 'FT4' ? 'text-violet-400' : decode.mode === 'FT2' ? 'text-orange-400' : 'text-slate-500'}">
              {decode.mode || ''}
            </div>

            <div class="px-1 py-0.5 font-mono truncate" title={decode.message}>
              {#if decode.myCall}
                <span class="text-cyan-300 font-bold">{decode.message}</span>
              {:else if decode.isCQ}
                <span class="text-green-300">{decode.message}</span>
              {:else}
                <span class="text-slate-200">{decode.message}</span>
              {/if}
            </div>

            <div class="pl-3 pr-1 py-0.5 text-slate-400 truncate" title={decode.countryName}>
              {decode.countryName || ''}
            </div>

            <div class="px-1 py-0.5 flex gap-1 items-center justify-center flex-wrap">
              {#if isActiveWatchlist(decode)}
                <span class="px-3 py-0 leading-none rounded text-[11px] font-semibold bg-orange-500/30 text-orange-300 border border-orange-500/50 whitespace-nowrap">WL</span>
              {/if}
              {#if decode.newDXCC}
                <span class="px-3 py-0 leading-none rounded text-[11px] font-semibold bg-green-500/30 text-green-300 border border-green-500/50 whitespace-nowrap">DXCC</span>
              {/if}
              {#if decode.newBand && !decode.newDXCC}
                <span class="px-3 py-0 leading-none rounded text-[11px] font-semibold bg-yellow-500/30 text-yellow-300 border border-yellow-500/50 whitespace-nowrap">Band</span>
              {/if}
              {#if decode.newMode && !decode.newDXCC}
                <span class="px-3 py-0 leading-none rounded text-[11px] font-semibold bg-orange-500/30 text-orange-300 border border-orange-500/50 whitespace-nowrap">Mode</span>
              {/if}
              {#if decode.newSlot && !decode.newDXCC}
                <span class="px-3 py-0 leading-none rounded text-[11px] font-semibold bg-cyan-500/30 text-cyan-300 border border-cyan-500/50 whitespace-nowrap">Slot</span>
              {/if}
              {#if decode.worked && !decode.newDXCC && !decode.newBand && !decode.newMode && !decode.newSlot}
                <span class="px-3 py-0 leading-none rounded text-[11px] bg-slate-500/30 text-slate-400 border border-slate-500/40 whitespace-nowrap">Wkd</span>
              {/if}
              {#if decode.lowConfidence}
                <span class="px-3 py-0 leading-none rounded text-[11px] bg-red-500/20 text-red-400 border border-red-500/30">?</span>
              {/if}
            </div>

          </div>
        {/each}
      {/each}
    </div>
  {/if}
</div>

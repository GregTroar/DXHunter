<script>
  import QRZPanel from './QRZPanel.svelte';
  import { onMount, onDestroy } from 'svelte';

  export let ftxEnabled = false;
  export let ftxDecodes = [];   // maintained by App.svelte — persists across tab switches
  export let watchlist = [];
  export let spots = [];
  export let ftxTXStatus = { transmitting: false, message: '', mode: '', clientId: '' };
  export let myGrid = '';
  export let contestMode = false;

  let filterCQOnly = false;
  let filterMyCall = false;

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

  const cols = '50px 34px 36px 52px 48px 36px minmax(0,1.6fr) minmax(0,1fr) minmax(170px,1.4fr)';

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

  // Deduplication: one Reply per (period, dxCall) regardless of how many times the
  // handler fires (reactive re-runs, enrichment patches, etc.).
  let _lastReplyKey = '';

  async function sendReply(decode) {
    const key = `${decode.time}|${(decode.dxCall || '').toUpperCase()}`;
    if (key === _lastReplyKey) {
      console.debug('FTx: duplicate sendReply suppressed', key);
      return;
    }
    _lastReplyKey = key;
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
  let autoCallEnabled      = false;
  let autoCallTarget       = null;   // decode currently being called
  let autoCallAttempts     = 0;      // RX periods: target decoded but no reply
  let autoCallMissed       = 0;      // RX periods: target not decoded at all
  let autoCallBusy         = false;  // prevent concurrent handlers
  let autoCallStopped      = false;  // hit attempt/miss limit (watch call: need manual restart)
  let autoCallManualLocked = false;  // user manually clicked a row — don't auto-pick after halt
  let priorityCall         = '';     // watch call field — comma/space separated list
  let autoCallStoppedCall  = '';     // which call triggered the stop
  // Callsigns worked this session — excluded from auto-pick until enrichment updates.
  // Prevents MSHV "already logged" dialog caused by re-selecting a just-worked station
  // before Log4OM cache refreshes (typically within 30s).
  let recentlyWorked       = new Set();

  $: priorityCalls = priorityCall
    .split(/[\s,]+/)
    .map(c => c.trim().toUpperCase())
    .filter(Boolean);

  // In contest mode, autocall is restricted to watchlist entries marked isContest=true.
  $: contestWatchlistUC = new Set(
    watchlist.filter(w => w.isContest).map(w => (w.callsign || '').toUpperCase()).filter(Boolean)
  );

  const AUTO_MISS_MAX             = 3;
  const AUTO_ATTEMPT_MAX          = 7;
  const AUTO_WATCHLIST_ATTEMPT_MAX = 15;

  // Most Wanted: adif (string) → rank (1 = most wanted globally)
  let mostWantedMap = {}; // populated on mount

  function mwRank(adif) {
    if (!adif || !mostWantedMap[adif]) return Infinity;
    return mostWantedMap[adif];
  }

  onMount(async () => {
    try {
      const res = await fetch('/api/mostwanted');
      if (res.ok) {
        const json = await res.json();
        mostWantedMap = json.data ?? json;
      }
    } catch (e) {
      console.warn('Most Wanted fetch failed:', e);
    }
  });

  function isWatchlisted(call) {
    if (!call || !watchlist?.length) return false;
    const uc = call.toUpperCase();
    return watchlist.some(w => (w.callsign || w || '').toUpperCase() === uc);
  }

  // Detect if a station is calling us directly (report/R-report) but not CQ/complete
  function acCallerToMe(decodes) {
    return decodes.find(d => {
      const uc = (d.dxCall || '').toUpperCase();
      if (contestMode && !contestWatchlistUC.has(uc)) return false;
      if (contestMode && d.workedToday) return false;
      return d.myCall && d.dxCall && !d.isCQ &&
        !recentlyWorked.has(uc) &&
        !acQSOComplete([d], d.dxCall);
    }) || null;
  }

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

  // Best candidate:
  //   Normal mode : new DXCC/band/mode/slot → watchlist first → CQ first → highest SNR
  //   Contest mode: any contest-watchlist station not yet worked this session → CQ first → highest SNR
  function acBestCandidate(decodes) {
    const wlUC = new Set(watchlist.map(w => w.callsign?.toUpperCase()).filter(Boolean));

    if (contestMode) {
      const eligible = decodes.filter(d => {
        const uc = (d.dxCall || '').toUpperCase();
        return contestWatchlistUC.has(uc) && !recentlyWorked.has(uc) && !d.workedToday;
      });
      if (!eligible.length) return null;
      eligible.sort((a, b) => {
        const aCQ = a.isCQ ? 1 : 0;
        const bCQ = b.isCQ ? 1 : 0;
        if (aCQ !== bCQ) return bCQ - aCQ;
        return b.snr - a.snr;
      });
      return eligible[0];
    }

    const eligible = decodes.filter(d => {
      const uc = (d.dxCall || '').toUpperCase();
      return acPriority(d) > 0 && !recentlyWorked.has(uc);
    });
    if (!eligible.length) return null;
    eligible.sort((a, b) => {
      const pd = acPriority(b) - acPriority(a);
      if (pd !== 0) return pd;
      const aWL = wlUC.has((a.dxCall || '').toUpperCase()) ? 1 : 0;
      const bWL = wlUC.has((b.dxCall || '').toUpperCase()) ? 1 : 0;
      if (aWL !== bWL) return bWL - aWL;
      const mwDiff = mwRank(a.dxcc) - mwRank(b.dxcc); // lower rank = more wanted
      if (mwDiff !== 0) return mwDiff;
      const aCQ = a.isCQ ? 1 : 0;
      const bCQ = b.isCQ ? 1 : 0;
      if (aCQ !== bCQ) return bCQ - aCQ;
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
    autoCallStopped     = false;
    autoCallStoppedCall = '';
    autoCallTarget      = null;
    autoCallAttempts    = 0;
    autoCallMissed      = 0;
  }

  // QRZ panel: auto-follows autoCallTarget, or clicked row
  let qrzCallsign = '';

  async function clearMSHVDXCall() {
    try {
      await fetch('/api/ftx/configure', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ clearDXCall: true })
      });
    } catch (err) {
      console.error('FTx configure error:', err);
    }
  }

  // Manual row click: send reply and, if auto is on, adopt clicked station as locked target.
  // Start attempts at 1 so the handler knows the initial call was already sent.
  function handleRowClick(decode) {
    sendReply(decode);
    qrzCallsign = decode.dxCall || '';
    if (autoCallEnabled) {
      autoCallTarget       = decode;
      autoCallAttempts     = 1;     // initial call already sent via click
      autoCallMissed       = 0;
      autoCallStopped      = false;
      autoCallManualLocked = true;  // prevent auto-picking a new candidate
    }
  }

  // Disable autocall: halt and reset everything
  // In contest mode, recentlyWorked is intentionally preserved across toggles —
  // it is the day-scoped worked guard, reset only at UTC midnight.
  $: if (!autoCallEnabled) {
    if (autoCallTarget) haltTX();
    autoCallTarget       = null;
    autoCallAttempts     = 0;
    autoCallMissed       = 0;
    autoCallStopped      = false;
    autoCallManualLocked = false;
    _acLastPeriod        = '';
    _acNeedRerun         = false;
    _acMissPendingPeriod = '';
    if (!contestMode) recentlyWorked = new Set();
  }

  // Trigger strategy:
  //  1. New period → fire immediately (handles misses + existing target tracking — no enrichment needed).
  //  2. No candidate found (handler sets _acNeedRerun) → re-fire as soon as enrichment arrives.
  //     Normal mode: wait for a positive flag (newDXCC/newBand/newMode/newSlot).
  //     Contest mode: rerun as soon as ANY decode is enriched — contest stations may have all
  //     new-flags=false (already-worked DXCC/band/mode) yet still be eligible (not worked today).
  //  3. Target absent on first check (sets _acMissPendingPeriod) → re-fire if target decode
  //     arrives late in the same period. Miss is only counted at the next period boundary if the
  //     target never appeared (grace window for late-arriving WSJT-X decodes, typically ~1 s).
  // TX periods produce no decodes → reactive never fires → not counted as miss (correct).
  let _acLastPeriod        = '';
  let _acNeedRerun         = false;
  let _acMissPendingPeriod = ''; // period where target was absent; counted on the next period if still absent

  $: if (autoCallEnabled && groupedDecodes.length > 0) {
    const g = groupedDecodes[0];
    if (g.time !== _acLastPeriod) {
      _acLastPeriod = g.time;
      _acNeedRerun  = false;
      _handleAutoCallPeriod(g.time);
    } else if (_acNeedRerun) {
      const hasPositive = contestMode
        ? g.decodes.some(d => typeof d.newDXCC !== 'undefined') // any enriched decode
        : g.decodes.some(d => d.newDXCC || d.newBand || d.newMode || d.newSlot);
      if (hasPositive) {
        _acNeedRerun = false;
        _handleAutoCallPeriod(g.time);
      }
    } else if (_acMissPendingPeriod === g.time && autoCallTarget) {
      // A miss was deferred this period — recheck if the target's decode has since arrived
      const pendingTargetUC = (autoCallTarget.dxCall || '').toUpperCase();
      if (g.decodes.some(d => (d.dxCall || '').toUpperCase() === pendingTargetUC)) {
        _handleAutoCallPeriod(g.time);
      }
    }
  }

  async function _handleAutoCallPeriod(periodTime) {
    if (!autoCallEnabled || autoCallBusy) return;
    autoCallBusy = true;
    let action = null;
    try {
      const group      = groupedDecodes.find(g => g.time === periodTime);
      const decodes    = group ? group.decodes : [];
      // Also check previous period: QSO-complete signal (RRR/RR73/73) may have arrived
      // in the preceding period and be absent from the current batch.
      const prevGroup   = groupedDecodes[1]; // second newest = previous period
      const prevDecodes = (prevGroup && prevGroup.time !== periodTime) ? prevGroup.decodes : [];

      outer: {
      // A miss from the previous period was deferred (grace window for late-arriving decodes).
      // Count it now that a full period has elapsed without the target appearing.
      if (_acMissPendingPeriod && _acMissPendingPeriod !== periodTime && autoCallTarget) {
        _acMissPendingPeriod = '';
        autoCallMissed++;
        if (autoCallMissed >= AUTO_MISS_MAX) {
          const stalledUC = (autoCallTarget.dxCall || '').toUpperCase();
          if (priorityCalls.length > 0) {
            autoCallStopped     = true;
            autoCallStoppedCall = stalledUC;
          } else {
            autoCallManualLocked = false;
          }
          autoCallTarget   = null;
          autoCallMissed   = 0;
          autoCallAttempts = 0;
          action = { type: 'halt' };
          break outer;
        }
      }

      if (priorityCalls.length > 0) {
        // ── Watch call mode: only call stations from the priority list ─────────

        if (autoCallStopped) {
          // Check if any priority call came back to call us
          let comeback = null;
          for (const prioUC of priorityCalls) {
            const d = decodes.find(dd => (dd.dxCall || '').toUpperCase() === prioUC);
            if (d?.myCall && !acQSOComplete(decodes, prioUC)) { comeback = d; break; }
          }
          if (comeback) {
            autoCallStopped     = false;
            autoCallStoppedCall = '';
            autoCallTarget      = comeback;
            qrzCallsign         = (comeback.dxCall || '').toUpperCase();
            autoCallAttempts    = 1;
            autoCallMissed      = 0;
            action = { type: 'reply', decode: comeback };
          } else {
            return; // still stopped, wait for manual restart or late response
          }
        }

        const targetUC       = (autoCallTarget?.dxCall || '').toUpperCase();
        const targetIsOnList = targetUC && priorityCalls.includes(targetUC);

        if (autoCallTarget && targetIsOnList) {
          // Continue with current target
          const targetDecode = decodes.find(d => (d.dxCall || '').toUpperCase() === targetUC);
          // Check QSO complete first — may have arrived in the previous period even if target
          // is absent from the current batch.
          if (acQSOComplete(decodes, targetUC) || acQSOComplete(prevDecodes, targetUC)) {
            recentlyWorked.add(targetUC);
            autoCallTarget   = null;
            autoCallAttempts = 0;
            action = { type: 'clearDXCall' };
          } else if (targetDecode) {
            autoCallTarget = targetDecode; // refresh: use current-period decode so Reply has fresh timestamp
            _acMissPendingPeriod = '';
            const wasInMiss = autoCallMissed > 0;
            autoCallMissed = 0;
            const replied = decodes.some(d =>
              (d.dxCall || '').toUpperCase() === targetUC && d.myCall
            );
            if (replied) {
              autoCallAttempts = 0; // in QSO, MSHV handles sequencing
            } else {
              const firstCall = autoCallAttempts === 1;
              autoCallAttempts++;
              const maxAttempts = isWatchlisted(targetUC) ? AUTO_WATCHLIST_ATTEMPT_MAX : AUTO_ATTEMPT_MAX;
              if (autoCallAttempts >= maxAttempts) {
                autoCallStopped     = true;
                autoCallStoppedCall = targetUC;
                autoCallTarget      = null;
                autoCallAttempts    = 0;
                action = { type: 'halt' };
              } else if (firstCall || wasInMiss) {
                action = { type: 'reply', decode: autoCallTarget };
              }
              // else: MSHV already transmitting — do not interrupt
            }
          } else {
            // Target not decoded this period — defer one period to allow late-arriving decodes
            _acMissPendingPeriod = periodTime;
          }
        } else {
          // No current target (or stale) — find best priority call decoded this period
          let bestDecode = null;
          for (const prioUC of priorityCalls) {
            if (recentlyWorked.has(prioUC)) continue;
            const d = decodes.find(dd => (dd.dxCall || '').toUpperCase() === prioUC);
            if (d && (!bestDecode || d.snr > bestDecode.snr)) bestDecode = d;
          }
          if (bestDecode) {
            autoCallTarget   = bestDecode;
            qrzCallsign      = (bestDecode.dxCall || '').toUpperCase();
            autoCallAttempts = 1;
            autoCallMissed   = 0;
            action = { type: 'reply', decode: bestDecode };
          }
          // Not yet decoded — wait silently
        }

      } else {
        // ── Normal mode: auto-pick best candidate ─────────────────────────────
        if (autoCallTarget) {
          const targetUC   = (autoCallTarget.dxCall || '').toUpperCase();
          const targetSeen = decodes.find(d => (d.dxCall || '').toUpperCase() === targetUC);

          // Check QSO complete first — RRR/RR73/73 may have arrived in the previous period
          // even if the target station is absent from the current batch.
          if (acQSOComplete(decodes, targetUC) || acQSOComplete(prevDecodes, targetUC)) {
            recentlyWorked.add(targetUC);
            autoCallTarget       = null;
            autoCallAttempts     = 0;
            autoCallManualLocked = false;
            action = { type: 'clearDXCall' };
          } else if (targetSeen) {
            autoCallTarget = targetSeen; // refresh: use current-period decode so Reply has fresh timestamp
            _acMissPendingPeriod = '';
            const wasInMiss = autoCallMissed > 0;
            autoCallMissed = 0;
            const replied = decodes.some(d =>
              (d.dxCall || '').toUpperCase() === targetUC && d.myCall
            );
            if (replied) {
              autoCallAttempts = 0; // in QSO, MSHV handles sequencing
            } else {
              const firstCall = autoCallAttempts === 1;
              autoCallAttempts++;
              const maxAttempts = isWatchlisted(targetUC) ? AUTO_WATCHLIST_ATTEMPT_MAX : AUTO_ATTEMPT_MAX;
              if (autoCallAttempts >= maxAttempts) {
                autoCallTarget       = null;
                autoCallAttempts     = 0;
                autoCallMissed       = 0;
                autoCallManualLocked = false;
                action = { type: 'halt' };
              } else if (firstCall || wasInMiss || contestMode) {
                // Normal: only (re)send on first call or when station reappears after miss.
                // Contest: resend every RX period — target may be working others, MSHV needs
                // a new Reply each period to keep transmitting in the next TX slot.
                action = { type: 'reply', decode: autoCallTarget };
              }
              // else: MSHV already transmitting — do not interrupt
            }
          } else {
            // Target not decoded this period — defer one period to allow late-arriving decodes
            _acMissPendingPeriod = periodTime;
          }
        } else if (!autoCallManualLocked && (!ftxTXStatus.transmitting || contestMode)) {
          // No target → check first if a station is calling us back (late response),
          // Contest mode ignores transmitting state: Reply is queued for the next TX slot anyway.
          // then fall back to best new-DXCC/band/mode/slot candidate.
          const candidate = acCallerToMe(decodes) || acBestCandidate(decodes);
          if (candidate) {
            autoCallTarget   = candidate;
            qrzCallsign      = (candidate.dxCall || '').toUpperCase();
            autoCallAttempts = 1;
            autoCallMissed   = 0;
            action = { type: 'reply', decode: candidate };
          } else {
            // No eligible candidate yet — enrichment may not have arrived.
            // Signal reactive to retry when a positive flag appears.
            _acNeedRerun = true;
          }
        }
      }
      } // end outer:
    } finally {
      autoCallBusy = false;
    }

    // Stats tracking
    // Network calls outside the busy lock so slow responses never block the next period
    if (action?.type === 'reply')       sendReply(action.decode).catch(e => console.error('FTx reply:', e));
    if (action?.type === 'halt')        haltTX().catch(e => console.error('FTx halt:', e));
    if (action?.type === 'clearDXCall') {
      // Delay by one full TX period + buffer so MSHV can send its final 73 before we clear
      // the DX call field. Sending clearDXCall immediately would abort MSHV's 73 transmission.
      const delay = (periodMs || 15000) + 3000;
      setTimeout(() => clearMSHVDXCall().catch(e => console.error('FTx configure:', e)), delay);
    }
  }

  // ── UTC midnight reset for contest mode ─────────────────────────────────────
  // recentlyWorked persists in memory; in contest mode it must be cleared at 00:00 UTC
  // so stations worked yesterday don't stay blocked for today's contest session.
  function msUntilUTCMidnight() {
    const now = new Date();
    const midnight = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() + 1));
    return midnight.getTime() - now.getTime();
  }

  let _midnightTimeout;
  function scheduleMidnightReset() {
    clearTimeout(_midnightTimeout);
    _midnightTimeout = setTimeout(() => {
      if (contestMode) {
        recentlyWorked = new Set();
        console.info('FTx: recentlyWorked cleared at UTC midnight (contest mode)');
      }
      scheduleMidnightReset(); // reschedule for the next day
    }, msUntilUTCMidnight());
  }

  onMount(()    => { scheduleMidnightReset(); });
  onDestroy(()  => { clearTimeout(_midnightTimeout); });

  // ── Period countdown timer ───────────────────────────────────────────────────
  let _tick = 0;
  let _tickInterval;
  onMount(()    => { _tickInterval = setInterval(() => _tick++, 250); });
  onDestroy(()  => clearInterval(_tickInterval));

  function getPeriodMs(mode) {
    switch ((mode || '').toUpperCase()) {
      case 'FT8': return 15000;
      case 'FT4': return 7500;
      case 'FT2': return 3250;
      default:    return 0;
    }
  }

  $: periodMs  = getPeriodMs(ftxTXStatus.mode);
  $: countdown = (() => {
    if (!periodMs || !ftxEnabled) return null;
    void _tick;
    const elapsed   = Date.now() % periodMs;
    const remaining = (periodMs - elapsed) / 1000;
    const pct       = (elapsed / periodMs) * 100;
    return { remaining, pct };
  })();
</script>

<div class="flex h-full overflow-hidden text-xs">

<!-- ── Left: toolbar + table ──────────────────────────────────────────────── -->
<div class="flex flex-col flex-1 min-w-0 overflow-hidden">

  <!-- Toolbar -->
  <div class="flex items-center gap-2 px-2 py-1.5 bg-slate-900/50 border-b border-slate-700/50 flex-shrink-0 flex-wrap">

    <button
      on:click={toggleEnabled}
      class="px-2 py-0.5 rounded text-xs font-semibold transition-colors {ftxEnabled ? 'bg-green-500/20 text-green-400 border border-green-500/40' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:border-slate-500'}">
      {ftxEnabled ? '● ON' : '○ OFF'}
    </button>

    {#if ftxTXStatus.clientId}
      {@const clientName = ftxTXStatus.clientId.split(' ')[0]}
      {@const clientStyle = clientName.includes('WSJT') ? 'bg-blue-500/20 text-blue-300 border-blue-500/50'
                          : clientName.includes('JTDX') ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/50'
                          : clientName.includes('MSHV') ? 'bg-orange-500/20 text-orange-300 border-orange-500/50'
                          : 'bg-slate-700/60 text-slate-400 border-slate-600/60'}
      <span class="px-2 py-0.5 rounded text-xs font-mono font-bold border tracking-wide {clientStyle}">
        {clientName}
      </span>
    {/if}

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
      on:click={() => { haltTX(); clearMSHVDXCall(); autoCallTarget = null; autoCallAttempts = 0; autoCallMissed = 0; autoCallStopped = false; autoCallManualLocked = false; }}
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
      {#if autoCallStopped && priorityCalls.length > 0}
        <span class="text-[10px] text-red-400 font-semibold">{autoCallStoppedCall || priorityCalls[0]} stopped</span>
        <button
          on:click={acRestartWatchCall}
          class="px-2 py-0.5 rounded text-xs font-semibold bg-amber-500/20 text-amber-300 border border-amber-500/40 hover:bg-amber-500/35 transition-colors">
          Restart
        </button>
      {:else if autoCallTarget}
        <span class="font-mono text-emerald-300 font-semibold">{autoCallTarget.dxCall}</span>
        <span class="text-[10px] text-emerald-500/80 font-semibold">{acPriorityLabel(autoCallTarget)}</span>
        {#if mwRank(autoCallTarget.dxcc) !== Infinity}
          <span class="text-[10px] text-amber-400/80" title="ClubLog Most Wanted rank">#{mwRank(autoCallTarget.dxcc)}</span>
        {/if}
        {#if autoCallAttempts > 0}
          <span class="text-[10px] text-orange-400">Call:{autoCallAttempts}/{isWatchlisted(autoCallTarget?.dxCall) ? AUTO_WATCHLIST_ATTEMPT_MAX : AUTO_ATTEMPT_MAX}</span>
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
        placeholder="Watch calls…"
        on:input={(e) => { priorityCall = e.target.value.toUpperCase(); autoCallStopped = false; autoCallStoppedCall = ''; autoCallTarget = null; autoCallAttempts = 0; autoCallMissed = 0; }}
        class="w-36 px-1.5 py-0.5 rounded text-xs font-mono bg-slate-800 border {priorityCalls.length > 0 ? 'border-amber-500/60 text-amber-300' : 'border-slate-600 text-slate-400'} placeholder-slate-600 focus:outline-none focus:border-amber-500/80"
        title="Watch calls: comma or space separated — only call these stations when decoded (Auto must be ON)" />

    {/if}

    <!-- Right side: period countdown + TX status -->
    <div class="ml-auto flex items-center gap-2">

      {#if countdown}
        <div class="flex items-center gap-1.5" title="{ftxTXStatus.mode} period — {ftxTXStatus.transmitting ? 'TX' : 'RX'}">
          <span class="text-[10px] font-mono text-slate-500">{ftxTXStatus.mode}</span>
          <div class="relative w-16 h-1.5 rounded-full bg-slate-700/80 overflow-hidden">
            <div class="absolute inset-y-0 left-0 rounded-full"
              style="width:{countdown.pct}%; background:{ftxTXStatus.transmitting ? '#ef4444' : '#3b82f6'}; transition:width 0.25s linear" />
          </div>
          <span class="text-[10px] font-mono w-7 text-right {ftxTXStatus.transmitting ? 'text-red-400' : 'text-slate-400'}">{countdown.remaining.toFixed(1)}s</span>
        </div>
      {/if}

      {#if ftxTXStatus.transmitting}
        <span class="flex items-center gap-1.5 px-2 py-0.5 rounded bg-red-500/15 border border-red-500/40 text-red-300 text-xs font-mono font-semibold animate-pulse">
          <span class="w-1.5 h-1.5 rounded-full bg-red-400 inline-block"></span>
          TX {ftxTXStatus.message}
        </span>
      {:else}
        <span class="text-slate-500">Last: <span class="text-slate-400">{lastPeriodCount}</span></span>
      {/if}

    </div>

  </div>


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
</div><!-- end left column -->

<!-- ── Right: QRZ panel ───────────────────────────────────────────────────── -->
<div class="w-72 flex-shrink-0">
  <QRZPanel callsign={qrzCallsign} {myGrid} {mostWantedMap} />
</div>

</div><!-- end outer flex -->

<script>
  import { onMount, onDestroy } from 'svelte';

  export let myGrid = '';

  let canvas;
  let now = new Date();
  let myLat = null;
  let myLon = null;
  let myStatus = 'unknown';  // 'day' | 'greyline' | 'night'
  let sunrise = null;        // UTC decimal hours
  let sunset  = null;

  // ── Math helpers ───────────────────────────────────────────────────────────

  function gridToLatLon(grid) {
    if (!grid || grid.length < 4) return null;
    const g = grid.toUpperCase();
    let lon = (g.charCodeAt(0) - 65) * 20 - 180 + parseInt(g[2]) * 2 + 1;
    let lat = (g.charCodeAt(1) - 65) * 10 - 90  + parseInt(g[3]) * 1 + 0.5;
    if (g.length >= 6) {
      lon = lon - 1 + (g.charCodeAt(4) - 65) * (2 / 24) + (1 / 24);
      lat = lat - 0.5 + (g.charCodeAt(5) - 65) * (1 / 24) + (1 / 48);
    }
    return { lat, lon };
  }

  function getSunSubsolar(date) {
    const JD = date.getTime() / 86400000 + 2440587.5;
    const n  = JD - 2451545.0;
    const L  = ((280.46646 + 0.9856474  * n) % 360 + 360) % 360;
    const M  = ((357.52911 + 0.98560028 * n) % 360 + 360) % 360;
    const Mr = M * Math.PI / 180;
    const C  = 1.914602 * Math.sin(Mr) + 0.019993 * Math.sin(2 * Mr) + 0.000289 * Math.sin(3 * Mr);
    const sl = (L + C) * Math.PI / 180;
    const e  = (23.439 - 0.0000004 * n) * Math.PI / 180;
    const RA = Math.atan2(Math.cos(e) * Math.sin(sl), Math.cos(sl)) * 180 / Math.PI;
    const dec = Math.asin(Math.sin(e) * Math.sin(sl)) * 180 / Math.PI;
    const GMST = ((280.46061837 + 360.98564736629 * (JD - 2451545.0)) % 360 + 360) % 360;
    let subLon = ((RA - GMST) % 360 + 360) % 360;
    if (subLon > 180) subLon -= 360;
    return { lat: dec, lon: subLon };
  }

  function angDistToSun(lat, lon, sun) {
    const φ1 = lat * Math.PI / 180, λ1 = lon * Math.PI / 180;
    const φ2 = sun.lat * Math.PI / 180, λ2 = sun.lon * Math.PI / 180;
    const a = Math.sin((φ2 - φ1) / 2) ** 2 + Math.cos(φ1) * Math.cos(φ2) * Math.sin((λ2 - λ1) / 2) ** 2;
    return 2 * Math.asin(Math.sqrt(Math.min(1, a))) * 180 / Math.PI;
  }

  function solarStatus(lat, lon, sun) {
    const d = angDistToSun(lat, lon, sun);
    if (d < 82) return { s: 'day',      d };
    if (d > 98) return { s: 'night',    d };
    return          { s: 'greyline', d };
  }

  function destination(lat, lon, az, km) {
    const R = 6371, δ = km / R, θ = az * Math.PI / 180;
    const φ1 = lat * Math.PI / 180, λ1 = lon * Math.PI / 180;
    const sinφ2 = Math.sin(φ1) * Math.cos(δ) + Math.cos(φ1) * Math.sin(δ) * Math.cos(θ);
    const φ2    = Math.asin(Math.max(-1, Math.min(1, sinφ2)));
    const λ2    = λ1 + Math.atan2(Math.sin(θ) * Math.sin(δ) * Math.cos(φ1), Math.cos(δ) - Math.sin(φ1) * sinφ2);
    return { lat: φ2 * 180 / Math.PI, lon: ((λ2 * 180 / Math.PI + 540) % 360) - 180 };
  }

  function calcSunriseSunset(lat, lon, date) {
    const JD = date.getTime() / 86400000 + 2440587.5;
    const n  = JD - 2451545.0;
    const sun = getSunSubsolar(date);
    const decR = sun.lat * Math.PI / 180, latR = lat * Math.PI / 180;
    const cosH = (Math.sin(-0.8333 * Math.PI / 180) - Math.sin(latR) * Math.sin(decR))
               / (Math.cos(latR) * Math.cos(decR));
    if (cosH < -1) return { sunrise: null, sunset: null, polar: 'day'   };
    if (cosH >  1) return { sunrise: null, sunset: null, polar: 'night' };
    const H = Math.acos(cosH) * 180 / Math.PI;
    const L = ((280.46646 + 0.9856474  * n) % 360 + 360) % 360;
    const M = ((357.52911 + 0.98560028 * n) % 360 + 360) % 360;
    const Mr = M * Math.PI / 180;
    const C  = 1.914602 * Math.sin(Mr) + 0.019993 * Math.sin(2 * Mr);
    const sl  = (L + C) * Math.PI / 180;
    const e   = (23.439 - 0.0000004 * n) * Math.PI / 180;
    const RA  = Math.atan2(Math.cos(e) * Math.sin(sl), Math.cos(sl)) * 180 / Math.PI;
    const GMST = ((280.46061837 + 360.98564736629 * (JD - 2451545.0)) % 360 + 360) % 360;
    let noon = (((RA - GMST - lon) % 360) + 360) % 360 / 15;
    noon = ((noon % 24) + 24) % 24;
    return {
      sunrise: ((noon - H / 15) + 24) % 24,
      sunset:  ((noon + H / 15) + 24) % 24,
      polar: null
    };
  }

  function sectorColor(s, d) {
    if (s === 'greyline') {
      const prox = 1 - Math.abs(d - 90) / 8;
      return `rgba(251,191,36,${(0.55 + prox * 0.45).toFixed(2)})`;
    }
    if (s === 'day') return 'rgba(186,230,253,0.72)';
    return 'rgba(6,12,30,0.93)';
  }

  // ── Canvas rendering ───────────────────────────────────────────────────────

  function drawChart(sun) {
    if (!canvas || myLat === null || myLon === null) return;

    const dpr  = window.devicePixelRatio || 1;
    const SIZE = 200;
    canvas.width        = SIZE * dpr;
    canvas.height       = SIZE * dpr;
    canvas.style.width  = SIZE + 'px';
    canvas.style.height = SIZE + 'px';

    const ctx   = canvas.getContext('2d');
    ctx.scale(dpr, dpr);

    const cx    = SIZE / 2;
    const cy    = SIZE / 2;
    const OUTER = SIZE / 2 - 14;
    const MID   = OUTER * 0.46;
    const STEP  = 2;

    // Dark-space background
    ctx.clearRect(0, 0, SIZE, SIZE);
    ctx.beginPath();
    ctx.arc(cx, cy, OUTER, 0, 2 * Math.PI);
    ctx.fillStyle = 'rgb(6,10,24)';
    ctx.fill();

    for (let az = 0; az < 360; az += STEP) {
      const aS   = (az - 90) * Math.PI / 180;
      const aE   = (az - 90 + STEP) * Math.PI / 180;
      const mid  = az + STEP / 2;

      // Inner ring — 2 000 km
      const p2k = destination(myLat, myLon, mid, 2000);
      const { s: s2, d: d2 } = solarStatus(p2k.lat, p2k.lon, sun);
      ctx.beginPath();
      ctx.moveTo(cx, cy);
      ctx.arc(cx, cy, MID, aS, aE);
      ctx.closePath();
      ctx.fillStyle = sectorColor(s2, d2);
      ctx.fill();

      // Outer ring — 7 000 km
      const p7k = destination(myLat, myLon, mid, 7000);
      const { s: s7, d: d7 } = solarStatus(p7k.lat, p7k.lon, sun);
      ctx.beginPath();
      ctx.arc(cx, cy, OUTER, aS, aE);
      ctx.arc(cx, cy, MID,   aE, aS, true);
      ctx.closePath();
      ctx.fillStyle = sectorColor(s7, d7);
      ctx.fill();
    }

    // Ring separator
    ctx.beginPath();
    ctx.arc(cx, cy, MID, 0, 2 * Math.PI);
    ctx.strokeStyle = 'rgba(148,163,184,0.2)';
    ctx.lineWidth = 0.5;
    ctx.stroke();

    // Outer border
    ctx.beginPath();
    ctx.arc(cx, cy, OUTER, 0, 2 * Math.PI);
    ctx.strokeStyle = 'rgba(148,163,184,0.45)';
    ctx.lineWidth = 1;
    ctx.stroke();

    // Cross-hairs
    ctx.strokeStyle = 'rgba(148,163,184,0.18)';
    ctx.lineWidth   = 0.5;
    ctx.setLineDash([2, 5]);
    for (const [dx, dy] of [[0,-1],[0,1],[1,0],[-1,0]]) {
      ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(cx + dx * OUTER, cy + dy * OUTER); ctx.stroke();
    }
    ctx.setLineDash([]);

    // Compass labels
    ctx.font          = 'bold 10px system-ui,sans-serif';
    ctx.textAlign     = 'center';
    ctx.textBaseline  = 'middle';
    ctx.fillStyle     = 'rgba(226,232,240,0.85)';
    const LBL = OUTER + 9;
    ctx.fillText('N', cx,       cy - LBL);
    ctx.fillText('S', cx,       cy + LBL);
    ctx.fillText('E', cx + LBL, cy);
    ctx.fillText('W', cx - LBL, cy);

    // My station dot
    const { s: ms } = solarStatus(myLat, myLon, sun);
    ctx.beginPath();
    ctx.arc(cx, cy, 5, 0, 2 * Math.PI);
    ctx.fillStyle   = ms === 'greyline' ? '#fbbf24' : ms === 'day' ? '#38bdf8' : '#64748b';
    ctx.fill();
    ctx.strokeStyle = 'rgba(255,255,255,0.8)';
    ctx.lineWidth   = 1.5;
    ctx.stroke();
  }

  // ── Data update ────────────────────────────────────────────────────────────

  function tick() {
    now = new Date();
    if (!myGrid) return;

    const pos = gridToLatLon(myGrid);
    if (!pos) return;
    myLat = pos.lat;
    myLon = pos.lon;

    const sun = getSunSubsolar(now);
    myStatus = solarStatus(myLat, myLon, sun).s;

    const sr = calcSunriseSunset(myLat, myLon, now);
    sunrise  = sr.sunrise;
    sunset   = sr.sunset;

    drawChart(sun);
  }

  let drawTimer, clockTimer;

  onMount(() => {
    tick();
    drawTimer  = setInterval(tick, 60000);          // full update every minute
    clockTimer = setInterval(() => { now = new Date(); }, 1000); // clock only
  });

  onDestroy(() => { clearInterval(drawTimer); clearInterval(clockTimer); });

  // Re-draw when grid changes
  $: myGrid && tick();

  // ── Helpers ────────────────────────────────────────────────────────────────

  function fmtUTC(decH) {
    if (decH == null) return '--:--';
    const t = ((decH % 24) + 24) % 24;
    const h = Math.floor(t);
    const m = Math.floor((t - h) * 60);
    return `${h.toString().padStart(2,'0')}:${m.toString().padStart(2,'0')}`;
  }

  $: timeUntilNext = (() => {
    if (sunrise == null || sunset == null) return null;
    const nowH = now.getUTCHours() + now.getUTCMinutes() / 60;
    const toSR = ((sunrise - nowH + 24) % 24);
    const toSS = ((sunset  - nowH + 24) % 24);
    const evt  = toSR <= toSS ? { type: 'sunrise', h: toSR } : { type: 'sunset', h: toSS };
    const min  = Math.round(evt.h * 60);
    const hh   = Math.floor(min / 60);
    const mm   = min % 60;
    return { type: evt.type, label: hh ? `${hh}h ${mm}m` : `${mm}m` };
  })();

  $: utcStr = `${now.getUTCHours().toString().padStart(2,'0')}:${now.getUTCMinutes().toString().padStart(2,'0')}:${now.getUTCSeconds().toString().padStart(2,'0')} UTC`;
</script>

<div class="h-full overflow-y-auto p-3 flex flex-col gap-2.5">

  {#if !myGrid || myLat === null}
    <div class="flex-1 flex items-center justify-center">
      <p class="text-slate-500 text-sm text-center leading-relaxed">
        Configure your Maidenhead<br>grid square in<br>
        <span class="text-slate-400">Settings → General</span>
      </p>
    </div>

  {:else}

    <!-- Polar chart -->
    <div class="flex flex-col items-center gap-1.5">
      <canvas bind:this={canvas} class="block mx-auto rounded-full"></canvas>

      <!-- Rings legend -->
      <div class="text-[9px] text-slate-600 text-center">
        Inner · 2 000 km &nbsp;|&nbsp; Outer · 7 000 km
      </div>

      <!-- Color legend -->
      <div class="flex gap-3 text-[10px]">
        {#each [['rgba(186,230,253,0.72)','Day'],['rgba(251,191,36,1)','Greyline'],['rgb(6,10,24)','Night','border border-slate-700']] as [c,lbl,extra]}
          <span class="flex items-center gap-1">
            <span class="w-3 h-3 rounded-sm {extra||''}" style="background:{c}"></span>
            <span class="text-slate-400">{lbl}</span>
          </span>
        {/each}
      </div>
    </div>

    <!-- My station status -->
    <div class="flex items-center justify-between px-2.5 py-1.5 rounded-lg border text-xs
      {myStatus === 'greyline' ? 'bg-amber-500/10 border-amber-500/30 text-amber-400' :
       myStatus === 'day'      ? 'bg-sky-500/10   border-sky-500/30   text-sky-400'   :
                                 'bg-slate-700/30 border-slate-600    text-slate-400'}">
      <span class="font-semibold">
        {myStatus === 'greyline' ? '🌅 Greyline' : myStatus === 'day' ? '☀ Day' : '🌙 Night'}
      </span>
      <span class="font-mono text-[10px] opacity-70">{myGrid.toUpperCase()}</span>
    </div>

    <!-- Sunrise / Sunset -->
    <div class="grid grid-cols-2 gap-2">
      <div class="flex flex-col items-center py-2 rounded bg-slate-800/60 border border-slate-700/40">
        <span class="text-[9px] text-slate-500 mb-0.5">🌅 Sunrise</span>
        <span class="text-sm font-mono font-bold text-yellow-400">{fmtUTC(sunrise)}</span>
        <span class="text-[9px] text-slate-600">UTC</span>
      </div>
      <div class="flex flex-col items-center py-2 rounded bg-slate-800/60 border border-slate-700/40">
        <span class="text-[9px] text-slate-500 mb-0.5">🌇 Sunset</span>
        <span class="text-sm font-mono font-bold text-orange-400">{fmtUTC(sunset)}</span>
        <span class="text-[9px] text-slate-600">UTC</span>
      </div>
    </div>

    <!-- Next event countdown -->
    {#if timeUntilNext}
      <div class="text-center text-xs bg-slate-800/40 rounded py-1.5 border border-slate-700/40">
        <span class="text-slate-500">Next:</span>
        <span class="font-semibold text-slate-300 ml-1">
          {timeUntilNext.type === 'sunrise' ? '🌅' : '🌇'}
          {timeUntilNext.type} in {timeUntilNext.label}
        </span>
      </div>
    {/if}

    <!-- UTC clock -->
    <div class="text-center text-[10px] text-slate-600 font-mono">{utcStr}</div>

  {/if}
</div>

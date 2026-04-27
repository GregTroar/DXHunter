<script>
  import { onMount, onDestroy } from 'svelte';
  import { worldPolys } from '../lib/worldPolys.js';

  export let myGrid = '';

  let canvas;
  let mapContainer;
  let now = new Date();
  let myLat = null;
  let myLon = null;
  let myStatus = 'unknown';  // 'day' | 'greyline' | 'night'
  let sunrise = null;
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

  function calcSunriseSunset(lat, lon, date) {
    const JD  = date.getTime() / 86400000 + 2440587.5;
    const n   = JD - 2451545.0;

    // GMST must be evaluated at midnight UTC (0h UT) so the result is an
    // absolute transit time in UTC hours, not a relative offset from now.
    const midnight = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
    const JD0  = midnight.getTime() / 86400000 + 2440587.5;
    const n0   = JD0 - 2451545.0;

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
    const GMST = ((280.46061837 + 360.98564736629 * n0) % 360 + 360) % 360;
    let noon = (((RA - GMST - lon) % 360) + 360) % 360 / 15;
    noon = ((noon % 24) + 24) % 24;
    return {
      sunrise: ((noon - H / 15) + 24) % 24,
      sunset:  ((noon + H / 15) + 24) % 24,
      polar: null
    };
  }

  // ── World map rendering ────────────────────────────────────────────────────

  function drawWorldMap(sun) {
    if (!canvas || myLat === null || myLon === null) return;

    const W = mapContainer ? mapContainer.clientWidth : 500;
    const H = Math.round(W / 2);
    const dpr = window.devicePixelRatio || 1;
    const iw  = Math.round(W * dpr);
    const ih  = Math.round(H * dpr);

    canvas.width  = iw;
    canvas.height = ih;
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';

    const ctx = canvas.getContext('2d');

    // geo → physical pixels
    const toXY = (lat, lon) => ({
      x: ((lon + 180) / 360) * iw,
      y: ((90 - lat)  / 180) * ih
    });

    // ── Ocean background ──────────────────────────────────────────────────────
    ctx.fillStyle = '#1e4080';
    ctx.fillRect(0, 0, iw, ih);

    // ── Land polygons ─────────────────────────────────────────────────────────
    // worldPolys entries: [[lon,lat], ...] (GeoJSON order)
    ctx.fillStyle   = '#4d7a4d';
    ctx.strokeStyle = 'rgba(60,100,60,0.6)';
    ctx.lineWidth   = 0.4 * dpr;

    for (const ring of worldPolys) {
      ctx.beginPath();
      for (let i = 0; i < ring.length; i++) {
        const p = toXY(ring[i][1], ring[i][0]); // [1]=lat, [0]=lon
        if (i === 0) ctx.moveTo(p.x, p.y);
        else         ctx.lineTo(p.x, p.y);
      }
      ctx.closePath();
      ctx.fill();
      ctx.stroke();
    }

    // ── Night + greyline overlay (offscreen canvas) ───────────────────────────
    const offscreen = document.createElement('canvas');
    offscreen.width  = iw;
    offscreen.height = ih;
    const octx = offscreen.getContext('2d');
    const mask = octx.createImageData(iw, ih);
    const d    = mask.data;

    const GL = 1.5; // greyline half-width in degrees (3° total band)

    for (let py = 0; py < ih; py++) {
      const lat = 90 - (py / ih) * 180;
      for (let px = 0; px < iw; px++) {
        const lon = (px / iw) * 360 - 180;
        const ang = angDistToSun(lat, lon, sun);
        const i   = (py * iw + px) * 4;

        if (ang <= 90 - GL) {
          // Full day — transparent
          d[i + 3] = 0;
        } else if (ang >= 90 + GL) {
          // Full night — dark navy overlay
          d[i] = 0; d[i + 1] = 5; d[i + 2] = 25; d[i + 3] = 185;
        } else {
          // Greyline zone — golden glow at terminator
          const t   = (ang - (90 - GL)) / (GL * 2); // 0=day, 1=night
          const gld = Math.sin(t * Math.PI);          // peaks at 90°
          if (t < 0.5) {
            // Day-side glow
            d[i] = 251; d[i + 1] = 160; d[i + 2] = 0;
            d[i + 3] = Math.round(gld * 180);
          } else {
            // Night-side glow fading into dark
            const nt    = (t - 0.5) * 2;
            const gold  = Math.round(gld * 180);
            const dark  = Math.round(185 * nt);
            if (gold >= dark) {
              d[i] = 251; d[i + 1] = 160; d[i + 2] = 0; d[i + 3] = gold;
            } else {
              d[i] = 0; d[i + 1] = 5; d[i + 2] = 25; d[i + 3] = dark;
            }
          }
        }
      }
    }

    octx.putImageData(mask, 0, 0);
    ctx.drawImage(offscreen, 0, 0);

    const lw = dpr;

    // ── Lat/lon grid ──────────────────────────────────────────────────────────
    ctx.strokeStyle = 'rgba(255,255,255,0.1)';
    ctx.lineWidth   = lw * 0.5;
    ctx.setLineDash([]);

    for (let lon = -150; lon <= 180; lon += 30) {
      const x = toXY(0, lon).x;
      ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, ih); ctx.stroke();
    }
    for (let lat = -60; lat <= 60; lat += 30) {
      const y = toXY(lat, 0).y;
      ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(iw, y); ctx.stroke();
    }

    // Equator & prime meridian slightly brighter
    ctx.strokeStyle = 'rgba(255,255,255,0.22)';
    ctx.lineWidth   = lw * 0.8;
    { const y = toXY(0, 0).y;  ctx.beginPath(); ctx.moveTo(0, y);  ctx.lineTo(iw, y);  ctx.stroke(); }
    { const x = toXY(0, 0).x;  ctx.beginPath(); ctx.moveTo(x, 0);  ctx.lineTo(x, ih);  ctx.stroke(); }

    // Tropics & polar circles — dashed
    ctx.setLineDash([4 * dpr, 6 * dpr]);
    ctx.strokeStyle = 'rgba(255,255,255,0.1)';
    ctx.lineWidth   = lw * 0.5;
    for (const lt of [23.4, -23.4, 66.6, -66.6]) {
      const y = toXY(lt, 0).y;
      ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(iw, y); ctx.stroke();
    }
    ctx.setLineDash([]);

    // ── Subsolar point ────────────────────────────────────────────────────────
    const sp = toXY(sun.lat, sun.lon);
    ctx.beginPath();
    ctx.arc(sp.x, sp.y, 6 * dpr, 0, 2 * Math.PI);
    ctx.fillStyle   = '#fbbf24';
    ctx.fill();
    ctx.strokeStyle = 'rgba(255,255,255,0.9)';
    ctx.lineWidth   = 1.5 * dpr;
    ctx.stroke();

    // ── My station crosshair ──────────────────────────────────────────────────
    const mp = toXY(myLat, myLon);
    const cs = 8 * dpr;
    ctx.strokeStyle = '#f87171';
    ctx.lineWidth   = 2 * dpr;
    ctx.beginPath(); ctx.moveTo(mp.x - cs, mp.y); ctx.lineTo(mp.x + cs, mp.y); ctx.stroke();
    ctx.beginPath(); ctx.moveTo(mp.x, mp.y - cs); ctx.lineTo(mp.x, mp.y + cs); ctx.stroke();
    ctx.beginPath();
    ctx.arc(mp.x, mp.y, 3 * dpr, 0, 2 * Math.PI);
    ctx.fillStyle = '#f87171';
    ctx.fill();
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
    drawWorldMap(sun);
  }

  let drawTimer, clockTimer, resizeObs;

  onMount(() => {
    tick();
    drawTimer  = setInterval(tick, 60000);
    clockTimer = setInterval(() => { now = new Date(); }, 1000);

    if (mapContainer) {
      resizeObs = new ResizeObserver(() => {
        if (myLat !== null) drawWorldMap(getSunSubsolar(now));
      });
      resizeObs.observe(mapContainer);
    }
  });

  onDestroy(() => {
    clearInterval(drawTimer);
    clearInterval(clockTimer);
    if (resizeObs) resizeObs.disconnect();
  });

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

    <!-- World map -->
    <div bind:this={mapContainer} class="w-full rounded-lg overflow-hidden border border-slate-700/50">
      <canvas bind:this={canvas} class="block"></canvas>
    </div>

    <!-- Legend -->
    <div class="flex gap-4 text-[10px] justify-center items-center">
      <span class="flex items-center gap-1.5">
        <span class="w-3 h-3 rounded-sm" style="background:#4d7a4d; border:1px solid rgba(60,100,60,0.6)"></span>
        <span class="text-slate-400">Land</span>
      </span>
      <span class="flex items-center gap-1.5">
        <span class="w-3 h-3 rounded-sm" style="background:#1e4080"></span>
        <span class="text-slate-400">Ocean</span>
      </span>
      <span class="flex items-center gap-1.5">
        <span class="w-3 h-3 rounded-sm" style="background:rgba(0,5,25,0.73)"></span>
        <span class="text-slate-400">Night</span>
      </span>
      <span class="flex items-center gap-1.5">
        <span class="w-3 h-3 rounded-sm" style="background:rgb(251,160,0)"></span>
        <span class="text-slate-400">Greyline</span>
      </span>
      <span class="flex items-center gap-1.5">
        <span class="w-2.5 h-2.5 rounded-full bg-yellow-400"></span>
        <span class="text-slate-400">Sun</span>
      </span>
      <span class="flex items-center gap-1.5">
        <span class="w-2.5 h-2.5 rounded-full bg-red-400"></span>
        <span class="text-slate-400">QTH</span>
      </span>
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

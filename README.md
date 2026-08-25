# DXHunter

<a href="https://www.paypal.com/donate?hosted_button_id=R9TC4KXWR96UE" target="_blank">
  <img src="https://www.paypalobjects.com/en_US/i/btn/btn_donate_SM.gif" alt="Donate with PayPal" />
</a>

<a href="https://discord.gg/tmVPdQ5A8" target="_blank">
  <img src="https://img.shields.io/badge/Discord-Join%20the%20server-5865F2?logo=discord&logoColor=white" alt="Join our Discord" />
</a>

**DXHunter is a DX cluster client built for one purpose: helping you chase new DXCC entities, new bands, new modes and new slots as well as any of the Hamaward.cloud awards.**

Every spot arriving from the cluster is cross-referenced against your logbook (**Log4OM** or **Ham Radio Deluxe** or **OpsLog**) and classified — New DXCC, New Band, New Mode, New Slot, or Already Worked. Interesting spots are pushed directly to your **FlexRadio panadapter** so you can tune with a single click. FT8/FT4 decodes are enriched the same way in real time.

Written in Go (backend) and Svelte + TailwindCSS (frontend). No external dependencies at runtime — the web dashboard is embedded in the binary.

---

## What it does

| Goal | How |
|---|---|
| Chase **New DXCC** | Every spot is cross-referenced against your logbook. New entities are highlighted in green. |
| Chase **New Band / New Mode** | Per-band and per-mode worked status computed from your log. Yellow = new band, orange = new mode, purple = new band+mode. |
| Chase **New Slots** | New Band+Mode combinations (slots) highlighted in light blue. |
| **CAT control** | **OmniRig** (any compatible transceiver: Kenwood, Yaesu, Icom, Elecraft…) or **FlexRadio**. Click a spot to tune the radio instantly. |
| **FlexRadio integration** | Spots pushed to the panadapter with color coding. Click a spot in the UI to tune the radio. |
| **FTx monitoring** | WSJT-X / JTDX / MSHV decodes enriched with log status badges in real time. |
| Chase **Hamaward awards** | Set a 3-letter award code — matching stations are auto-watched and prioritised in autocall. |
| **Stay informed** | Watchlist alerts, POTA/SOTA activations, DXpedition calendar, DX-World news, solar data. |
| **Track your progress** | ClubLog-style Logbook dashboard: per-band DXCC chart, newest entity worked, current QSO streak, longest path (30d), 365-day activity counter. |
| **Use it anywhere** | Responsive web UI — phone, tablet, desktop. Open `http://<host>:8080` from any device on the network. |

---

## First Run — Setup Wizard

On first launch (no `config.yml` found), a **setup wizard** opens automatically in the browser. No manual config editing needed.

The wizard collects:
- Your callsign and Maidenhead grid locator
- **CAT control**: OmniRig (rig 1 or 2) and/or FlexRadio IP (leave empty for auto-discovery)
- Logbook database path (Log4OM or Ham Radio Deluxe SQLite)
- DX cluster servers (pre-filled with F4BPO Cluster and POTA Cluster)

After completing the wizard, the application starts all services automatically and the dashboard loads.

---

## Features

### DX Cluster
- Connect to **multiple DX cluster servers simultaneously** (DXSpider, CC-Cluster, AR-Cluster, **GoCluster** (N2WQ) — auto-detected or forced via config)
- Designate a **master cluster** for command routing; additional clusters are read-only spot sources
- Per-cluster filters: Skimmer, FT8, FT4, Beacon — automatically translated to the cluster's syntax (`set/skimmer` for DXSpider/CC, `PASS SOURCE SKIMMER` for GoCluster, etc.)
- Console tab with quick-command buttons per cluster type, including GoCluster's `PASS DEZONE` for regional (EU/NA/…) filtering
- Send custom login commands per cluster (e.g. `SET/FILTER` for geographic filtering)
- Automatic reconnection on disconnect
- Spot deduplication and configurable TTL (spot lifetime)
- **Suffix-aware dedup**: when the same operator is spotted both as bare call and with `/P`, `/M`, `/MM`, `/AM`, `/QRP`, `/A` on the same band, the more specific suffix variant wins — no more "OH8NW next to OH8NW/P" duplicates
- **Indexed SQLite DB** with WAL + tuned pragmas: handles flood-level skimmer traffic (300+ spots/sec from GoCluster + RBN) without backing up the pipeline
- **Non-blocking TCP reader**: each spot is processed in its own goroutine so the socket is never stalled

### Spot Display & Filtering
- Real-time spot list: callsign, frequency, band, mode, DXCC country, spotter, time, comment
- **Log status badges**: New DXCC / New Band / New Mode / New Slot / Worked — resolved from in-memory logbook cache (refreshed every 30 seconds)
- **LoTW indicator**: marks spots from confirmed LoTW users (~150k users downloaded at startup)
- **Color-coded spots** — fully configurable in Settings:
  - New DXCC → green text
  - New Band → yellow text
  - New Mode → orange text
  - New Band+Mode → purple text
  - New Slot → light blue text
  - My callsign → red text
  - Already Worked → white on cyan background
- Filter by: type (New DXCC, New Band, New Mode, New Slot, Worked, Watchlist, POTA, SOTA), band (160M–70cm), mode (CW, SSB, FT8, FT4, FT2, RTTY)
- Bands supported: 160M, 80M, 60M, 40M, 30M, 20M, 17M, 15M, 12M, 10M, 6M, 2M, 70cm
- Web Worker-based filtering for smooth UI on large spot lists
- Sortable columns (callsign, country, mode, frequency)
- **Smooth display**: spot list refreshes at 500 ms cadence regardless of burst rate from the cluster

### Logbook Integration & Dashboard
- Supports **Log4OM 2** (SQLite or MySQL), **Ham Radio Deluxe** (SQLite) and **OpsLog** (SQLite or MySQL)
- **In-memory cache**: all contacts are loaded at startup into indexed maps and refreshed every 30 seconds — spot enrichment is O(1) with zero database I/O on the critical path
- Computes worked status for every spot: New DXCC, New Band, New Mode, New Band+Mode Slot, Worked
- **ClubLog-style Logbook dashboard**:
  - Counters: Today / This Week / This Month / **365 days** / Total
  - **Newest entity worked**: most recent first-worked DXCC + date
  - **Current QSO streak**: consecutive UTC days with at least one QSO
  - **Longest path (30d)**: greatest QSO distance over the last month (haversine from your Maidenhead grid to the worked DXCC's center)
  - **DXCC Progress** overall bar (worked + confirmed via LoTW/QSL)
  - **DXCC progress by band**: 160M → 6M with worked vs confirmed bars
  - **Total slots progress bar**: confirmed / worked across all visible bands
  - **Recent QSOs**: last 10 contacts
- **Auto-refresh on QSO log**: a freshly logged contact in Log4OM is detected within 30 s; matching panadapter spots are immediately re-painted with the *Worked* color and `[Worked]` tag so the operator sees the change without restarting anything
- **DXCC name normalization**: cty.plist regional labels that share an ADIF code with their parent (Sicily → Italy) are collapsed to the canonical name
### CAT Control
Tune the radio with a single click on any spot. Two backends supported, **OmniRig has priority** when enabled:

- **OmniRig (Windows)** — works with **any OmniRig-compatible transceiver** (Kenwood, Yaesu, Icom, Elecraft…). Sets frequency and mode directly via COM. No need for Log4OM-as-CAT-bridge. OmniRig v2 mode constants are mapped correctly (CW_U at bit 23, DIG_U at bit 27, etc.).
- **FlexRadio** — direct SmartSDR TCP, sets slice frequency and mode, auto-zooms the panadapter and adjusts AGC per mode.

Selectable from Settings → CAT Control. Hot-reload: toggling enabled/rig restarts the OmniRig COM client without restarting the application.

### FlexRadio Integration
- **Auto-discovery** via SmartSDR UDP broadcast or **direct IP** connection (local or remote)
- Pushes DX spots directly to the **panadapter** with callsign, frequency, color and comment
- **Live frequency tracking**: spots are filtered to the current active band automatically
- **On spot click (panadapter)** — DXHunter intercepts the trigger and:
  - Sends the callsign to Log4OM (UDP)
  - Switches the slice **mode** (CW / USB / LSB / DIGU / DIGL — DIGU/DIGL chosen by frequency for FT8/FT4/RTTY)
  - Re-tunes the slice exactly on the spot frequency (no audio offset artefacts when crossing mode families)
  - Adjusts panadapter **bandwidth** and slice **AGC** based on the spot's mode group
- **Configurable spot-click behavior** (Settings → FlexRadio → On spot click):
  - Toggle "Center panadapter on spot"
  - Per-mode-group **bandwidth** override: CW / SSB+AM+FM / Digital (FT8/FT4/RTTY/DIGU/…)
  - Per-mode-group **AGC** override: Off / Fast / Medium / Slow
- **Auto-refresh** of any panadapter spot when its QSO gets logged in Log4OM — color updates to Worked instantly
- **Async spot writer**: spot add/remove commands flow through a 2000-slot queue drained by a dedicated goroutine — the cluster pipeline never blocks on Flex's TCP receive, even during PASS SOURCE SKIMMER floods
- **Fast disconnect detection**: 5 s write deadline + 30 s TCP keep-alive → broken connections detected within ~1 min instead of 5
- **Panadapter preserved on reconnect**: transient TCP drops (Wi-Fi blip, keep-alive miss) no longer wipe your spot list — `spot clear` runs once on first connect, never on reconnects
- **Race-orphan cleanup**: when two clusters spot the same DX+band within milliseconds, the duplicate marker that previously stayed on the panadapter is now detected and removed automatically
- Spot lifecycle management: removes expired spots from the panadapter
- Automatic reconnection with indefinite retries

### FTx Monitor — WSJT-X / JTDX / MSHV
- Listens on **UDP multicast** (default `239.255.0.1:2237`) with `SO_REUSEADDR` — shares the port with WSJT-X, GridTracker, Log4OM simultaneously
- Decodes **FT8, FT4 and FT2** messages in real time
- Decodes grouped by 15-second period with a clear visual separator
- **Log status badges** on each decode: New DXCC / New Band / New Mode / New Slot / Worked — enriched from in-memory logbook cache
- Country lookup from callsign (in-memory, instant)
- **LoTW indicator** on each decode
- Watchlist highlight: watched callsigns highlighted directly in the decode list
- Filters: CQ only, My Call only
- **Autocall**: click a decode to send a Reply command back to WSJT-X/MSHV — or let the engine pick the best candidate automatically (prioritises New DXCC > Band > Mode > Slot, then **ClubLog Most Wanted** rank, then SNR)
- **ClubLog Most Wanted** ranking is personalised to your QTH: the list is fetched using your configured callsign so rankings reflect what stations *you* need most, not a global average. A station with a higher Most Wanted rank wins the tiebreaker in autocall and its rank is shown as a badge (`MW #N`) in the autocall status bar
- **Halt TX**: cancel auto-sequence from the dashboard
- Countdown timer to next TX/RX period (15s FT8 / 7.5s FT4 / 3.25s FT2)
- Pause / Resume / Clear

### Watchlist
- Track specific callsigns with optional notes
- Alerts (toast + optional push notification) when a watched station appears on the cluster
- **Suffix-aware matching**: an entry for `RI0SP` also matches `RI0SP/MM`, `RI0SP/P`, prefix-area calls like `W1/F4XYZ`, etc. — no more stale `lastSeen` when the operator is portable
- "Only active" filter: show only watchlist entries with a live spot
- "Only not worked" filter
- **ON AIR badge** based on the server-side `lastSeen` (not the spotter's UTC clock) so the indicator is accurate even when the cluster lags
- Per-callsign **notification toggle**
- **Last seen** timestamp per entry, refreshed every 15 s in the UI
- Spot count per callsign
- **WWFF references** (`FFF-0188`, `DLFF-…`) detected and treated as POTA — same color, priority and click behavior
- **ClubLog enrichment** (optional API key): expedition status, OQRS availability, live 24h QSO count, total QSOs
- 5-minute notification cooldown per callsign to avoid spam
- Contest mode integration: contest-prefixed callsigns auto-added; **auto-cleanup** when the prefix changes — old contest entries that don't match the new prefix are removed automatically
- Worked status on watchlist spots queried live from database (always current)

### QRZ Callsign Lookup
- Look up any callsign directly from the dashboard
- Displays name, QTH, DXCC country, grid locator, license class and bio
- Distance and azimuth calculated from your grid locator to the station's grid
- **ClubLog Most Wanted rank** displayed for the looked-up station (personalised to your QTH)
- Requires a QRZ XML subscription (API key configured in Settings)

### Greyline
- Interactive **greyline map** showing day/night terminator in real time
- Visualise propagation windows toward a target region
- Based on your configured grid locator

### POTA / SOTA Enrichment
- Automatic detection of POTA references (`AA-0036`) and SOTA references (`F/AB-123`) in spot comments
- Fetches **park name** from pota.app API (SQLite cache, 30-day TTL)
- Fetches **summit name** from sota.org.uk API (SQLite cache)

### DXpedition & News
- **Activations tab**: live ADXO DXpedition feed (ng3k.com, refreshed hourly) with bands, modes, QSL info
- **DX-World tab**: latest DX-World.net news articles refreshed automatically

### Notifications
- **Gotify** push notifications: New DXCC, New Band, New Mode, New Band+Mode, Watchlist hit — each individually configurable
- **Windows native toast notifications** for watchlist hits (callsign, country, frequency, mode, spotter)
- Click a toast to tune FlexRadio directly to the spot frequency

### Award Chasing Mode

Designed for chasing **[Hamaward](https://hamaward.cloud/awards)** awards. Each award on Hamaward is identified by a 3-letter code (e.g. `SAC`, `WAE`, `EU`). Set that code as the contest prefix and DXHunter will focus exclusively on eligible stations.

- Toggle award chasing on/off
- Set the **3-letter Hamaward award code** as the contest prefix — only stations whose callsign matches that prefix are tracked
- Optionally add a list of specific callsigns to chase
- Matching stations are **auto-added to the watchlist** at startup
- Watchlist filtered to show only award-eligible stations during a chase session
- FTx autocall prioritises award stations when enabled

### Web Dashboard
- Embedded Svelte SPA served at `http://localhost:8080`
- **Real-time WebSocket** connection for live updates (spots, FTx decodes, band changes, stats)
- **12 themes** selectable from Settings: Dark, Midnight, Dracula, Solarized Dark, Tokyo Night, Gruvbox, Catppuccin Mocha, One Dark, Nord, Cyberpunk, Sepia, Light. Theme is persisted per-browser (localStorage).
- **Stats bar**: total spots received / processed / success rate / New DXCCs / connected clients / total QSOs / active filters
- **Solar data**: SFI, Sunspot number, A-index, K-index
- Main tabs: **Recent Spots**, **FTx Decodes**, **DX-World**
- Sidebar tabs: **DXpedition**, **DX-World**, **Watchlist**, **Logbook**, **Propagation**, **Console**, **App Logs**
- Console tab: real-time cluster message feed + send commands to the master cluster, with quick-command buttons per cluster type (DXSpider, CC, AR, GoCluster)
- App Logs tab: live application log viewer (INFO / WARN / ERROR filter)
- **Settings UI**: edit all configuration from the browser — General, CAT Control (OmniRig + FlexRadio), FlexRadio spot-click behavior, Database/Logbook, FTx, QRZ, ClubLog API key, Spot Colors (with color picker), Telnet server, Gotify, Clusters. No manual `config.yml` editing needed.
- **Mobile / tablet responsive**: layout stacks vertically below 1024 px, StatsCards adapts 2→3→4→6→7 columns, sidebar tabs collapse to icons on phones, page scroll unlocked below `lg` so all panels are reachable on a single column
- **Update check**: polls Gitea/GitHub releases every 10 minutes (toggleable in Settings → General → "Check for new version")

### Telnet Server
- Built-in Telnet server (default `0.0.0.0:7300`) — connect Log4OM or any DX software to receive spots
- Re-broadcasts all incoming cluster spots to connected Telnet clients in standard DX format
- Routes commands from Telnet clients to the master cluster

### Performance & Reliability
- **Indexed spots DB**: covering indexes on `(dx, band)`, `flexSpotNumber` and `timestamp` — every hot-path lookup (dedup, suffix-variant check, click resolution, cleanup) is O(log n)
- **SQLite WAL + tuned pragmas** (`synchronous=NORMAL`, `temp_store=MEMORY`, `busy_timeout=5000`) for sustained burst writes
- **Bounded async queues**: 5000-slot spot ingest, 2000-slot Flex writer — saturated Flex never freezes the cluster pipeline
- **TCP keep-alive 30 s** on Flex socket → dead links detected in ~1 min instead of 5
- **WebSocket concurrent-write panic fixed**: handler replies now flow through the per-client send channel; the write pump remains the sole writer on each socket
- **Panic-to-log on Windows GUI mode**: stderr is redirected to `dxhunter.log` when *Log to file* is enabled so runtime panics are captured instead of vanishing

### Other
- **LoTW user list**: downloaded at startup, used to mark spots and decodes from LoTW-active stations
- **ClubLog API**: expedition and OQRS detection for watchlist entries
- **Hot-reload config**: edit `config.yml` while running — log level, cluster filters and FlexRadio spot-click settings apply immediately
- **DXCC data**: cty.plist loaded at startup, auto-updated in background
- Configurable log level: DEBUG / INFO / WARN
- Optional log to file

---

## Requirements

| Requirement | Notes |
|---|---|
| **Log4OM 2** or **Ham Radio Deluxe** | Required for spot enrichment. Provides the contact database (SQLite or MySQL). |
| FlexRadio + SmartSDR | Optional. Required for panadapter spot display. |
| **OmniRig** (Windows) | Optional. Provides CAT control to any compatible transceiver — alternative to FlexRadio for non-Flex users. |
| WSJT-X / JTDX / MSHV | Optional. Required for FTx decode monitoring. |
| Windows | Primary platform (Windows toast notifications, multicast, OmniRig COM). |

Without a logbook, the following features are unavailable: New DXCC / Band / Mode / Slot detection, Worked status, FTx log badges, QSO stats, DXCC progress.

---

## Setup

### First run
Just launch `DXHunter.exe`. If no `config.yml` is found, the setup wizard opens at `http://localhost:8080`. Fill in your details and click **Save & Start**. Done.

### Manual configuration
If you prefer to edit `config.yml` directly:

```yaml
general:
  callsign: "F4BPO"
  grid: "JN03"
  flexradiospot: true
  telnetserver: true
  delete_log_file_at_start: true
  log_level: INFO

database:
  sqlite: true
  logbook_type: "log4om"   # "log4om" or "hrd"

sqlite:
  sqlite_path: 'C:\Users\you\AppData\Roaming\Log4OM2\Data\log4om.db'

clusters:
  - name: "F4BPO Cluster"
    server: "cluster.f4bpo.com"
    port: "7300"
    login: "F4BPO"
    login_prompt: "login:"
    skimmer: true
    ft8: true
    enabled: true
    master: true

  - name: "POTA Cluster"
    server: "pota-cluster.iz2lsc.eu"
    port: "7373"
    login: "F4BPO"
    login_prompt: "login:"
    enabled: true
    master: false

flex:
  discover: true             # true = auto-discover on LAN
  ip: ""                     # or set a fixed IP

omnirig:
  enabled: false             # set true to use OmniRig for CAT instead of FlexRadio
  rig: 1                     # OmniRig rig number (1 or 2)

telnetserver:
  host: "0.0.0.0"
  port: "7300"
```

### Log4OM setup
1. Log4OM → **Network** → **DX Cluster** → server: `localhost:7300`
2. Log4OM → **Database** → note the SQLite path and set it in `sqlite_path`
3. Optionally enable **UDP Callsign** in Log4OM to receive clicked callsigns from DXHunter

### OmniRig setup
1. Install [OmniRig](http://www.dxatlas.com/OmniRig/) (free, Windows-only)
2. Open OmniRig and configure Rig 1 (or Rig 2) with your transceiver type, COM port and baud rate
3. Verify OmniRig shows status "Online" — DXHunter connects via COM (CoCreateInstance), same mechanism as WSJT-X
4. In DXHunter: Settings → CAT Control → OmniRig → Enabled + select Rig 1 or 2

### Ham Radio Deluxe setup
1. Set `logbook_type: "hrd"` in `config.yml`
2. Set `sqlite_path` to your HRD SQLite database file

### WSJT-X / MSHV / JTDX setup
- **UDP Server address:** `239.255.0.1` (multicast) or `127.0.0.1` (unicast)
- **Port:** 2237
- Enable *Accept UDP requests*

DXHunter shares the UDP port with other apps already listening on 2237.

---

## Running

```bash
DXHunter.exe
# or with explicit config path:
DXHunter.exe --config C:\path\to\config.yml
```

Open `http://localhost:8080` in a browser (or any browser on the network).

---

## Architecture

```
DX Cluster servers ──TCP──► TCPClient.go (DXSpider / CC / AR / GoCluster,
                                          per-cluster filter dialect, async)
WSJT-X / MSHV ─────UDP──► ftx.go
FlexRadio SmartSDR ─TCP──► flexradio.go (async writer goroutine + queue,
                                          5s write deadline, 30s keep-alive)
Transceiver ────CAT──► omnirig_windows.go (COM, STA worker)
Log4OM / HRD DB ───────► database.go ──► logbookcache.go (in-memory, 30s
                                          refresh, per-band stats, newest
                                          entity, streak, longest-path 30d,
                                          last-365-day count, fires
                                          OnNewlyWorked → repaint panadapter)

                    ┌──────────────────────┐
                    │  Go backend          │
                    │  spotprocessor.go    │  dedup (DX+band + /P suffix), TTL, color, click→Worked refresh
                    │  httpserver.go       │  HTTP + WebSocket (500ms spot tick, 15s watchlist tick)
                    │  watchlist.go        │  watchlist engine (prefix-match, suffix-aware)
                    │  pota.go             │  POTA/SOTA cache (WWFF treated as POTA)
                    │  dxcc.go             │  DXCC lookup (cty.plist, Sicily→Italy normalization)
                    │  grid.go             │  Maidenhead grid + haversine (longest-path widget)
                    │  lotw.go             │  LoTW user list
                    │  setup.go            │  first-run wizard
                    │  releasecheck.go     │  update check (10 min)
                    └──────────┬───────────┘
                               │ WebSocket
                    ┌──────────▼───────────┐
                    │  Svelte frontend     │
                    │  (embedded, :8080)   │  responsive: phone / tablet / desktop
                    └──────────────────────┘
                               │ Telnet
                    Log4OM ────┘  (:7300)
```

---

## License

Personal / amateur radio use.

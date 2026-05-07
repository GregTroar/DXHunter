# FlexDXCluster

**FlexDXCluster is a DX cluster client built for one purpose: helping you chase new DXCC entities, new bands, new modes and new slots.**

Every spot arriving from the cluster is cross-referenced against your logbook (**Log4OM** or **Ham Radio Deluxe**) and classified — New DXCC, New Band, New Mode, New Slot, or Already Worked. Interesting spots are pushed directly to your **FlexRadio panadapter** so you can tune with a single click. FT8/FT4 decodes are enriched the same way in real time.

Written in Go (backend) and Svelte + TailwindCSS (frontend). No external dependencies at runtime — the web dashboard is embedded in the binary.

---

## What it does

| Goal | How |
|---|---|
| Chase **New DXCC** | Every spot is cross-referenced against your logbook. New entities are highlighted in green. |
| Chase **New Band / New Mode** | Per-band and per-mode worked status computed from your log. Yellow = new band, orange = new mode, purple = new band+mode. |
| Chase **New Slots** | New Band+Mode combinations (slots) highlighted in light blue. |
| **FlexRadio integration** | Spots pushed to the panadapter with color coding. Click a spot in the UI to tune the radio. |
| **FTx monitoring** | WSJT-X / JTDX / MSHV decodes enriched with log status badges in real time. |
| Chase **Hamaward awards** | Set a 3-letter award code — matching stations are auto-watched and prioritised in autocall. |
| **Stay informed** | Watchlist alerts, POTA/SOTA activations, DXpedition calendar, DX-World news, solar data. |

---

## First Run — Setup Wizard

On first launch (no `config.yml` found), a **setup wizard** opens automatically in the browser. No manual config editing needed.

The wizard collects:
- Your callsign and Maidenhead grid locator
- FlexRadio IP address (or leave empty for auto-discovery)
- Logbook database path (Log4OM or Ham Radio Deluxe SQLite)
- DX cluster servers (pre-filled with F4BPO Cluster and POTA Cluster)

After completing the wizard, the application starts all services automatically and the dashboard loads.

---

## Features

### DX Cluster
- Connect to **multiple DX cluster servers simultaneously** (DXSpider, CC-Cluster, AR-Cluster — auto-detected or forced via config)
- Designate a **master cluster** for command routing; additional clusters are read-only spot sources
- Per-cluster filters: Skimmer, FT8, FT4, Beacon
- Send custom login commands per cluster (e.g. `SET/FILTER` for geographic filtering)
- Automatic reconnection on disconnect
- Spot deduplication and configurable TTL (spot lifetime)
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

### Logbook Integration
- Supports **Log4OM 2** (SQLite or MySQL) and **Ham Radio Deluxe** (SQLite)
- **In-memory cache**: all contacts are loaded at startup into indexed maps and refreshed every 30 seconds — spot enrichment is O(1) with zero database I/O on the critical path
- Computes worked status for every spot: New DXCC, New Band, New Mode, New Band+Mode Slot, Worked
- QSO statistics: today, this week, this month, all-time
- Recent QSO list (Log4OM tab, updated every 5 seconds)
- DXCC progress counter (entities worked / 340)
- Optionally sends **frequency and mode changes** to Log4OM radio control (UDP)

### FlexRadio Integration
- **Auto-discovery** via SmartSDR UDP broadcast or **direct IP** connection (local or remote)
- Pushes DX spots directly to the **panadapter** with callsign, frequency, color and comment
- **Live frequency tracking**: spots are filtered to the current active band automatically
- **Auto-zoom**: panadapter bandwidth adjusts to the mode of each tuned spot (SSB, CW, FT8…)
- **Auto-AGC**: AGC mode adjusted per mode (slow for SSB, fast for CW, med for digital)
- Spot lifecycle management: removes expired spots from the panadapter
- Sends the clicked callsign to Log4OM when a spot is triggered on the panadapter
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
- "Only active" filter: show only watchlist entries with a live spot
- "Only not worked" filter
- Per-callsign **notification toggle**
- **Last seen** timestamp per entry
- Spot count per callsign
- **ClubLog enrichment** (optional API key): expedition status, OQRS availability, live 24h QSO count, total QSOs
- 5-minute notification cooldown per callsign to avoid spam
- Contest mode integration: contest-prefixed callsigns auto-added
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
- Dark theme, responsive layout
- **Stats bar**: total spots received / processed / success rate / New DXCCs / connected clients / total QSOs / active filters
- **Solar data**: SFI, Sunspot number, A-index, K-index
- Main tabs: **Recent Spots**, **FTx Decodes**, **DX-World**
- Sidebar tabs: **Watchlist**, **Activations**, **Log4OM**, **Greyline**, **Console**, **App Logs**
- Console tab: real-time cluster message feed + send commands to the master cluster
- App Logs tab: live application log viewer (INFO / WARN / ERROR filter)
- **Settings UI**: edit all configuration from the browser — no manual `config.yml` editing

### Telnet Server
- Built-in Telnet server (default `0.0.0.0:7300`) — connect Log4OM or any DX software to receive spots
- Re-broadcasts all incoming cluster spots to connected Telnet clients in standard DX format
- Routes commands from Telnet clients to the master cluster

### Other
- **LoTW user list**: downloaded at startup, used to mark spots and decodes from LoTW-active stations
- **ClubLog API**: expedition and OQRS detection for watchlist entries
- **Hot-reload config**: edit `config.yml` while running — log level and cluster filters apply immediately
- **DXCC data**: cty.plist loaded at startup, auto-updated in background
- Configurable log level: DEBUG / INFO / WARN
- Optional log to file

---

## Requirements

| Requirement | Notes |
|---|---|
| **Log4OM 2** or **Ham Radio Deluxe** | Required for spot enrichment. Provides the contact database (SQLite or MySQL). |
| FlexRadio + SmartSDR | Optional. Required for panadapter spot display and one-click tuning. |
| WSJT-X / JTDX / MSHV | Optional. Required for FTx decode monitoring. |
| Windows | Primary platform (Windows toast notifications, multicast). |

Without a logbook, the following features are unavailable: New DXCC / Band / Mode / Slot detection, Worked status, FTx log badges, QSO stats, DXCC progress.

---

## Setup

### First run
Just launch `FlexDXCluster.exe`. If no `config.yml` is found, the setup wizard opens at `http://localhost:8080`. Fill in your details and click **Save & Start**. Done.

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

telnetserver:
  host: "0.0.0.0"
  port: "7300"
```

### Log4OM setup
1. Log4OM → **Network** → **DX Cluster** → server: `localhost:7300`
2. Log4OM → **Database** → note the SQLite path and set it in `sqlite_path`
3. Optionally enable **UDP Callsign** in Log4OM to receive frequency/mode from FlexDXCluster

### Ham Radio Deluxe setup
1. Set `logbook_type: "hrd"` in `config.yml`
2. Set `sqlite_path` to your HRD SQLite database file

### WSJT-X / MSHV / JTDX setup
- **UDP Server address:** `239.255.0.1` (multicast) or `127.0.0.1` (unicast)
- **Port:** 2237
- Enable *Accept UDP requests*

FlexDXCluster shares the UDP port with other apps already listening on 2237.

---

## Running

```bash
FlexDXCluster.exe
# or with explicit config path:
FlexDXCluster.exe --config C:\path\to\config.yml
```

Open `http://localhost:8080` in a browser (or any browser on the network).

---

## Architecture

```
DX Cluster servers ──TCP──► TCPClient.go  (goroutine per spot)
WSJT-X / MSHV ─────UDP──► ftx.go
FlexRadio SmartSDR ─TCP──► flexradio.go
Log4OM / HRD DB ───────► database.go ──► logbookcache.go (in-memory, 30s refresh)

                    ┌──────────────────────┐
                    │  Go backend          │
                    │  spotprocessor.go    │  dedup, TTL, color
                    │  httpserver.go       │  HTTP + WebSocket (500ms spot tick)
                    │  watchlist.go        │  watchlist engine
                    │  pota.go             │  POTA/SOTA cache
                    │  dxcc.go             │  DXCC lookup (cty.plist)
                    │  lotw.go             │  LoTW user list
                    │  setup.go            │  first-run wizard
                    └──────────┬───────────┘
                               │ WebSocket
                    ┌──────────▼───────────┐
                    │  Svelte frontend     │
                    │  (embedded, :8080)   │
                    └──────────────────────┘
                               │ Telnet
                    Log4OM ────┘  (:7300)
```

---

## License

Personal / amateur radio use.

# FlexDXCluster

A real-time DX cluster client with a web-based GUI, designed for FlexRadio users running **Log4OM**. Written in Go (backend) and Svelte + TailwindCSS (frontend).

---

## ⚠️ Log4OM is required

**This software is designed to work with [Log4OM](https://www.log4om.com/) and will not function correctly without it.**

FlexDXCluster reads your Log4OM database (SQLite or MySQL) to determine the worked/new status of every spot and FTx decode. Without a valid Log4OM database connection, the following features will not work:

- New DXCC / New Band / New Mode / New Slot detection
- Already Worked status on spots
- FTx decode log status badges
- QSO statistics and recent contact list
- DXCC progress counter

Log4OM must also be configured to connect to the built-in Telnet server (default `localhost:7301`) to receive DX spots directly in your logging software.

---

## Features

### DX Cluster
- Connect to **multiple DX cluster servers simultaneously** (DXSpider, CC-Cluster, AR-Cluster — auto-detected or forced via config)
- Designate a **master cluster** for command routing; additional clusters are read-only spot sources
- Per-cluster filters: Skimmer, FT8, FT4, Beacon
- Send custom login commands per cluster (e.g. `SET/FILTER` for geographic filtering)
- Automatic reconnection on disconnect
- Spot deduplication and configurable TTL (spot lifetime)

### Spot Display & Filtering
- Real-time spot list with: callsign, frequency, band, mode, DXCC country, spotter, time, comment
- **Log status badges**: New DXCC / New Band / New Mode / New Slot / Worked — queried live from Log4OM
- **LoTW indicator**: marks spots from confirmed LoTW users (downloaded at startup, ~150k users)
- **Color-coded spots** — fully configurable in `config.yml`:
  - New DXCC → green text
  - New Band → yellow text
  - New Mode → orange text
  - New Band+Mode → purple text
  - New Slot → light blue text
  - My callsign → red text
  - Already Worked → white on cyan background
- Filter by: type (New DXCC, New Band, New Mode, New Slot, Worked, Watchlist, POTA, SOTA), band (160M–6M), mode (CW, SSB, FT8, FT4, FT2, RTTY)
- Web Worker-based filtering for smooth UI on large spot lists
- Sortable columns (callsign, country, mode, frequency)

### Log4OM Integration
- Reads your **Log4OM SQLite or MySQL database** directly
- Computes worked status for every spot: New DXCC, New Band, New Mode, New Band+Mode Slot, Worked
- QSO statistics: today, this week, this month, all-time
- Recent QSO list
- DXCC progress counter (entities worked / 340)
- Optionally sends **frequency and mode changes** to Log4OM radio control (UDP)
- Per-DXCC contact cache to minimise database load during high-traffic FTx periods

### FlexRadio Integration (optional)
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
- **Log status badges** on each decode: New DXCC / New Band / New Mode / New Slot / Worked — enriched asynchronously from Log4OM
- Country lookup from callsign (in-memory, instant)
- **LoTW indicator** on each decode
- Watchlist highlight: watched callsigns are highlighted directly in the decode list
- Filters: CQ only, My Call only
- **Autocall**: click a decode to send a Reply command back to WSJT-X/MSHV — triggers auto-sequence
- **Halt TX**: cancel auto-sequence from the dashboard
- **TX Slot Advisor**: analyses the last decoded period (1000–3000 Hz passband) and suggests the best TX frequency — finds the largest clear gap to minimise QRM for the DX station's decoder
- Pause / Resume / Clear
- Deduplication across multiple network interfaces (multicast on Ethernet + Wi-Fi)

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

### POTA / SOTA Enrichment
- Automatic detection of POTA references (`AA-0036`) and SOTA references (`F/AB-123`) in spot comments
- Fetches **park name** from pota.app API (SQLite cache, 30-day TTL)
- Fetches **summit name** from sota.org.uk API (SQLite cache)
- Dedicated **Activations tab**: live ADXO DXpedition feed (ng3k.com, refreshed hourly) with bands, modes, QSL info

### Notifications
- **Gotify** push notifications: New DXCC, New Band, New Mode, New Band+Mode, Watchlist hit — each individually configurable
- **Windows native toast notifications** for watchlist hits (callsign, country, frequency, mode, spotter)
- Click a toast to tune FlexRadio directly to the spot frequency

### Contest Mode
- Toggle contest mode on/off
- Configure contest prefix and a list of special contest callsigns
- Contest stations auto-added to watchlist
- Watchlist filtered to show contest callsigns during a contest

### Web Dashboard
- Embedded Svelte SPA served at `http://localhost:8080`
- **Real-time WebSocket** connection for live updates (spots, FTx decodes, band changes, stats)
- Dark theme, responsive layout
- **Stats bar**: total spots received / processed / success rate / New DXCCs / connected clients / total QSOs / active filters
- **Solar data**: SFI, Sunspot number, A-index, K-index
- Tabs in main area: **Recent Spots**, **FTx Decodes**
- Sidebar tabs: **Watchlist**, **Activations**, **Log4OM**, **Console**, **App Logs**
- Console tab: real-time cluster message feed + send commands to the master cluster
- App Logs tab: live application log viewer (INFO / WARN / ERROR filter)

### Telnet Server
- Built-in Telnet server (default `0.0.0.0:7301`) — connect Log4OM or any DX software to receive spots
- Re-broadcasts all incoming cluster spots to connected Telnet clients in standard DX format
- Routes commands from Telnet clients to the master cluster

### Other
- **LoTW user list**: downloaded at startup, used to mark spots and decodes from LoTW-active stations
- **ClubLog API**: expedition and OQRS detection for watchlist entries
- **Hot-reload config**: edit `config.yml` while running — log level and cluster filters apply immediately without restart
- Configurable log level: DEBUG / INFO / WARN
- Optional log to file

---

## Requirements

| Requirement | Notes |
|---|---|
| **Log4OM 2** | **Mandatory.** Provides the contact database (SQLite or MySQL). |
| FlexRadio + SmartSDR | Optional. Required for panadapter spot display and tuning features. |
| WSJT-X / JTDX / MSHV | Optional. Required for FTx decode monitoring. |
| Windows | Primary platform (Windows toast notifications, SO_REUSEADDR multicast). |

---

## Configuration

Copy `config.default.yml` to `config.yml` and edit to match your setup:

```yaml
general:
  callsign: "F4BPO"          # Your callsign (must match Log4OM)
  flexradiospot: true        # Enable FlexRadio panadapter spots
  log_level: INFO

database:
  sqlite: true               # Use Log4OM SQLite database
  # mysql: true              # Or MySQL — only one can be true

sqlite:
  sqlite_path: 'C:\Users\you\AppData\Roaming\Log4OM2\Data\log4om.db'

clusters:
  - name: "EU Cluster"
    server: "dxfun.com"
    port: 7300
    login: "F4BPO"
    skimmer: true
    ft8: false
    enabled: true
    master: true

flex:
  discovery: false           # true = auto-discover on LAN
  ip: "10.0.0.1"            # FlexRadio IP (local or remote)
  spot_life: 600             # Spot lifetime on panadapter (seconds)

ftx:
  enabled: true
  multicast: true
  multicast_ip: "239.255.0.1"
  port: 2237

telnetserver:
  host: 0.0.0.0
  port: 7301                 # Connect Log4OM here to receive spots
```

### Log4OM setup

1. In Log4OM → **Network** → **DX Cluster** → set server to `localhost:7301`
2. In Log4OM → **Database** → note the SQLite path and set it in `sqlite_path`
3. Optionally enable **UDP Callsign** in Log4OM to receive frequency/mode from FlexDXCluster

### WSJT-X / MSHV / JTDX setup

Enable UDP reporting in your FT8 software:
- **UDP Server address:** `239.255.0.1` (multicast) or `127.0.0.1` (unicast)
- **Port:** 2237
- Enable *Accept UDP requests* (WSJT-X) or equivalent

FlexDXCluster shares the port with other apps already listening on 2237.

---

## Building from source

**Prerequisites:** Go 1.21+, Node.js 18+

```bash
# Build frontend
cd frontend
npm install
npm run build
cd ..

# Build binary (embeds the frontend automatically)
go build -ldflags="-s -w" -o FlexDXCluster.exe .
```

## Running

```bash
FlexDXCluster.exe --config config.yml
```

Open `http://localhost:8080` in a browser (or any browser on the network).

---

## Architecture

```
DX Cluster servers ──TCP──► clusterconnection.go
WSJT-X / MSHV ─────UDP──► ftx.go
FlexRadio SmartSDR ─TCP──► flexradio.go
Log4OM DB ─────────────► database.go

                    ┌──────────────────────┐
                    │  Go backend          │
                    │  spotprocessor.go    │  dedup, TTL, enrich
                    │  httpserver.go       │  HTTP + WebSocket
                    │  watchlist.go        │  watchlist engine
                    │  pota.go             │  POTA/SOTA cache
                    │  dxcc.go             │  DXCC lookup
                    │  lotw.go             │  LoTW users
                    └──────────┬───────────┘
                               │ WebSocket
                    ┌──────────▼───────────┐
                    │  Svelte frontend     │
                    │  (embedded, port 8080│
                    └──────────────────────┘
                               │ Telnet
                    Log4OM ────┘  (port 7301)
```

---

## License

Personal / amateur radio use.

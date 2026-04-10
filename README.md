# FlexDXClusterGui

A DX cluster client with a web-based GUI, designed for FlexRadio users and amateur radio operators. Written in Go (backend) and Svelte + TailwindCSS (frontend).

## Features

### DX Cluster
- Connect to one or multiple DX cluster servers simultaneously (DX Spider, CC Cluster, AR Cluster — auto-detected)
- Real-time spot display with band, mode, DXCC country, and log status (New DXCC / New Band / New Mode / New Slot / Worked)
- Spot filtering by band, mode (CW, SSB, FT8, FT4, FT2, RTTY), type (New DXCC, New Band, New Mode, Watchlist, POTA, SOTA…)
- Spot deduplication and configurable TTL
- Web Worker-based filtering for smooth UI on large spot lists

### Log Integration (Log4OM via SQLite or MySQL)
- Queries your log to determine worked/new status for each spot and FTx decode
- Supports SQLite and MySQL databases
- Per-DXCC contact cache to minimise DB load during high-traffic FTx periods

### FlexRadio Integration
- SmartSDR discovery or direct IP connection
- Automatic band changes reflected in spot filters
- Spot lifecycle tied to FlexRadio slice frequency

### FTx Monitor (WSJT-X / JTDX / MSHV)
- Listens on UDP multicast (default 224.0.0.1:2237) — shares the port with other apps (SO_REUSEADDR)
- Decodes FT8, FT4, and FT2 messages
- Displays decodes grouped by 15-second period with a clear period separator
- Country lookup from callsign (in-memory, instant)
- Log status badges: DXCC / Band / Mode / Slot / Wkd — enriched asynchronously after display
- Filters: CQ only, My Call
- Pause / Resume / Clear
- ON/OFF toggle (persisted to config)
- **TX Slot Advisor**: analyses the last decoded period (1000–3000 Hz passband) to suggest the best TX frequency — largest clear gap = least QRM for the DX station's decoder

### Watchlist
- Track specific callsigns with custom notes
- Alert when a watched station appears on the cluster

### Notifications
- Gotify push notifications for New DXCC, New Band, New Mode, Watchlist hits
- Windows native notifications

### Other
- Built-in Telnet server (re-broadcast spots to logging software)
- Solar/propagation data display
- POTA / SOTA spot enrichment
- LoTW user database (downloaded at startup, used for spot enrichment)
- ClubLog API integration
- Auto-updater
- Contest mode with prefix/callsign filtering

## Architecture

```
┌─────────────────────────────────┐
│  Go backend (single binary)     │
│                                 │
│  main.go          — startup, wiring        │
│  httpserver.go    — HTTP API + WebSocket   │
│  spot.go          — spot processing        │
│  spotprocessor.go — dedup, TTL, broadcast  │
│  ftx.go           — UDP multicast listener │
│  flexradio.go     — SmartSDR TCP client    │
│  database.go      — Log4OM SQLite/MySQL    │
│  dxcc.go          — cty.dat DXCC lookup    │
│  lotw.go          — LoTW user list         │
│  watchlist.go     — watchlist management   │
│  config.go        — YAML config + watcher  │
└────────────────┬────────────────┘
                 │ HTTP + WebSocket
┌────────────────▼────────────────┐
│  Svelte frontend (embedded)     │
│                                 │
│  App.svelte       — state, WS   │
│  Sidebar.svelte   — tab routing │
│  FilterBar.svelte — spot filters│
│  FTxTab.svelte    — FTx monitor │
│  WatchlistTab.svelte            │
│  spot-worker.js   — Web Worker  │
└─────────────────────────────────┘
```

## Configuration

Copy `config.yml.example` to `config.yml` and edit:

```yaml
general:
  callsign: "F4BPO"

database:
  sqlite: true

sqlite:
  sqlite_path: "C:/Users/you/AppData/Roaming/Log4OM2/Data/log4om.db"

clusters:
  - name: "EU Cluster"
    server: "dxfun.com"
    port: "7300"
    login: "F4BPO"
    enabled: true
    master: true

ftx:
  enabled: true
  multicast_ip: "224.0.0.1"
  port: 2237
```

## Building

**Prerequisites:** Go 1.21+, Node.js 18+

```bash
# Build frontend
cd frontend
npm install
npm run build
cd ..

# Build backend (embeds the frontend)
go build -o FlexDXClusterGui.exe .
```

## Running

```bash
FlexDXClusterGui.exe --config config.yml
```

Then open `http://localhost:8080` in a browser.

## FTx Setup

In MSHV / WSJT-X / JTDX, enable UDP reporting:
- **Host:** 224.0.0.1 (multicast) or 127.0.0.1 (unicast)
- **Port:** 2237
- Enable "Accept UDP requests" if available

FlexDXClusterGui will share the port with other apps already listening on it.

## License

Personal / amateur radio use.

package main

// FTx — WSJT-X/JTDX/MSHV UDP multicast listener
//
// Protocol: WSJT-X UDP Schema v2 (QDataStream, big-endian)
//   Magic   : 0xADBCCBDA  (uint32)
//   Schema  : 2           (uint32)
//   Type    : uint32  — 0=Heartbeat, 1=Status, 2=Decode, 3=Clear, 5=QSOLogged ...
//   ID      : UTF-8 pascal string (uint32 len + bytes, 0xFFFFFFFF = null)
//
// Decode message (type 2):
//   bool   New         — true on new decode
//   uint32 Time        — ms since midnight UTC
//   int32  SNR
//   float64 DeltaTime
//   uint32 DeltaFrequency (Hz offset from dial)
//   utf8   Mode        — "FT8","FT4","FT2"...
//   utf8   Message     — decoded text e.g. "CQ F4BPO JN03"
//   bool   LowConfidence
//   bool   OffAir
//
// Status message (type 1) — gives dial frequency so we can compute band:
//   uint64 DialFrequency (Hz)
//   utf8   Mode
//   utf8   DXCall
//   utf8   Report
//   utf8   TXMode
//   bool   TXEnabled
//   bool   Transmitting
//   bool   Decoding
//   uint32 RXdf
//   uint32 TXdf
//   utf8   DECall
//   utf8   DEGrid
//   utf8   DXGrid
//   bool   TXWatchdog
//   utf8   SubMode
//   bool   FastMode
//   uint8  SpecialOpMode
//   utf8   FrequencyTolerance
//   utf8   TRPeriod
//   utf8   ConfigurationName
//   utf8   TXMessage (schema ≥ 3)
//
// Reply message (type 4) sent back to trigger a call:
//   magic+schema+type+id  (header)
//   uint32 Time
//   int32  SNR
//   float64 DeltaTime
//   uint32 DeltaFrequency
//   utf8   Mode
//   utf8   Message
//   bool   LowConfidence
//   uint8  Modifiers

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	wsjtxMagic  uint32 = 0xADBCCBDA
	wsjtxSchema uint32 = 2

	msgStatus    uint32 = 1
	msgDecode    uint32 = 2
	msgClear     uint32 = 3
	msgReply     uint32 = 4
	msgQSOLogged uint32 = 5
	msgHaltTX    uint32 = 8
	msgHighlight uint32 = 13
	msgConfigure uint32 = 15
)

// FTxDecode is one decoded line broadcast to the frontend via WebSocket.
type FTxDecode struct {
	Time          string  `json:"time"` // "HHmmss"
	SNR           int32   `json:"snr"`
	DeltaTime     float64 `json:"dt"`
	DeltaFreq     uint32  `json:"df"`        // Hz offset from dial
	DialFreq      uint64  `json:"dialFreq"`  // Hz
	Frequency     uint64  `json:"frequency"` // dial + deltaFreq
	Mode          string  `json:"mode"`
	Message       string  `json:"message"`
	DXCall        string  `json:"dxCall"` // parsed from message
	MyCall        bool    `json:"myCall"` // message contains my callsign
	IsCQ          bool    `json:"isCQ"`
	CountryName   string  `json:"countryName"`
	DXCC          string  `json:"dxcc"`
	Band          string  `json:"band"`
	NewDXCC       bool    `json:"newDXCC"`
	NewBand       bool    `json:"newBand"`
	NewMode       bool    `json:"newMode"`
	NewSlot       bool    `json:"newSlot"`
	Worked        bool    `json:"worked"`
	WorkedToday   bool    `json:"workedToday"`
	Unconfirmed   bool    `json:"unconfirmed"`
	LowConfidence bool    `json:"lowConfidence"`
	SourceAddr    string  `json:"-"` // UDP source — used to send Reply
}

// logCache holds all log contacts in memory so enrichment never hits the DB during a FT8 period.
type logCache struct {
	mu      sync.RWMutex
	byDXCC  map[string][]Contact // uppercase dxcc → contacts
	loaded  bool
}

func (lc *logCache) load(repo LogbookProvider) {
	if repo == nil {
		return
	}
	contacts := repo.ListAll()
	m := make(map[string][]Contact, 512)
	for _, c := range contacts {
		key := strings.ToUpper(c.DXCC)
		m[key] = append(m[key], c)
	}
	lc.mu.Lock()
	lc.byDXCC = m
	lc.loaded = true
	lc.mu.Unlock()
	Log.Infof("FTx logCache: loaded %d contacts into memory", len(contacts))
}

func (lc *logCache) add(c Contact) {
	key := strings.ToUpper(c.DXCC)
	lc.mu.Lock()
	lc.byDXCC[key] = append(lc.byDXCC[key], c)
	lc.mu.Unlock()
}

func (lc *logCache) byDXCCContacts(dxcc string) []Contact {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.byDXCC[strings.ToUpper(dxcc)]
}

// FTxService manages the UDP listener and status state.
type FTxService struct {
	contactRepo LogbookProvider
	broadcast   chan WSMessage
	lc          *logCache

	mu           sync.RWMutex
	dialFreq     uint64       // last known dial frequency from Status message
	statusMode   string       // mode from Status message (e.g. "FT8") — decode messages may send "~"
	transmitting bool         // MSHV is currently transmitting
	txMessage    string       // current TX message text
	sourceAddr   *net.UDPAddr // last seen sender (MSHV/WSJT-X/JTDX)
	clientID     string       // client ID from last received Status message
	myCall       string

	// Period batching: group decodes by timeMs, flush after last UDP packet.
	batchMu      sync.Mutex
	batchTime    uint32
	batchDecodes []FTxDecode
	batchTimer   *time.Timer

	// Deduplication: on Windows, joining a multicast group on every interface
	// causes each UDP packet to be delivered once per interface. We drop dupes
	// within the same period (same timeMs) using a "timeMs|message" key.
	deduMu   sync.Mutex
	deduTime uint32
	deduSeen map[string]struct{}
}

func NewFTxService(contactRepo LogbookProvider, broadcast chan WSMessage) *FTxService {
	lc := &logCache{byDXCC: make(map[string][]Contact)}
	go lc.load(contactRepo) // async so startup isn't blocked
	return &FTxService{
		contactRepo: contactRepo,
		broadcast:   broadcast,
		myCall:      strings.ToUpper(Cfg.General.Callsign),
		lc:          lc,
	}
}

// Start begins listening on the configured UDP/multicast address.
func (f *FTxService) Start() {
	if !Cfg.FTx.Enabled {
		return
	}

	port := Cfg.FTx.Port
	mcIP := net.ParseIP(Cfg.FTx.MulticastIP)

	// Bind to 0.0.0.0:port (NOT the multicast IP) with SO_REUSEADDR so that:
	//  - Windows actually delivers multicast packets to the socket
	//  - Multiple apps can share the port simultaneously
	lc := net.ListenConfig{Control: reuseAddrControl}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		Log.Errorf("FTx: cannot bind UDP port %d: %v", port, err)
		return
	}
	Log.Infof("FTx: bound to %s", pc.LocalAddr())

	conn := pc.(*net.UDPConn)

	// Join the multicast group if multicast is enabled.
	// Joining on all eligible interfaces allows multiple apps (GridTracker, Log4OM, …)
	// to share port 2237 simultaneously. Duplicate deliveries caused by multiple active
	// interfaces are absorbed by the deduplication in handleDecode.
	if Cfg.FTx.Multicast && mcIP != nil && mcIP.IsMulticast() {
		p := ipv4.NewPacketConn(conn)
		joined := 0
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
				continue
			}
			if err := p.JoinGroup(&iface, &net.UDPAddr{IP: mcIP}); err != nil {
				Log.Debugf("FTx: JoinGroup failed on %s: %v", iface.Name, err)
				continue
			}
			Log.Infof("FTx: joined multicast %s on %s", mcIP, iface.Name)
			joined++
		}
		if joined == 0 {
			Log.Warnf("FTx: could not join multicast group on any interface")
		}
	}

	go f.readLoop(conn)
}

func (f *FTxService) readLoop(conn *net.UDPConn) {
	defer conn.Close()
	buf := make([]byte, 65536)
	Log.Infof("FTx: readLoop started")
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			Log.Warnf("FTx: read error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		Log.Debugf("FTx: received %d bytes from %s", n, src)
		f.mu.Lock()
		f.sourceAddr = src
		f.mu.Unlock()
		f.handlePacket(buf[:n], src)
	}
}

func (f *FTxService) handlePacket(data []byte, src *net.UDPAddr) {
	r := bytes.NewReader(data)

	var magic uint32
	if err := binary.Read(r, binary.BigEndian, &magic); err != nil {
		Log.Debugf("FTx: cannot read magic from %s: %v", src, err)
		return
	}
	if magic != wsjtxMagic {
		Log.Debugf("FTx: bad magic %08X from %s (expected %08X)", magic, src, wsjtxMagic)
		return
	}

	var schema, msgType uint32
	if err := binary.Read(r, binary.BigEndian, &schema); err != nil {
		return
	}
	if err := binary.Read(r, binary.BigEndian, &msgType); err != nil {
		return
	}
	// Read client ID and store it so we can echo it back in Configure/Reply/HaltTX.
	clientID, _ := readQString(r)
	Log.Debugf("FTx: msg type=%d schema=%d id=%q from %s", msgType, schema, clientID, src)
	if clientID != "" {
		f.mu.Lock()
		f.clientID = clientID
		f.mu.Unlock()
	}

	switch msgType {
	case msgStatus:
		f.handleStatus(r)
	case msgDecode:
		f.handleDecode(r, src)
	case msgClear:
		select {
		case f.broadcast <- WSMessage{Type: "ftxClear", Data: nil}:
		default:
		}
	case msgQSOLogged:
		f.handleQSOLogged(r)
	}
}

func (f *FTxService) handleStatus(r *bytes.Reader) {
	// WSJT-X Status message fields (after magic/schema/type/id):
	//  DialFrequency uint64
	//  Mode          string
	//  DXCall        string
	//  Report        string
	//  TXMode        string
	//  TXEnabled     bool
	//  Transmitting  bool
	//  Decoding      bool
	//  RXDF          uint32
	//  TXDF          uint32
	//  DECall        string
	//  DEGrid        string
	//  DXGrid        string
	//  TXWatchdog    bool
	//  SubMode       string   ← "4" when FT4 sub-mode (WSJT-X)
	//  FastMode      bool
	//  SpecialOp     uint8
	//  FreqTolerance uint32
	//  TRPeriod      uint32   ← 7500 = FT4, 15000 = FT8, 3750 = FT2
	//  ConfigName    string
	//  TXMessage     string

	var dialFreq uint64
	if err := binary.Read(r, binary.BigEndian, &dialFreq); err != nil {
		return
	}
	mode, _ := readQString(r) // field: Mode

	// Read intermediate fields to reach SubMode and TRPeriod
	readQString(r) // DXCall
	readQString(r) // Report
	readQString(r) // TXMode
	var boolBuf [3]bool
	binary.Read(r, binary.BigEndian, &boolBuf[0]) // TXEnabled
	binary.Read(r, binary.BigEndian, &boolBuf[1]) // Transmitting
	binary.Read(r, binary.BigEndian, &boolBuf[2]) // Decoding
	var u32 uint32
	binary.Read(r, binary.BigEndian, &u32) // RXDF
	binary.Read(r, binary.BigEndian, &u32) // TXDF
	readQString(r)                         // DECall
	readQString(r)                         // DEGrid
	readQString(r)                         // DXGrid
	var watchdog bool
	binary.Read(r, binary.BigEndian, &watchdog) // TXWatchdog
	subMode, _ := readQString(r)                // SubMode ("4" = FT4 in WSJT-X)
	var fastMode bool
	binary.Read(r, binary.BigEndian, &fastMode) // FastMode
	var specialOp uint8
	binary.Read(r, binary.BigEndian, &specialOp) // SpecialOp
	binary.Read(r, binary.BigEndian, &u32)       // FreqTolerance
	var trPeriod uint32
	binary.Read(r, binary.BigEndian, &trPeriod) // T/R Period in ms

	// Determine actual mode: T/R period and sub-mode are more reliable than
	// the Mode string (MSHV may report "FT8" even when doing FT4).
	resolvedMode := strings.ToUpper(mode)
	switch {
	case trPeriod == 7500, strings.ToUpper(subMode) == "4", resolvedMode == "FT4":
		resolvedMode = "FT4"
	case trPeriod == 3750:
		resolvedMode = "FT2"
	case trPeriod == 15000 && resolvedMode == "":
		resolvedMode = "FT8"
	}

	// Read ConfigName then TXMessage (both may be absent in older schemas)
	configName, _ := readQString(r)
	_ = configName
	txMsg, _ := readQString(r)

	f.mu.Lock()
	f.dialFreq = dialFreq
	prevMode := f.statusMode
	if resolvedMode != "" {
		f.statusMode = resolvedMode
	}
	prevTX := f.transmitting
	prevMsg := f.txMessage
	f.transmitting = boolBuf[1]
	f.txMessage = txMsg
	changed := f.transmitting != prevTX || f.txMessage != prevMsg || f.statusMode != prevMode
	currentMode := f.statusMode
	f.mu.Unlock()

	f.mu.RLock()
	currentClientID := f.clientID
	f.mu.RUnlock()

	if changed {
		type txStatus struct {
			Transmitting bool   `json:"transmitting"`
			Message      string `json:"message"`
			Mode         string `json:"mode"`
			ClientID     string `json:"clientId"`
		}
		select {
		case f.broadcast <- WSMessage{Type: "ftxTXStatus", Data: txStatus{
			Transmitting: boolBuf[1],
			Message:      txMsg,
			Mode:         currentMode,
			ClientID:     currentClientID,
		}}:
		default:
		}
	}
}

// skipQDateTime skips a Qt5 QDataStream-encoded QDateTime field.
// Format: QDate(int64 Julian day) + QTime(uint32 msecs) + timespec(uint8) + optional extra.
func skipQDateTime(r *bytes.Reader) {
	var jd int64
	binary.Read(r, binary.BigEndian, &jd) // QDate julian day
	var ms uint32
	binary.Read(r, binary.BigEndian, &ms) // QTime msecs since midnight
	var ts uint8
	binary.Read(r, binary.BigEndian, &ts) // timespec
	switch ts {
	case 2: // OffsetFromUTC: int32 seconds
		var off int32
		binary.Read(r, binary.BigEndian, &off)
	case 3: // TimeZone: QByteArray (uint32 size + bytes)
		var sz uint32
		binary.Read(r, binary.BigEndian, &sz)
		if sz > 0 && sz < 512 {
			buf := make([]byte, sz)
			r.Read(buf)
		}
	}
}

// handleQSOLogged processes UDP message type 5 (QSO Logged).
// Extracts callsign, mode and frequency, updates the in-memory log cache immediately
// (so the next FT8 period sees the new QSO without a DB reload), then broadcasts.
func (f *FTxService) handleQSOLogged(r *bytes.Reader) {
	skipQDateTime(r)              // DateTimeOff
	dxCall, err := readQString(r) // DX Call
	if err != nil || dxCall == "" {
		return
	}
	dxCall = strings.ToUpper(strings.TrimSpace(dxCall))

	readQString(r)          // DX Grid
	var txFreq uint64
	binary.Read(r, binary.BigEndian, &txFreq) // TX Frequency (Hz)
	mode, _ := readQString(r)                 // Mode

	band := getBandFromHz(txFreq)
	dxccInfo := GetDXCC(dxCall)

	// Update in-memory cache immediately so the next period enrichment is correct.
	if f.lc != nil && dxccInfo.DXCC != "" {
		f.lc.add(Contact{
			Callsign: dxCall,
			Band:     band,
			Mode:     strings.ToUpper(mode),
			DXCC:     dxccInfo.DXCC,
			Country:  dxccInfo.CountryName,
			Date:     time.Now().UTC().Format("2006-01-02"),
		})
		Log.Debugf("FTx: logCache updated — added %s %s %s", dxCall, band, mode)
	}

	Log.Debugf("FTx: QSO logged with %s", dxCall)
	select {
	case f.broadcast <- WSMessage{Type: "ftxQSOLogged", Data: map[string]string{"dxCall": dxCall}}:
	default:
	}
}

func (f *FTxService) handleDecode(r *bytes.Reader, src *net.UDPAddr) {
	var isNew bool
	if err := binary.Read(r, binary.BigEndian, &isNew); err != nil {
		return
	}

	var timeMs uint32
	if err := binary.Read(r, binary.BigEndian, &timeMs); err != nil {
		return
	}

	var snr int32
	if err := binary.Read(r, binary.BigEndian, &snr); err != nil {
		return
	}

	var dtBits uint64
	if err := binary.Read(r, binary.BigEndian, &dtBits); err != nil {
		return
	}
	dt := math.Float64frombits(dtBits)

	var df uint32
	if err := binary.Read(r, binary.BigEndian, &df); err != nil {
		return
	}

	mode, err := readQString(r)
	if err != nil {
		return
	}

	message, err := readQString(r)
	if err != nil {
		return
	}

	// Deduplicate: same packet delivered on multiple interfaces has identical
	// timeMs and message. Drop it if already seen in this period.
	deduKey := message // timeMs is the period bucket; message is unique within it
	f.deduMu.Lock()
	if f.deduTime != timeMs {
		f.deduSeen = make(map[string]struct{})
		f.deduTime = timeMs
	}
	_, alreadySeen := f.deduSeen[deduKey]
	if !alreadySeen {
		f.deduSeen[deduKey] = struct{}{}
	}
	f.deduMu.Unlock()
	if alreadySeen {
		Log.Debugf("FTx: duplicate decode dropped (multi-iface multicast): %s", message)
		return
	}

	var lowConf bool
	binary.Read(r, binary.BigEndian, &lowConf)

	// OffAir = true means this is an echo of our own TX (WSJT-X standard).
	// Drop it — we don't want our own transmissions appearing as decoded spots.
	var offAir bool
	binary.Read(r, binary.BigEndian, &offAir)
	if offAir {
		return
	}

	// Format time from ms since midnight
	totalSec := timeMs / 1000
	hh := totalSec / 3600
	mm := (totalSec % 3600) / 60
	ss := totalSec % 60
	timeStr := formatTime2(hh, mm, ss)

	f.mu.RLock()
	dialFreq := f.dialFreq
	statusMode := f.statusMode
	f.mu.RUnlock()

	// Mode field in Decode messages varies by client:
	//   WSJT-X: "~" = FT8 (fall back to statusMode), "+" = FT4
	//   JTDX:   "~" = FT8 (fall back to statusMode), ":" = FT4
	//   ""      = fall back to statusMode
	switch mode {
	case "+", ":":
		mode = "FT4"
	case "~", "":
		if statusMode != "" {
			mode = statusMode
		}
	}

	frequency := dialFreq + uint64(df)
	band := getBandFromHz(frequency)

	// Parse callsign from FT8/FT4 message
	dxCall, isCQ := parseFTxMessage(message, f.myCall)
	callingMe := strings.Contains(strings.ToUpper(message), f.myCall)

	decode := FTxDecode{
		Time:          timeStr,
		SNR:           snr,
		DeltaTime:     dt,
		DeltaFreq:     df,
		DialFreq:      dialFreq,
		Frequency:     frequency,
		Mode:          mode,
		Message:       message,
		DXCall:        dxCall,
		MyCall:        callingMe,
		IsCQ:          isCQ,
		Band:          band,
		LowConfidence: lowConf,
		SourceAddr:    src.String(),
	}

	// DXCC lookup is in-memory — do it synchronously before batching.
	if dxCall != "" {
		dxccInfo := GetDXCC(dxCall)
		decode.CountryName = dxccInfo.CountryName
		decode.DXCC = dxccInfo.DXCC

		Log.Debugf("FTx: dxCall=%q country=%q", dxCall, decode.CountryName)
	} else {
		Log.Debugf("FTx: no dxCall parsed from message %q", message)
	}

	// Add to batch immediately with country already filled in.
	f.addToBatch(timeMs, decode)

	// Only the SQLite log-status queries run async. If the batch is still pending
	// when they finish, the result is patched in place; otherwise discarded.
	if f.contactRepo != nil && dxCall != "" && decode.DXCC != "" && band != "" {
		go func(d FTxDecode, tMs uint32) {
			d.NewDXCC, d.NewBand, d.NewMode, d.NewSlot, d.Worked, d.WorkedToday, d.Unconfirmed =
				f.checkLogStatus(dxCall, d.DXCC, band, mode)
			f.enrichInBatch(tMs, d)
		}(decode, timeMs)
	}
}

// addToBatch adds a raw decode immediately and debounces the flush timer on UDP arrival.
func (f *FTxService) addToBatch(timeMs uint32, decode FTxDecode) {
	f.batchMu.Lock()
	defer f.batchMu.Unlock()

	// New period — flush previous immediately.
	if f.batchTime != 0 && timeMs != f.batchTime {
		f.flushLocked()
	}

	f.batchTime = timeMs
	f.batchDecodes = append(f.batchDecodes, decode)

	// Debounce on UDP packet arrival (not DB completion).
	// 200ms after the last UDP packet = all raw decodes are in.
	if f.batchTimer != nil {
		f.batchTimer.Stop()
	}
	f.batchTimer = time.AfterFunc(200*time.Millisecond, func() {
		f.batchMu.Lock()
		defer f.batchMu.Unlock()
		f.flushLocked()
	})
}

// FTxEnrichUpdate is a minimal status update sent after the batch is already displayed.
type FTxEnrichUpdate struct {
	Message     string `json:"message"`
	DF          uint32 `json:"df"`
	Time        string `json:"time"`
	NewDXCC     bool   `json:"newDXCC"`
	NewBand     bool   `json:"newBand"`
	NewMode     bool   `json:"newMode"`
	NewSlot     bool   `json:"newSlot"`
	Worked      bool   `json:"worked"`
	WorkedToday bool   `json:"workedToday"`
	Unconfirmed bool   `json:"unconfirmed"`
}

// enrichInBatch patches a decode still in the pending batch, or sends a
// ftxEnrich WS message if the batch was already flushed.
func (f *FTxService) enrichInBatch(timeMs uint32, enriched FTxDecode) {
	f.batchMu.Lock()

	if f.batchTime == timeMs {
		// Batch still pending — patch in place.
		for i := range f.batchDecodes {
			if f.batchDecodes[i].Message == enriched.Message && f.batchDecodes[i].DeltaFreq == enriched.DeltaFreq {
				f.batchDecodes[i].NewDXCC = enriched.NewDXCC
				f.batchDecodes[i].NewBand = enriched.NewBand
				f.batchDecodes[i].NewMode = enriched.NewMode
				f.batchDecodes[i].NewSlot = enriched.NewSlot
				f.batchDecodes[i].Worked = enriched.Worked
				f.batchDecodes[i].WorkedToday = enriched.WorkedToday
				f.batchDecodes[i].Unconfirmed = enriched.Unconfirmed
				break
			}
		}
		f.batchMu.Unlock()
		return
	}
	f.batchMu.Unlock()

	// Batch already flushed — send a lightweight update so the frontend can
	// patch the status on the already-displayed row.
	update := FTxEnrichUpdate{
		Message:     enriched.Message,
		DF:          enriched.DeltaFreq,
		Time:        enriched.Time,
		NewDXCC:     enriched.NewDXCC,
		NewBand:     enriched.NewBand,
		NewMode:     enriched.NewMode,
		NewSlot:     enriched.NewSlot,
		Worked:      enriched.Worked,
		WorkedToday: enriched.WorkedToday,
		Unconfirmed: enriched.Unconfirmed,
	}
	select {
	case f.broadcast <- WSMessage{Type: "ftxEnrich", Data: update}:
	default:
	}
}

// flushLocked sends the current batch as a single WS message. Must be called with batchMu held.
func (f *FTxService) flushLocked() {
	if len(f.batchDecodes) == 0 {
		return
	}
	batch := make([]FTxDecode, len(f.batchDecodes))
	copy(batch, f.batchDecodes)
	f.batchDecodes = f.batchDecodes[:0]
	f.batchTime = 0
	f.batchTimer = nil

	select {
	case f.broadcast <- WSMessage{Type: "ftxBatch", Data: batch}:
	default:
		Log.Debugf("FTx: broadcast channel full, dropping batch")
	}

}

// checkLogStatus reads the in-memory log cache — zero DB calls, microsecond latency.
func (f *FTxService) checkLogStatus(callsign, dxcc, band, mode string) (newDXCC, newBand, newMode, newSlot, worked, workedToday, unconfirmed bool) {
	contacts := f.lc.byDXCCContacts(dxcc)

	modeUpper := strings.ToUpper(mode)
	bandUpper := strings.ToUpper(band)
	callUpper := strings.ToUpper(callsign)
	today := time.Now().UTC().Format("2006-01-02")

	useUnconf := Cfg.General.WorkUnconfirmed && len(Cfg.General.ConfirmationSources) > 0

	var hasCountry, hasBand, hasMode, hasBandMode, hasCall, hasCallToday bool
	var confCountry, confBand, confMode, confBandMode bool

	for _, c := range contacts {
		cMode := strings.ToUpper(c.Mode)
		cBand := strings.ToUpper(c.Band)
		cCall := strings.ToUpper(c.Callsign)
		modeMatch := cMode == modeUpper || (modeUpper == "SSB" && (cMode == "USB" || cMode == "LSB"))

		hasCountry = true
		if cBand == bandUpper {
			hasBand = true
		}
		if modeMatch {
			hasMode = true
		}
		if cBand == bandUpper && modeMatch {
			hasBandMode = true
		}
		if cCall == callUpper && cBand == bandUpper && modeMatch {
			hasCall = true
			if strings.HasPrefix(c.Date, today) {
				hasCallToday = true
			}
		}
		if useUnconf && c.LoTWConfirmed {
			confCountry = true
			if cBand == bandUpper {
				confBand = true
			}
			if modeMatch {
				confMode = true
			}
			if cBand == bandUpper && modeMatch {
				confBandMode = true
			}
		}
	}

	if useUnconf {
		newDXCC = !confCountry
		newBand = !confBand
		newMode = !confMode
		newSlot = !confBandMode && !newDXCC && !newBand && !newMode
		unconfirmed = (newDXCC && hasCountry) ||
			(newBand && hasBand) ||
			(newMode && hasMode) ||
			(newSlot && hasBandMode)
		// QSO already done — suppress "new unconfirmed" so it shows Wkd instead.
		if hasCall {
			newDXCC, newBand, newMode, newSlot, unconfirmed = false, false, false, false, false
		}
	} else {
		newDXCC = !hasCountry
		newBand = !hasBand
		newMode = !hasMode
		newSlot = !hasBandMode && !newDXCC && !newBand && !newMode
	}
	worked = hasCall
	workedToday = hasCallToday
	return
}

// replyAddr returns the UDP address to use for outgoing control messages (Reply, HaltTX, etc.).
//
// Multicast is only used for WSJT-X: on Windows both WSJT-X and DXHunter bind to the
// same port with SO_REUSEADDR, so a unicast packet goes to the last-bound socket only.
// Sending to the multicast group guarantees delivery to WSJT-X even in that scenario.
// MSHV and JTDX use true unicast and do not join any multicast group, so they must always
// receive replies at their actual source address.
func (f *FTxService) replyAddr() (*net.UDPAddr, error) {
	f.mu.RLock()
	src := f.sourceAddr
	id := f.clientID
	f.mu.RUnlock()

	isWSJTX := strings.Contains(strings.ToUpper(id), "WSJT")
	if isWSJTX && Cfg.FTx.Multicast && Cfg.FTx.MulticastIP != "" {
		ip := net.ParseIP(Cfg.FTx.MulticastIP)
		if ip != nil {
			return &net.UDPAddr{IP: ip, Port: Cfg.FTx.Port}, nil
		}
	}
	if src == nil {
		return nil, fmt.Errorf("no source address known yet")
	}
	return src, nil
}

// SendReply sends a WSJT-X "Reply" message (type 4) back to the source app.
func (f *FTxService) SendReply(decode FTxDecode, _ string) error {
	dst, err := f.replyAddr()
	if err != nil {
		return err
	}
	f.mu.RLock()
	id := f.clientID // always echo the ID captured from the app's own packets
	f.mu.RUnlock()

	conn, err := net.DialUDP("udp4", nil, dst)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := new(bytes.Buffer)
	writeUint32(buf, wsjtxMagic)
	writeUint32(buf, wsjtxSchema)
	writeUint32(buf, msgReply)
	writeQString(buf, id)

	// Reconstruct time ms from "HHmmss"
	writeUint32(buf, parseTimeToMs(decode.Time))
	writeInt32(buf, decode.SNR)
	binary.Write(buf, binary.BigEndian, math.Float64bits(decode.DeltaTime))
	writeUint32(buf, decode.DeltaFreq)
	writeQString(buf, decode.Mode)
	writeQString(buf, decode.Message)
	writeBool(buf, decode.LowConfidence)
	buf.WriteByte(0) // Modifiers

	_, err = conn.Write(buf.Bytes())
	return err
}

// HaltTX sends a WSJT-X "Halt TX" message (type 8).
// autoOnly: true = only halt if auto-sequence is active, false = halt immediately.
func (f *FTxService) HaltTX(_ string, autoOnly bool) error {
	dst, err := f.replyAddr()
	if err != nil {
		return err
	}
	f.mu.RLock()
	id := f.clientID
	f.mu.RUnlock()
	conn, err := net.DialUDP("udp4", nil, dst)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := new(bytes.Buffer)
	writeUint32(buf, wsjtxMagic)
	writeUint32(buf, wsjtxSchema)
	writeUint32(buf, msgHaltTX)
	writeQString(buf, id)
	writeBool(buf, autoOnly)

	_, err = conn.Write(buf.Bytes())
	return err
}

// HighlightCallsign sends a WSJT-X "Highlight Callsign" message (type 13).
// Passing empty colors clears the highlight. Set last = true to highlight
// the last compound callsign only.
func (f *FTxService) HighlightCallsign(_, callsign string, bgColor, fgColor [4]uint8, highlight bool) error {
	dst, err := f.replyAddr()
	if err != nil {
		return err
	}
	f.mu.RLock()
	id := f.clientID
	f.mu.RUnlock()
	conn, err := net.DialUDP("udp4", nil, dst)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := new(bytes.Buffer)
	writeUint32(buf, wsjtxMagic)
	writeUint32(buf, wsjtxSchema)
	writeUint32(buf, msgHighlight)
	writeQString(buf, id)
	writeQString(buf, callsign)
	writeQColor(buf, bgColor)
	writeQColor(buf, fgColor)
	writeBool(buf, highlight) // highlight last callsign only

	_, err = conn.Write(buf.Bytes())
	return err
}

// writeQColor writes a Qt QColor in QDataStream format:
//
//	uint8  colorSpec  (0=invalid, 1=RGB)
//	uint16 alpha
//	uint16 red
//	uint16 green
//	uint16 blue
//	uint16 pad
//
// 8-bit component → 16-bit: multiply by 257 (0xFF * 257 = 0xFFFF).
// Pass all-zero [4]uint8 to write an invalid color (clears highlight).
func writeQColor(buf *bytes.Buffer, c [4]uint8) {
	if c[0] == 0 && c[1] == 0 && c[2] == 0 && c[3] == 0 {
		// Invalid color — clears any existing highlight
		buf.WriteByte(0)
		binary.Write(buf, binary.BigEndian, uint16(0)) // alpha
		binary.Write(buf, binary.BigEndian, uint16(0)) // red
		binary.Write(buf, binary.BigEndian, uint16(0)) // green
		binary.Write(buf, binary.BigEndian, uint16(0)) // blue
		binary.Write(buf, binary.BigEndian, uint16(0)) // pad
		return
	}
	buf.WriteByte(1)                                      // ColorSpec = RGB
	binary.Write(buf, binary.BigEndian, uint16(c[3])*257) // alpha
	binary.Write(buf, binary.BigEndian, uint16(c[0])*257) // red
	binary.Write(buf, binary.BigEndian, uint16(c[1])*257) // green
	binary.Write(buf, binary.BigEndian, uint16(c[2])*257) // blue
	binary.Write(buf, binary.BigEndian, uint16(0))        // pad
}

// SendConfigure sends a WSJT-X "Configure" message (type 15).
// targetMode: desired mode ("FT4", "FT8") — empty means no change.
// clearDXCall: true = clear the DX call field (stops Log4OM broadcast).
// The client ID is taken from the stored f.clientID (learned from incoming Status packets).
func (f *FTxService) SendConfigure(targetMode string, clearDXCall bool) error {
	dst, err := f.replyAddr()
	if err != nil {
		return err
	}
	f.mu.RLock()
	clientID := f.clientID
	f.mu.RUnlock()
	if clientID == "" {
		clientID = "MSHV"
	}

	// Adapt mode/submode fields per client:
	//   WSJT-X  → Mode="FT4" works directly
	//   JTDX    → uses ":" internally for FT4; needs Mode="FT8" + SubMode=":"
	//   MSHV    → Mode="FT4" (may or may not be supported)
	isJTDX := strings.Contains(strings.ToUpper(clientID), "JTDX")

	modeField := targetMode // "" = null = no change
	subModeField := ""      // "" = null = no change
	var trPeriod uint32 = 0xFFFFFFFF

	if targetMode != "" {
		switch strings.ToUpper(targetMode) {
		case "FT4":
			trPeriod = 7500
			if isJTDX {
				modeField = "FT8"
				subModeField = ":"
			}
		case "FT8":
			trPeriod = 15000
			if isJTDX {
				subModeField = "" // clear submode
			}
		}
	}

	Log.Infof("FTx Configure → clientID=%q mode=%q subMode=%q trPeriod=%d clearDXCall=%v",
		clientID, modeField, subModeField, trPeriod, clearDXCall)

	conn, err := net.DialUDP("udp4", nil, dst)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := new(bytes.Buffer)
	writeUint32(buf, wsjtxMagic)
	writeUint32(buf, wsjtxSchema)
	writeUint32(buf, msgConfigure)
	writeQString(buf, clientID)

	writeQString(buf, modeField)    // "" → null = no change; "FT4"/"FT8" = switch
	writeUint32(buf, 0xFFFFFFFF)    // FrequencyTolerance: no change
	writeQString(buf, subModeField) // "" → null = no change; ":" = FT4 on JTDX
	writeBool(buf, false)           // FastMode: false for FT8 and FT4
	writeUint32(buf, trPeriod)      // T/R period in ms; 0xFFFFFFFF = no change
	writeUint32(buf, 0xFFFFFFFF)    // RxDF: no change

	if clearDXCall {
		writeQStringEmpty(buf) // 0x00000000 = explicitly clear DX call
	} else {
		writeQString(buf, "") // null = no change
	}

	writeQString(buf, "") // DXGrid: null = no change
	writeBool(buf, false) // GenerateMessages: keep current

	_, err = conn.Write(buf.Bytes())
	return err
}

// ============================================================================
// Helpers
// ============================================================================

// readQString reads a Qt QString (uint32 length + utf8 bytes; 0xFFFFFFFF = null).
func readQString(r *bytes.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	if length == 0xFFFFFFFF {
		return "", nil
	}
	b := make([]byte, length)
	if _, err := r.Read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func writeQString(buf *bytes.Buffer, s string) {
	if s == "" {
		binary.Write(buf, binary.BigEndian, uint32(0xFFFFFFFF))
		return
	}
	binary.Write(buf, binary.BigEndian, uint32(len(s)))
	buf.WriteString(s)
}

// writeQStringEmpty writes a Qt empty string (length=0, no bytes) — distinct from
// writeQString("") which writes 0xFFFFFFFF (Qt null = "no change" in Configure messages).
// Use this to explicitly clear a field such as DX Call.
func writeQStringEmpty(buf *bytes.Buffer) {
	binary.Write(buf, binary.BigEndian, uint32(0))
}

func writeUint32(buf *bytes.Buffer, v uint32) { binary.Write(buf, binary.BigEndian, v) }
func writeInt32(buf *bytes.Buffer, v int32)   { binary.Write(buf, binary.BigEndian, v) }
func writeBool(buf *bytes.Buffer, v bool) {
	if v {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
}

func formatTime2(h, m, s uint32) string {
	return fmt.Sprintf("%02d%02d%02d", h, m, s)
}

func parseTimeToMs(t string) uint32 {
	if len(t) < 6 {
		return 0
	}
	h := uint32((t[0]-'0')*10 + (t[1] - '0'))
	m := uint32((t[2]-'0')*10 + (t[3] - '0'))
	s := uint32((t[4]-'0')*10 + (t[5] - '0'))
	return (h*3600 + m*60 + s) * 1000
}

// getBandFromHz converts a frequency in Hz to a band string like "20M".
func getBandFromHz(hz uint64) string {
	mhz := float64(hz) / 1e6
	for _, b := range AmateurBands {
		if mhz >= b.MinFreqMHz && mhz <= b.MaxFreqMHz {
			return b.Name
		}
	}
	return ""
}

// callsignRe matches a ham callsign loosely.
var callsignRe = regexp.MustCompile(`\b([A-Z0-9]{1,3}[0-9][A-Z0-9]{1,4}(?:/[A-Z0-9]+)?)\b`)

// angledCallRe matches a hashed/angle-bracket callsign like <3X3A>.
var angledCallRe = regexp.MustCompile(`^<([A-Z0-9/]+)>$`)

// parseFTxMessage extracts the DX callsign and CQ flag from a decoded FT8/FT4 message.
// Typical formats:
//
//	CQ F4BPO JN03
//	CQ DX F4BPO JN03
//	F4BPO TK5EP KN06
//	TK5EP F4BPO -12
//	CQ NA F4BPO JN03
//
// MSHV compound / Type-4 formats:
//
//	F4BPO RR73; SP7IFM <3X3A> -06   — 3X3A is transmitter (angled = DX station)
//	SP7IFM <3X3A> RR73               — 3X3A is transmitter
func parseFTxMessage(msg, myCall string) (dxCall string, isCQ bool) {
	upper := strings.ToUpper(msg)

	// For compound messages ("F4BPO RR73; SP7IFM <3X3A> -06") focus on the
	// last segment — that contains the current transmitter.
	if idx := strings.LastIndex(upper, ";"); idx >= 0 {
		upper = strings.TrimSpace(upper[idx+1:])
	}

	parts := strings.Fields(upper)
	if len(parts) == 0 {
		return "", false
	}

	// Angle-bracket token <CALL> = hashed callsign in MSHV/WSJT-X Type-4 messages.
	// Position determines role:
	//   pos 0 → addressed station (DX being called), next token = transmitter
	//           e.g. "<3X3A> E7/K9AW"  → transmitter = E7/K9AW
	//   pos 1+ → the transmitter itself
	//           e.g. "SP7IFM <3X3A> -06" → transmitter = 3X3A
	for i, p := range parts {
		if m := angledCallRe.FindStringSubmatch(p); m != nil {
			if i == 0 {
				// <DX> CALLER REPORT — transmitter is the next token
				for _, next := range parts[1:] {
					if callsignRe.MatchString(next) {
						dxCall = next
						return
					}
				}
			} else {
				dxCall = m[1]
				return
			}
		}
	}

	if parts[0] == "CQ" {
		isCQ = true
		// CQ [modifier] CALL [grid]  — call is first non-modifier token
		for _, p := range parts[1:] {
			if callsignRe.MatchString(p) && len(p) > 2 {
				dxCall = p
				break
			}
		}
		return
	}

	// Standard format: CALL1 CALL2 REPORT
	// CALL2 is always the transmitting station (the one sending this decode).
	// CALL1 is the station being addressed.
	if len(parts) >= 2 {
		if parts[1] == myCall {
			// I am being called — interesting station is CALL1 (the caller)
			dxCall = parts[0]
		} else {
			// Normal case: CALL2 is the transmitter
			dxCall = parts[1]
		}
	}
	return
}

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
	"syscall"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/sys/windows"
)

// reuseAddrControl sets SO_REUSEADDR before the socket is bound so that
// multiple processes (MSHV, WSJT-X, GridTracker, us) can share port 2237.
func reuseAddrControl(network, address string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1) //nolint
	})
}

const (
	wsjtxMagic  uint32 = 0xADBCCBDA
	wsjtxSchema uint32 = 2

	msgStatus uint32 = 1
	msgDecode uint32 = 2
	msgClear  uint32 = 3
	msgReply  uint32 = 4
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
	LowConfidence bool    `json:"lowConfidence"`
	SourceAddr    string  `json:"-"` // UDP source — used to send Reply
}

// FTxService manages the UDP listener and status state.
type FTxService struct {
	contactRepo *Log4OMContactsRepository
	broadcast   chan WSMessage

	mu         sync.RWMutex
	dialFreq   uint64       // last known dial frequency from Status message
	sourceAddr *net.UDPAddr // last seen sender (MSHV/WSJT-X/JTDX)
	myCall     string

	// Period batching: group decodes by timeMs period, flush 1s after last decode
	batchMu      sync.Mutex
	batchTime    uint32       // current period timeMs
	batchDecodes []FTxDecode
	batchTimer   *time.Timer
}

func NewFTxService(contactRepo *Log4OMContactsRepository, broadcast chan WSMessage) *FTxService {
	return &FTxService{
		contactRepo: contactRepo,
		broadcast:   broadcast,
		myCall:      strings.ToUpper(Cfg.General.Callsign),
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

	// Join the multicast group on every eligible interface.
	if mcIP != nil && mcIP.IsMulticast() {
		p := ipv4.NewPacketConn(conn)
		ifaces, _ := net.Interfaces()
		joined := 0
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
		Log.Infof("FTx: received %d bytes from %s", n, src)
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
	// Skip client ID string
	clientID, _ := readQString(r)
	Log.Debugf("FTx: msg type=%d schema=%d id=%q from %s", msgType, schema, clientID, src)

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
	}
}

func (f *FTxService) handleStatus(r *bytes.Reader) {
	var dialFreq uint64
	if err := binary.Read(r, binary.BigEndian, &dialFreq); err != nil {
		return
	}
	f.mu.Lock()
	f.dialFreq = dialFreq
	f.mu.Unlock()
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

	var lowConf bool
	binary.Read(r, binary.BigEndian, &lowConf)

	// Format time from ms since midnight
	totalSec := timeMs / 1000
	hh := totalSec / 3600
	mm := (totalSec % 3600) / 60
	ss := totalSec % 60
	timeStr := formatTime2(hh, mm, ss)

	f.mu.RLock()
	dialFreq := f.dialFreq
	f.mu.RUnlock()

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

	// Run DXCC lookup + log status check in a goroutine so readLoop is not
	// blocked and all decodes of a period are processed concurrently.
	go func(d FTxDecode, tMs uint32) {
		if dxCall != "" {
			dxccInfo := GetDXCC(dxCall)
			d.CountryName = dxccInfo.CountryName
			d.DXCC = dxccInfo.DXCC
		}
		if f.contactRepo != nil && dxCall != "" && d.DXCC != "" && band != "" {
			d.NewDXCC, d.NewBand, d.NewMode, d.NewSlot, d.Worked =
				f.checkLogStatus(dxCall, d.DXCC, band, mode)
		}
		Log.Debugf("FTx: buffering decode %s %s %s", d.Time, d.Mode, d.Message)
		f.addToBatch(tMs, d)
	}(decode, timeMs)
}

// addToBatch accumulates decodes by period and flushes 1s after the last one.
func (f *FTxService) addToBatch(timeMs uint32, decode FTxDecode) {
	f.batchMu.Lock()
	defer f.batchMu.Unlock()

	// New period detected — flush the previous one immediately
	if f.batchTime != 0 && timeMs != f.batchTime {
		f.flushLocked()
	}

	f.batchTime = timeMs
	f.batchDecodes = append(f.batchDecodes, decode)

	// Start the timer only once per period (on the first decode).
	// Do NOT reset it — resetting would delay the flush by 1s after every decode.
	if f.batchTimer == nil {
		f.batchTimer = time.AfterFunc(1*time.Second, func() {
			f.batchMu.Lock()
			defer f.batchMu.Unlock()
			f.flushLocked()
		})
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

// checkLogStatus queries Log4OM to determine worked/new status.
// Mirrors the logic in spot.go ProcessTelnetSpot.
func (f *FTxService) checkLogStatus(callsign, dxcc, band, mode string) (newDXCC, newBand, newMode, newSlot, worked bool) {
	chCountry := make(chan []Contact)
	chMode := make(chan []Contact)
	chBand := make(chan []Contact)
	chBandMode := make(chan []Contact)
	chCall := make(chan []Contact)

	wg := new(sync.WaitGroup)
	wg.Add(5)

	go f.contactRepo.ListByCountry(dxcc, chCountry, wg)
	contacts := <-chCountry

	go f.contactRepo.ListByCountryMode(dxcc, mode, chMode, wg)
	contactsMode := <-chMode

	go f.contactRepo.ListByCountryBand(dxcc, band, chBand, wg)
	contactsBand := <-chBand

	go f.contactRepo.ListByCallSign(callsign, band, mode, chCall, wg)
	contactsCall := <-chCall

	go f.contactRepo.ListByCountryModeBand(dxcc, band, mode, chBandMode, wg)
	contactsBandMode := <-chBandMode

	wg.Wait()

	newDXCC = len(contacts) == 0
	newMode = len(contactsMode) == 0
	newBand = len(contactsBand) == 0
	newSlot = len(contactsBandMode) == 0 && !newDXCC && !newBand && !newMode
	worked = len(contactsCall) > 0
	return
}

// SendReply sends a WSJT-X "Reply" message (type 4) back to the source app.
func (f *FTxService) SendReply(decode FTxDecode, clientID string) error {
	f.mu.RLock()
	src := f.sourceAddr
	f.mu.RUnlock()

	if src == nil {
		return fmt.Errorf("no source address known yet")
	}

	conn, err := net.DialUDP("udp4", nil, src)
	if err != nil {
		return err
	}
	defer conn.Close()

	buf := new(bytes.Buffer)
	writeUint32(buf, wsjtxMagic)
	writeUint32(buf, wsjtxSchema)
	writeUint32(buf, msgReply)
	writeQString(buf, clientID)

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

// parseFTxMessage extracts the DX callsign and CQ flag from a decoded FT8/FT4 message.
// Typical formats:
//
//	CQ F4BPO JN03
//	CQ DX F4BPO JN03
//	F4BPO TK5EP KN06
//	TK5EP F4BPO -12
//	CQ NA F4BPO JN03
func parseFTxMessage(msg, myCall string) (dxCall string, isCQ bool) {
	parts := strings.Fields(strings.ToUpper(msg))
	if len(parts) == 0 {
		return "", false
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

	// Format: CALL1 CALL2 REPORT
	// CALL2 is always the transmitting station (the one sending this decode).
	// CALL1 is the station being addressed.
	// We want the transmitter for country/status lookup.
	if len(parts) >= 2 {
		if parts[1] == myCall {
			// I am transmitting — interesting station is CALL1 (the one I'm working)
			dxCall = parts[0]
		} else {
			// Normal case: CALL2 is the transmitter
			dxCall = parts[1]
		}
	}
	return
}

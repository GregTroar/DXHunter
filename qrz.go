package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// QRZ XML API — https://www.qrz.com/page/xml_data.html
// ============================================================================

// qrzSession holds the authenticated session key (valid ~24h).
type qrzSession struct {
	mu        sync.Mutex
	key       string
	expiresAt time.Time
}

var qrzSess = &qrzSession{}

// qrzSessionResp is the auth response envelope.
type qrzSessionResp struct {
	XMLName xml.Name `xml:"QRZDatabase"`
	Session struct {
		Key     string `xml:"Key"`
		Error   string `xml:"Error"`
		Message string `xml:"Message"`
	} `xml:"Session"`
}

// QRZCallsign holds all fields from a QRZ lookup response.
type QRZCallsign struct {
	Call    string `xml:"call"    json:"call"`
	FName   string `xml:"fname"   json:"fname"`
	Name    string `xml:"name"    json:"name"`
	Addr1   string `xml:"addr1"   json:"addr1"`
	Addr2   string `xml:"addr2"   json:"addr2"`
	State   string `xml:"state"   json:"state"`
	Zip     string `xml:"zip"     json:"zip"`
	Country string `xml:"country" json:"country"`
	Grid    string `xml:"grid"    json:"grid"`
	Lat     string `xml:"lat"     json:"lat"`
	Lon     string `xml:"lon"     json:"lon"`
	Class   string `xml:"class"   json:"class"`
	Image   string `xml:"image"   json:"image"`
	QSLMgr string  `xml:"qslmgr"  json:"qslmgr"`
	Email   string `xml:"email"   json:"email"`
	URL     string `xml:"url"     json:"url"`
	Land    string `xml:"land"    json:"land"`
	Born    string `xml:"born"    json:"born"`
	DXCC    string `xml:"dxcc"    json:"dxcc"`
	CCode   string `xml:"ccode"   json:"ccode"`
	BIO     string `xml:"bio"     json:"bio"`
	Aliases string `xml:"aliases" json:"aliases"`
	Error   string `xml:"-"       json:"error,omitempty"`
}

type qrzLookupResp struct {
	XMLName  xml.Name    `xml:"QRZDatabase"`
	Callsign QRZCallsign `xml:"Callsign"`
	Session  struct {
		Key   string `xml:"Key"`
		Error string `xml:"Error"`
	} `xml:"Session"`
}

// ── Lookup cache ──────────────────────────────────────────────────────────────
type qrzCache struct {
	mu    sync.RWMutex
	store map[string]*QRZCallsign
}

var qrzLookupCache = &qrzCache{store: make(map[string]*QRZCallsign)}

func (c *qrzCache) get(call string) (*QRZCallsign, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[call]
	return v, ok
}

func (c *qrzCache) set(call string, q *QRZCallsign) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[call] = q
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func qrzLogin() (string, error) {
	qrzSess.mu.Lock()
	defer qrzSess.mu.Unlock()

	if qrzSess.key != "" && time.Now().Before(qrzSess.expiresAt) {
		return qrzSess.key, nil
	}

	user := Cfg.QRZ.Username
	pass := Cfg.QRZ.Password
	if user == "" || pass == "" {
		return "", fmt.Errorf("QRZ credentials not configured")
	}

	url := fmt.Sprintf("https://xmldata.qrz.com/xml/current/?username=%s&password=%s&agent=FlexDXClusterGui",
		user, pass)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("QRZ auth request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed qrzSessionResp
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("QRZ auth parse: %w", err)
	}
	if parsed.Session.Error != "" {
		return "", fmt.Errorf("QRZ auth error: %s", parsed.Session.Error)
	}
	if parsed.Session.Key == "" {
		return "", fmt.Errorf("QRZ auth: empty session key")
	}

	qrzSess.key = parsed.Session.Key
	qrzSess.expiresAt = time.Now().Add(23 * time.Hour)
	Log.Infof("QRZ session established for %s", user)
	return qrzSess.key, nil
}

// ── Lookup ────────────────────────────────────────────────────────────────────

func QRZLookup(callsign string) (*QRZCallsign, error) {
	call := strings.ToUpper(strings.TrimSpace(callsign))
	if call == "" {
		return nil, fmt.Errorf("empty callsign")
	}

	// Return from cache if available
	if cached, ok := qrzLookupCache.get(call); ok {
		return cached, nil
	}

	key, err := qrzLogin()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://xmldata.qrz.com/xml/current/?s=%s&callsign=%s", key, call)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("QRZ lookup request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed qrzLookupResp
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("QRZ lookup parse: %w", err)
	}

	// Session expired — re-auth and retry once
	if parsed.Session.Error == "Session Timeout" || parsed.Session.Key == "" {
		qrzSess.mu.Lock()
		qrzSess.key = ""
		qrzSess.mu.Unlock()
		return QRZLookup(callsign)
	}

	if parsed.Session.Error != "" {
		result := &QRZCallsign{Call: call, Error: parsed.Session.Error}
		qrzLookupCache.set(call, result)
		return result, nil
	}

	result := &parsed.Callsign
	if result.Call == "" {
		result.Call = call
	}
	qrzLookupCache.set(call, result)
	return result, nil
}

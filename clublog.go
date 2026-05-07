package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ClubLogWatchData contient les données retournées par watch.php
type ClubLogWatchData struct {
	IsExpedition bool            `json:"is_expedition"`
	HasOQRS      bool            `json:"has_oqrs"`
	LiveStream   bool            `json:"livestream"`
	ClubLogUser  bool            `json:"clublog_user"`
	QRA          string          `json:"qra"`
	UpdatedAt    string          `json:"updated_at"`
	ClubLogInfo  *ClubLogInfo    `json:"clublog_info"`
	QSOsPerBand  json.RawMessage `json:"24h_qsos_per_band_mode"`
	SpotsPerBand json.RawMessage `json:"24h_spots_per_band"`
	QSOTotals7d  json.RawMessage `json:"7days_qso_totals"`
}

type ClubLogInfo struct {
	TotalQSOs       int    `json:"total_qsos"`
	FirstQSO        string `json:"first_qso"`
	LastQSO         string `json:"last_qso"`
	LogDurationDays int    `json:"log_duration_days"`
	LastUpload      string `json:"last_clublog_upload"`
	OQRSRequests24h int    `json:"24h_oqrs_requests_made"`
}

// ClubLogClient gère les appels à l'API ClubLog
type ClubLogClient struct {
	apiKey     string
	httpClient *http.Client
	cache      *ClubLogCache
}

func NewClubLogClient(apiKey string) *ClubLogClient {
	return &ClubLogClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetCache attache le cache SQLite au client
func (c *ClubLogClient) SetCache(cache *ClubLogCache) {
	c.cache = cache
}

// ClubLogCache — cache SQLite pour les résultats watch.php
type ClubLogCache struct {
	db *sql.DB
}

func NewClubLogCache(db *sql.DB) (*ClubLogCache, error) {
	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS clublog_cache (
			callsign        TEXT PRIMARY KEY,
			is_expedition   INTEGER DEFAULT 0,
			has_oqrs        INTEGER DEFAULT 0,
			livestream      INTEGER DEFAULT 0,
			qsos_24h        INTEGER DEFAULT 0,
			total_qsos      INTEGER DEFAULT 0,
			bands           TEXT DEFAULT '',
			fetched_at      INTEGER NOT NULL
		)`)
	if err != nil {
		return nil, fmt.Errorf("clublog cache create table: %w", err)
	}
	return &ClubLogCache{db: db}, nil
}

type ClubLogCacheEntry struct {
	Callsign     string
	IsExpedition bool
	HasOQRS      bool
	LiveStream   bool
	QSOs24h      int
	TotalQSOs    int
	FetchedAt    int64
}

// TTL : 1 heure pour les expéditions actives, 6 heures pour les autres
func (c *ClubLogCache) Get(callsign string) (*ClubLogCacheEntry, bool) {
	var e ClubLogCacheEntry
	var isExp, hasOQRS, ls int
	err := c.db.QueryRowContext(context.Background(),
		`SELECT callsign, is_expedition, has_oqrs, livestream, qsos_24h, total_qsos, fetched_at
		 FROM clublog_cache WHERE callsign = ?`,
		strings.ToUpper(callsign),
	).Scan(&e.Callsign, &isExp, &hasOQRS, &ls, &e.QSOs24h, &e.TotalQSOs, &e.FetchedAt)
	if err != nil {
		return nil, false
	}
	e.IsExpedition = isExp == 1
	e.HasOQRS = hasOQRS == 1
	e.LiveStream = ls == 1
	// TTL: 1h si expedition active, 6h sinon
	ttl := int64(6 * 3600)
	if e.IsExpedition {
		ttl = int64(3600)
	}
	if time.Now().Unix()-e.FetchedAt > ttl {
		return nil, false
	}
	return &e, true
}

func (c *ClubLogCache) Set(callsign string, data *ClubLogWatchData) {
	isExp := 0
	if data.IsExpedition {
		isExp = 1
	}
	hasOQRS := 0
	if data.HasOQRS {
		hasOQRS = 1
	}
	ls := 0
	if data.LiveStream {
		ls = 1
	}
	totalQSOs := 0
	if data.ClubLogInfo != nil {
		totalQSOs = data.ClubLogInfo.TotalQSOs
	}
	_, err := c.db.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO clublog_cache
		 (callsign, is_expedition, has_oqrs, livestream, qsos_24h, total_qsos, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.ToUpper(callsign), isExp, hasOQRS, ls,
		data.Get24hQSOCount(), totalQSOs, time.Now().Unix(),
	)
	if err != nil {
		Log.Warnf("clublog cache set %s: %v", callsign, err)
	}
}

// clubLogCache instance globale
var clubLogCache *ClubLogCache

// WatchCallsign retourne les données ClubLog pour un callsign.
// Cherche d'abord dans le cache SQLite (TTL 1h/6h), sinon appelle l'API.
func (c *ClubLogClient) WatchCallsign(callsign string) (*ClubLogWatchData, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("ClubLog API key not configured")
	}

	// Hit cache
	if c.cache != nil {
		if entry, ok := c.cache.Get(callsign); ok {
			Log.Infof("ClubLog [%s] cache hit — expedition=%v hasOQRS=%v", callsign, entry.IsExpedition, entry.HasOQRS)
			return &ClubLogWatchData{
				IsExpedition: entry.IsExpedition,
				HasOQRS:      entry.HasOQRS,
				LiveStream:   entry.LiveStream,
			}, nil
		}
	}
	Log.Infof("ClubLog [%s] fetching from API...", callsign)

	url := fmt.Sprintf("https://clublog.org/watch.php?call=%s&api=%s",
		strings.ToUpper(callsign), c.apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("clublog request %s: %w", callsign, err)
	}
	req.Header.Set("User-Agent", "FlexDXCluster/2.1 (amateur radio dx cluster)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clublog watch %s: %w", callsign, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Callsign inconnu de ClubLog — pas une erreur
		return &ClubLogWatchData{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clublog watch %s: HTTP %d", callsign, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("clublog read %s: %w", callsign, err)
	}

	var data ClubLogWatchData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("clublog decode %s: %w", callsign, err)
	}

	Log.Infof("ClubLog [%s] result — expedition=%v hasOQRS=%v livestream=%v totalQSOs=%d",
		callsign, data.IsExpedition, data.HasOQRS, data.LiveStream,
		func() int {
			if data.ClubLogInfo != nil {
				return data.ClubLogInfo.TotalQSOs
			}
			return 0
		}())

	// Stocker en cache
	if c.cache != nil {
		c.cache.Set(callsign, &data)
	}

	return &data, nil
}

// Get24hQSOCount retourne le total de QSOs des 24 dernières heures
// ClubLog retourne [] quand vide ou {} quand peuplé — RawMessage gère les deux cas
func (d *ClubLogWatchData) Get24hQSOCount() int {
	if len(d.QSOsPerBand) == 0 {
		return 0
	}
	var parsed map[string]map[string]int
	if err := json.Unmarshal(d.QSOsPerBand, &parsed); err != nil {
		return 0
	}
	total := 0
	for _, modes := range parsed {
		for _, count := range modes {
			total += count
		}
	}
	return total
}

// clubLogClient est l'instance globale (initialisée dans main.go si clé configurée)
var clubLogClient *ClubLogClient

// StartClubLogRefresher lance la goroutine de refresh ClubLog pour la watchlist
func StartClubLogRefresher(watchlist *Watchlist, broadcast chan WSMessage) {
	if clubLogClient == nil {
		Log.Info("ClubLog API key not configured — expedition detection disabled")
		return
	}

	go func() {
		// Premier fetch immédiat
		refreshClubLogWatchlist(watchlist, broadcast)

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			refreshClubLogWatchlist(watchlist, broadcast)
		}
	}()

	Log.Info("ClubLog expedition watcher started (refresh every 10 minutes)")
}

func refreshClubLogWatchlist(watchlist *Watchlist, broadcast chan WSMessage) {
	callsigns := watchlist.GetAllCallsigns()
	if len(callsigns) == 0 {
		return
	}

	Log.Infof("ClubLog: refreshing %d callsigns (cache TTL: 1h expedition / 6h other)...", len(callsigns))

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, callsign := range callsigns {
		callsign := callsign
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := clubLogClient.WatchCallsign(callsign)
			if err != nil {
				Log.Debugf("ClubLog watch failed for %s: %v", callsign, err)
				return
			}

			watchlist.mutex.Lock()
			entry, exists := watchlist.entries[callsign]
			if exists {
				wasExpedition := entry.IsExpedition
				entry.IsExpedition = data.IsExpedition
				entry.ClubLogQSOs24h = data.Get24hQSOCount()
				entry.ClubLogHasOQRS = data.HasOQRS
				entry.ClubLogLiveStream = data.LiveStream
				if data.ClubLogInfo != nil {
					entry.ClubLogTotalQSOs = data.ClubLogInfo.TotalQSOs
				}
				entry.ClubLogUpdatedAt = time.Now()
				if data.IsExpedition && !wasExpedition {
					Log.Infof("ClubLog: %s is now flagged as a DXpedition!", callsign)
				}
			}
			watchlist.mutex.Unlock()
		}()
	}

	wg.Wait()
	Log.Infof("ClubLog: refresh complete")

	if broadcast != nil {
		broadcast <- WSMessage{
			Type: "watchlist",
			Data: watchlist.GetAll(),
		}
	}
}

// ── CTY prefixes & exceptions (ClubLog) ───────────────────────────────────────

type clbXMLRoot struct {
	XMLName    xml.Name       `xml:"clublog"`
	Prefixes   []clbXMLRecord `xml:"prefixes>prefix"`
	Exceptions []clbXMLRecord `xml:"exceptions>exception"`
}

type clbXMLRecord struct {
	Call   string  `xml:"call"`
	Entity string  `xml:"entity"`
	ADIF   int     `xml:"adif"`
	CQZone int     `xml:"cqz"`
	Cont   string  `xml:"cont"`
	Lat    float64 `xml:"lat"`
	Lon    float64 `xml:"long"`
	Start  string  `xml:"start"`
	End    string  `xml:"end"`
}

type ClubLogCTYEntry struct {
	ADIF   int
	Name   string
	CQZone int
	Cont   string
	Lat    float64
	Lon    float64
	Start  time.Time
	End    time.Time
}

type clbPrefixEntry struct {
	Prefix string
	Entry  ClubLogCTYEntry
}

type ClubLogCTYDB struct {
	exceptions map[string]ClubLogCTYEntry
	prefixes   []clbPrefixEntry // sorted longest first
	mu         sync.RWMutex
}

var clCtyDB *ClubLogCTYDB

func LookupClubLogException(callsign string) *ClubLogCTYEntry {
	if clCtyDB == nil {
		return nil
	}
	clCtyDB.mu.RLock()
	defer clCtyDB.mu.RUnlock()
	if e, ok := clCtyDB.exceptions[callsign]; ok && clbDateValid(e, time.Now().UTC()) {
		cp := e
		return &cp
	}
	return nil
}

func LookupClubLogPrefix(callsign string) *ClubLogCTYEntry {
	if clCtyDB == nil {
		return nil
	}
	clCtyDB.mu.RLock()
	defer clCtyDB.mu.RUnlock()
	now := time.Now().UTC()
	for _, pe := range clCtyDB.prefixes {
		if strings.HasPrefix(callsign, pe.Prefix) && clbDateValid(pe.Entry, now) {
			cp := pe.Entry
			return &cp
		}
	}
	return nil
}

func clbDateValid(e ClubLogCTYEntry, now time.Time) bool {
	if !e.Start.IsZero() && now.Before(e.Start) {
		return false
	}
	if !e.End.IsZero() && now.After(e.End) {
		return false
	}
	return true
}

func parseClubLogCTY(data []byte) (*ClubLogCTYDB, error) {
	var root clbXMLRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("clublog cty xml parse: %w", err)
	}
	db := &ClubLogCTYDB{
		exceptions: make(map[string]ClubLogCTYEntry, len(root.Exceptions)),
	}
	for _, r := range root.Exceptions {
		call := strings.ToUpper(strings.TrimSpace(r.Call))
		if call != "" {
			db.exceptions[call] = clbRecordToEntry(r)
		}
	}
	db.prefixes = make([]clbPrefixEntry, 0, len(root.Prefixes))
	for _, r := range root.Prefixes {
		pfx := strings.ToUpper(strings.TrimSpace(r.Call))
		if pfx != "" {
			db.prefixes = append(db.prefixes, clbPrefixEntry{Prefix: pfx, Entry: clbRecordToEntry(r)})
		}
	}
	sort.Slice(db.prefixes, func(i, j int) bool {
		return len(db.prefixes[i].Prefix) > len(db.prefixes[j].Prefix)
	})
	return db, nil
}

func clbRecordToEntry(r clbXMLRecord) ClubLogCTYEntry {
	const layout = "2006-01-02T15:04:05"
	e := ClubLogCTYEntry{ADIF: r.ADIF, Name: r.Entity, CQZone: r.CQZone, Cont: r.Cont, Lat: r.Lat, Lon: r.Lon}
	if r.Start != "" {
		if t, err := time.Parse(layout, r.Start); err == nil {
			e.Start = t
		}
	}
	if r.End != "" {
		if t, err := time.Parse(layout, r.End); err == nil {
			e.End = t
		}
	}
	return e
}

const clbCTYPath      = "clublog_cty.xml"
const clbCTYURL       = "https://cdn.clublog.org/cty.php?api=%s"
const clbLastChangeURL = "https://clublog.org/cty_last_change.php"

func FetchClubLogCTY(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("no ClubLog API key configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(clbCTYURL, apiKey), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clublog cty: HTTP %d", resp.StatusCode)
	}
	body, err := readMaybeGzip(resp)
	if err != nil {
		return fmt.Errorf("clublog cty read: %w", err)
	}
	db, err := parseClubLogCTY(body)
	if err != nil {
		return err
	}
	path := resolveSiblingPath(clbCTYPath)
	if err := os.WriteFile(path, body, 0644); err != nil {
		Log.Warnf("ClubLog CTY: could not save to disk: %v", err)
	}
	clCtyDB = db
	Log.Infof("ClubLog CTY loaded: %d exceptions, %d prefixes", len(db.exceptions), len(db.prefixes))
	return nil
}

func LoadClubLogCTYFromDisk() error {
	path := resolveSiblingPath(clbCTYPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	db, err := parseClubLogCTY(data)
	if err != nil {
		return fmt.Errorf("clublog cty parse from disk: %w", err)
	}
	clCtyDB = db
	Log.Infof("ClubLog CTY loaded from disk: %d exceptions, %d prefixes", len(db.exceptions), len(db.prefixes))
	return nil
}

func ClubLogCTYLastChange() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clbLastChangeURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readMaybeGzip(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return raw, nil
	}
	defer gr.Close()
	return io.ReadAll(gr)
}

// ── Most Wanted cache ─────────────────────────────────────────────────────────

// mostWantedCache holds adif_string → rank, refreshed daily.
var (
	mwMu        sync.RWMutex
	mwRankMap   map[string]int // adif (string) → rank (1 = most wanted)
	mwFetchedAt time.Time
)

const mwTTL = 24 * time.Hour
const mwURL = "https://clublog.org/mostwanted.php?api=1"

// GetMostWanted returns the cached adif→rank map, fetching if stale.
func GetMostWanted() map[string]int {
	mwMu.RLock()
	if mwRankMap != nil && time.Since(mwFetchedAt) < mwTTL {
		cp := make(map[string]int, len(mwRankMap))
		for k, v := range mwRankMap {
			cp[k] = v
		}
		mwMu.RUnlock()
		return cp
	}
	mwMu.RUnlock()

	mwMu.Lock()
	defer mwMu.Unlock()
	// Double-check after acquiring write lock
	if mwRankMap != nil && time.Since(mwFetchedAt) < mwTTL {
		cp := make(map[string]int, len(mwRankMap))
		for k, v := range mwRankMap {
			cp[k] = v
		}
		return cp
	}

	fresh, err := fetchMostWanted()
	if err != nil {
		Log.Warnf("ClubLog MostWanted fetch failed: %v", err)
		if mwRankMap != nil {
			cp := make(map[string]int, len(mwRankMap))
			for k, v := range mwRankMap {
				cp[k] = v
			}
			return cp // return stale data on error
		}
		return nil
	}
	mwRankMap = fresh
	mwFetchedAt = time.Now()
	cp := make(map[string]int, len(fresh))
	for k, v := range fresh {
		cp[k] = v
	}
	return cp
}

// fetchMostWanted calls the ClubLog API and returns adif→rank.
func fetchMostWanted() (map[string]int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mwURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// API returns { "1": "344", "2": "123", ... }  rank → adif
	var raw map[string]string
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	out := make(map[string]int, len(raw))
	for rankStr, adif := range raw {
		var rank int
		fmt.Sscanf(rankStr, "%d", &rank)
		if rank > 0 && adif != "" {
			out[adif] = rank
		}
	}
	return out, nil
}

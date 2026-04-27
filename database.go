package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	log "github.com/sirupsen/logrus"
)

type Contact struct {
	Callsign        string
	Band            string
	Mode            string
	DXCC            string
	StationCallsign string
	Country         string
}

type QSO struct {
	Callsign string `json:"callsign"`
	Band     string `json:"band"`
	Mode     string `json:"mode"`
	Date     string `json:"date"`
	RSTSent  string `json:"rstSent"`
	RSTRcvd  string `json:"rstRcvd"`
	Country  string `json:"country"`
	DXCC     string `json:"dxcc"`
}

type QSOStats struct {
	Today     int `json:"today"`
	ThisWeek  int `json:"thisWeek"`
	ThisMonth int `json:"thisMonth"`
	Total     int `json:"total"`
}

// LogbookProvider is the interface that any logbook backend must implement.
// Add a new file (e.g. hrd.go, wavelog.go) with a struct satisfying this interface.
type LogbookProvider interface {
	CountEntries() int
	ListAll() []Contact
	ListByCountrySync(countryID string) []Contact
	ListByCountry(countryID string, contactsChan chan []Contact, wg *sync.WaitGroup)
	ListByCountryMode(countryID string, mode string, contactsModeChan chan []Contact, wg *sync.WaitGroup)
	ListByCountryModeBand(countryID string, band string, mode string, contactsModeBandChan chan []Contact, wg *sync.WaitGroup)
	ListByCountryBand(countryID string, band string, contactsBandChan chan []Contact, wg *sync.WaitGroup)
	ListByCallSign(callSign string, band string, mode string, contactsCallChan chan []Contact, wg *sync.WaitGroup)
	GetRecentQSOs(limit string) []QSO
	GetQSOStats() QSOStats
	GetDXCCCount() int
	GetWorkedCallsignsBandMode(callsigns []string, band string, mode string) map[string]bool
	HasWorkedCallsignToday(callsign, band, mode string) bool
	GetWorkedCallsignsBandModeToday(callsigns []string, band string, mode string) map[string]bool
	GetCallsignBandModes(callsign string) CallsignBandModeInfo
	HasWorkedCallsignBandMode(callsign, band, mode string) bool
	Close()
}

type Log4OMContactsRepository struct {
	db  *sql.DB
	Log *log.Logger
}

type FlexDXClusterRepository struct {
	db  *sql.DB
	Log *log.Logger
}

func NewLog4OMContactsRepository(filePath string) LogbookProvider {

	if Cfg.Database.MySQL {
		db, err := sql.Open("mysql", Cfg.Database.MySQLUser+":"+Cfg.Database.MySQLPassword+"@tcp("+Cfg.Database.MySQLHost+":"+Cfg.Database.MySQLPort+")/"+Cfg.Database.MySQLDbName)
		if err != nil {
			Log.Errorf("Cannot open MySQL database: %v", err)
		}

		// Configure connection pool
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		return &Log4OMContactsRepository{
			db:  db,
			Log: Log}

	} else if Cfg.Database.SQLite {
		db, err := sql.Open("sqlite", filePath)
		if err != nil {
			Log.Errorf("Cannot open SQLite database: %v", err)
			return nil
		}

		// SQLite works best with a single connection for reads alongside Log4OM
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		// Log4OM uses WAL mode while running — we must match it to read live data.
		// If Log4OM holds an exclusive lock at this instant the pragma may fail;
		// that's harmless (we fall back to whatever mode is active).
		if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			Log.Warnf("Could not set WAL mode on Log4OM database (non-fatal): %v", err)
		}

		Log.Infof("Log4OM SQLite opened: %s", filePath)
		return &Log4OMContactsRepository{
			db:  db,
			Log: Log}
	}

	return nil
}

func NewFlexDXDatabase(filePath string) *FlexDXClusterRepository {

	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		Log.Errorf("Cannot open db: %v", err)
	}

	Log.Debugln("Opening SQLite database")

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS "spots" (
	"id"	INTEGER NOT NULL UNIQUE,
	"commandNumber"	INTEGER NOT NULL UNIQUE,
	"flexSpotNumber" INTEGER,
	"dx"	TEXT NOT NULL,
	"freqMhz"	TEXT,
	"freqHz"	TEXT,
	"band"		TEXT,
	"mode"	TEXT,
	"spotter"	TEXT,
	"flexMode"	TEXT,
	"source"	TEXT,
	"time"		TEXT,
	"timestamp"	INTEGER,
	"lifeTime"	TEXT,
	"priority"	TEXT,
	"originalComment"	TEXT,
	"comment"	TEXT,
	"color"	TEXT,
	"backgroundColor"	TEXT,
	"countryName"	TEXT,
	"dxcc"	TEXT,
	"newDXCC"	INTEGER DEFAULT 0,
	"newBand"	INTEGER DEFAULT 0,
	"newMode"	INTEGER DEFAULT 0,
	"newSlot"	INTEGER DEFAULT 0,
	"worked"	INTEGER DEFAULT 0,
	"clusterName"	TEXT DEFAULT '',
	"potaRef"		TEXT DEFAULT '',
	"sotaRef"		TEXT DEFAULT '',
	"parkName"		TEXT DEFAULT '',
	"summitName"		TEXT DEFAULT '',
	PRIMARY KEY("id" AUTOINCREMENT)
)`,
	)

	if err != nil {
		log.Warn("Cannot create table", err)
	}

	return &FlexDXClusterRepository{
		db:  db,
		Log: Log,
	}
}

func (r *Log4OMContactsRepository) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

func (r *Log4OMContactsRepository) CountEntries() int {
	var contacts int
	err := r.db.QueryRow("SELECT COUNT(*) FROM Log").Scan(&contacts)
	if err != nil {
		log.Error("could not query database", err)
	}

	return contacts
}

// ListAll fetches every contact from the log in one query — used to populate the in-memory cache.
func (r *Log4OMContactsRepository) ListAll() []Contact {
	rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log")
	if err != nil {
		r.Log.Errorf("ListAll: query error: %v", err)
		return nil
	}
	defer rows.Close()
	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			continue
		}
		contacts = append(contacts, c)
	}
	return contacts
}

// ListByCountrySync fetches all contacts for a DXCC in a single blocking call.
func (r *Log4OMContactsRepository) ListByCountrySync(countryID string) []Contact {
	rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ?", countryID)
	if err != nil {
		r.Log.Error("could not query database", err)
		return nil
	}
	defer rows.Close()
	var contacts []Contact
	for rows.Next() {
		c := Contact{}
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			r.Log.Error("could not scan row", err)
			continue
		}
		contacts = append(contacts, c)
	}
	return contacts
}

func (r *Log4OMContactsRepository) ListByCountry(countryID string, contactsChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ?", countryID)
	if err != nil {
		log.Error("could not query database", err)
	}

	defer rows.Close()

	contacts := []Contact{}
	for rows.Next() {
		c := Contact{}
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			log.Error("could not query database", err)
		}
		contacts = append(contacts, c)
	}
	contactsChan <- contacts
}

func buildSSBModeCondition(mode string) (condition string, params []interface{}) {
	modeUpper := strings.ToUpper(mode)

	if modeUpper == "USB" || modeUpper == "LSB" || modeUpper == "SSB" {
		// Pour SSB/USB/LSB, chercher les 3
		return "(mode = ? OR mode = ? OR mode = ?)",
			[]interface{}{"USB", "LSB", "SSB"}
	}

	// Pour les autres modes, recherche exacte
	return "mode = ?", []interface{}{modeUpper}
}

func (r *Log4OMContactsRepository) ListByCountryMode(countryID string, mode string, contactsModeChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()

	// ✅ Utiliser le helper pour construire la condition
	modeCondition, modeParams := buildSSBModeCondition(mode)

	// Construire la requête avec la condition
	query := fmt.Sprintf(
		"SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND %s",
		modeCondition,
	)

	// Construire les paramètres (countryID + mode params)
	args := []interface{}{countryID}
	args = append(args, modeParams...)

	// Exécuter la requête
	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Error("could not query database", err)
		contactsModeChan <- []Contact{}
		return
	}
	defer rows.Close()

	// Scanner les résultats
	contacts := []Contact{}
	for rows.Next() {
		c := Contact{}
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			log.Error("could not scan row", err)
			continue
		}
		contacts = append(contacts, c)
	}

	contactsModeChan <- contacts
}

func (r *Log4OMContactsRepository) ListByCountryModeBand(countryID string, band string, mode string, contactsModeBandChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()

	// ✅ Utiliser le helper pour construire la condition
	modeCondition, modeParams := buildSSBModeCondition(mode)

	// Construire la requête avec la condition
	query := fmt.Sprintf(
		"SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND band = ? AND %s",
		modeCondition,
	)

	// Construire les paramètres (countryID + band + mode params)
	args := []interface{}{countryID, band}
	args = append(args, modeParams...)

	// Exécuter la requête
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Error("could not query database", err)
		contactsModeBandChan <- []Contact{}
		return
	}
	defer rows.Close()

	// Scanner les résultats
	contacts := []Contact{}
	for rows.Next() {
		c := Contact{}
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			r.Log.Error("could not scan row", err)
			continue
		}
		contacts = append(contacts, c)
	}

	contactsModeBandChan <- contacts
}

func (r *Log4OMContactsRepository) ListByCountryBand(countryID string, band string, contactsBandChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND band = ?", countryID, band)
	if err != nil {
		r.Log.Error(err)
	}

	defer rows.Close()

	contacts := []Contact{}
	for rows.Next() {
		c := Contact{}
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			r.Log.Error(err)

		}
		contacts = append(contacts, c)
	}
	contactsBandChan <- contacts
}

func (r *Log4OMContactsRepository) ListByCallSign(callSign string, band string, mode string, contactsCallChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()

	modeCondition, modeParams := buildSSBModeCondition(mode)
	query := fmt.Sprintf(
		"SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE callsign = ? AND band = ? AND %s",
		modeCondition,
	)
	args := []interface{}{callSign, band}
	args = append(args, modeParams...)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Error(err)
	}

	defer rows.Close()

	contacts := []Contact{}
	for rows.Next() {
		c := Contact{}
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			r.Log.Error(err)

		}
		contacts = append(contacts, c)
	}
	contactsCallChan <- contacts
}

func (r *Log4OMContactsRepository) GetRecentQSOs(limit string) []QSO {
	query := fmt.Sprintf("SELECT callsign, band, mode, qsodate, rstsent, rstrcvd, country, dxcc FROM log ORDER BY qsodate DESC, qsodate DESC LIMIT %s", limit)

	rows, err := r.db.Query(query)
	if err != nil {
		log.Error("could not query recent QSOs:", err)
		return []QSO{}
	}
	defer rows.Close()

	qsos := []QSO{}
	for rows.Next() {
		q := QSO{}
		if err := rows.Scan(&q.Callsign, &q.Band, &q.Mode, &q.Date, &q.RSTSent, &q.RSTRcvd, &q.Country, &q.DXCC); err != nil {
			log.Error("could not scan QSO:", err)
			continue
		}
		qsos = append(qsos, q)
	}

	return qsos
}

func (r *Log4OMContactsRepository) GetQSOStats() QSOStats {
	stats := QSOStats{}

	// QSOs du jour
	err := r.db.QueryRow("SELECT COUNT(*) FROM log WHERE qsodate >= DATE('now')").Scan(&stats.Today)
	if err != nil {
		log.Error("could not get today's QSOs:", err)
	}

	// QSOs de la semaine
	err = r.db.QueryRow("SELECT COUNT(*) FROM log WHERE qsodate >= DATE('now', '-7 days')").Scan(&stats.ThisWeek)
	if err != nil {
		log.Error("could not get week's QSOs:", err)
	}

	// QSOs du mois
	err = r.db.QueryRow("SELECT COUNT(*) FROM log WHERE qsodate >= DATE('now', 'start of month')").Scan(&stats.ThisMonth)
	if err != nil {
		log.Error("could not get month's QSOs:", err)
	}

	// Total QSOs
	stats.Total = r.CountEntries()

	return stats
}

func (r *Log4OMContactsRepository) GetDXCCCount() int {
	var count int
	err := r.db.QueryRow("SELECT COUNT(DISTINCT dxcc) FROM log WHERE dxcc != '' AND dxcc IS NOT NULL AND dxcc != 0").Scan(&count)
	if err != nil {
		log.Error("could not get DXCC count:", err)
		return 0
	}
	return count
}

func (r *Log4OMContactsRepository) GetWorkedCallsignsBandMode(callsigns []string, band string, mode string) map[string]bool {
	if len(callsigns) == 0 {
		return make(map[string]bool)
	}

	result := make(map[string]bool)

	// Construire les placeholders pour la requête IN
	placeholders := make([]string, len(callsigns))
	args := make([]interface{}, 0, len(callsigns)+3) // +3 pour band et mode params

	for i, callsign := range callsigns {
		placeholders[i] = "?"
		args = append(args, callsign)
	}

	// Ajouter la bande
	args = append(args, band)

	// ✅ Utiliser le helper pour la condition de mode
	modeCondition, modeParams := buildSSBModeCondition(mode)
	args = append(args, modeParams...)

	// Construire la requête
	query := fmt.Sprintf(
		"SELECT DISTINCT callsign FROM log WHERE callsign IN (%s) AND band = ? AND %s",
		strings.Join(placeholders, ","),
		modeCondition,
	)

	// Exécuter la requête
	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Error("could not check worked band/mode status:", err)
		return result
	}
	defer rows.Close()

	// Scanner les résultats
	for rows.Next() {
		var callsign string
		if err := rows.Scan(&callsign); err != nil {
			log.Error("error scanning callsign:", err)
			continue
		}
		result[callsign] = true
	}

	return result
}

func (r *Log4OMContactsRepository) HasWorkedCallsignToday(callsign, band, mode string) bool {
	var count int

	// ✅ Utiliser le helper pour la condition de mode
	modeCondition, modeParams := buildSSBModeCondition(mode)

	// Construire la requête
	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM log 
		WHERE callsign = ? 
		AND band = ? 
		AND %s
		AND qsodate >= DATE('now')`,
		modeCondition,
	)

	// Construire les paramètres
	args := []interface{}{callsign, band}
	args = append(args, modeParams...)

	// Exécuter la requête
	err := r.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		log.Error("could not check today's contact:", err)
		return false
	}

	return count > 0
}

func (r *Log4OMContactsRepository) GetWorkedCallsignsBandModeToday(callsigns []string, band string, mode string) map[string]bool {
	if len(callsigns) == 0 {
		return make(map[string]bool)
	}

	result := make(map[string]bool)

	// Construire les placeholders pour la requête IN
	placeholders := make([]string, len(callsigns))
	args := make([]interface{}, 0, len(callsigns)+3)

	for i, callsign := range callsigns {
		placeholders[i] = "?"
		args = append(args, callsign)
	}

	// Ajouter la bande
	args = append(args, band)

	// ✅ Utiliser le helper pour la condition de mode
	modeCondition, modeParams := buildSSBModeCondition(mode)
	args = append(args, modeParams...)

	// Construire la requête
	query := fmt.Sprintf(
		`SELECT DISTINCT callsign FROM log 
		WHERE callsign IN (%s) 
		AND band = ? 
		AND %s
		AND qsodate >= DATE('now')`,
		strings.Join(placeholders, ","),
		modeCondition,
	)

	// Exécuter la requête
	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Error("could not check today's worked band/mode status:", err)
		return result
	}
	defer rows.Close()

	// Scanner les résultats
	for rows.Next() {
		var callsign string
		if err := rows.Scan(&callsign); err != nil {
			log.Error("error scanning callsign:", err)
			continue
		}
		result[callsign] = true
	}

	return result
}

type CallsignQSOEntry struct {
	Band  string `json:"band"`
	Mode  string `json:"mode"`
	Count int    `json:"count"`
}

type CallsignBandModeInfo struct {
	Callsign  string             `json:"callsign"`
	Country   string             `json:"country"`
	DXCC      string             `json:"dxcc"`
	TotalQSOs int                `json:"totalQSOs"`
	FirstQSO  string             `json:"firstQSO"`
	LastQSO   string             `json:"lastQSO"`
	BandModes []CallsignQSOEntry `json:"bandModes"`
}

func (r *Log4OMContactsRepository) GetCallsignBandModes(callsign string) CallsignBandModeInfo {
	info := CallsignBandModeInfo{Callsign: strings.ToUpper(callsign)}

	rows, err := r.db.Query(`
		SELECT band, mode, COUNT(*) as cnt,
		       MIN(qsodate) as first_qso, MAX(qsodate) as last_qso,
		       COALESCE(country, '') as country, COALESCE(dxcc, '') as dxcc
		FROM log
		WHERE callsign = ?
		GROUP BY band, mode
		ORDER BY band, mode
	`, strings.ToUpper(callsign))
	if err != nil {
		r.Log.Errorf("GetCallsignBandModes %s: %v", callsign, err)
		return info
	}
	defer rows.Close()

	for rows.Next() {
		var q CallsignQSOEntry
		var firstQSO, lastQSO, country, dxcc string
		if err := rows.Scan(&q.Band, &q.Mode, &q.Count, &firstQSO, &lastQSO, &country, &dxcc); err != nil {
			continue
		}
		info.BandModes = append(info.BandModes, q)
		info.TotalQSOs += q.Count
		if info.FirstQSO == "" || firstQSO < info.FirstQSO {
			info.FirstQSO = firstQSO
		}
		if lastQSO > info.LastQSO {
			info.LastQSO = lastQSO
		}
		if info.Country == "" {
			info.Country = country
			info.DXCC = dxcc
		}
	}

	return info
}

// Garder aussi l'ancienne méthode pour compatibilité (optionnel)
func (r *Log4OMContactsRepository) HasWorkedCallsignBandMode(callsign, band, mode string) bool {
	result := r.GetWorkedCallsignsBandMode([]string{callsign}, band, mode)
	return result[callsign]
}

//
// Flex from now on
//

func (r *FlexDXClusterRepository) GetSpotsByCallsign(callsign string, limit int) []FlexSpot {
	query := fmt.Sprintf(
		"SELECT * FROM spots WHERE dx = ? ORDER BY id DESC LIMIT %d", limit,
	)
	rows, err := r.db.Query(query, strings.ToUpper(callsign))
	if err != nil {
		r.Log.Errorf("GetSpotsByCallsign %s: %v", callsign, err)
		return nil
	}
	defer rows.Close()

	var spots []FlexSpot
	for rows.Next() {
		s := FlexSpot{}
		if err := rows.Scan(&s.ID, &s.CommandNumber, &s.FlexSpotNumber, &s.DX, &s.FrequencyMhz, &s.FrequencyHz, &s.Band, &s.Mode, &s.SpotterCallsign, &s.FlexMode, &s.Source, &s.UTCTime, &s.TimeStamp, &s.LifeTime, &s.Priority,
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked, &s.ClusterName, &s.POTARef, &s.SOTARef, &s.ParkName, &s.SummitName); err != nil {
			r.Log.Errorf("GetSpotsByCallsign scan: %v", err)
			continue
		}
		spots = append(spots, s)
	}
	return spots
}

func (r *FlexDXClusterRepository) GetAllSpots(limit string) []FlexSpot {

	Spots := []FlexSpot{}

	var query string

	if limit == "0" {
		query = "SELECT * from spots ORDER BY id DESC"
	} else {
		query = fmt.Sprintf("SELECT * from spots ORDER BY id DESC LIMIT %s", limit)
	}

	rows, err := r.db.Query(query)

	if err != nil {
		r.Log.Error(err)
		return nil
	}

	defer rows.Close()

	s := FlexSpot{}
	for rows.Next() {
		if err := rows.Scan(&s.ID, &s.CommandNumber, &s.FlexSpotNumber, &s.DX, &s.FrequencyMhz, &s.FrequencyHz, &s.Band, &s.Mode, &s.SpotterCallsign, &s.FlexMode, &s.Source, &s.UTCTime, &s.TimeStamp, &s.LifeTime, &s.Priority,
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked, &s.ClusterName, &s.POTARef, &s.SOTARef, &s.ParkName, &s.SummitName); err != nil {

			return nil // Arrête le traitement s'il y a une erreur sur une ligne
		}

		Spots = append(Spots, s)
	}

	return Spots
}

func (r *FlexDXClusterRepository) FindDXSameBand(spot FlexSpot) (*FlexSpot, error) {
	rows, err := r.db.Query("SELECT * from spots WHERE dx = ? AND band = ?", spot.DX, spot.Band)
	if err != nil {
		r.Log.Error(err)
		return nil, err
	}

	defer rows.Close()

	s := FlexSpot{}
	for rows.Next() {
		if err := rows.Scan(&s.ID, &s.CommandNumber, &s.FlexSpotNumber, &s.DX, &s.FrequencyMhz, &s.FrequencyHz, &s.Band, &s.Mode, &s.SpotterCallsign, &s.FlexMode, &s.Source, &s.UTCTime, &s.TimeStamp, &s.LifeTime, &s.Priority,
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked, &s.ClusterName, &s.POTARef, &s.SOTARef, &s.ParkName, &s.SummitName); err != nil {
			r.Log.Error(err)
			return nil, err
		}
	}
	return &s, nil
}

func (r *FlexDXClusterRepository) CreateSpot(spot FlexSpot) {
	query := "INSERT INTO `spots` (`commandNumber`, `flexSpotNumber`, `dx`, `freqMhz`, `freqHz`, `band`, `mode`, `spotter`, `flexMode`, `source`, `time`, `timestamp`, `lifeTime`, `priority`, `originalComment`, `comment`, `color`, `backgroundColor`, `countryName`, `dxcc`, `newDXCC`, `newBand`, `newMode`, `newSlot`, `worked`, `clusterName`, `potaRef`, `sotaRef`, `parkName`, `summitName`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	insertResult, err := r.db.ExecContext(context.Background(), query, spot.CommandNumber, spot.CommandNumber, spot.DX, spot.FrequencyMhz, spot.FrequencyHz, spot.Band, spot.Mode, spot.SpotterCallsign, spot.FlexMode, spot.Source, spot.UTCTime, time.Now().Unix(), spot.LifeTime, spot.Priority, spot.OriginalComment, spot.Comment, spot.Color, spot.BackgroundColor, spot.CountryName, spot.DXCC, spot.NewDXCC, spot.NewBand, spot.NewMode, spot.NewSlot, spot.Worked, spot.ClusterName, spot.POTARef, spot.SOTARef, spot.ParkName, spot.SummitName)
	if err != nil {
		Log.Errorf("cannot insert spot in database: %s", err)
	}

	_, err = insertResult.LastInsertId()
	if err != nil {
		Log.Errorf("impossible to retrieve last inserted id: %s", err)
	}

}

func (r *FlexDXClusterRepository) UpdateSpotSameBand(spot FlexSpot) error {
	_, err := r.db.Exec(`UPDATE spots SET commandNumber = ?, DX = ?, freqMhz = ?, freqHz = ?, band = ?, mode = ?, spotter = ?, flexMode = ?, source = ?, time = ?, timestamp = ?, lifeTime = ?, priority = ?, originalComment = ?, comment = ?, color = ?, backgroundColor = ?, countryName = ?, dxcc = ?, newDXCC = ?, newBand = ?, newMode = ?, newSlot = ?, worked = ? WHERE DX = ? AND band = ?`,
		spot.CommandNumber, spot.DX, spot.FrequencyMhz, spot.FrequencyHz, spot.Band, spot.Mode, spot.SpotterCallsign, spot.FlexMode, spot.Source, spot.UTCTime, spot.TimeStamp, spot.LifeTime, spot.Priority, spot.OriginalComment, spot.Comment, spot.Color, spot.BackgroundColor, spot.CountryName, spot.DXCC, spot.NewDXCC, spot.NewBand, spot.NewMode, spot.NewSlot, spot.Worked, spot.DX, spot.Band)
	if err != nil {
		r.Log.Errorf("could not update database: %s", err)
		return err
	}
	return nil
}

func (r *FlexDXClusterRepository) FindSpotByCommandNumber(commandNumber string) (*FlexSpot, error) {
	rows, err := r.db.Query("SELECT * from spots WHERE commandNumber = ?", commandNumber)
	if err != nil {
		r.Log.Error(err)
		return nil, err
	}

	defer rows.Close()

	s := FlexSpot{}
	for rows.Next() {
		if err := rows.Scan(&s.ID, &s.CommandNumber, &s.FlexSpotNumber, &s.DX, &s.FrequencyMhz, &s.FrequencyHz, &s.Band, &s.Mode, &s.SpotterCallsign, &s.FlexMode, &s.Source, &s.UTCTime, &s.TimeStamp, &s.LifeTime, &s.Priority,
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked, &s.ClusterName, &s.POTARef, &s.SOTARef, &s.ParkName, &s.SummitName); err != nil {
			r.Log.Error(err)
			return nil, err
		}
	}
	return &s, nil
}

func (r *FlexDXClusterRepository) FindSpotByFlexSpotNumber(spotNumber string) (*FlexSpot, error) {
	rows, err := r.db.Query("SELECT * from spots WHERE flexSpotNumber = ?", spotNumber)
	if err != nil {
		r.Log.Error(err)
		return nil, err
	}

	defer rows.Close()

	s := FlexSpot{}
	for rows.Next() {
		if err := rows.Scan(&s.ID, &s.CommandNumber, &s.FlexSpotNumber, &s.DX, &s.FrequencyMhz, &s.FrequencyHz, &s.Band, &s.Mode, &s.SpotterCallsign, &s.FlexMode, &s.Source, &s.UTCTime, &s.TimeStamp, &s.LifeTime, &s.Priority,
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked, &s.ClusterName, &s.POTARef, &s.SOTARef, &s.ParkName, &s.SummitName); err != nil {
			r.Log.Error(err)
			return nil, err
		}
	}
	return &s, nil
}

func (r *FlexDXClusterRepository) UpdateFlexSpotNumberByID(flexSpotNumber string, spot FlexSpot) (*FlexSpot, error) {
	flexSpotNumberInt, _ := strconv.Atoi(flexSpotNumber)
	rows, err := r.db.Query(`UPDATE spots SET flexSpotNumber = ? WHERE id = ? RETURNING *`, flexSpotNumberInt, spot.ID)
	if err != nil {
		r.Log.Errorf("could not update database: %s", err)
	}

	defer rows.Close()

	s := FlexSpot{}
	for rows.Next() {
		if err := rows.Scan(&s.ID, &s.CommandNumber, &s.FlexSpotNumber, &s.DX, &s.FrequencyMhz, &s.FrequencyHz, &s.Band, &s.Mode, &s.SpotterCallsign, &s.FlexMode, &s.Source, &s.UTCTime, &s.TimeStamp, &s.LifeTime, &s.Priority,
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked, &s.ClusterName, &s.POTARef, &s.SOTARef, &s.ParkName, &s.SummitName); err != nil {
			r.Log.Error(err)
			return nil, err
		}
	}

	return &s, nil
}

func (r *FlexDXClusterRepository) DeleteSpotByFlexSpotNumber(flexSpotNumber string) {
	flexSpotNumberInt, _ := strconv.Atoi(flexSpotNumber)
	query := "DELETE from spots WHERE flexSpotNumber = ?"
	_, err := r.db.Exec(query, flexSpotNumberInt)
	if err != nil {
		r.Log.Errorf("could not delete spot %v from database", flexSpotNumberInt)
	}
}

func DeleteDatabase(filePath string, log *log.Logger) {
	_, err := os.Stat(filePath)
	if !os.IsNotExist(err) {
		err := os.Remove(filePath)
		if err != nil {
			log.Error("could not delete existing database")
		}
		log.Debug("deleting existing database")
	}
}

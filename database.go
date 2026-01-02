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

type Log4OMContactsRepository struct {
	db  *sql.DB
	Log *log.Logger
}

type FlexDXClusterRepository struct {
	db  *sql.DB
	Log *log.Logger
}

func NewLog4OMContactsRepository(filePath string) *Log4OMContactsRepository {

	if Cfg.Database.MySQL {
		db, err := sql.Open("mysql", Cfg.Database.MySQLUser+":"+Cfg.Database.MySQLPassword+"@tcp("+Cfg.Database.MySQLHost+":"+Cfg.Database.MySQLPort+")/"+Cfg.Database.MySQLDbName)
		if err != nil {
			Log.Errorf("Cannot open db", err)
		}

		// Configure connection pool
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)

		return &Log4OMContactsRepository{
			db:  db,
			Log: Log}

	} else if Cfg.Database.SQLite {
		db, err := sql.Open("sqlite3", filePath)
		if err != nil {
			Log.Errorf("Cannot open db", err)
		}

		// Configure connection pool for SQLite
		db.SetMaxOpenConns(1) // SQLite works best with single connection for writes
		db.SetMaxIdleConns(1)

		_, err = db.Exec("PRAGMA journal_mode=WAL")
		if err != nil {
			panic(err)
		}

		return &Log4OMContactsRepository{
			db:  db,
			Log: Log}
	}

	return nil
}

func NewFlexDXDatabase(filePath string) *FlexDXClusterRepository {

	db, err := sql.Open("sqlite3", filePath)
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

func (r *Log4OMContactsRepository) CountEntries() int {
	var contacts int
	err := r.db.QueryRow("SELECT COUNT(*) FROM log").Scan(&contacts)
	if err != nil {
		log.Error("could not query database", err)
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

func (r *Log4OMContactsRepository) ListByCountryMode(countryID string, mode string, contactsModeChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()

	if mode == "USB" || mode == "LSB" || mode == "SSB" {

		rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND (mode = ? OR mode = ? OR mode = ?)", countryID, "USB", "LSB", "SSB")
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
		contactsModeChan <- contacts

	} else {

		rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND mode = ?", countryID, mode)
		if err != nil {
			log.Error("could not query the database", err)
		}

		defer rows.Close()

		contacts := []Contact{}
		for rows.Next() {
			c := Contact{}
			if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
				r.Log.Println(err)

			}
			contacts = append(contacts, c)
		}
		contactsModeChan <- contacts

	}
}

func (r *Log4OMContactsRepository) ListByCountryModeBand(countryID string, band string, mode string, contactsModeBandChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()

	if mode == "USB" || mode == "LSB" {

		rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND (mode = ? OR mode = ?) AND band = ?", countryID, "USB", "LSB", band)
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
		contactsModeBandChan <- contacts

	} else {

		rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE dxcc = ? AND mode = ? AND band = ?", countryID, mode, band)
		if err != nil {
			r.Log.Error("could not query the database", err)
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
		contactsModeBandChan <- contacts

	}
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
	rows, err := r.db.Query("SELECT callsign, band, mode, dxcc, stationcallsign, country FROM log WHERE callsign = ? AND band = ? AND mode = ?", callSign, band, mode)
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

// Nouvelle méthode optimisée - remplacer HasWorkedCallsignBandMode par celle-ci
func (r *Log4OMContactsRepository) GetWorkedCallsignsBandMode(callsigns []string, band string, mode string) map[string]bool {
	if len(callsigns) == 0 {
		return make(map[string]bool)
	}

	result := make(map[string]bool)

	// Construire les placeholders pour la requête IN
	placeholders := make([]string, len(callsigns))
	args := make([]interface{}, 0, len(callsigns)+2)

	for i, callsign := range callsigns {
		placeholders[i] = "?"
		args = append(args, callsign)
	}

	args = append(args, band)

	var query string

	// Gérer les cas SSB/USB/LSB
	if mode == "USB" || mode == "LSB" || mode == "SSB" {
		query = fmt.Sprintf(
			"SELECT DISTINCT callsign FROM log WHERE callsign IN (%s) AND band = ? AND (mode = 'USB' OR mode = 'LSB' OR mode = 'SSB')",
			strings.Join(placeholders, ","),
		)
	} else {
		query = fmt.Sprintf(
			"SELECT DISTINCT callsign FROM log WHERE callsign IN (%s) AND band = ? AND mode = ?",
			strings.Join(placeholders, ","),
		)
		args = append(args, mode)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Error("could not check worked band/mode status:", err)
		return result
	}
	defer rows.Close()

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

	// Gérer les modes SSB/USB/LSB
	if mode == "USB" || mode == "LSB" || mode == "SSB" {
		err := r.db.QueryRow(
			`SELECT COUNT(*) FROM log 
			WHERE callsign = ? 
			AND band = ? 
			AND (mode = 'USB' OR mode = 'LSB' OR mode = 'SSB')
			AND qsodate >= DATE('now')`,
			callsign, band,
		).Scan(&count)

		if err != nil {
			log.Error("could not check today's contact:", err)
			return false
		}
	} else {
		err := r.db.QueryRow(
			`SELECT COUNT(*) FROM log 
			WHERE callsign = ? 
			AND band = ? 
			AND mode = ?
			AND qsodate >= DATE('now')`,
			callsign, band, mode,
		).Scan(&count)

		if err != nil {
			log.Error("could not check today's contact:", err)
			return false
		}
	}

	return count > 0
}

// GetWorkedCallsignsBandModeToday - version optimisée pour plusieurs callsigns (aujourd'hui uniquement)
func (r *Log4OMContactsRepository) GetWorkedCallsignsBandModeToday(callsigns []string, band string, mode string) map[string]bool {
	if len(callsigns) == 0 {
		return make(map[string]bool)
	}

	result := make(map[string]bool)

	placeholders := make([]string, len(callsigns))
	args := make([]interface{}, 0, len(callsigns)+2)

	for i, callsign := range callsigns {
		placeholders[i] = "?"
		args = append(args, callsign)
	}

	args = append(args, band)

	var query string

	if mode == "USB" || mode == "LSB" || mode == "SSB" {
		query = fmt.Sprintf(
			`SELECT DISTINCT callsign FROM log 
			WHERE callsign IN (%s) 
			AND band = ? 
			AND (mode = 'USB' OR mode = 'LSB' OR mode = 'SSB')
			AND qsodate >= DATE('now')`,
			strings.Join(placeholders, ","),
		)
	} else {
		query = fmt.Sprintf(
			`SELECT DISTINCT callsign FROM log 
			WHERE callsign IN (%s) 
			AND band = ? 
			AND mode = ?
			AND qsodate >= DATE('now')`,
			strings.Join(placeholders, ","),
		)
		args = append(args, mode)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Error("could not check today's worked band/mode status:", err)
		return result
	}
	defer rows.Close()

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

// Garder aussi l'ancienne méthode pour compatibilité (optionnel)
func (r *Log4OMContactsRepository) HasWorkedCallsignBandMode(callsign, band, mode string) bool {
	result := r.GetWorkedCallsignsBandMode([]string{callsign}, band, mode)
	return result[callsign]
}

//
// Flex from now on
//

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
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked); err != nil {

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
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked); err != nil {
			r.Log.Error(err)
			return nil, err
		}
	}
	return &s, nil
}

func (r *FlexDXClusterRepository) CreateSpot(spot FlexSpot) {
	query := "INSERT INTO `spots` (`commandNumber`, `flexSpotNumber`, `dx`, `freqMhz`, `freqHz`, `band`, `mode`, `spotter`, `flexMode`, `source`, `time`, `timestamp`, `lifeTime`, `priority`, `originalComment`, `comment`, `color`, `backgroundColor`, `countryName`, `dxcc`, `newDXCC`, `newBand`, `newMode`, `newSlot`, `worked`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	insertResult, err := r.db.ExecContext(context.Background(), query, spot.CommandNumber, spot.CommandNumber, spot.DX, spot.FrequencyMhz, spot.FrequencyHz, spot.Band, spot.Mode, spot.SpotterCallsign, spot.FlexMode, spot.Source, spot.UTCTime, time.Now().Unix(), spot.LifeTime, spot.Priority, spot.OriginalComment, spot.Comment, spot.Color, spot.BackgroundColor, spot.CountryName, spot.DXCC, spot.NewDXCC, spot.NewBand, spot.NewMode, spot.NewSlot, spot.Worked)
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
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked); err != nil {
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
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked); err != nil {
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
			&s.OriginalComment, &s.Comment, &s.Color, &s.BackgroundColor, &s.CountryName, &s.DXCC, &s.NewDXCC, &s.NewBand, &s.NewMode, &s.NewSlot, &s.Worked); err != nil {
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

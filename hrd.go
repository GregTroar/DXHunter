package main

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const hrdTable = "TABLE_HRD_CONTACTS_V07"

// hrdDateExpr converts HRD's YYYYMMDD + HHMMSS columns to "YYYY-MM-DD HH:MM:SS".
const hrdDateExpr = "substr(COL_QSO_DATE,1,4)||'-'||substr(COL_QSO_DATE,5,2)||'-'||substr(COL_QSO_DATE,7,2)" +
	"||' '||COALESCE(substr(COL_TIME_ON,1,2)||':'||substr(COL_TIME_ON,3,2)||':'||substr(COL_TIME_ON,5,2),'00:00:00')"

// COL_DXCC is INTEGER in HRD — CAST to TEXT so Go can scan it into a string.
const hrdContactCols = "COL_CALL, COL_BAND, COL_MODE, COALESCE(CAST(COL_DXCC AS TEXT),''), COALESCE(COL_STATION_CALLSIGN,''), COALESCE(COL_COUNTRY,'')"

type HRDContactsRepository struct {
	db  *sql.DB
	Log *log.Logger
}

func NewHRDContactsRepository(filePath string) LogbookProvider {
	db, err := sql.Open("sqlite", filePath)
	if err != nil {
		Log.Errorf("Cannot open HRD SQLite database: %v", err)
		return nil
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err = db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		Log.Warnf("Could not set WAL mode on HRD database (non-fatal): %v", err)
	}
	Log.Infof("HRD SQLite opened: %s", filePath)
	return &HRDContactsRepository{db: db, Log: Log}
}

func (r *HRDContactsRepository) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

// hrdSSBModeCondition mirrors buildSSBModeCondition but uses COL_MODE column name.
func hrdSSBModeCondition(mode string) (condition string, params []interface{}) {
	modeUpper := strings.ToUpper(mode)
	if modeUpper == "USB" || modeUpper == "LSB" || modeUpper == "SSB" {
		return "(COL_MODE = ? OR COL_MODE = ? OR COL_MODE = ?)", []interface{}{"USB", "LSB", "SSB"}
	}
	return "COL_MODE = ?", []interface{}{modeUpper}
}

func (r *HRDContactsRepository) hrdScan(rows *sql.Rows) []Contact {
	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.Callsign, &c.Band, &c.Mode, &c.DXCC, &c.StationCallsign, &c.Country); err != nil {
			continue
		}
		c.Band = strings.ToUpper(strings.TrimSpace(c.Band))
		c.Mode = strings.ToUpper(strings.TrimSpace(c.Mode))
		contacts = append(contacts, c)
	}
	return contacts
}

func (r *HRDContactsRepository) CountEntries() int {
	var count int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM " + hrdTable).Scan(&count); err != nil {
		r.Log.Errorf("HRD CountEntries: %v", err)
	}
	return count
}

func (r *HRDContactsRepository) ListAll() []Contact {
	rows, err := r.db.Query("SELECT " + hrdContactCols + " FROM " + hrdTable)
	if err != nil {
		r.Log.Errorf("HRD ListAll: %v", err)
		return nil
	}
	defer rows.Close()
	return r.hrdScan(rows)
}

func (r *HRDContactsRepository) ListByCountrySync(countryID string) []Contact {
	rows, err := r.db.Query(
		"SELECT "+hrdContactCols+" FROM "+hrdTable+" WHERE COL_DXCC = ?", countryID)
	if err != nil {
		r.Log.Errorf("HRD ListByCountrySync: %v", err)
		return nil
	}
	defer rows.Close()
	return r.hrdScan(rows)
}

func (r *HRDContactsRepository) ListByCountry(countryID string, contactsChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	rows, err := r.db.Query(
		"SELECT "+hrdContactCols+" FROM "+hrdTable+" WHERE COL_DXCC = ?", countryID)
	if err != nil {
		r.Log.Errorf("HRD ListByCountry: %v", err)
		contactsChan <- nil
		return
	}
	defer rows.Close()
	contactsChan <- r.hrdScan(rows)
}

func (r *HRDContactsRepository) ListByCountryMode(countryID string, mode string, contactsModeChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	modeCondition, modeParams := hrdSSBModeCondition(mode)
	query := fmt.Sprintf("SELECT "+hrdContactCols+" FROM "+hrdTable+" WHERE COL_DXCC = ? AND %s", modeCondition)
	args := append([]interface{}{countryID}, modeParams...)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Errorf("HRD ListByCountryMode: %v", err)
		contactsModeChan <- nil
		return
	}
	defer rows.Close()
	contactsModeChan <- r.hrdScan(rows)
}

func (r *HRDContactsRepository) ListByCountryModeBand(countryID string, band string, mode string, contactsModeBandChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	modeCondition, modeParams := hrdSSBModeCondition(mode)
	query := fmt.Sprintf("SELECT "+hrdContactCols+" FROM "+hrdTable+" WHERE COL_DXCC = ? AND LOWER(COL_BAND) = LOWER(?) AND %s", modeCondition)
	args := append([]interface{}{countryID, band}, modeParams...)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Errorf("HRD ListByCountryModeBand: %v", err)
		contactsModeBandChan <- nil
		return
	}
	defer rows.Close()
	contactsModeBandChan <- r.hrdScan(rows)
}

func (r *HRDContactsRepository) ListByCountryBand(countryID string, band string, contactsBandChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	rows, err := r.db.Query(
		"SELECT "+hrdContactCols+" FROM "+hrdTable+" WHERE COL_DXCC = ? AND LOWER(COL_BAND) = LOWER(?)",
		countryID, band)
	if err != nil {
		r.Log.Errorf("HRD ListByCountryBand: %v", err)
		contactsBandChan <- nil
		return
	}
	defer rows.Close()
	contactsBandChan <- r.hrdScan(rows)
}

func (r *HRDContactsRepository) ListByCallSign(callSign string, band string, mode string, contactsCallChan chan []Contact, wg *sync.WaitGroup) {
	defer wg.Done()
	modeCondition, modeParams := hrdSSBModeCondition(mode)
	query := fmt.Sprintf("SELECT "+hrdContactCols+" FROM "+hrdTable+" WHERE COL_CALL = ? AND LOWER(COL_BAND) = LOWER(?) AND %s", modeCondition)
	args := append([]interface{}{callSign, band}, modeParams...)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Errorf("HRD ListByCallSign: %v", err)
		contactsCallChan <- nil
		return
	}
	defer rows.Close()
	contactsCallChan <- r.hrdScan(rows)
}

func (r *HRDContactsRepository) GetRecentQSOs(limit string) []QSO {
	query := fmt.Sprintf(
		"SELECT COL_CALL, COL_BAND, COL_MODE, %s,"+
			" COALESCE(COL_RST_SENT,''), COALESCE(COL_RST_RCVD,''),"+
			" COALESCE(COL_COUNTRY,''), COALESCE(CAST(COL_DXCC AS TEXT),'')"+
			" FROM %s"+
			" WHERE COL_QSO_DATE IS NOT NULL AND COL_QSO_DATE != ''"+
			" ORDER BY COL_QSO_DATE DESC, COL_TIME_ON DESC"+
			" LIMIT %s",
		hrdDateExpr, hrdTable, limit)

	rows, err := r.db.Query(query)
	if err != nil {
		r.Log.Errorf("HRD GetRecentQSOs: %v", err)
		return []QSO{}
	}
	defer rows.Close()

	var qsos []QSO
	for rows.Next() {
		var q QSO
		if err := rows.Scan(&q.Callsign, &q.Band, &q.Mode, &q.Date, &q.RSTSent, &q.RSTRcvd, &q.Country, &q.DXCC); err != nil {
			continue
		}
		q.Band = strings.ToUpper(strings.TrimSpace(q.Band))
		q.Mode = strings.ToUpper(strings.TrimSpace(q.Mode))
		qsos = append(qsos, q)
	}
	return qsos
}

func (r *HRDContactsRepository) GetQSOStats() QSOStats {
	stats := QSOStats{}
	r.db.QueryRow("SELECT COUNT(*) FROM " + hrdTable + " WHERE COL_QSO_DATE >= strftime('%Y%m%d', 'now')").Scan(&stats.Today)
	r.db.QueryRow("SELECT COUNT(*) FROM " + hrdTable + " WHERE COL_QSO_DATE >= strftime('%Y%m%d', date('now', '-7 days'))").Scan(&stats.ThisWeek)
	r.db.QueryRow("SELECT COUNT(*) FROM " + hrdTable + " WHERE COL_QSO_DATE >= strftime('%Y%m%d', date('now', 'start of month'))").Scan(&stats.ThisMonth)
	stats.Total = r.CountEntries()
	return stats
}

func (r *HRDContactsRepository) GetDXCCCount() int {
	var count int
	r.db.QueryRow("SELECT COUNT(DISTINCT COL_DXCC) FROM " + hrdTable + " WHERE COL_DXCC IS NOT NULL AND COL_DXCC != 0").Scan(&count)
	return count
}

func (r *HRDContactsRepository) GetWorkedCallsignsBandMode(callsigns []string, band string, mode string) map[string]bool {
	result := make(map[string]bool)
	if len(callsigns) == 0 {
		return result
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(callsigns)), ",")
	modeCondition, modeParams := hrdSSBModeCondition(mode)
	query := fmt.Sprintf(
		"SELECT DISTINCT COL_CALL FROM "+hrdTable+" WHERE COL_CALL IN (%s) AND LOWER(COL_BAND) = LOWER(?) AND %s",
		placeholders, modeCondition)
	args := make([]interface{}, 0, len(callsigns)+1+len(modeParams))
	for _, c := range callsigns {
		args = append(args, c)
	}
	args = append(args, band)
	args = append(args, modeParams...)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Errorf("HRD GetWorkedCallsignsBandMode: %v", err)
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var cs string
		if rows.Scan(&cs) == nil {
			result[cs] = true
		}
	}
	return result
}

func (r *HRDContactsRepository) HasWorkedCallsignToday(callsign, band, mode string) bool {
	modeCondition, modeParams := hrdSSBModeCondition(mode)
	query := fmt.Sprintf(
		"SELECT COUNT(*) FROM "+hrdTable+" WHERE COL_CALL = ? AND LOWER(COL_BAND) = LOWER(?) AND %s AND COL_QSO_DATE >= strftime('%%Y%%m%%d', 'now')",
		modeCondition)
	args := append([]interface{}{callsign, band}, modeParams...)
	var count int
	r.db.QueryRow(query, args...).Scan(&count)
	return count > 0
}

func (r *HRDContactsRepository) GetWorkedCallsignsBandModeToday(callsigns []string, band string, mode string) map[string]bool {
	result := make(map[string]bool)
	if len(callsigns) == 0 {
		return result
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(callsigns)), ",")
	modeCondition, modeParams := hrdSSBModeCondition(mode)
	query := fmt.Sprintf(
		"SELECT DISTINCT COL_CALL FROM "+hrdTable+
			" WHERE COL_CALL IN (%s) AND LOWER(COL_BAND) = LOWER(?) AND %s AND COL_QSO_DATE >= strftime('%%Y%%m%%d', 'now')",
		placeholders, modeCondition)
	args := make([]interface{}, 0, len(callsigns)+1+len(modeParams))
	for _, c := range callsigns {
		args = append(args, c)
	}
	args = append(args, band)
	args = append(args, modeParams...)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.Log.Errorf("HRD GetWorkedCallsignsBandModeToday: %v", err)
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var cs string
		if rows.Scan(&cs) == nil {
			result[cs] = true
		}
	}
	return result
}

func (r *HRDContactsRepository) GetCallsignBandModes(callsign string) CallsignBandModeInfo {
	info := CallsignBandModeInfo{Callsign: strings.ToUpper(callsign)}
	const dateOnly = "substr(COL_QSO_DATE,1,4)||'-'||substr(COL_QSO_DATE,5,2)||'-'||substr(COL_QSO_DATE,7,2)"
	query := fmt.Sprintf(`
		SELECT COL_BAND, COL_MODE, COUNT(*),
		       MIN(%s), MAX(%s),
		       COALESCE(COL_COUNTRY,''), COALESCE(COL_DXCC,'')
		FROM %s
		WHERE COL_CALL = ?
		GROUP BY COL_BAND, COL_MODE
		ORDER BY COL_BAND, COL_MODE
	`, dateOnly, dateOnly, hrdTable)
	rows, err := r.db.Query(query, strings.ToUpper(callsign))
	if err != nil {
		r.Log.Errorf("HRD GetCallsignBandModes %s: %v", callsign, err)
		return info
	}
	defer rows.Close()
	for rows.Next() {
		var q CallsignQSOEntry
		var firstQSO, lastQSO, country, dxcc string
		if err := rows.Scan(&q.Band, &q.Mode, &q.Count, &firstQSO, &lastQSO, &country, &dxcc); err != nil {
			continue
		}
		q.Band = strings.ToUpper(strings.TrimSpace(q.Band))
		q.Mode = strings.ToUpper(strings.TrimSpace(q.Mode))
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

func (r *HRDContactsRepository) HasWorkedCallsignBandMode(callsign, band, mode string) bool {
	return r.GetWorkedCallsignsBandMode([]string{callsign}, band, mode)[callsign]
}


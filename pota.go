package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// POTAPark représente un parc POTA depuis l'API
type POTAPark struct {
	Reference   string `json:"reference"`
	Name        string `json:"name"`
	EntityName  string `json:"entityName"`
	LocationDesc string `json:"locationDesc"`
}

// potaCache est le cache SQLite des parcs POTA (persistant, fichier séparé)
var potaCache *POTACache

type POTACache struct {
	db *sql.DB
}

// NewPOTACache ouvre (ou crée) la base de cache pota.sqlite
func NewPOTACache(path string) (*POTACache, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("pota cache open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS pota_parks (
			reference   TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			entity      TEXT DEFAULT '',
			location    TEXT DEFAULT '',
			fetched_at  INTEGER NOT NULL
		)`)
	if err != nil {
		return nil, fmt.Errorf("pota cache create table: %w", err)
	}

	return &POTACache{db: db}, nil
}

// Get retourne le nom d'un parc depuis le cache, ou "" si absent/expiré (TTL 30 jours)
func (c *POTACache) Get(ref string) (POTAPark, bool) {
	var p POTAPark
	var fetchedAt int64
	err := c.db.QueryRowContext(context.Background(),
		`SELECT reference, name, entity, location, fetched_at FROM pota_parks WHERE reference = ?`,
		strings.ToUpper(ref),
	).Scan(&p.Reference, &p.Name, &p.EntityName, &p.LocationDesc, &fetchedAt)
	if err != nil {
		return POTAPark{}, false
	}
	// TTL 30 jours
	if time.Now().Unix()-fetchedAt > 30*24*3600 {
		return POTAPark{}, false
	}
	return p, true
}

// Set stocke un parc dans le cache
func (c *POTACache) Set(p POTAPark) {
	_, err := c.db.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO pota_parks (reference, name, entity, location, fetched_at)
		 VALUES (?, ?, ?, ?, ?)`,
		strings.ToUpper(p.Reference), p.Name, p.EntityName, p.LocationDesc, time.Now().Unix(),
	)
	if err != nil {
		Log.Warnf("pota cache set %s: %v", p.Reference, err)
	}
}

// Close ferme la base de cache
func (c *POTACache) Close() {
	if c.db != nil {
		c.db.Close()
	}
}

// FetchPOTAPark appelle api.pota.app/park/{ref} et retourne le parc
func FetchPOTAPark(ref string) (POTAPark, error) {
	url := fmt.Sprintf("https://api.pota.app/park/%s", strings.ToUpper(ref))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return POTAPark{}, fmt.Errorf("pota fetch %s: %w", ref, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return POTAPark{}, fmt.Errorf("pota fetch %s: HTTP %d", ref, resp.StatusCode)
	}

	// L'API retourne un tableau même pour un seul parc
	var parks []POTAPark
	if err := json.NewDecoder(resp.Body).Decode(&parks); err != nil {
		// Essayer en objet direct
		return POTAPark{}, fmt.Errorf("pota decode %s: %w", ref, err)
	}

	if len(parks) == 0 {
		return POTAPark{}, fmt.Errorf("pota: no result for %s", ref)
	}
	return parks[0], nil
}

// GetPOTAParkName retourne le nom complet d'un parc POTA.
// Cherche d'abord dans le cache SQLite, sinon appelle l'API (non-bloquant via goroutine).
// Retourne "" immédiatement si pas en cache (l'enrichissement arrivera au prochain spot du même parc).
func GetPOTAParkName(ref string) string {
	if potaCache == nil || ref == "" {
		return ""
	}

	// Hit cache
	if p, ok := potaCache.Get(ref); ok {
		return p.Name
	}

	// Miss cache — fetch en arrière-plan, disponible au prochain spot
	go func() {
		p, err := FetchPOTAPark(ref)
		if err != nil {
			Log.Debugf("POTA fetch failed for %s: %v", ref, err)
			return
		}
		potaCache.Set(p)
		Log.Infof("🏕️  POTA park cached: %s = %s (%s)", p.Reference, p.Name, p.EntityName)
	}()

	return ""
}

// potaRefRe extrait une référence POTA d'un commentaire brut
var potaRefRe = regexp.MustCompile(`\b([A-Z]{1,4}-\d{4,6})\b`)

// ExtractPOTARef extrait la première référence POTA trouvée dans un commentaire
func ExtractPOTARef(comment string) string {
	upper := strings.ToUpper(comment)
	if !strings.Contains(upper, "POTA") && !strings.Contains(upper, "[-POTA-]") {
		return ""
	}
	if m := potaRefRe.FindString(upper); m != "" {
		return m
	}
	return ""
}

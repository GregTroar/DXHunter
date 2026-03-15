package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// POTAPark représente un parc POTA depuis l'API
// Les champs nullables utilisent des pointeurs pour gérer les null JSON
type POTAPark struct {
	Reference    string  `json:"reference"`
	Name         string  `json:"name"`
	EntityName   *string `json:"entityName"`
	LocationDesc *string `json:"locationDesc"`
	LocationName *string `json:"locationName"`
}

// entityName retourne une string safe depuis le pointeur nullable
func (p POTAPark) EntityNameStr() string {
	if p.EntityName != nil {
		return *p.EntityName
	}
	return ""
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
		strings.ToUpper(p.Reference), p.Name, p.EntityNameStr(), p.EntityNameStr(), time.Now().Unix(),
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

// FetchPOTAPark appelle api.pota.app/park/{ref} et retourne le parc.
// Gère les deux formats possibles : objet direct {…} ou tableau [{…}]
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return POTAPark{}, fmt.Errorf("pota read %s: %w", ref, err)
	}

	Log.Debugf("POTA API raw response for %s: %s", ref, string(body))

	// Essayer objet direct en premier
	var park POTAPark
	if err := json.Unmarshal(body, &park); err == nil && park.Name != "" {
		return park, nil
	}

	// Essayer tableau
	var parks []POTAPark
	if err := json.Unmarshal(body, &parks); err == nil && len(parks) > 0 && parks[0].Name != "" {
		return parks[0], nil
	}

	return POTAPark{}, fmt.Errorf("pota: no result for %s (body: %s)", ref, string(body))
}

// GetPOTAParkName retourne le nom complet d'un parc POTA.
// Cache hit -> retour immédiat.
// Cache miss -> appel bloquant avec timeout 2s. Si réponse dans les temps, nom retourné dès le premier spot.
// Si timeout dépassé, retourne "" et met en cache en background pour les spots suivants.
func GetPOTAParkName(ref string) string {
	if potaCache == nil || ref == "" {
		return ""
	}

	// Hit cache
	if p, ok := potaCache.Get(ref); ok {
		return p.Name
	}

	// Miss cache — appel bloquant avec timeout 2s
	type result struct {
		park POTAPark
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		p, err := FetchPOTAPark(ref)
		ch <- result{p, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			Log.Debugf("POTA fetch failed for %s: %v", ref, r.err)
			return ""
		}
		potaCache.Set(r.park)
		Log.Infof("🏕️  POTA park: %s = %s (%s)", r.park.Reference, r.park.Name, r.park.EntityNameStr())
		return r.park.Name
	case <-time.After(2 * time.Second):
		// Timeout — on met en cache en background pour les prochains spots
		go func() {
			p, err := FetchPOTAPark(ref)
			if err == nil {
				potaCache.Set(p)
				Log.Infof("🏕️  POTA park cached (delayed): %s = %s", p.Reference, p.Name)
			}
		}()
		return ""
	}
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

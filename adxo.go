package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// Structures
// ============================================================================

type ADXOItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
}

type ADXOFeed struct {
	Items []ADXOItem `xml:"channel>item"`
}

type Activation struct {
	DXCC      string   `json:"dxcc"`
	Callsign  string   `json:"callsign"`
	StartDate string   `json:"startDate"`
	EndDate   string   `json:"endDate"`
	Bands     []string `json:"bands"`
	Modes     []string `json:"modes"`
	QSL       string   `json:"qsl"`
	Operators string   `json:"operators"`
	Source    string   `json:"source"`
	Link      string   `json:"link"`
	Status    string   `json:"status"` // "active" | "upcoming" | "ended"
}

// ============================================================================
// Cache
// ============================================================================

type ADXOCache struct {
	mu          sync.RWMutex
	activations []Activation
	lastFetch   time.Time
}

var adxoCache = &ADXOCache{}

func (c *ADXOCache) Get() []Activation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activations
}

func (c *ADXOCache) Set(a []Activation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activations = a
	c.lastFetch = time.Now()
}

func (c *ADXOCache) NeedsRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastFetch) > 1*time.Hour
}

// ============================================================================
// Fetch & Parse
// ============================================================================

func FetchADXO() ([]Activation, error) {
	resp, err := http.Get("https://www.ng3k.com/adxo.xml")
	if err != nil {
		return nil, fmt.Errorf("fetch error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var feed ADXOFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	var activations []Activation
	now := time.Now()

	for _, item := range feed.Items {
		a := parseActivation(item.Description, item.Link)
		if a == nil {
			continue
		}
		// Déterminer le statut
		start, errS := parseADXODate(a.StartDate, now.Year())
		end, errE := parseADXODate(a.EndDate, now.Year())
		if errS == nil && errE == nil {
			if now.Before(start) {
				a.Status = "upcoming"
			} else if now.After(end) {
				a.Status = "ended"
			} else {
				a.Status = "active"
			}
		} else {
			a.Status = "upcoming"
		}
		// Ne pas inclure les activations terminées
		if a.Status == "ended" {
			continue
		}
		activations = append(activations, *a)
	}

	return activations, nil
}

// parseActivation parse la description d'un item RSS ADXO
// Format: " Feb 17-Mar 30, 2026 -- DXCC -- CALLSIGN -- QSL: xxx -- Source: xxx -- By operators; bands; modes"
func parseActivation(desc, link string) *Activation {
	// Nettoyer les newlines et espaces multiples
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, "\r", " ")
	// Réduire les espaces multiples à un seul
	spaceRe := regexp.MustCompile(`\s+`)
	desc = spaceRe.ReplaceAllString(desc, " ")
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return nil
	}

	parts := strings.Split(desc, " -- ")
	if len(parts) < 3 {
		return nil
	}

	a := &Activation{Link: link}

	// Dates (première partie)
	dateStr := strings.TrimSpace(parts[0])
	start, end := parseDateRange(dateStr)
	a.StartDate = start
	a.EndDate = end

	// DXCC
	a.DXCC = strings.TrimSpace(parts[1])

	// Callsign
	a.Callsign = strings.TrimSpace(parts[2])

	// QSL
	for _, p := range parts[3:] {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "QSL:") {
			a.QSL = strings.TrimSpace(strings.TrimPrefix(p, "QSL:"))
		} else if strings.HasPrefix(p, "Source:") {
			a.Source = strings.TrimSpace(strings.TrimPrefix(p, "Source:"))
		}
	}

	// Opérateurs, bandes, modes — dans la dernière partie après "By "
	lastPart := parts[len(parts)-1]
	if idx := strings.Index(lastPart, "By "); idx >= 0 {
		detail := lastPart[idx+3:]
		// Séparer par ";" : opérateurs ; bandes ; modes [; reste]
		subParts := strings.Split(detail, ";")
		if len(subParts) >= 1 {
			a.Operators = strings.TrimSpace(subParts[0])
		}
		if len(subParts) >= 2 {
			a.Bands = parseBands(strings.TrimSpace(subParts[1]))
		}
		if len(subParts) >= 3 {
			a.Modes = parseModes(strings.TrimSpace(subParts[2]))
		}
	}

	// Extraire le(s) vrai(s) callsign(s) depuis operators
	if a.Operators != "" {
		if calls := extractCallsignsFromAs(a.Operators); len(calls) > 0 {
			// Normaliser : le préfixe DXCC doit être en premier (ex: JD1/JG8NQJ pas JG8NQJ/JD1)
			dxccPrefix := strings.ToUpper(strings.TrimSpace(parts[2]))
			for i, call := range calls {
				calls[i] = normalizeCallsign(call, dxccPrefix)
			}
			a.Callsign = strings.Join(calls, ", ")
		}
	}

	return a
}

// parseDateRange extrait start/end depuis "Feb 17-Mar 30, 2026" ou "Mar 3-20, 2026"
func parseDateRange(s string) (string, string) {
	// Regex pour "Mon DD-Mon DD, YYYY" ou "Mon DD-DD, YYYY"
	reFullRange := regexp.MustCompile(`(?i)(\w+ \d+)-(\w+ \d+),\s*(\d{4})`)
	reShortRange := regexp.MustCompile(`(?i)(\w+) (\d+)-(\d+),\s*(\d{4})`)

	if m := reFullRange.FindStringSubmatch(s); m != nil {
		return m[1] + ", " + m[3], m[2] + ", " + m[3]
	}
	if m := reShortRange.FindStringSubmatch(s); m != nil {
		return m[1] + " " + m[2] + ", " + m[4], m[1] + " " + m[3] + ", " + m[4]
	}
	return s, s
}

// normalizeCallsign s'assure que le préfixe DXCC est en première position
// Ex: normalizeCallsign("JG8NQJ/JD1", "JD1") → "JD1/JG8NQJ"
// Ex: normalizeCallsign("PJ2/W2APF", "PJ2") → "PJ2/W2APF" (déjà correct)
func normalizeCallsign(call, dxccPrefix string) string {
	if !strings.Contains(call, "/") {
		return call
	}
	parts := strings.SplitN(call, "/", 2)
	left, right := parts[0], parts[1]
	// Si le préfixe DXCC est à droite, on inverse
	if strings.HasPrefix(right, dxccPrefix) {
		return right + "/" + left
	}
	return call
}

// extractCallsignsFromAs extrait les callsigns après "as " dans la chaîne operators
// Ex: "W2APF as PJ2/W2APF" → ["PJ2/W2APF"]
// Ex: "SQ2RAD as VP2EAD, M0PLX as VP2ELX" → ["VP2EAD", "VP2ELX"]
// Ex: "JG8NQJ as JG8NQJ/JD1 fm IOTA OC-073" → ["JG8NQJ/JD1"]
func extractCallsignsFromAs(operators string) []string {
	// Regex : "as CALLSIGN" — callsign = au moins une lettre + chiffre (format ham)
	// On s'arrête avant "fm", "and", virgule, parenthèse
	re := regexp.MustCompile(`(?i)\bas\s+([A-Z0-9]+(?:/[A-Z0-9]+)*)`)
	matches := re.FindAllStringSubmatch(operators, -1)

	// Mots à ignorer (pas des callsigns)
	ignore := map[string]bool{
		"FM": true, "AND": true, "DE": true, "THE": true, "INFO": true,
	}

	var calls []string
	seen := make(map[string]bool)
	for _, m := range matches {
		call := strings.ToUpper(strings.TrimRight(m[1], ",;."))
		// Un callsign ham contient au moins un chiffre et au moins 2 lettres
		if ignore[call] {
			continue
		}
		hasDigit := regexp.MustCompile(`\d`).MatchString(call)
		hasLetter := regexp.MustCompile(`[A-Z]{2}`).MatchString(call)
		if hasDigit && hasLetter && !seen[call] {
			calls = append(calls, call)
			seen[call] = true
		}
	}
	return calls
}

// parseBands convertit "160-6m" ou "20 15 10m" en slice
func parseBands(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Cas "160-6m" ou "HF" ou "1.8-54 MHz"
	if strings.Contains(s, "-") && !strings.Contains(s, " ") {
		return []string{s}
	}

	// Sinon split par espaces
	parts := strings.Fields(s)
	var bands []string
	for _, p := range parts {
		p = strings.TrimRight(p, ",;")
		if p != "" {
			bands = append(bands, p)
		}
	}
	return bands
}

// parseModes extrait les modes connus
func parseModes(s string) []string {
	knownModes := []string{"CW", "SSB", "FT8", "FT4", "RTTY", "PSK", "AM", "FM", "DIGI", "DATA"}
	s = strings.ToUpper(s)
	var modes []string
	for _, m := range knownModes {
		if strings.Contains(s, m) {
			modes = append(modes, m)
		}
	}
	return modes
}

// parseADXODate parse "Feb 17, 2026" en time.Time
func parseADXODate(s string, fallbackYear int) (time.Time, error) {
	formats := []string{
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan 2, 06",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}

// ============================================================================
// Background refresh
// ============================================================================

func StartADXORefresher(ctx context.Context, broadcast chan WSMessage, watchlist *Watchlist) {
	go func() {
		refreshADXO(broadcast, watchlist)

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refreshADXO(broadcast, watchlist)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func refreshADXO(broadcast chan WSMessage, watchlist *Watchlist) {
	activations, err := FetchADXO()
	if err != nil {
		Log.Errorf("ADXO fetch error: %v", err)
		return
	}
	adxoCache.Set(activations)
	active := 0
	for _, a := range activations {
		if a.Status == "active" {
			active++
		}
	}
	Log.Infof("ADXO: %d activations loaded (%d active)", len(activations), active)

	// Nettoyer la watchlist des activations terminées depuis plus de 7 jours
	if watchlist != nil {
		cleanupWatchlistFromADXO(activations, watchlist, broadcast)
	}

	// Broadcaster vers tous les clients WebSocket connectés
	if broadcast != nil {
		select {
		case broadcast <- WSMessage{Type: "adxo", Data: activations}:
		default:
			Log.Errorf("ADXO broadcast channel full, skipping")
		}
	}
}

// cleanupWatchlistFromADXO supprime les callsigns dont l'activation ADXO
// est terminée depuis plus de 7 jours
func cleanupWatchlistFromADXO(activations []Activation, watchlist *Watchlist, broadcast chan WSMessage) {
	cutoff := time.Now().AddDate(0, 0, -7)
	removed := 0

	for _, call := range watchlist.GetAllCallsigns() {
		callUpper := strings.ToUpper(call)

		var matchedEnd time.Time
		found := false
		for _, a := range activations {
			for _, ac := range strings.Split(a.Callsign, ",") {
				if strings.TrimSpace(strings.ToUpper(ac)) == callUpper {
					t, err := parseADXODate(a.EndDate, time.Now().Year())
					if err == nil {
						matchedEnd = t
						found = true
					}
					break
				}
			}
			if found {
				break
			}
		}

		if found && !matchedEnd.IsZero() && matchedEnd.Before(cutoff) {
			if err := watchlist.Remove(call); err == nil {
				Log.Infof("🧹 Watchlist cleanup: removed %s (activation ended %s)", call, matchedEnd.Format("Jan 2, 2006"))
				removed++
			}
		}
	}

	if removed > 0 {
		// Broadcaster la watchlist mise à jour
		if broadcast != nil {
			select {
			case broadcast <- WSMessage{Type: "watchlist", Data: watchlist.GetAll()}:
			default:
			}
		}
		Log.Infof("🧹 Watchlist cleanup: removed %d expired activations", removed)
	}
}

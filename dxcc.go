package main

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"sync"
)

//go:embed cty.plist
var embeddedCtyPlist []byte

// ============================================================================
// TYPES - Apple plist XML
// ============================================================================

type PlistRoot struct {
	XMLName xml.Name  `xml:"plist"`
	Dict    PlistDict `xml:"dict"`
}

type PlistDict struct {
	Keys  []string      `xml:"key"`
	Items []interface{} // non utilisé directement, on parse manuellement
}

// CtyEntry représente une entrée du fichier cty.plist
type CtyEntry struct {
	Prefix        string
	Country       string
	ADIF          int
	CQZone        int
	ITUZone       int
	Continent     string
	Latitude      float64
	Longitude     float64
	GMTOffset     float64
	ExactCallsign bool
}

// DXCC est le résultat d'une résolution de callsign (compatible avec l'ancien xml.go)
type DXCC struct {
	Callsign    string
	CountryName string
	DXCC        string
}

// ============================================================================
// LOADER - Parse cty.plist
// ============================================================================

type CtyDatabase struct {
	entries map[string]*CtyEntry // clé = préfixe ou callsign exact
	mu      sync.RWMutex
}

var ctyDB *CtyDatabase

// LoadCtyPlist charge cty.plist.
// Priorité : fichier sur disque (permet les mises à jour) → sinon version embarquée dans l'exe.
func LoadCtyPlist(filePath string) (*CtyDatabase, error) {
	var data []byte

	if _, err := os.Stat(filePath); err == nil {
		// Fichier présent sur disque → l'utiliser (version mise à jour)
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("cannot read cty.plist from disk: %w", err)
		}
		Log.Debugf("cty.plist loaded from disk: %s", filePath)
	} else {
		// Pas de fichier sur disque → utiliser la version embarquée
		data = embeddedCtyPlist
		Log.Debug("cty.plist loaded from embedded data")
	}

	db, err := parseCtyPlist(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse cty.plist: %w", err)
	}

	Log.Debugf("Loaded cty.plist: %d entries", len(db.entries))
	return db, nil
}

// parseCtyPlist parse le format Apple plist XML de cty.plist.
// Structure : <dict> avec alternance <key>PREFIX</key><dict>...</dict>
func parseCtyPlist(data []byte) (*CtyDatabase, error) {
	db := &CtyDatabase{
		entries: make(map[string]*CtyEntry),
	}

	type xmlKey struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	}

	// Parser manuellement le plist car le format alterne key/dict sans structure Go naturelle
	decoder := xml.NewDecoder(strings.NewReader(string(data)))

	// Avancer jusqu'au <dict> racine
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "dict" {
			break
		}
	}

	// Lire les paires <key>PREFIX</key><dict>...</dict>
	for {
		// Lire le token suivant
		tok, err := decoder.Token()
		if err != nil {
			break
		}

		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		if se.Name.Local == "key" {
			// Lire la valeur de la clé (préfixe)
			var prefix string
			decoder.DecodeElement(&prefix, &se)
			prefix = strings.TrimSpace(prefix)

			// Lire le <dict> suivant
			entry, err := parseCtyEntryDict(decoder, prefix)
			if err != nil {
				Log.Warnf("Error parsing entry %s: %v", prefix, err)
				continue
			}
			db.entries[prefix] = entry

		} else if se.Name.Local == "dict" {
			// dict racine fermant ou imbriqué inattendu
			break
		}
	}

	return db, nil
}

// parseCtyEntryDict lit le <dict> d'une entrée cty et retourne un CtyEntry
func parseCtyEntryDict(decoder *xml.Decoder, prefix string) (*CtyEntry, error) {
	entry := &CtyEntry{Prefix: prefix}

	// Avancer jusqu'au prochain <dict>
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "dict" {
			break
		}
	}

	// Lire les paires <key>name</key><value> dans ce dict
	var currentKey string
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				var k string
				decoder.DecodeElement(&k, &t)
				currentKey = strings.TrimSpace(k)

			case "string":
				var v string
				decoder.DecodeElement(&v, &t)
				switch currentKey {
				case "Country":
					entry.Country = v
				case "Continent":
					entry.Continent = v
				}

			case "integer":
				var v int
				decoder.DecodeElement(&v, &t)
				switch currentKey {
				case "ADIF":
					entry.ADIF = v
				case "CQZone":
					entry.CQZone = v
				case "ITUZone":
					entry.ITUZone = v
				}

			case "real":
				var v float64
				decoder.DecodeElement(&v, &t)
				switch currentKey {
				case "Latitude":
					entry.Latitude = v
				case "Longitude":
					entry.Longitude = v
				case "GMTOffset":
					entry.GMTOffset = v
				}

			case "true":
				decoder.DecodeElement(nil, &t)
				if currentKey == "ExactCallsign" {
					entry.ExactCallsign = true
				}

			case "false":
				decoder.DecodeElement(nil, &t)
				if currentKey == "ExactCallsign" {
					entry.ExactCallsign = false
				}
			}

		case xml.EndElement:
			if t.Name.Local == "dict" {
				return entry, nil
			}
		}
	}
}

// ============================================================================
// LOOKUP - Résolution callsign → DXCC
// ============================================================================

// GetDXCC résout un callsign vers son pays/DXCC.
// Priorité :
//  1. ClubLog exceptions (callsign exact, date-valid) — le plus précis
//  2. cty.plist correspondance exacte
//  3. ClubLog prefixes (préfixe le plus long, date-valid)
//  4. cty.plist préfixe le plus long
func GetDXCC(callsign string) DXCC {
	if ctyDB == nil {
		Log.Warn("ctyDB not initialized")
		return DXCC{}
	}

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	// 1. ClubLog exception (exact callsign, highest priority)
	if e := LookupClubLogException(callsign); e != nil {
		return DXCC{
			Callsign:    callsign,
			CountryName: e.Name,
			DXCC:        fmt.Sprintf("%d", e.ADIF),
		}
	}

	ctyDB.mu.RLock()
	defer ctyDB.mu.RUnlock()

	// 2. cty.plist correspondance exacte
	if entry, ok := ctyDB.entries[callsign]; ok && entry.ExactCallsign {
		return DXCC{
			Callsign:    callsign,
			CountryName: entry.Country,
			DXCC:        fmt.Sprintf("%d", entry.ADIF),
		}
	}

	// 3. ClubLog prefix (longest match)
	if clb := LookupClubLogPrefix(callsign); clb != nil {
		return DXCC{
			Callsign:    callsign,
			CountryName: clb.Name,
			DXCC:        fmt.Sprintf("%d", clb.ADIF),
		}
	}

	// 4. cty.plist préfixe le plus long
	var best *CtyEntry
	bestLen := 0

	for prefix, entry := range ctyDB.entries {
		if entry.ExactCallsign {
			continue
		}
		if strings.HasPrefix(callsign, prefix) && len(prefix) > bestLen {
			best = entry
			bestLen = len(prefix)
		}
	}

	if best != nil {
		best = applyDXCCExceptions(callsign, best, bestLen)
		return DXCC{
			Callsign:    callsign,
			CountryName: best.Country,
			DXCC:        fmt.Sprintf("%d", best.ADIF),
		}
	}

	Log.Warnf("Could not find DXCC for callsign: %s", callsign)
	return DXCC{}
}

// applyDXCCExceptions corrects prefix matches for callsigns that require
// suffix-length disambiguation (e.g. KG4: 2-letter suffix = Guantanamo, else USA).
func applyDXCCExceptions(callsign string, matched *CtyEntry, matchedLen int) *CtyEntry {
	// KG4XX (exactly 2 alpha letters after KG4) = Guantanamo Bay
	// KG4X / KG4XXX+ = USA
	if matchedLen == 3 && len(callsign) >= 3 && callsign[:3] == "KG4" {
		suffix := callsign[3:]
		if len(suffix) != 2 || !isAllAlpha(suffix) {
			if kEntry, ok := ctyDB.entries["K"]; ok {
				return kEntry
			}
		}
	}
	return matched
}

func isAllAlpha(s string) bool {
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// GetCtyEntry retourne l'entrée complète (avec CQZone, continent, etc.)
func GetCtyEntry(callsign string) *CtyEntry {
	if ctyDB == nil {
		return nil
	}

	ctyDB.mu.RLock()
	defer ctyDB.mu.RUnlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	// Exact
	if entry, ok := ctyDB.entries[callsign]; ok && entry.ExactCallsign {
		return entry
	}

	// Préfixe le plus long
	var best *CtyEntry
	bestLen := 0
	for prefix, entry := range ctyDB.entries {
		if entry.ExactCallsign {
			continue
		}
		if strings.HasPrefix(callsign, prefix) && len(prefix) > bestLen {
			best = entry
			bestLen = len(prefix)
		}
	}
	if best != nil {
		best = applyDXCCExceptions(callsign, best, bestLen)
	}
	return best
}

// ReloadCtyDB recharge la base depuis le disque (appelé après mise à jour)
func ReloadCtyDB(filePath string) error {
	db, err := LoadCtyPlist(filePath)
	if err != nil {
		return err
	}

	if ctyDB != nil {
		ctyDB.mu.Lock()
		ctyDB.entries = db.entries
		ctyDB.mu.Unlock()
	} else {
		ctyDB = db
	}

	Log.Debugf("cty.plist reloaded: %d entries", len(db.entries))
	return nil
}

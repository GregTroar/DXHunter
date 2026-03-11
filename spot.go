package main

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type TelnetSpot struct {
	DX             string
	Spotter        string
	Frequency      string
	Mode           string
	Band           string
	Time           string
	DXCC           string
	CountryName    string
	Comment        string
	CommandNumber  int
	FlexSpotNumber int
	NewDXCC        bool
	NewBand        bool
	NewMode        bool
	NewSlot        bool
	CallsignWorked bool
	ClusterName    string // Nom du cluster source
	POTARef        string // ex: "BB-0036"
	SOTARef        string // ex: "F/AB-123"
}

func ProcessTelnetSpot(re *regexp.Regexp, reShort *regexp.Regexp, spotRaw string, SpotChanToFlex chan TelnetSpot, SpotChanToHTTPServer chan TelnetSpot, contactRepo *Log4OMContactsRepository, clusterName string) {

	match := re.FindStringSubmatch(spotRaw)

	var spot TelnetSpot

	if len(match) > 0 {
		// Format standard : DX de SPOTTER:  FREQ  DX  MODE  COMMENT  TIME
		spot = TelnetSpot{
			DX:        match[3],
			Spotter:   match[1],
			Frequency: match[2],
			Mode:      match[4],
			Comment:   strings.Trim(match[5], " "),
			Time:      match[6],
		}
	} else {
		// ✅ Essayer le format court : FREQ DX DATE TIME COMMENT <SPOTTER>
		match = reShort.FindStringSubmatch(spotRaw)

		if len(match) == 0 {
			IncrementSpotsRejected()
			Log.Warnf("❌ Regex no match: %s", spotRaw)
			return
		}

		spot = TelnetSpot{
			DX:        match[2],
			Spotter:   match[5],
			Frequency: match[1],
			Mode:      "", // Mode détecté plus tard
			Comment:   strings.Trim(match[4], " "),
			Time:      match[3],
		}

		Log.Debugf("📡 Spot format court: %s @ %s MHz spotted by %s",
			spot.DX, spot.Frequency, spot.Spotter)
	}

	DXCC := GetDXCC(spot.DX)
	spot.DXCC = DXCC.DXCC
	spot.CountryName = DXCC.CountryName

	if spot.DXCC == "" {
		IncrementSpotsRejected()
		Log.Warnf("❌ DXCC not found: %s", spot.DX)
		return
	}

	spot.GetBand()
	spot.GuessMode(spotRaw)
	spot.ClusterName = clusterName
	spot.DetectPOTASOTA()
	spot.CallsignWorked = false
	spot.NewBand = false
	spot.NewMode = false
	spot.NewDXCC = false
	spot.NewSlot = false

	contactsChan := make(chan []Contact)
	contactsModeChan := make(chan []Contact)
	contactsModeBandChan := make(chan []Contact)
	contactsBandChan := make(chan []Contact)
	contactsCallChan := make(chan []Contact)

	wg := new(sync.WaitGroup)
	wg.Add(5)

	go contactRepo.ListByCountry(spot.DXCC, contactsChan, wg)
	contacts := <-contactsChan

	go contactRepo.ListByCountryMode(spot.DXCC, spot.Mode, contactsModeChan, wg)
	contactsMode := <-contactsModeChan

	go contactRepo.ListByCountryBand(spot.DXCC, spot.Band, contactsBandChan, wg)
	contactsBand := <-contactsBandChan

	go contactRepo.ListByCallSign(spot.DX, spot.Band, spot.Mode, contactsCallChan, wg)
	contactsCall := <-contactsCallChan

	go contactRepo.ListByCountryModeBand(spot.DXCC, spot.Band, spot.Mode, contactsModeBandChan, wg)
	contactsModeBand := <-contactsModeBandChan

	wg.Wait()

	// ✅ Déterminer le statut
	if len(contacts) == 0 {
		spot.NewDXCC = true
	}
	if len(contactsMode) == 0 {
		spot.NewMode = true
	}
	if len(contactsBand) == 0 {
		spot.NewBand = true
	}
	if len(contactsModeBand) == 0 && !spot.NewDXCC && !spot.NewBand && !spot.NewMode {
		spot.NewSlot = true
	}
	if len(contactsCall) > 0 {
		spot.CallsignWorked = true
	}

	// ✅ Envoyer le spot
	select {
	case SpotChanToHTTPServer <- spot:
		IncrementSpotsProcessed()
	default:
		IncrementSpotsRejected()
		Log.Errorf("❌ Spot dropped (channel full): %s @ %s", spot.DX, spot.Frequency)
		return
	}

	// ✅ LOGS CONCIS ET ADAPTES
	statusIcon := ""
	statusText := ""

	if spot.NewDXCC {
		statusIcon = "🆕"
		statusText = "NEW DXCC"
	} else if spot.NewBand && spot.NewMode {
		statusIcon = "📻"
		statusText = "NEW BAND+MODE"
	} else if spot.NewBand {
		statusIcon = "📡"
		statusText = "NEW BAND"
	} else if spot.NewMode {
		statusIcon = "🔧"
		statusText = "NEW MODE"
	} else if spot.NewSlot {
		statusIcon = "✨"
		statusText = "NEW SLOT"
	} else if spot.CallsignWorked {
		statusIcon = "✓"
		statusText = "WORKED"
	} else {
		statusIcon = "·"
		statusText = "SPOT"
	}

	// ✅ Log unique et concis
	Log.Debugf("%s [%s] %s on %.1f kHz (%s %s) - %s @ %s",
		statusIcon,
		statusText,
		spot.DX,
		mustParseFloat(spot.Frequency),
		spot.Band,
		spot.Mode,
		spot.CountryName,
		spot.Time,
	)
}

// ✅ Helper pour convertir la fréquence
func mustParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (spot *TelnetSpot) GetBand() {
	// ✅ Utiliser la fonction centralisée de bands.go
	// GetBandFromFrequencyString gère automatiquement kHz et MHz
	spot.Band = GetBandFromFrequencyString(spot.Frequency)

	// ✅ Normaliser SSB si nécessaire
	if spot.Mode == "SSB" {
		spot.Mode = NormalizeSSBMode(spot.Mode, spot.Band)
	}

	Log.Debugf("GetBand: %s @ %s → Band: %s", spot.DX, spot.Frequency, spot.Band)
}

func (spot *TelnetSpot) GuessMode(rawSpot string) {
	freqMHz, err := strconv.ParseFloat(spot.Frequency, 64)
	if err != nil {
		Log.Errorf("Could not convert frequency: %v", err)
		return
	}

	// Convertir kHz en MHz si nécessaire
	if freqMHz > 1000 {
		freqMHz = freqMHz / 1000.0
	}

	// ✅ Utiliser la fonction centralisée de modes.go
	spot.Mode = DetermineMode(spot.Mode, spot.Comment, freqMHz)

	Log.Debugf("GuessMode: %s @ %.3f MHz → Mode: %s", spot.DX, freqMHz, spot.Mode)
}

// DetectPOTASOTA extrait les références POTA et SOTA du commentaire
// Formats reconnus :
//
//	POTA : [-POTA-] BB-0036  ou  POTA BB-0036  ou  @BB-0036
//	SOTA : [SOTA] F/AB-123  ou  SOTA F/AB-123
func (spot *TelnetSpot) DetectPOTASOTA() {
	comment := spot.Comment
	upper := strings.ToUpper(comment)

	// POTA — référence de type XX-NNNNN (2 lettres, tiret, chiffres)
	if strings.Contains(upper, "POTA") || strings.Contains(upper, "[-POTA-]") {
		re := regexp.MustCompile(`\b([A-Z]{1,4}-\d{4,6})\b`)
		if m := re.FindString(comment); m != "" {
			spot.POTARef = strings.ToUpper(m)
		}
	}

	// SOTA — référence de type XX/XX-NNN (association/sommet)
	if strings.Contains(upper, "SOTA") {
		re := regexp.MustCompile(`\b([A-Z]{1,4}/[A-Z]{2}-\d{3})\b`)
		if m := re.FindStringSubmatch(strings.ToUpper(comment)); len(m) > 1 {
			spot.SOTARef = m[1]
		}
	}
}

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
}

func ProcessTelnetSpot(re *regexp.Regexp, spotRaw string, SpotChanToFlex chan TelnetSpot, SpotChanToHTTPServer chan TelnetSpot, Countries Countries, contactRepo *Log4OMContactsRepository) {

	match := re.FindStringSubmatch(spotRaw)

	if len(match) == 0 {
		IncrementSpotsRejected()
		Log.Warnf("❌ Regex no match: %s", spotRaw)
		return
	}

	spot := TelnetSpot{
		DX:        match[3],
		Spotter:   match[1],
		Frequency: match[2],
		Mode:      match[4],
		Comment:   strings.Trim(match[5], " "),
		Time:      match[6],
	}

	DXCC := GetDXCC(spot.DX, Countries)
	spot.DXCC = DXCC.DXCC
	spot.CountryName = DXCC.CountryName

	if spot.DXCC == "" {
		IncrementSpotsRejected()
		Log.Warnf("❌ DXCC not found: %s", spot.DX)
		return
	}

	spot.GetBand()
	spot.GuessMode(spotRaw)
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
	freq := FreqMhztoHz(spot.Frequency)
	switch true {
	case strings.HasPrefix(freq, "1.8"):
		spot.Band = "160M"
		if spot.Mode == "SSB" {
			spot.Mode = "LSB"
		}
	case strings.HasPrefix(freq, "3."):
		spot.Band = "80M"
		if spot.Mode == "SSB" {
			spot.Mode = "LSB"
		}
	case strings.HasPrefix(freq, "5."):
		spot.Band = "60M"
		if spot.Mode == "SSB" {
			spot.Mode = "LSB"
		}
	case strings.HasPrefix(freq, "7."):
		spot.Band = "40M"
		if spot.Mode == "SSB" {
			spot.Mode = "LSB"
		}
	case strings.HasPrefix(freq, "10."):
		spot.Band = "30M"
	case strings.HasPrefix(freq, "14."):
		spot.Band = "20M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	case strings.HasPrefix(freq, "18."):
		spot.Band = "17M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	case strings.HasPrefix(freq, "21."):
		spot.Band = "15M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	case strings.HasPrefix(freq, "24."):
		spot.Band = "12M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	case strings.HasPrefix(freq, "28."):
		spot.Band = "10M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	case strings.HasPrefix(freq, "29."):
		spot.Band = "10M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	case strings.HasPrefix(freq, "50."):
		spot.Band = "6M"
		if spot.Mode == "SSB" {
			spot.Mode = "USB"
		}
	default:
		spot.Band = "N/A"
	}
}

func (spot *TelnetSpot) GuessMode(rawSpot string) {
	// ✅ D'ABORD : Chercher le mode dans le commentaire
	if spot.Mode == "" {
		spot.Mode = extractModeFromComment(spot.Comment)
		if spot.Mode != "" {
			Log.Debugf("Mode extracted from comment: %s", spot.Mode)
		}
	}

	// ✅ Normaliser SSB avant de deviner
	if spot.Mode == "SSB" {
		if spot.Band == "10M" || spot.Band == "12M" || spot.Band == "6M" || spot.Band == "15M" || spot.Band == "17M" || spot.Band == "20M" {
			spot.Mode = "USB"
		} else {
			spot.Mode = "LSB"
		}
		Log.Debugf("Converted SSB to %s for band %s", spot.Mode, spot.Band)
		return
	}

	// ✅ Si pas de mode, deviner depuis la fréquence
	if spot.Mode == "" {
		freqInt, err := strconv.ParseFloat(spot.Frequency, 32)
		Log.Debugf("No mode specified in spot, will guess from frequency %v with spot: %s", spot.Frequency, strings.TrimSpace(rawSpot))
		if err != nil {
			Log.Errorf("could not convert frequency string to float: %v", err)
			return
		}

		switch spot.Band {
		case "160M": // 1.800 - 2.000 MHz
			if freqInt < 1838 {
				spot.Mode = "CW"
			} else if freqInt < 1843 {
				spot.Mode = "FT8"
			} else {
				spot.Mode = "LSB"
			}

		case "80M": // 3.500 - 4.000 MHz
			if freqInt < 3560 {
				spot.Mode = "CW"
			} else if freqInt < 3575 {
				spot.Mode = "FT8"
			} else if freqInt < 3578 {
				spot.Mode = "FT4"
			} else if freqInt < 3590 {
				spot.Mode = "RTTY"
			} else {
				spot.Mode = "LSB"
			}

		case "60M": // 5.330 - 5.405 MHz
			if freqInt < 5357 {
				spot.Mode = "CW"
			} else if freqInt < 5359 {
				spot.Mode = "FT8"
			} else {
				spot.Mode = "USB"
			}

		case "40M": // 7.000 - 7.300 MHz
			if freqInt < 7040 {
				spot.Mode = "CW"
			} else if freqInt < 7047 {
				spot.Mode = "RTTY"
			} else if freqInt < 7050 {
				spot.Mode = "FT4"
			} else if freqInt < 7080 {
				spot.Mode = "FT8" // ✅ 7.056 = FT8
			} else if freqInt < 7125 {
				spot.Mode = "RTTY" // ✅ 7.112 = RTTY
			} else {
				spot.Mode = "LSB"
			}

		case "30M": // 10.100 - 10.150 MHz (CW/Digital seulement)
			if freqInt < 10130 {
				spot.Mode = "CW"
			} else if freqInt < 10142 {
				spot.Mode = "FT8"
			} else {
				spot.Mode = "FT4"
			}

		case "20M": // 14.000 - 14.350 MHz
			if freqInt < 14070 {
				spot.Mode = "CW"
			} else if freqInt < 14078 {
				spot.Mode = "FT8"
			} else if freqInt < 14083 {
				spot.Mode = "FT4"
			} else if freqInt < 14095 {
				spot.Mode = "FT8"
			} else if freqInt < 14112 {
				spot.Mode = "RTTY"
			} else {
				spot.Mode = "USB"
			}

		case "17M": // 18.068 - 18.168 MHz
			if freqInt < 18090 {
				spot.Mode = "CW"
			} else if freqInt < 18104 {
				spot.Mode = "FT8"
			} else if freqInt < 18106 {
				spot.Mode = "FT4"
			} else if freqInt < 18110 {
				spot.Mode = "RTTY"
			} else {
				spot.Mode = "USB"
			}

		case "15M": // 21.000 - 21.450 MHz
			if freqInt < 21070 {
				spot.Mode = "CW"
			} else if freqInt < 21078 {
				spot.Mode = "FT8"
			} else if freqInt < 21120 {
				spot.Mode = "RTTY"
			} else if freqInt < 21143 {
				spot.Mode = "FT4"
			} else {
				spot.Mode = "USB"
			}

		case "12M": // 24.890 - 24.990 MHz
			if freqInt < 24910 {
				spot.Mode = "CW" // ✅ 24.896 = CW
			} else if freqInt < 24918 {
				spot.Mode = "FT8"
			} else if freqInt < 24922 {
				spot.Mode = "FT4"
			} else if freqInt < 24930 {
				spot.Mode = "RTTY"
			} else {
				spot.Mode = "USB"
			}

		case "10M": // 28.000 - 29.700 MHz
			if freqInt < 28070 {
				spot.Mode = "CW"
			} else if freqInt < 28095 {
				spot.Mode = "FT8"
			} else if freqInt < 28179 {
				spot.Mode = "RTTY"
			} else if freqInt < 28190 {
				spot.Mode = "FT4"
			} else if freqInt < 29000 {
				spot.Mode = "USB"
			} else {
				spot.Mode = "FM"
			}

		case "6M": // 50.000 - 54.000 MHz
			if freqInt < 50100 {
				spot.Mode = "CW"
			} else if freqInt < 50313 {
				spot.Mode = "USB" // ✅ DX Window + général
			} else if freqInt < 50318 {
				spot.Mode = "FT8" // ✅ 50.313-50.318
			} else if freqInt < 50323 {
				spot.Mode = "FT4" // ✅ 50.318-50.323
			} else if freqInt < 51000 {
				spot.Mode = "USB" // ✅ Retour à USB
			} else {
				spot.Mode = "FM"
			}

		default:
			// ✅ Bande inconnue
			if freqInt < 10.0 {
				spot.Mode = "LSB"
			} else {
				spot.Mode = "USB"
			}
		}

		if spot.Mode != "" {
			Log.Debugf("✅ Guessed mode %s for %s on %s MHz (band %s)", spot.Mode, spot.DX, spot.Frequency, spot.Band)
		} else {
			Log.Warnf("❌ Could not guess mode for %s on %s MHz (band %s), raw spot: %s", spot.DX, spot.Frequency, spot.Band, rawSpot)
		}
	} else {
		spot.Mode = strings.ToUpper(spot.Mode)
	}
}

// ✅ Extraire le mode depuis le commentaire
func extractModeFromComment(comment string) string {
	commentUpper := strings.ToUpper(comment)

	// ✅ 1. Détecter FT8/FT4 avec leurs patterns typiques (dB + Hz)
	if strings.Contains(commentUpper, "FT8") ||
		(strings.Contains(commentUpper, "DB") && strings.Contains(commentUpper, "HZ")) {
		return "FT8"
	}

	if strings.Contains(commentUpper, "FT4") {
		return "FT4"
	}

	// ✅ 2. Détecter CW avec WPM (Words Per Minute)
	if strings.Contains(commentUpper, "WPM") || strings.Contains(commentUpper, " CW ") ||
		strings.HasSuffix(commentUpper, "CW") || strings.HasPrefix(commentUpper, "CW ") {
		return "CW"
	}

	// ✅ 3. Autres modes digitaux
	digitalModes := []string{"RTTY", "PSK31", "PSK63", "PSK", "MFSK", "OLIVIA", "CONTESTIA", "JT65", "JT9"}
	for _, mode := range digitalModes {
		if strings.Contains(commentUpper, mode) {
			return mode
		}
	}

	// ✅ 4. Modes voice
	voiceModes := []string{"USB", "LSB", "SSB", "FM", "AM"}
	for _, mode := range voiceModes {
		// Chercher le mode comme mot complet (pas dans "SSBC" par exemple)
		if strings.Contains(commentUpper, " "+mode+" ") ||
			strings.HasPrefix(commentUpper, mode+" ") ||
			strings.HasSuffix(commentUpper, " "+mode) ||
			commentUpper == mode {
			return mode
		}
	}

	return ""
}

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

// var spotNumber = 1

func ProcessTelnetSpot(re *regexp.Regexp, spotRaw string, SpotChanToFlex chan TelnetSpot, SpotChanToHTTPServer chan TelnetSpot, Countries Countries, contactRepo *Log4OMContactsRepository) {

	match := re.FindStringSubmatch(spotRaw)

	if len(match) != 0 {
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
			Log.Errorf("Could not identify the DXCC for %s", spot.DX)
			return
		}

		spot.GetBand()
		spot.GuessMode()
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

		// Send spots to FlexRadio
		SpotChanToFlex <- spot

		if spot.NewDXCC {
			Log.Debugf("(** New DXCC **) DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}

		if !spot.NewDXCC && spot.NewBand && spot.NewMode {
			Log.Debugf("(** New Band/Mode **) DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}

		if !spot.NewDXCC && spot.NewBand && !spot.NewMode {
			Log.Debugf("(** New Band **) DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}

		if !spot.NewDXCC && !spot.NewBand && spot.NewMode && spot.Mode != "" {
			Log.Debugf("(** New Mode **) DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}

		if !spot.NewDXCC && !spot.NewBand && !spot.NewMode && spot.NewSlot && spot.Mode != "" {
			Log.Debugf("(** New Slot **) DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}

		if !spot.NewDXCC && !spot.NewBand && !spot.NewMode && spot.CallsignWorked {
			Log.Debugf("(** Worked **) DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}

		if !spot.NewDXCC && !spot.NewBand && !spot.NewMode && !spot.CallsignWorked {
			Log.Debugf("DX: %s - Spotter: %s - Freq: %s - Band: %s - Mode: %s - Comment: %s - Time: %s - DXCC: %s",
				spot.DX, spot.Spotter, spot.Frequency, spot.Band, spot.Mode, spot.Comment, spot.Time, spot.DXCC)
		}
	} else {
		// Log.Infof("Could not decode: %s", strings.Trim(spotRaw, "\n"))
	}

	// Log.Infof("Spots Processed: %v", spotNumber)
	// spotNumber++
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

func (spot *TelnetSpot) GuessMode() {
	if spot.Mode == "" {
		freqInt, err := strconv.ParseFloat(spot.Frequency, 32)
		if err != nil {
			Log.Errorf("could not convert frequency string in float64:", err)
		}

		switch spot.Band {
		case "160M":
			if freqInt >= 1800 && freqInt <= 1840 {
				spot.Mode = "CW"
			}
			if freqInt >= 1840 && freqInt <= 1844 {
				spot.Mode = "FT8"
			}
		case "80M":
			if freqInt >= 3500 && freqInt < 3568 {
				spot.Mode = "CW"
			}
			if freqInt >= 3568 && freqInt < 3573 {
				spot.Mode = "FT4"
			}
			if freqInt >= 3573 && freqInt < 3580 {
				spot.Mode = "FT8"
			}
			if freqInt >= 3580 && freqInt < 3600 {
				spot.Mode = "CW"
			}
			if freqInt >= 3600 && freqInt <= 3800 {
				spot.Mode = "LSB"
			}
		case "60M":
			if freqInt >= 5351.5 && freqInt < 5354 {
				spot.Mode = "CW"
			}
			if freqInt >= 5354 && freqInt < 5366 {
				spot.Mode = "LSB"
			}
			if freqInt >= 5366 && freqInt <= 5266.5 {
				spot.Mode = "FT8"
			}
		case "40M":
			if freqInt >= 7000 && freqInt < 7045.5 {
				spot.Mode = "CW"
			}
			if freqInt >= 7045.5 && freqInt < 7048.5 {
				spot.Mode = "FT4"
			}
			if freqInt >= 7048.5 && freqInt < 7074 {
				spot.Mode = "CW"
			}
			if freqInt >= 7074 && freqInt < 7078 {
				spot.Mode = "FT8"
			}
			if freqInt >= 7078 && freqInt <= 7300 {
				spot.Mode = "LSB"
			}
		case "30M":
			if freqInt >= 10100 && freqInt < 10130 {
				spot.Mode = "CW"
			}
			if freqInt >= 10130 && freqInt < 10140 {
				spot.Mode = "FT8"
			}
			if freqInt >= 10140 && freqInt <= 10150 {
				spot.Mode = "FT4"
			}
		case "20M":
			if freqInt >= 14000 && freqInt < 14074 {
				spot.Mode = "CW"
			}
			if freqInt >= 14074 && freqInt < 14078 {
				spot.Mode = "FT8"
			}
			if freqInt >= 14074 && freqInt < 14078 {
				spot.Mode = "FT8"
			}
			if freqInt >= 14078 && freqInt < 14083 {
				spot.Mode = "FT4"
			}
			if freqInt >= 14083 && freqInt < 14119 {
				spot.Mode = "FT8"
			}
			if freqInt >= 14119 && freqInt < 14350 {
				spot.Mode = "USB"
			}

		case "17M":
			if freqInt >= 18068 && freqInt < 18095 {
				spot.Mode = "CW"
			}
			if freqInt >= 18095 && freqInt < 18104 {
				spot.Mode = "FT8"
			}
			if freqInt >= 18104 && freqInt < 18108 {
				spot.Mode = "FT4"
			}
			if freqInt >= 18108 && freqInt <= 18168 {
				spot.Mode = "USB"
			}

		case "15M":
			if freqInt >= 21000 && freqInt < 21074 {
				spot.Mode = "CW"
			}
			if freqInt >= 21074 && freqInt < 21100 {
				spot.Mode = "FT8"
			}
			if freqInt >= 21100 && freqInt < 21140 {
				spot.Mode = "RTTY"
			}
			if freqInt >= 21140 && freqInt < 21144 {
				spot.Mode = "FT4"
			}
			if freqInt >= 21144 && freqInt <= 21450 {
				spot.Mode = "USB"
			}

		case "12M":
			if freqInt >= 24890 && freqInt < 24915 {
				spot.Mode = "CW"
			}
			if freqInt >= 24915 && freqInt < 24919 {
				spot.Mode = "FT8"
			}
			if freqInt >= 24919 && freqInt < 24930 {
				spot.Mode = "CW"
			}
			if freqInt >= 24930 && freqInt <= 24990 {
				spot.Mode = "USB"
			}

		case "10M":
			if freqInt >= 28000 && freqInt < 28074 {
				spot.Mode = "CW"
			}
			if freqInt >= 28074 && freqInt < 28080 {
				spot.Mode = "FT8"
			}
			if freqInt >= 28080 && freqInt < 28100 {
				spot.Mode = "RTTY"
			}
			if freqInt >= 28100 && freqInt < 28300 {
				spot.Mode = "CW"
			}
			if freqInt >= 28300 && freqInt < 29000 {
				spot.Mode = "USB"
			}
			if freqInt >= 29000 && freqInt <= 29700 {
				spot.Mode = "FM"
			}
		case "6M":
			if freqInt >= 50000 && freqInt < 50100 {
				spot.Mode = "CW"
			}
			if freqInt >= 50100 && freqInt < 50313 {
				spot.Mode = "USB"
			}
			if freqInt >= 50313 && freqInt < 50320 {
				spot.Mode = "FT8"
			}
			if freqInt >= 50320 && freqInt < 50400 {
				spot.Mode = "USB"
			}
			if freqInt >= 50400 && freqInt < +52000 {
				spot.Mode = "FM"
			}
		}
	}

	if spot.Mode == "" {
		Log.Errorf("Could not identify mode for %s on %s", spot.DX, spot.Frequency)
	}
}

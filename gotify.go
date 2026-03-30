package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	toast "gopkg.in/toast.v1"
)

// toastThrottle évite les notifications répétées pour le même callsign
var (
	toastLastSent = make(map[string]time.Time)
	toastMutex    sync.Mutex
	toastCooldown = 5 * time.Minute
)

type GotifyMessage struct {
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
}

func Gotify(spot FlexSpot) {

	ExceptionList := "4U1UN 5Z4B OH2B CS3B"

	if Cfg.Gotify.Enable && !strings.Contains(ExceptionList, spot.DX) {

		message := fmt.Sprintf("DX: %s\nFrom: %s\nFreq: %s\nMode: %s\nCountry: %s\nTime: %s\n", spot.DX, spot.SpotterCallsign, spot.FrequencyMhz, spot.Mode, spot.CountryName, spot.UTCTime)

		gotifyMsg := GotifyMessage{
			Title:    "",
			Message:  message,
			Priority: 10,
		}

		if spot.NewDXCC && Cfg.Gotify.NewDXCC {
			title := "🆕 FlexDXCluster - New DXCC"
			gotifyMsg.Title = title
			gotifyMsg.Message = message
			sendToGotify(gotifyMsg)
			return
		}

		// Notification pour callsign dans watchlist
		if spot.InWatchlist && !spot.Worked {
			title := "🎯 FlexDXCluster - Watchlist Alert"
			gotifyMsg.Title = title
			gotifyMsg.Message = message
			sendToGotify(gotifyMsg)
			return
		}

		// Autres notifications (conservées pour compatibilité, mais ne seront plus appelées normalement)
		if spot.NewBand && spot.NewMode && Cfg.Gotify.NewBandAndMode {
			title := "📻 FlexDXCluster - New Mode & Band"
			gotifyMsg.Title = title
			gotifyMsg.Message = message
			sendToGotify(gotifyMsg)
		}

		if spot.NewMode && Cfg.Gotify.NewMode && !spot.NewBand {
			title := "🔧 FlexDXCluster - New Mode"
			gotifyMsg.Title = title
			gotifyMsg.Message = message
			sendToGotify(gotifyMsg)
		}

		if spot.NewBand && Cfg.Gotify.NewBand && !spot.NewMode {
			title := "📡 FlexDXCluster - New Band"
			gotifyMsg.Title = title
			gotifyMsg.Message = message
			sendToGotify(gotifyMsg)
		}
	}
}

func sendToGotify(mess GotifyMessage) {
	jsonData, err := json.Marshal(mess)
	if err != nil {
		Log.Errorln("Error marshaling JSON:", err)
		return
	}

	req, err := http.NewRequest("POST", Cfg.Gotify.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		Log.Errorln("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+Cfg.Gotify.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		Log.Errorln("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Log.Errorln("Gotify server returned non-OK status:", resp.Status)
	} else {
		Log.Debugln("Push successfully sent to Gotify")
	}
}

// WindowsToast envoie une notification Windows native (toast)
// Uniquement pour les callsigns de la watchlist avec notifications activées
// Un cooldown de 5 minutes évite les notifications répétées pour le même callsign
func WindowsToast(spot FlexSpot) {
	// Throttle — un seul toast par callsign toutes les 5 minutes
	toastMutex.Lock()
	last, seen := toastLastSent[spot.DX]
	if seen && time.Since(last) < toastCooldown {
		toastMutex.Unlock()
		Log.Debugf("Windows toast throttled for %s (cooldown)", spot.DX)
		return
	}
	toastLastSent[spot.DX] = time.Now()
	toastMutex.Unlock()

	freq := spot.FrequencyMhz
	if freq == "" {
		freq = "?"
	}

	title := fmt.Sprintf("FlexDXCluster - %s (%s)", spot.DX, spot.CountryName)
	msg := fmt.Sprintf("Freq: %s  Mode: %s\nSpotter: %s  Time: %s", freq, spot.Mode, spot.SpotterCallsign, spot.UTCTime)

	tuneURL := fmt.Sprintf("http://localhost:8080/api/tune?callsign=%s&freq=%s&mode=%s",
		spot.DX, spot.FrequencyMhz, spot.Mode)

	notification := toast.Notification{
		AppID:               "FlexDXCluster",
		Title:               title,
		Message:             msg,
		Audio:               toast.Default,
		ActivationArguments: tuneURL,
	}

	if err := notification.Push(); err != nil {
		Log.Warnf("Windows toast failed for %s: %v", spot.DX, err)
	} else {
		Log.Infof("Windows toast sent for %s", spot.DX)
	}
}

//go:build windows

package main

import (
	"fmt"
	"time"

	toast "gopkg.in/toast.v1"
)

func WindowsToast(spot FlexSpot) {
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

package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const lotwCSVURL = "https://lotw.arrl.org/lotw-user-activity.csv"

var (
	lotwUsers   = make(map[string]bool)
	lotwMu      sync.RWMutex
	lotwReady   bool
	lotwCount   int
)

// IsLoTWUser returns true if the callsign (or its base callsign) is a LoTW user.
func IsLoTWUser(callsign string) bool {
	if !lotwReady {
		return false
	}
	lotwMu.RLock()
	defer lotwMu.RUnlock()

	upper := strings.ToUpper(strings.TrimSpace(callsign))

	// Direct match
	if lotwUsers[upper] {
		return true
	}

	// Handle portable suffixes: F4BPO/P → check F4BPO
	// Handle portable prefixes: VK9/W4ABC → check W4ABC and VK9
	if strings.Contains(upper, "/") {
		parts := strings.Split(upper, "/")
		for _, p := range parts {
			if lotwUsers[p] {
				return true
			}
		}
	}

	return false
}

// LoadLoTWUsers downloads the LoTW user activity CSV and populates the in-memory map.
// Runs in background; sets lotwReady when done.
func LoadLoTWUsers() {
	Log.Info("LoTW: downloading user list...")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(lotwCSVURL)
	if err != nil {
		Log.Errorf("LoTW: download failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Log.Errorf("LoTW: HTTP %d", resp.StatusCode)
		return
	}

	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1 // variable number of fields
	r.LazyQuotes = true

	newMap := make(map[string]bool, 150000)
	count := 0
	first := true

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			Log.Warnf("LoTW: CSV parse error: %v", err)
			continue
		}
		if len(record) == 0 {
			continue
		}

		callsign := strings.ToUpper(strings.TrimSpace(record[0]))

		// Skip header row
		if first {
			first = false
			if callsign == "CALLSIGN" || strings.HasPrefix(callsign, "CALL") {
				continue
			}
		}

		if callsign == "" {
			continue
		}

		newMap[callsign] = true
		count++
	}

	lotwMu.Lock()
	lotwUsers = newMap
	lotwCount = count
	lotwReady = true
	lotwMu.Unlock()

	Log.Infof("LoTW: loaded %s users", formatCount(count))
}

func formatCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}

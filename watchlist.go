package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

type Watchlist struct {
	Callsigns []string `json:"callsigns"`
	mu        sync.RWMutex
	filename  string
}

func NewWatchlist(filename string) *Watchlist {
	wl := &Watchlist{
		Callsigns: []string{},
		filename:  filename,
	}
	wl.Load()
	return wl
}

func (wl *Watchlist) Load() error {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	data, err := os.ReadFile(wl.filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty watchlist
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &wl.Callsigns)
}

// save est une méthode privée sans lock (appelée par Add/Remove qui ont déjà le lock)
func (wl *Watchlist) save() error {
	data, err := json.Marshal(wl.Callsigns)
	if err != nil {
		return err
	}

	return os.WriteFile(wl.filename, data, 0644)
}

func (wl *Watchlist) Add(callsign string) error {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	// Check if already exists
	for _, c := range wl.Callsigns {
		if c == callsign {
			return nil // Already in watchlist
		}
	}

	wl.Callsigns = append(wl.Callsigns, callsign)
	return wl.save() // ← Appel de save() minuscule (sans lock)
}

func (wl *Watchlist) Remove(callsign string) error {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	for i, c := range wl.Callsigns {
		if c == callsign {
			wl.Callsigns = append(wl.Callsigns[:i], wl.Callsigns[i+1:]...)
			return wl.save() // ← Appel de save() minuscule (sans lock)
		}
	}

	return nil
}

func (wl *Watchlist) GetAll() []string {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	result := make([]string, len(wl.Callsigns))
	copy(result, wl.Callsigns)
	return result
}

func (wl *Watchlist) Matches(callsign string) bool {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	callsign = strings.ToUpper(callsign)

	for _, pattern := range wl.Callsigns {
		// Exact match
		if callsign == pattern {
			return true
		}

		// Prefix match (e.g., VK9 matches VK9XX)
		if strings.HasPrefix(callsign, pattern) {
			return true
		}
	}

	return false
}

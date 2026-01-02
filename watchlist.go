package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type WatchlistEntry struct {
	Callsign      string       `json:"callsign"`
	LastSeen      time.Time    `json:"lastSeen"`
	LastSeenStr   string       `json:"lastSeenStr"`
	AddedAt       time.Time    `json:"addedAt"`
	SpotCount     int          `json:"spotCount"`
	PlaySound     bool         `json:"playSound"`
	IsContest     bool         `json:"isContest"`
	ActiveSpotIDs map[int]bool `json:"-"`
}

type Watchlist struct {
	entries  map[string]*WatchlistEntry
	filePath string
	mutex    sync.RWMutex
}

func NewWatchlist(filePath string) *Watchlist {
	w := &Watchlist{
		entries:  make(map[string]*WatchlistEntry),
		filePath: filePath,
	}
	w.load()
	return w
}

func (w *Watchlist) load() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	data, err := os.ReadFile(w.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			Log.Errorf("Error reading watchlist file: %v", err)
		} else {
			Log.Debug("Watchlist file does not exist yet, will be created on first save")
		}
		return
	}

	var entries []WatchlistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		Log.Errorf("Error parsing watchlist file: %v", err)
		return
	}

	for i := range entries {
		entry := &entries[i]
		// Initialize ActiveSpotIDs map
		entry.ActiveSpotIDs = make(map[int]bool)
		if !entry.LastSeen.IsZero() {
			entry.LastSeenStr = formatLastSeen(entry.LastSeen)
		} else {
			entry.LastSeenStr = "Never"
		}
		w.entries[entry.Callsign] = entry
	}

	Log.Infof("Loaded %d entries from watchlist", len(w.entries))
}

func (w *Watchlist) saveUnsafe() error {
	entries := make([]WatchlistEntry, 0, len(w.entries))
	for _, entry := range w.entries {
		entries = append(entries, *entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		Log.Errorf("Error marshaling watchlist: %v", err)
		return fmt.Errorf("error marshaling watchlist: %w", err)
	}

	if err := os.WriteFile(w.filePath, data, 0644); err != nil {
		Log.Errorf("Error writing watchlist file %s: %v", w.filePath, err)
		return fmt.Errorf("error writing watchlist file: %w", err)
	}

	Log.Debugf("Watchlist saved successfully (%d entries)", len(entries))
	return nil
}

func (w *Watchlist) save() error {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.saveUnsafe()
}

func (w *Watchlist) Add(callsign string) error {
	return w.addWithContestFlag(callsign, false)
}

func (w *Watchlist) AddContest(callsign string) error {
	return w.addWithContestFlag(callsign, true)
}

func (w *Watchlist) addWithContestFlag(callsign string, isContest bool) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	Log.Debugf("Attempting to add callsign: %s (contest: %v)", callsign, isContest)

	if callsign == "" {
		Log.Warn("Attempted to add empty callsign")
		return fmt.Errorf("callsign cannot be empty")
	}

	if _, exists := w.entries[callsign]; exists {
		Log.Warnf("Callsign %s already exists in watchlist", callsign)
		return fmt.Errorf("callsign already in watchlist")
	}

	w.entries[callsign] = &WatchlistEntry{
		Callsign:      callsign,
		AddedAt:       time.Now(),
		LastSeen:      time.Time{},
		LastSeenStr:   "Never",
		SpotCount:     0,
		PlaySound:     true,
		IsContest:     isContest,
		ActiveSpotIDs: make(map[int]bool),
	}

	Log.Infof("Added %s to watchlist (contest: %v)", callsign, isContest)

	if err := w.saveUnsafe(); err != nil {
		Log.Errorf("Failed to save watchlist after adding %s: %v", callsign, err)
		return err
	}

	Log.Debugf("Watchlist saved successfully after adding %s", callsign)
	return nil
}

func (w *Watchlist) Remove(callsign string) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	if _, exists := w.entries[callsign]; !exists {
		Log.Warnf("Attempted to remove non-existent callsign: %s", callsign)
		return fmt.Errorf("callsign not in watchlist")
	}

	delete(w.entries, callsign)
	Log.Infof("Removed %s from watchlist", callsign)

	if err := w.saveUnsafe(); err != nil {
		Log.Errorf("Failed to save watchlist after removing %s: %v", callsign, err)
		return err
	}

	return nil
}

func (w *Watchlist) UpdateSound(callsign string, playSound bool) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	entry, exists := w.entries[callsign]
	if !exists {
		Log.Warnf("Attempted to update sound for non-existent callsign: %s", callsign)
		return fmt.Errorf("callsign not in watchlist")
	}

	entry.PlaySound = playSound
	Log.Debugf("Updated sound setting for %s to %v", callsign, playSound)

	if err := w.saveUnsafe(); err != nil {
		Log.Errorf("Failed to save watchlist after updating sound for %s: %v", callsign, err)
		return err
	}

	return nil
}

// AddActiveSpot marks a spot as active for a watchlist entry
func (w *Watchlist) AddActiveSpot(callsign string, spotID int) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	entry, exists := w.entries[callsign]
	if !exists {
		// Check if any entry matches as a prefix
		for pattern, e := range w.entries {
			if callsign == pattern || strings.HasPrefix(callsign, pattern) {
				entry = e
				break
			}
		}
		if entry == nil {
			return
		}
	}

	if entry.ActiveSpotIDs == nil {
		entry.ActiveSpotIDs = make(map[int]bool)
	}

	entry.ActiveSpotIDs[spotID] = true
	Log.Debugf("Added active spot ID %d for watchlist entry %s", spotID, entry.Callsign)
}

// RemoveActiveSpot removes a spot from the active list for a watchlist entry
func (w *Watchlist) RemoveActiveSpot(spotID int) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Search all entries for this spot ID
	for _, entry := range w.entries {
		if entry.ActiveSpotIDs != nil {
			if _, exists := entry.ActiveSpotIDs[spotID]; exists {
				delete(entry.ActiveSpotIDs, spotID)
				Log.Debugf("Removed active spot ID %d from watchlist entry %s", spotID, entry.Callsign)
			}
		}
	}
}

// RemoveActiveSpotByFlexNumber removes a spot by its Flex spot number
func (w *Watchlist) RemoveActiveSpotByFlexNumber(flexSpotNumber string) {
	// This needs to be coordinated with the FlexRepo to get the actual spot ID
	// For now, we'll need the httpServer to handle this coordination
	Log.Debugf("Request to remove spot with Flex number %s from watchlist", flexSpotNumber)
}

// HasActiveSpots returns true if the entry has any active spots
func (w *Watchlist) HasActiveSpots(callsign string) bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	callsign = strings.ToUpper(callsign)

	for pattern, entry := range w.entries {
		if callsign == pattern || strings.HasPrefix(callsign, pattern) {
			return entry.ActiveSpotIDs != nil && len(entry.ActiveSpotIDs) > 0
		}
	}

	return false
}

// GetActiveSpotCount returns the number of active spots for a callsign
func (w *Watchlist) GetActiveSpotCount(callsign string) int {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	callsign = strings.ToUpper(callsign)

	for pattern, entry := range w.entries {
		if callsign == pattern || strings.HasPrefix(callsign, pattern) {
			if entry.ActiveSpotIDs != nil {
				return len(entry.ActiveSpotIDs)
			}
			return 0
		}
	}

	return 0
}

func (w *Watchlist) MarkSeen(callsign string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	callsign = strings.ToUpper(strings.TrimSpace(callsign))

	entry, exists := w.entries[callsign]
	if !exists {
		// Check if any entry matches as a prefix
		for pattern, e := range w.entries {
			if callsign == pattern || strings.HasPrefix(callsign, pattern) {
				entry = e
				break
			}
		}
		if entry == nil {
			return
		}
	}

	entry.LastSeen = time.Now()
	entry.LastSeenStr = formatLastSeen(entry.LastSeen)
	entry.SpotCount++

	Log.Debugf("Marked %s as seen (count: %d)", entry.Callsign, entry.SpotCount)
}

func (w *Watchlist) Matches(callsign string) bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	callsign = strings.ToUpper(callsign)

	for pattern := range w.entries {
		if callsign == pattern || strings.HasPrefix(callsign, pattern) {
			return true
		}
	}

	return false
}

func (w *Watchlist) GetEntry(callsign string) *WatchlistEntry {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	callsign = strings.ToUpper(callsign)

	for pattern, entry := range w.entries {
		if callsign == pattern || strings.HasPrefix(callsign, pattern) {
			entryCopy := *entry
			return &entryCopy
		}
	}

	return nil
}

// GetAllWithActiveStatus returns all watchlist entries with their active spot status
func (w *Watchlist) GetAllWithActiveStatus() []map[string]interface{} {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	entries := make([]map[string]interface{}, 0, len(w.entries))
	for _, entry := range w.entries {
		entryMap := map[string]interface{}{
			"callsign":       entry.Callsign,
			"lastSeen":       entry.LastSeen,
			"lastSeenStr":    entry.LastSeenStr,
			"addedAt":        entry.AddedAt,
			"spotCount":      entry.SpotCount,
			"playSound":      entry.PlaySound,
			"hasActiveSpots": len(entry.ActiveSpotIDs) > 0,
			"activeCount":    len(entry.ActiveSpotIDs),
		}

		if !entry.LastSeen.IsZero() {
			entryMap["lastSeenStr"] = formatLastSeen(entry.LastSeen)
		}

		entries = append(entries, entryMap)
	}

	return entries
}

func (w *Watchlist) GetAll() []WatchlistEntry {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	entries := make([]WatchlistEntry, 0, len(w.entries))
	for _, entry := range w.entries {
		entryCopy := *entry
		if !entryCopy.LastSeen.IsZero() {
			entryCopy.LastSeenStr = formatLastSeen(entryCopy.LastSeen)
		}
		entries = append(entries, entryCopy)
	}

	return entries
}

func (w *Watchlist) GetAllCallsigns() []string {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	callsigns := make([]string, 0, len(w.entries))
	for callsign := range w.entries {
		callsigns = append(callsigns, callsign)
	}

	return callsigns
}

func formatLastSeen(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	duration := time.Since(t)

	if duration < time.Minute {
		return "Just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

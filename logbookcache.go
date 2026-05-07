package main

import (
	"context"
	"strings"
	"sync"
	"time"
)

// LogbookCache holds an in-memory snapshot of the logbook indexed for O(1) lookups.
// It replaces per-spot DB queries in ProcessTelnetSpot, eliminating the main bottleneck.
type LogbookCache struct {
	mu           sync.RWMutex
	dxcc         map[string]bool
	dxccMode     map[string]bool
	dxccBand     map[string]bool
	dxccBandMode map[string]bool
	callBandMode map[string]bool
	// confirmed maps — only contacts with LoTWConfirmed=true
	confDXCC       map[string]bool
	confDXCCMode   map[string]bool
	confDXCCBand   map[string]bool
	confDXCCBandMode map[string]bool
}

var globalLogbookCache *LogbookCache

func NewLogbookCache(repo LogbookProvider) *LogbookCache {
	c := &LogbookCache{}
	c.rebuild(repo)
	return c
}

func (c *LogbookCache) rebuild(repo LogbookProvider) {
	contacts := repo.ListAll()

	dxcc := make(map[string]bool, 512)
	dxccMode := make(map[string]bool, 512)
	dxccBand := make(map[string]bool, 512)
	dxccBandMode := make(map[string]bool, 512)
	callBandMode := make(map[string]bool, len(contacts))
	confDXCC := make(map[string]bool, 128)
	confDXCCMode := make(map[string]bool, 256)
	confDXCCBand := make(map[string]bool, 256)
	confDXCCBandMode := make(map[string]bool, 512)

	for _, ct := range contacts {
		mode := cacheNormalizeMode(ct.Mode)
		band := strings.ToUpper(ct.Band)
		dxcc[ct.DXCC] = true
		dxccMode[ct.DXCC+"|"+mode] = true
		dxccBand[ct.DXCC+"|"+band] = true
		dxccBandMode[ct.DXCC+"|"+band+"|"+mode] = true
		callBandMode[ct.Callsign+"|"+band+"|"+mode] = true
		if ct.LoTWConfirmed {
			confDXCC[ct.DXCC] = true
			confDXCCMode[ct.DXCC+"|"+mode] = true
			confDXCCBand[ct.DXCC+"|"+band] = true
			confDXCCBandMode[ct.DXCC+"|"+band+"|"+mode] = true
		}
	}

	c.mu.Lock()
	c.dxcc = dxcc
	c.dxccMode = dxccMode
	c.dxccBand = dxccBand
	c.dxccBandMode = dxccBandMode
	c.callBandMode = callBandMode
	c.confDXCC = confDXCC
	c.confDXCCMode = confDXCCMode
	c.confDXCCBand = confDXCCBand
	c.confDXCCBandMode = confDXCCBandMode
	c.mu.Unlock()

	Log.Infof("LogbookCache: refreshed with %d contacts", len(contacts))
}

// StartAutoRefresh rebuilds the cache on the given interval in a background goroutine.
func (c *LogbookCache) StartAutoRefresh(ctx context.Context, repo LogbookProvider, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.rebuild(repo)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (c *LogbookCache) HasDXCC(dxcc string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dxcc[dxcc]
}

func (c *LogbookCache) HasDXCCMode(dxcc, mode string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dxccMode[dxcc+"|"+cacheNormalizeMode(mode)]
}

func (c *LogbookCache) HasDXCCBand(dxcc, band string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dxccBand[dxcc+"|"+band]
}

func (c *LogbookCache) HasDXCCBandMode(dxcc, band, mode string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dxccBandMode[dxcc+"|"+band+"|"+cacheNormalizeMode(mode)]
}

func (c *LogbookCache) HasCallBandMode(call, band, mode string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.callBandMode[call+"|"+band+"|"+cacheNormalizeMode(mode)]
}

func (c *LogbookCache) IsConfirmedDXCC(dxcc string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.confDXCC[dxcc]
}

func (c *LogbookCache) IsConfirmedDXCCBand(dxcc, band string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.confDXCCBand[dxcc+"|"+strings.ToUpper(band)]
}

func (c *LogbookCache) IsConfirmedDXCCMode(dxcc, mode string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.confDXCCMode[dxcc+"|"+cacheNormalizeMode(mode)]
}

func (c *LogbookCache) IsConfirmedDXCCBandMode(dxcc, band, mode string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.confDXCCBandMode[dxcc+"|"+strings.ToUpper(band)+"|"+cacheNormalizeMode(mode)]
}

// cacheNormalizeMode collapses USB/LSB/SSB into a single key so the cache
// matches the same equivalence that buildSSBModeCondition uses in DB queries.
func cacheNormalizeMode(mode string) string {
	switch strings.ToUpper(mode) {
	case "USB", "LSB", "SSB":
		return "SSB"
	default:
		return strings.ToUpper(mode)
	}
}

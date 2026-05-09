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
	// per-source confirmation counts
	lotwDXCC map[string]bool
	qslDXCC  map[string]bool
}

var globalLogbookCache *LogbookCache

func NewLogbookCache(repo LogbookProvider) *LogbookCache {
	c := &LogbookCache{}
	c.Rebuild(repo)
	return c
}

func (c *LogbookCache) Rebuild(repo LogbookProvider) {
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
	lotwDXCC := make(map[string]bool, 128)
	qslDXCC := make(map[string]bool, 128)

	for _, ct := range contacts {
		mode := cacheNormalizeMode(ct.Mode)
		band := strings.ToUpper(ct.Band)

		// Index using Log4OM's stored DXCC code.
		indexContact(ct.DXCC, band, mode, ct.Callsign, ct.LoTWConfirmed,
			dxcc, dxccMode, dxccBand, dxccBandMode, callBandMode,
			confDXCC, confDXCCMode, confDXCCBand, confDXCCBandMode)
		if ct.LoTWQSL {
			lotwDXCC[ct.DXCC] = true
		}
		if ct.QSLCard {
			qslDXCC[ct.DXCC] = true
		}

		// Only cross-index via cty.plist when Log4OM stored DXCC as "0" or empty —
		// typical for newer entities (e.g. Kosovo) added after an older Log4OM install.
		// Calling GetDXCC for every contact would be O(contacts × prefixes) and adds
		// several seconds of startup delay for large logs.
		if ct.DXCC == "0" || ct.DXCC == "" {
			if ctyInfo := GetDXCC(ct.Callsign); ctyInfo.DXCC != "" {
				indexContact(ctyInfo.DXCC, band, mode, ct.Callsign, ct.LoTWConfirmed,
					dxcc, dxccMode, dxccBand, dxccBandMode, callBandMode,
					confDXCC, confDXCCMode, confDXCCBand, confDXCCBandMode)
				if ct.LoTWQSL {
					lotwDXCC[ctyInfo.DXCC] = true
				}
				if ct.QSLCard {
					qslDXCC[ctyInfo.DXCC] = true
				}
			}
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
	c.lotwDXCC = lotwDXCC
	c.qslDXCC = qslDXCC
	c.mu.Unlock()

	Log.Infof("LogbookCache: refreshed with %d contacts", len(contacts))
}

func indexContact(
	dxccKey, band, mode, callsign string, confirmed bool,
	dxcc, dxccMode, dxccBand, dxccBandMode, callBandMode map[string]bool,
	confDXCC, confDXCCMode, confDXCCBand, confDXCCBandMode map[string]bool,
) {
	dxcc[dxccKey] = true
	dxccMode[dxccKey+"|"+mode] = true
	dxccBand[dxccKey+"|"+band] = true
	dxccBandMode[dxccKey+"|"+band+"|"+mode] = true
	callBandMode[callsign+"|"+band+"|"+mode] = true
	if confirmed {
		confDXCC[dxccKey] = true
		confDXCCMode[dxccKey+"|"+mode] = true
		confDXCCBand[dxccKey+"|"+band] = true
		confDXCCBandMode[dxccKey+"|"+band+"|"+mode] = true
	}
}

// StartAutoRefresh rebuilds the cache on the given interval in a background goroutine.
func (c *LogbookCache) StartAutoRefresh(ctx context.Context, repo LogbookProvider, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Rebuild(repo)
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

func (c *LogbookCache) DXCCWorkedCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.dxcc)
}

func (c *LogbookCache) DXCCLoTWCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.lotwDXCC)
}

func (c *LogbookCache) DXCCQSLCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.qslDXCC)
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

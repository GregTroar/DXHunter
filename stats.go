package main

import "sync"

var (
	spotsReceived  int64
	spotsProcessed int64
	spotsRejected  int64
	spotsMutex     sync.RWMutex
)

func IncrementSpotsReceived() {
	spotsMutex.Lock()
	spotsReceived++
	spotsMutex.Unlock()
}

func IncrementSpotsProcessed() {
	spotsMutex.Lock()
	spotsProcessed++
	spotsMutex.Unlock()
}

func IncrementSpotsRejected() {
	spotsMutex.Lock()
	spotsRejected++
	spotsMutex.Unlock()
}

func GetSpotStats() (int64, int64, int64) {
	spotsMutex.RLock()
	defer spotsMutex.RUnlock()
	return spotsReceived, spotsProcessed, spotsRejected
}

func GetSpotSuccessRate() float64 {
	spotsMutex.RLock()
	defer spotsMutex.RUnlock()

	if spotsReceived == 0 {
		return 0.0
	}

	return float64(spotsProcessed) / float64(spotsReceived) * 100.0
}

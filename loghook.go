package main

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

// LogBuffer garde les derniers logs en mémoire
type LogBuffer struct {
	entries []LogEntry
	maxSize int
	mutex   sync.RWMutex
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

var logBuffer *LogBuffer

func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

func (lb *LogBuffer) Add(entry LogEntry) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.entries = append(lb.entries, entry)

	// Garder seulement les N derniers
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[1:]
	}
}

func (lb *LogBuffer) GetAll() []LogEntry {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	// Retourner une copie
	result := make([]LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// Hook pour capturer les logs
type LogHook struct {
	buffer *LogBuffer
}

func (h *LogHook) Levels() []log.Level {
	return log.AllLevels
}

func (h *LogHook) Fire(entry *log.Entry) error {
	logEntry := LogEntry{
		Timestamp: entry.Time.Format("02-01-2006 15:04:05"),
		Level:     entry.Level.String(),
		Message:   entry.Message,
	}

	h.buffer.Add(logEntry)

	// Broadcaster vers les clients WebSocket connectés
	if httpServerInstance != nil {
		httpServerInstance.broadcast <- WSMessage{
			Type: "appLog",
			Data: logEntry,
		}
	}

	return nil
}

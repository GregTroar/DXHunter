package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var Log *log.Logger
var logFile *os.File
var logWriter *syncWriter
var logCtx context.Context
var logCancel context.CancelFunc

// syncWriter écrit de manière synchrone (pas de buffer)
type syncWriter struct {
	file  *os.File
	mutex sync.Mutex
}

func (w *syncWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	n, err = w.file.Write(p)
	if err == nil {
		w.file.Sync() // Force l'écriture immédiate sur disque
	}
	return n, err
}

func NewLog() *log.Logger {

	// ✅ Vérifier que Cfg existe
	if Cfg == nil {
		panic("Config not initialized! Call NewConfig() before NewLog()")
	}

	// ✅ Chemin du log à côté de l'exe
	exe, _ := os.Executable()
	exePath := filepath.Dir(exe)
	logPath := filepath.Join(exePath, "dxhunter.log")

	if Cfg.General.DeleteLogFileAtStart {
		if _, err := os.Stat(logPath); err == nil {
			os.Remove(logPath)
		}
	}

	logCtx, logCancel = context.WithCancel(context.Background())

	var w io.Writer

	if Cfg.General.LogToFile {
		f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(fmt.Sprintf("Cannot open log file %s: %v", logPath, err))
		}

		logFile = f
		logWriter = &syncWriter{file: f}

		// ✅ IMPORTANT: Vérifier si Stdout est disponible (mode console vs GUI)
		if isConsoleAvailable() {
			// Mode console : log vers fichier ET console
			w = io.MultiWriter(os.Stdout, logWriter)
		} else {
			// Mode GUI (windowsgui) : log SEULEMENT vers fichier
			w = logWriter
		}
	} else {
		// Log uniquement vers console (si disponible)
		if isConsoleAvailable() {
			w = os.Stdout
		} else {
			// Pas de console, pas de log fichier -> log vers null
			w = io.Discard
		}
	}

	Log = &log.Logger{
		Out: w,
		Formatter: &prefixed.TextFormatter{
			DisableColors:    !isConsoleAvailable(),
			TimestampFormat:  "02-01-2006 15:04:05",
			FullTimestamp:    true,
			ForceFormatting:  true,
			DisableSorting:   true, // ✅ Ajoute
			QuoteEmptyFields: true, // ✅ Ajoute
			SpacePadding:     0,    // ✅ Ajoute (pas d'espace)
		},
		Hooks: make(log.LevelHooks),
	}

	if Cfg.General.LogLevel == "DEBUG" {
		Log.Level = log.DebugLevel
	} else if Cfg.General.LogLevel == "INFO" {
		Log.Level = log.InfoLevel
	} else if Cfg.General.LogLevel == "WARN" {
		Log.Level = log.WarnLevel
	} else {
		Log.Level = log.InfoLevel
	}

	logBuffer = NewLogBuffer(500) // Garde les 500 derniers logs
	// Log.AddHook(&LogHook{buffer: logBuffer})

	// ✅ Premier vrai log
	Log.Infof("Logger initialized - Level: %s, ToFile: %v, LogPath: %s",
		Cfg.General.LogLevel, Cfg.General.LogToFile, logPath)

	return Log
}

func InitLogHook() {
	if logBuffer == nil {
		logBuffer = NewLogBuffer(500)
	}

	Log.AddHook(&LogHook{buffer: logBuffer})
	Log.Debug("Log hook initialized and broadcasting enabled")
}

// ✅ Détecter si on a une console (fonctionne sur Windows)
func isConsoleAvailable() bool {
	// Si Stdout est nil ou invalide, on n'a pas de console
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// Si c'est un char device, on a une console
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// ✅ Fonction pour fermer proprement le log
func CloseLog() {
	if Log != nil {
		Log.Info("Closing log file...")
	}

	if logCancel != nil {
		logCancel()
	}

	time.Sleep(200 * time.Millisecond) // Donne le temps d'écrire

	if logWriter != nil {
		logWriter.mutex.Lock()
		if logFile != nil {
			logFile.Sync()
			logFile.Close()
			logFile = nil
		}
		logWriter.mutex.Unlock()
	}
}

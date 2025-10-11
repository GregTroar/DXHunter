package main

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var Log *log.Logger

func NewLog() *log.Logger {

	if Cfg.General.DeleteLogFileAtStart {
		if _, err := os.Stat("flexradio.log"); err == nil {
			os.Remove("flexradio.log")
		}
	}

	var w io.Writer
	if Cfg.General.LogToFile {
		f, err := os.OpenFile("flexradio.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		w = io.MultiWriter(os.Stdout, f)
	} else {
		w = io.Writer(os.Stdout)
	}

	Log = &log.Logger{
		Out: w,
		Formatter: &prefixed.TextFormatter{
			DisableColors:   false,
			TimestampFormat: "02-01-2006 15:04:05",
			FullTimestamp:   true,
			ForceFormatting: true,
		},
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

	return Log
}

// Info ...
// func Info(format string, v ...interface{}) {
// 	log.Infof(format, v...)
// }

// // Warn ...
// func Warn(format string, v ...interface{}) {
// 	log.Warnf(format, v...)
// }

// // Error ...
// func Error(format string, v ...interface{}) {
// 	log.Errorf(format, v...)
// }

// func Debug(format string, v ...interface{}) {
// 	log.Debugf(format, v...)
// }

package logging

import (
	"log"
	"os"
)

// Logger is a tiny wrapper around the standard logger for tests and verbosity control.
type Logger struct {
	debug bool
	std   *log.Logger
}

// NewLogger returns a logger. If debug is true, debug logs are enabled.
func NewLogger(debug bool) *Logger {
	return &Logger{debug: debug, std: log.New(os.Stdout, "cfgw: ", log.LstdFlags)}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.debug {
		l.std.Printf("DEBUG: "+format, v...)
	}
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.std.Printf("INFO: "+format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.std.Printf("ERROR: "+format, v...)
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.std.Fatalf("FATAL: "+format, v...)
}

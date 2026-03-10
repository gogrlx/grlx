package log

import (
	charmlog "github.com/charmbracelet/log"
)

// Level represents a log severity level.
type Level int

const (
	LTrace Level = iota
	LDebug
	LInfo
	LNotice
	LWarn
	LError
	LPanic
	LFatal
)

// toCharmLevel maps our Level to charmbracelet/log's Level.
func toCharmLevel(l Level) charmlog.Level {
	switch l {
	case LTrace:
		return charmlog.DebugLevel - 1 // charm has no trace; use sub-debug
	case LDebug:
		return charmlog.DebugLevel
	case LInfo, LNotice:
		return charmlog.InfoLevel
	case LWarn:
		return charmlog.WarnLevel
	case LError:
		return charmlog.ErrorLevel
	case LPanic, LFatal:
		return charmlog.FatalLevel
	default:
		return charmlog.InfoLevel
	}
}

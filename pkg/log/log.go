// Package log provides a multiplexed logger for grlx that fans out to
// charmbracelet/log (structured terminal output) and log-nats (NATS bus).
// It exposes package-level functions matching the log-socket API for
// drop-in replacement.
package log

import (
	"os"
	"sync"

	charmlog "github.com/charmbracelet/log"
	mux "github.com/taigrr/log-mux/log"
	nlog "github.com/taigrr/log-nats/v2/log"
)

var (
	mu     sync.RWMutex
	logger *mux.Logger
	charm  *charmlog.Logger
	natsUp bool
)

func init() {
	charm = charmlog.NewWithOptions(os.Stderr, charmlog.Options{
		ReportTimestamp: true,
		ReportCaller:    true,
	})
	charm.SetLevel(charmlog.DebugLevel)

	logger = mux.Default()
	logger.SubLoggers = append(logger.SubLoggers, newCharmAdapter(charm))
}

// SetLogLevel sets the minimum log level for the charm terminal logger.
// The NATS logger always receives all messages (filtering is done by
// subscribers).
func SetLogLevel(l Level) {
	mu.Lock()
	defer mu.Unlock()
	charm.SetLevel(toCharmLevel(l))
}

// ConnectNATS connects the NATS logger backend. Call this after NATS
// is available. The NATS logger publishes all levels regardless of
// the terminal log level.
func ConnectNATS(url string) error {
	mu.Lock()
	defer mu.Unlock()
	if natsUp {
		return nil
	}
	if err := nlog.ConnectDefault(url); err != nil {
		return err
	}
	natsLogger := nlog.Default()
	logger.SubLoggers = append(logger.SubLoggers, natsLogger)
	natsUp = true
	return nil
}

// Flush drains any buffered log entries.
func Flush() {
	nlog.Flush()
}

// Package-level logging functions — drop-in replacements for log-socket.

func Trace(args ...any)                 { logger.Trace(args...) }
func Tracef(format string, args ...any) { logger.Tracef(format, args...) }
func Traceln(args ...any)               { logger.Traceln(args...) }

func Debug(args ...any)                 { logger.Debug(args...) }
func Debugf(format string, args ...any) { logger.Debugf(format, args...) }
func Debugln(args ...any)               { logger.Debugln(args...) }

func Info(args ...any)                 { logger.Info(args...) }
func Infof(format string, args ...any) { logger.Infof(format, args...) }
func Infoln(args ...any)               { logger.Infoln(args...) }

func Notice(args ...any)                 { logger.Notice(args...) }
func Noticef(format string, args ...any) { logger.Noticef(format, args...) }
func Noticeln(args ...any)               { logger.Noticeln(args...) }

func Warn(args ...any)                 { logger.Warn(args...) }
func Warnf(format string, args ...any) { logger.Warnf(format, args...) }
func Warnln(args ...any)               { logger.Warnln(args...) }

func Error(args ...any)                 { logger.Error(args...) }
func Errorf(format string, args ...any) { logger.Errorf(format, args...) }
func Errorln(args ...any)               { logger.Errorln(args...) }

func Panic(args ...any)                 { logger.Panic(args...) }
func Panicf(format string, args ...any) { logger.Panicf(format, args...) }
func Panicln(args ...any)               { logger.Panicln(args...) }

func Fatal(args ...any)                 { logger.Fatal(args...) }
func Fatalf(format string, args ...any) { logger.Fatalf(format, args...) }
func Fatalln(args ...any)               { logger.Fatalln(args...) }

func Print(args ...any)                 { logger.Print(args...) }
func Printf(format string, args ...any) { logger.Printf(format, args...) }
func Println(args ...any)               { logger.Println(args...) }

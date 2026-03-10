package log

import (
	"fmt"

	charmlog "github.com/charmbracelet/log"
)

// charmAdapter wraps a charmbracelet/log.Logger to satisfy the
// log-mux LevelLogger interface.
type charmAdapter struct {
	logger *charmlog.Logger
}

func newCharmAdapter(logger *charmlog.Logger) *charmAdapter {
	return &charmAdapter{logger: logger}
}

func (a *charmAdapter) Trace(v ...any)                  { a.logger.Debug(fmt.Sprint(v...)) }
func (a *charmAdapter) Tracef(format string, v ...any)  { a.logger.Debugf(format, v...) }
func (a *charmAdapter) Traceln(v ...any)                { a.logger.Debug(fmt.Sprintln(v...)) }
func (a *charmAdapter) Debug(v ...any)                  { a.logger.Debug(fmt.Sprint(v...)) }
func (a *charmAdapter) Debugf(format string, v ...any)  { a.logger.Debugf(format, v...) }
func (a *charmAdapter) Debugln(v ...any)                { a.logger.Debug(fmt.Sprintln(v...)) }
func (a *charmAdapter) Info(v ...any)                   { a.logger.Info(fmt.Sprint(v...)) }
func (a *charmAdapter) Infof(format string, v ...any)   { a.logger.Infof(format, v...) }
func (a *charmAdapter) Infoln(v ...any)                 { a.logger.Info(fmt.Sprintln(v...)) }
func (a *charmAdapter) Notice(v ...any)                 { a.logger.Info(fmt.Sprint(v...)) }
func (a *charmAdapter) Noticef(format string, v ...any) { a.logger.Infof(format, v...) }
func (a *charmAdapter) Noticeln(v ...any)               { a.logger.Info(fmt.Sprintln(v...)) }
func (a *charmAdapter) Warn(v ...any)                   { a.logger.Warn(fmt.Sprint(v...)) }
func (a *charmAdapter) Warnf(format string, v ...any)   { a.logger.Warnf(format, v...) }
func (a *charmAdapter) Warnln(v ...any)                 { a.logger.Warn(fmt.Sprintln(v...)) }
func (a *charmAdapter) Error(v ...any)                  { a.logger.Error(fmt.Sprint(v...)) }
func (a *charmAdapter) Errorf(format string, v ...any)  { a.logger.Errorf(format, v...) }
func (a *charmAdapter) Errorln(v ...any)                { a.logger.Error(fmt.Sprintln(v...)) }
func (a *charmAdapter) Panic(v ...any)                  { a.logger.Fatal(fmt.Sprint(v...)); panic(fmt.Sprint(v...)) }
func (a *charmAdapter) Panicf(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	a.logger.Fatal(msg)
	panic(msg)
}
func (a *charmAdapter) Panicln(v ...any)               { msg := fmt.Sprintln(v...); a.logger.Fatal(msg); panic(msg) }
func (a *charmAdapter) Fatal(v ...any)                 { a.logger.Fatal(fmt.Sprint(v...)) }
func (a *charmAdapter) Fatalf(format string, v ...any) { a.logger.Fatalf(format, v...) }
func (a *charmAdapter) Fatalln(v ...any)               { a.logger.Fatal(fmt.Sprintln(v...)) }
func (a *charmAdapter) Print(v ...any)                 { a.logger.Print(fmt.Sprint(v...)) }
func (a *charmAdapter) Printf(format string, v ...any) { a.logger.Printf(format, v...) }
func (a *charmAdapter) Println(v ...any)               { a.logger.Print(fmt.Sprintln(v...)) }

package log

// Logger is an instance that wraps the global mux logger.
// It satisfies the nats-server Logger interface (Noticef, Warnf,
// Fatalf, Errorf, Debugf, Tracef) and can be passed to SetLogger.
type Logger struct {
	// LogLevel is kept for compatibility with code that sets it
	// directly (e.g., farmer's CreateClient pattern). It does not
	// affect filtering — use SetLogLevel for that.
	LogLevel Level
}

// SetInfoDepth is a no-op retained for compatibility. Caller info
// is handled by charmbracelet/log's ReportCaller option.
func (l *Logger) SetInfoDepth(_ int) {}

func (l Logger) Trace(args ...any)                  { logger.Trace(args...) }
func (l Logger) Tracef(format string, args ...any)  { logger.Tracef(format, args...) }
func (l Logger) Traceln(args ...any)                { logger.Traceln(args...) }
func (l Logger) Debug(args ...any)                  { logger.Debug(args...) }
func (l Logger) Debugf(format string, args ...any)  { logger.Debugf(format, args...) }
func (l Logger) Debugln(args ...any)                { logger.Debugln(args...) }
func (l Logger) Info(args ...any)                   { logger.Info(args...) }
func (l Logger) Infof(format string, args ...any)   { logger.Infof(format, args...) }
func (l Logger) Infoln(args ...any)                 { logger.Infoln(args...) }
func (l Logger) Notice(args ...any)                 { logger.Notice(args...) }
func (l Logger) Noticef(format string, args ...any) { logger.Noticef(format, args...) }
func (l Logger) Noticeln(args ...any)               { logger.Noticeln(args...) }
func (l Logger) Warn(args ...any)                   { logger.Warn(args...) }
func (l Logger) Warnf(format string, args ...any)   { logger.Warnf(format, args...) }
func (l Logger) Warnln(args ...any)                 { logger.Warnln(args...) }
func (l Logger) Error(args ...any)                  { logger.Error(args...) }
func (l Logger) Errorf(format string, args ...any)  { logger.Errorf(format, args...) }
func (l Logger) Errorln(args ...any)                { logger.Errorln(args...) }
func (l Logger) Panic(args ...any)                  { logger.Panic(args...) }
func (l Logger) Panicf(format string, args ...any)  { logger.Panicf(format, args...) }
func (l Logger) Panicln(args ...any)                { logger.Panicln(args...) }
func (l Logger) Fatal(args ...any)                  { logger.Fatal(args...) }
func (l Logger) Fatalf(format string, args ...any)  { logger.Fatalf(format, args...) }
func (l Logger) Fatalln(args ...any)                { logger.Fatalln(args...) }
func (l Logger) Print(args ...any)                  { logger.Print(args...) }
func (l Logger) Printf(format string, args ...any)  { logger.Printf(format, args...) }
func (l Logger) Println(args ...any)                { logger.Println(args...) }

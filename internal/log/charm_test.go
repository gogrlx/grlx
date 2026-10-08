package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	charmlog "github.com/charmbracelet/log"
	mux "github.com/taigrr/log-mux/log"
)

func TestToCharmLevel(t *testing.T) {
	tests := []struct {
		name string
		in   Level
		want charmlog.Level
	}{
		{name: "trace", in: LTrace, want: charmlog.DebugLevel - 1},
		{name: "debug", in: LDebug, want: charmlog.DebugLevel},
		{name: "info", in: LInfo, want: charmlog.InfoLevel},
		{name: "notice", in: LNotice, want: charmlog.InfoLevel},
		{name: "warn", in: LWarn, want: charmlog.WarnLevel},
		{name: "error", in: LError, want: charmlog.ErrorLevel},
		{name: "panic", in: LPanic, want: charmlog.FatalLevel},
		{name: "fatal", in: LFatal, want: charmlog.FatalLevel},
		{name: "unknown", in: Level(99), want: charmlog.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toCharmLevel(tt.in); got != tt.want {
				t.Fatalf("toCharmLevel(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSetLogLevelUpdatesCharmLogger(t *testing.T) {
	mu.RLock()
	previousLevel := charm.GetLevel()
	mu.RUnlock()

	SetLogLevel(LWarn)
	t.Cleanup(func() {
		mu.Lock()
		charm.SetLevel(previousLevel)
		mu.Unlock()
	})

	mu.RLock()
	got := charm.GetLevel()
	mu.RUnlock()

	if got != charmlog.WarnLevel {
		t.Fatalf("charm level = %v, want %v", got, charmlog.WarnLevel)
	}
}

func TestCharmAdapterWritesNonFatalLevels(t *testing.T) {
	var buf bytes.Buffer
	adapter := newTestCharmAdapter(&buf)

	adapter.Trace("trace message")
	adapter.Tracef("trace %s", "formatted")
	adapter.Traceln("trace line")
	adapter.Debug("debug message")
	adapter.Debugf("debug %s", "message")
	adapter.Debugln("debug line")
	adapter.Info("info message")
	adapter.Infof("info %s", "formatted")
	adapter.Infoln("info line")
	adapter.Notice("notice message")
	adapter.Noticef("notice %s", "message")
	adapter.Noticeln("notice line")
	adapter.Warn("warn message")
	adapter.Warnf("warn %s", "formatted")
	adapter.Warnln("warn message")
	adapter.Error("error message")
	adapter.Errorf("error %s", "formatted")
	adapter.Errorln("error line")
	adapter.Print("print message")
	adapter.Printf("print %s", "formatted")
	adapter.Println("print line")

	output := buf.String()
	for _, want := range []string{
		"trace message",
		"trace formatted",
		"trace line",
		"debug message",
		"debug line",
		"info message",
		"info formatted",
		"info line",
		"notice message",
		"notice line",
		"warn message",
		"warn formatted",
		"error message",
		"error formatted",
		"error line",
		"print message",
		"print formatted",
		"print line",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("adapter output missing %q:\n%s", want, output)
		}
	}
}

func TestPackageLoggingWrappers(t *testing.T) {
	var buf bytes.Buffer
	restore := captureGlobalLogger(t, &buf)
	defer restore()

	Trace("trace message")
	Tracef("trace %s", "formatted")
	Traceln("trace line")
	Debug("debug message")
	Debugf("debug %s", "formatted")
	Debugln("debug line")
	Info("info message")
	Infof("info %s", "formatted")
	Infoln("info line")
	Notice("notice message")
	Noticef("notice %s", "formatted")
	Noticeln("notice line")
	Warn("warn message")
	Warnf("warn %s", "formatted")
	Warnln("warn line")
	Error("error message")
	Errorf("error %s", "formatted")
	Errorln("error line")
	Print("print message")
	Printf("print %s", "formatted")
	Println("print line")

	assertLogOutput(t, buf.String(), []string{
		"trace message",
		"trace formatted",
		"trace line",
		"debug message",
		"debug formatted",
		"debug line",
		"info message",
		"info formatted",
		"info line",
		"notice message",
		"notice formatted",
		"notice line",
		"warn message",
		"warn formatted",
		"warn line",
		"error message",
		"error formatted",
		"error line",
		"print message",
		"print formatted",
		"print line",
	})
}

func TestLoggerMethodsUseGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	restore := captureGlobalLogger(t, &buf)
	defer restore()

	wrapped := Logger{}
	wrapped.SetInfoDepth(2)
	wrapped.Trace("logger trace")
	wrapped.Tracef("logger trace %s", "formatted")
	wrapped.Traceln("logger trace line")
	wrapped.Debug("logger debug")
	wrapped.Debugf("logger debug %s", "formatted")
	wrapped.Debugln("logger debug line")
	wrapped.Info("logger info")
	wrapped.Infof("logger info %s", "formatted")
	wrapped.Infoln("logger info line")
	wrapped.Notice("logger notice")
	wrapped.Noticef("logger notice %s", "formatted")
	wrapped.Noticeln("logger notice line")
	wrapped.Warn("logger warn")
	wrapped.Warnf("logger warn %s", "formatted")
	wrapped.Warnln("logger warn line")
	wrapped.Error("logger error")
	wrapped.Errorf("logger error %s", "formatted")
	wrapped.Errorln("logger error line")
	wrapped.Print("logger print")
	wrapped.Printf("logger print %s", "formatted")
	wrapped.Println("logger print line")

	assertLogOutput(t, buf.String(), []string{
		"logger trace",
		"logger trace formatted",
		"logger trace line",
		"logger debug",
		"logger debug formatted",
		"logger debug line",
		"logger info",
		"logger info formatted",
		"logger info line",
		"logger notice",
		"logger notice formatted",
		"logger notice line",
		"logger warn",
		"logger warn formatted",
		"logger warn line",
		"logger error",
		"logger error formatted",
		"logger error line",
		"logger print",
		"logger print formatted",
		"logger print line",
	})
}

func TestConnectNATSAlreadyConnected(t *testing.T) {
	mu.Lock()
	previousNatsUp := natsUp
	natsUp = true
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		natsUp = previousNatsUp
		mu.Unlock()
	})

	if err := ConnectNATS("not-a-real-url"); err != nil {
		t.Fatalf("ConnectNATS returned error while already connected: %v", err)
	}
}

func TestCharmAdapterPanicLogsAndPanics(t *testing.T) {
	tests := []struct {
		name string
		call func(*charmAdapter)
		want string
	}{
		{
			name: "panic",
			call: func(adapter *charmAdapter) {
				adapter.Panic("plain panic")
			},
			want: "plain panic",
		},
		{
			name: "panicf",
			call: func(adapter *charmAdapter) {
				adapter.Panicf("formatted %s", "panic")
			},
			want: "formatted panic",
		},
		{
			name: "panicln",
			call: func(adapter *charmAdapter) {
				adapter.Panicln("line panic")
			},
			want: "line panic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			adapter := newTestCharmAdapter(&buf)

			got := recoverPanic(func() {
				tt.call(adapter)
			})
			if got == nil {
				t.Fatal("panic method returned without panicking")
			}
			if fmt.Sprint(got) != tt.want && !strings.Contains(fmt.Sprint(got), tt.want) {
				t.Fatalf("panic value = %q, want %q", got, tt.want)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Fatalf("adapter output missing panic message %q:\n%s", tt.want, buf.String())
			}
		})
	}
}

func newTestCharmAdapter(buf *bytes.Buffer) *charmAdapter {
	logger := charmlog.NewWithOptions(buf, charmlog.Options{})
	logger.SetLevel(charmlog.DebugLevel - 1)
	logger.SetStyles(charmlog.DefaultStyles())
	return newCharmAdapter(logger)
}

func captureGlobalLogger(t *testing.T, buf *bytes.Buffer) func() {
	t.Helper()

	testCharm := charmlog.NewWithOptions(buf, charmlog.Options{})
	testCharm.SetLevel(charmlog.DebugLevel - 1)
	testCharm.SetStyles(charmlog.DefaultStyles())

	mu.Lock()
	previousLogger := logger
	previousCharm := charm
	previousNatsUp := natsUp
	logger = mux.Default()
	logger.SubLoggers = append(logger.SubLoggers, newCharmAdapter(testCharm))
	charm = testCharm
	natsUp = false
	mu.Unlock()

	return func() {
		mu.Lock()
		logger = previousLogger
		charm = previousCharm
		natsUp = previousNatsUp
		mu.Unlock()
	}
}

func assertLogOutput(t *testing.T, output string, wants []string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("log output missing %q:\n%s", want, output)
		}
	}
}

func recoverPanic(fn func()) (value any) {
	defer func() {
		value = recover()
	}()
	fn()
	return nil
}

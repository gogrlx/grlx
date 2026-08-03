package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	charmlog "github.com/charmbracelet/log"
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
	adapter.Debugf("debug %s", "message")
	adapter.Info("info message")
	adapter.Noticef("notice %s", "message")
	adapter.Warnln("warn message")
	adapter.Error("error message")
	adapter.Print("print message")

	output := buf.String()
	for _, want := range []string{
		"trace message",
		"debug message",
		"info message",
		"notice message",
		"warn message",
		"error message",
		"print message",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("adapter output missing %q:\n%s", want, output)
		}
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

func recoverPanic(fn func()) (value any) {
	defer func() {
		value = recover()
	}()
	fn()
	return nil
}

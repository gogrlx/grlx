package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestHub() *logHub {
	return &logHub{
		clients: make(map[*logClient]struct{}),
		recent:  make([]LogEntry, 0, maxRecentLogs),
	}
}

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name   string
		entry  LogEntry
		level  string
		source string
		want   bool
	}{
		{
			name:   "no filters",
			entry:  LogEntry{Level: "debug", Source: "sprout"},
			level:  "",
			source: "",
			want:   true,
		},
		{
			name:   "level filter passes",
			entry:  LogEntry{Level: "error", Source: "sprout"},
			level:  "warn",
			source: "",
			want:   true,
		},
		{
			name:   "level filter blocks",
			entry:  LogEntry{Level: "debug", Source: "sprout"},
			level:  "info",
			source: "",
			want:   false,
		},
		{
			name:   "source filter passes",
			entry:  LogEntry{Level: "info", Source: "farmer"},
			level:  "",
			source: "farmer",
			want:   true,
		},
		{
			name:   "source filter blocks",
			entry:  LogEntry{Level: "info", Source: "sprout"},
			level:  "",
			source: "farmer",
			want:   false,
		},
		{
			name:   "both filters pass",
			entry:  LogEntry{Level: "warn", Source: "sprout"},
			level:  "info",
			source: "sprout",
			want:   true,
		},
		{
			name:   "level passes source blocks",
			entry:  LogEntry{Level: "error", Source: "farmer"},
			level:  "warn",
			source: "sprout",
			want:   false,
		},
		{
			name:   "exact level match",
			entry:  LogEntry{Level: "info", Source: "sprout"},
			level:  "info",
			source: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilter(tt.entry, tt.level, tt.source)
			if got != tt.want {
				t.Errorf("matchesFilter(%+v, %q, %q) = %v, want %v",
					tt.entry, tt.level, tt.source, got, tt.want)
			}
		})
	}
}

func TestSplitSubject(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"grlx.cook.sprout1.jid123", []string{"grlx", "cook", "sprout1", "jid123"}},
		{"single", []string{"single"}},
		{"a.b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitSubject(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitSubject(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitSubject(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLogHubStoreAndGetRecent(t *testing.T) {
	h := newTestHub()

	entries := []LogEntry{
		{Timestamp: "2026-01-01T00:00:00Z", Level: "debug", Source: "sprout", SourceID: "s1", Message: "debug msg"},
		{Timestamp: "2026-01-01T00:00:01Z", Level: "info", Source: "sprout", SourceID: "s1", Message: "info msg"},
		{Timestamp: "2026-01-01T00:00:02Z", Level: "warn", Source: "farmer", Message: "warn msg"},
		{Timestamp: "2026-01-01T00:00:03Z", Level: "error", Source: "sprout", SourceID: "s2", Message: "error msg"},
	}

	for _, e := range entries {
		h.storeRecent(e)
	}

	// Get all
	all := h.getRecent("", "", 0)
	if len(all) != 4 {
		t.Errorf("getRecent (all) = %d entries, want 4", len(all))
	}

	// Filter by level
	warns := h.getRecent("warn", "", 0)
	if len(warns) != 2 {
		t.Errorf("getRecent (warn+) = %d entries, want 2", len(warns))
	}

	// Filter by source
	farmers := h.getRecent("", "farmer", 0)
	if len(farmers) != 1 {
		t.Errorf("getRecent (farmer) = %d entries, want 1", len(farmers))
	}

	// Limit
	limited := h.getRecent("", "", 2)
	if len(limited) != 2 {
		t.Errorf("getRecent (limit 2) = %d entries, want 2", len(limited))
	}
	// Should be the most recent 2
	if limited[0].Level != "warn" || limited[1].Level != "error" {
		t.Errorf("getRecent (limit 2) returned wrong entries: %+v", limited)
	}
}

func TestLogHubRecentRingBuffer(t *testing.T) {
	h := newTestHub()

	// Store more than maxRecentLogs entries
	for i := 0; i < maxRecentLogs+50; i++ {
		h.storeRecent(LogEntry{
			Timestamp: "2026-01-01T00:00:00Z",
			Level:     "info",
			Source:    "sprout",
			Message:   "msg",
		})
	}

	all := h.getRecent("", "", 0)
	if len(all) != maxRecentLogs {
		t.Errorf("ring buffer size = %d, want %d", len(all), maxRecentLogs)
	}
}

func TestHandleRecentLogsEndpoint(t *testing.T) {
	h := newTestHub()
	h.storeRecent(LogEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		Level:     "info",
		Source:    "sprout",
		SourceID:  "test-sprout",
		Message:   "test message",
	})

	handler := handleRecentLogsWithHub(h)
	req := httptest.NewRequest("GET", "/api/v1/logs?level=info&source=sprout&limit=10", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Logs []LogEntry `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Logs) != 1 {
		t.Fatalf("logs count = %d, want 1", len(resp.Logs))
	}
	if resp.Logs[0].Message != "test message" {
		t.Errorf("message = %q, want %q", resp.Logs[0].Message, "test message")
	}
}

func TestWebSocketLogStream(t *testing.T) {
	h := newTestHub()
	handler := handleLogStreamWithHub(h)

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	// Wait for client to be registered
	time.Sleep(50 * time.Millisecond)

	entry := LogEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		Level:     "info",
		Source:    "sprout",
		SourceID:  "test-sprout",
		Message:   "streamed message",
	}
	h.broadcast(entry)

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var received LogEntry
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Message != "streamed message" {
		t.Errorf("message = %q, want %q", received.Message, "streamed message")
	}
}

func TestWebSocketLevelFilter(t *testing.T) {
	h := newTestHub()
	handler := handleLogStreamWithHub(h)

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?level=warn"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	// Send a debug entry (should be filtered out)
	h.broadcast(LogEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		Level:     "debug",
		Source:    "sprout",
		Message:   "debug msg",
	})

	// Send an error entry (should pass)
	h.broadcast(LogEntry{
		Timestamp: "2026-01-01T00:00:01Z",
		Level:     "error",
		Source:    "sprout",
		Message:   "error msg",
	})

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var received LogEntry
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Message != "error msg" {
		t.Errorf("got %q, want %q (debug should have been filtered)", received.Message, "error msg")
	}
}

func TestWebSocketSourceFilter(t *testing.T) {
	h := newTestHub()
	handler := handleLogStreamWithHub(h)

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?source=farmer"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	// Send sprout entry (should be filtered out)
	h.broadcast(LogEntry{
		Timestamp: "2026-01-01T00:00:00Z",
		Level:     "info",
		Source:    "sprout",
		Message:   "sprout msg",
	})

	// Send farmer entry (should pass)
	h.broadcast(LogEntry{
		Timestamp: "2026-01-01T00:00:01Z",
		Level:     "info",
		Source:    "farmer",
		Message:   "farmer msg",
	})

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var received LogEntry
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if received.Message != "farmer msg" {
		t.Errorf("got %q, want %q", received.Message, "farmer msg")
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"42", 42, false},
		{"100", 100, false},
		{"abc", 0, true},
		{"12x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLogHubBroadcast(t *testing.T) {
	h := newTestHub()

	// Create two clients with different filters
	c1 := &logClient{send: make(chan LogEntry, 10), level: "", source: ""}
	c2 := &logClient{send: make(chan LogEntry, 10), level: "error", source: ""}

	h.register(c1)
	h.register(c2)
	defer h.unregister(c1)
	defer h.unregister(c2)

	h.broadcast(LogEntry{Level: "info", Source: "sprout", Message: "info msg"})

	// c1 should receive (no filter)
	select {
	case msg := <-c1.send:
		if msg.Message != "info msg" {
			t.Errorf("c1 got %q, want %q", msg.Message, "info msg")
		}
	default:
		t.Error("c1 should have received message")
	}

	// c2 should NOT receive (level filter = error)
	select {
	case msg := <-c2.send:
		t.Errorf("c2 should not have received message, got %q", msg.Message)
	default:
		// expected
	}
}

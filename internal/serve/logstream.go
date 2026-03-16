package serve

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/gogrlx/grlx/v2/internal/log"
)

// LogEntry matches the JSON structure expected by the web UI's LogEntry type.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	SourceID  string `json:"sourceId,omitempty"`
	Message   string `json:"message"`
}

// logHub manages WebSocket subscribers for log streaming. It subscribes
// to NATS cook events and fans out parsed log entries to all connected
// WebSocket clients. Each client can filter by level and source.
type logHub struct {
	mu      sync.Mutex
	clients map[*logClient]struct{}
	sub     *nats.Subscription

	// recentMu guards the recent log ring buffer.
	recentMu sync.RWMutex
	recent   []LogEntry
}

// logClient represents a single WebSocket subscriber.
type logClient struct {
	conn   *websocket.Conn
	send   chan LogEntry
	level  string // minimum level filter: debug < info < warn < error
	source string // source filter: farmer | sprout | "" (all)
}

const (
	maxRecentLogs = 200
	writeWait     = 10 * time.Second
	pongWait      = 60 * time.Second
	pingPeriod    = 54 * time.Second
	sendBufSize   = 256
)

var (
	hub     *logHub
	hubOnce sync.Once

	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(_ *http.Request) bool {
			return true // local server; all origins allowed
		},
	}

	// levelOrder maps log levels to numeric severity for filtering.
	levelOrder = map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}
)

// getHub returns the singleton logHub, initializing it on first call.
func getHub() *logHub {
	hubOnce.Do(func() {
		hub = &logHub{
			clients: make(map[*logClient]struct{}),
			recent:  make([]LogEntry, 0, maxRecentLogs),
		}
		hub.subscribeNATS()
	})
	return hub
}

// subscribeNATS subscribes to NATS cook event subjects and converts
// them into LogEntry values for connected WebSocket clients.
func (h *logHub) subscribeNATS() {
	if client.NatsConn == nil {
		log.Errorf("logstream: NATS connection not available; log streaming disabled")
		return
	}

	sub, err := client.NatsConn.Subscribe("grlx.cook.*.*", func(msg *nats.Msg) {
		entry := h.parseCookEvent(msg)
		if entry == nil {
			return
		}
		h.storeRecent(*entry)
		h.broadcast(*entry)
	})
	if err != nil {
		log.Errorf("logstream: failed to subscribe to NATS cook events: %v", err)
		return
	}
	h.sub = sub
	log.Printf("logstream: subscribed to grlx.cook.*.* for log streaming")
}

// parseCookEvent converts a NATS cook message into a LogEntry.
// Subject format: grlx.cook.<sproutID>.<jid>
func (h *logHub) parseCookEvent(msg *nats.Msg) *LogEntry {
	parts := splitSubject(msg.Subject)
	if len(parts) < 4 {
		return nil
	}
	sproutID := parts[2]
	jid := parts[3]

	var step cook.StepCompletion
	if err := json.Unmarshal(msg.Data, &step); err != nil {
		return &LogEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Level:     "warn",
			Source:    "sprout",
			SourceID:  sproutID,
			Message:   "failed to parse cook event for job " + jid,
		}
	}

	level := "info"
	message := formatStepMessage(step, jid)
	if step.Error != nil {
		level = "error"
	} else if step.ChangesMade {
		level = "info"
	} else {
		level = "debug"
	}

	return &LogEntry{
		Timestamp: step.Started.UTC().Format(time.RFC3339),
		Level:     level,
		Source:    "sprout",
		SourceID:  sproutID,
		Message:   message,
	}
}

// formatStepMessage builds a human-readable log message from a StepCompletion.
func formatStepMessage(step cook.StepCompletion, jid string) string {
	status := "completed"
	switch step.CompletionStatus {
	case cook.StepNotStarted:
		status = "not started"
	case cook.StepInProgress:
		status = "in progress"
	case cook.StepFailed:
		status = "failed"
	case cook.StepSkipped:
		status = "skipped"
	case cook.StepCompleted:
		status = "completed"
	}

	msg := "job " + jid + " step " + string(step.ID) + ": " + status
	if step.Error != nil {
		msg += " — " + step.Error.Error()
	}
	if step.ChangesMade && len(step.Changes) > 0 {
		msg += " (changes: "
		for i, c := range step.Changes {
			if i > 0 {
				msg += ", "
			}
			msg += c
			if i >= 4 {
				msg += "..."
				break
			}
		}
		msg += ")"
	}
	if step.Duration > 0 {
		msg += " [" + step.Duration.String() + "]"
	}
	return msg
}

// storeRecent adds an entry to the ring buffer of recent logs.
func (h *logHub) storeRecent(entry LogEntry) {
	h.recentMu.Lock()
	defer h.recentMu.Unlock()

	h.recent = append(h.recent, entry)
	if len(h.recent) > maxRecentLogs {
		// Drop the oldest entries.
		h.recent = h.recent[len(h.recent)-maxRecentLogs:]
	}
}

// getRecent returns a copy of recent log entries, optionally filtered.
func (h *logHub) getRecent(level, source string, limit int) []LogEntry {
	h.recentMu.RLock()
	defer h.recentMu.RUnlock()

	result := make([]LogEntry, 0, len(h.recent))
	for _, entry := range h.recent {
		if !matchesFilter(entry, level, source) {
			continue
		}
		result = append(result, entry)
	}

	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result
}

// broadcast sends a log entry to all connected clients that match filters.
func (h *logHub) broadcast(entry LogEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for c := range h.clients {
		if !matchesFilter(entry, c.level, c.source) {
			continue
		}
		select {
		case c.send <- entry:
		default:
			// Client is too slow; drop the message.
		}
	}
}

// register adds a client to the hub.
func (h *logHub) register(c *logClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

// unregister removes a client from the hub and closes its send channel.
func (h *logHub) unregister(c *logClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// matchesFilter returns true if entry passes the level and source filters.
func matchesFilter(entry LogEntry, level, source string) bool {
	if source != "" && entry.Source != source {
		return false
	}
	if level != "" {
		entryOrd, ok1 := levelOrder[entry.Level]
		filterOrd, ok2 := levelOrder[level]
		if ok1 && ok2 && entryOrd < filterOrd {
			return false
		}
	}
	return true
}

// splitSubject splits a NATS subject into its dot-delimited parts.
func splitSubject(subject string) []string {
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(subject); i++ {
		if subject[i] == '.' {
			parts = append(parts, subject[start:i])
			start = i + 1
		}
	}
	parts = append(parts, subject[start:])
	return parts
}

// HandleLogStream upgrades an HTTP request to a WebSocket connection
// and streams log entries to the client.
func HandleLogStream(w http.ResponseWriter, r *http.Request) {
	handleLogStreamWithHub(getHub())(w, r)
}

// handleLogStreamWithHub returns a handler that uses the given hub.
// This is used for testing without the global singleton.
func handleLogStreamWithHub(h *logHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("logstream: websocket upgrade failed: %v", err)
			return
		}

		level := r.URL.Query().Get("level")
		source := r.URL.Query().Get("source")

		c := &logClient{
			conn:   conn,
			send:   make(chan LogEntry, sendBufSize),
			level:  level,
			source: source,
		}

		h.register(c)

		go c.writePump()
		go c.readPump(h)
	}
}

// HandleRecentLogs returns recent log entries as JSON. Supports level,
// source, and limit query parameters.
func HandleRecentLogs(w http.ResponseWriter, r *http.Request) {
	handleRecentLogsWithHub(getHub())(w, r)
}

// handleRecentLogsWithHub returns a handler that uses the given hub.
func handleRecentLogsWithHub(h *logHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		level := r.URL.Query().Get("level")
		source := r.URL.Query().Get("source")
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := parseInt(v); err == nil && n > 0 {
				limit = n
			}
		}

		logs := h.getRecent(level, source, limit)
		WriteJSON(w, http.StatusOK, map[string]any{"logs": logs})
	}
}

// parseInt is a small helper to parse an int from a string.
func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &json.InvalidUnmarshalError{}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// writePump writes log entries from the send channel to the WebSocket.
func (c *logClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case entry, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads from the WebSocket to detect client disconnection.
// It discards all incoming messages.
func (c *logClient) readPump(h *logHub) {
	defer func() {
		h.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

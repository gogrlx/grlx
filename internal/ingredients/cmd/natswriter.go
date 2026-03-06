package cmd

import (
	"encoding/json"

	nats "github.com/nats-io/nats.go"
)

// OutputChunk represents a chunk of command output streamed over NATS.
type OutputChunk struct {
	Stream string `json:"stream"` // "stdout" or "stderr"
	Data   string `json:"data"`
}

// natsWriter implements io.Writer and publishes each Write to a NATS subject.
type natsWriter struct {
	conn   *nats.Conn
	topic  string
	stream string // "stdout" or "stderr"
}

func (w *natsWriter) Write(p []byte) (int, error) {
	if w.conn == nil {
		return len(p), nil
	}
	chunk := OutputChunk{
		Stream: w.stream,
		Data:   string(p),
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return len(p), nil // Don't fail the command for publish errors.
	}
	_ = w.conn.Publish(w.topic, data)
	return len(p), nil
}

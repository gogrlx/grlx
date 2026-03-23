package cmd

import (
	"encoding/json"
	"testing"
)

func TestNatsWriterNilConn(t *testing.T) {
	w := &natsWriter{conn: nil, topic: "test.topic", stream: "stdout"}
	data := []byte("hello world")
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
}

func TestNatsWriterEmptyData(t *testing.T) {
	w := &natsWriter{conn: nil, topic: "test.topic", stream: "stderr"}
	n, err := w.Write([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
}

func TestNatsWriterLargeData(t *testing.T) {
	w := &natsWriter{conn: nil, topic: "test.topic", stream: "stdout"}
	data := make([]byte, 64*1024)
	for i := range data {
		data[i] = 'x'
	}
	n, err := w.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}
}

func TestOutputChunkJSON(t *testing.T) {
	chunk := OutputChunk{
		Stream: "stdout",
		Data:   "hello world\n",
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded OutputChunk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.Stream != "stdout" {
		t.Errorf("expected stream 'stdout', got %q", decoded.Stream)
	}
	if decoded.Data != "hello world\n" {
		t.Errorf("expected data 'hello world\\n', got %q", decoded.Data)
	}
}

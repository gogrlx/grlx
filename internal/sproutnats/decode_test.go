package sproutnats

import (
	"strings"
	"testing"
)

func TestDecodeCmdRunRejectsMalformedPayload(t *testing.T) {
	_, response, err := DecodeCmdRun([]byte("{"))
	if err == nil {
		t.Fatal("expected decode error")
	}
	if response.ErrCode != -1 {
		t.Fatalf("expected errcode -1, got %d", response.ErrCode)
	}
	if !strings.Contains(response.Stderr, "invalid cmd.run request") {
		t.Fatalf("expected invalid request message, got %q", response.Stderr)
	}
}

func TestDecodePingRejectsMalformedPayload(t *testing.T) {
	_, response, err := DecodePing([]byte("{"))
	if err == nil {
		t.Fatal("expected decode error")
	}
	if response.Ping || response.Pong {
		t.Fatalf("malformed ping should return false values, got %#v", response)
	}
}

func TestDecodeCookRejectsMalformedPayload(t *testing.T) {
	_, response, err := DecodeCook([]byte("{"))
	if err == nil {
		t.Fatal("expected decode error")
	}
	if response.Acknowledged {
		t.Fatal("malformed cook request should not be acknowledged")
	}
}

package test

import (
	"testing"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
)

func TestSPing(t *testing.T) {
	ping := apitypes.PingPong{
		Ping: true,
		Pong: false,
	}
	pong, err := SPing(ping)
	if err != nil {
		t.Fatalf("SPing error: %v", err)
	}
	if !pong.Pong {
		t.Error("expected Pong to be true after SPing")
	}
	if !pong.Ping {
		t.Error("expected Ping to still be true")
	}
}

func TestSPingPreservesFields(t *testing.T) {
	ping := apitypes.PingPong{
		Ping: false,
		Pong: false,
	}
	pong, err := SPing(ping)
	if err != nil {
		t.Fatalf("SPing error: %v", err)
	}
	if !pong.Pong {
		t.Error("expected Pong true")
	}
	// Ping should remain as passed.
	if pong.Ping {
		t.Error("expected Ping to remain false")
	}
}

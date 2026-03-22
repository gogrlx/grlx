package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	apitypes "github.com/gogrlx/grlx/v2/internal/api/types"
	"github.com/gogrlx/grlx/v2/internal/pki"
)

// startTestNATS starts an embedded NATS server and returns a connection plus cleanup.
func startTestNATS(t *testing.T) (*nats.Conn, func()) {
	t.Helper()

	opts := &server.Options{
		Host: "127.0.0.1",
		Port: -1,
	}
	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("start test NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to become ready")
	}

	conn, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		t.Fatalf("connect to test NATS: %v", err)
	}

	return conn, func() {
		conn.Close()
		ns.Shutdown()
	}
}

func TestRegisterNatsConn(t *testing.T) {
	conn, cleanup := startTestNATS(t)
	defer cleanup()

	// Verify the package-level nc is set.
	RegisterNatsConn(conn)
	if nc != conn {
		t.Error("RegisterNatsConn did not set the package-level connection")
	}

	// Clean up.
	RegisterNatsConn(nil)
	if nc != nil {
		t.Error("RegisterNatsConn(nil) did not clear the connection")
	}
}

func TestFPingSuccess(t *testing.T) {
	conn, cleanup := startTestNATS(t)
	defer cleanup()
	RegisterNatsConn(conn)
	defer RegisterNatsConn(nil)

	target := pki.KeyManager{SproutID: "sprout-1"}
	topic := "grlx.sprouts." + target.SproutID + ".test.ping"

	// Subscribe to simulate a sprout responding to pings.
	sub, err := conn.Subscribe(topic, func(msg *nats.Msg) {
		var req apitypes.PingPong
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			t.Errorf("unmarshal request: %v", err)
			return
		}
		resp := apitypes.PingPong{
			Ping: req.Ping,
			Pong: true,
		}
		data, _ := json.Marshal(resp)
		if err := msg.Respond(data); err != nil {
			t.Errorf("respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	ping := apitypes.PingPong{Ping: false, Pong: false}
	pong, err := FPing(target, ping)
	if err != nil {
		t.Fatalf("FPing error: %v", err)
	}
	if !pong.Pong {
		t.Error("expected Pong to be true")
	}
	if !pong.Ping {
		t.Error("expected Ping to be true (FPing sets it before sending)")
	}
}

func TestFPingTimeout(t *testing.T) {
	conn, cleanup := startTestNATS(t)
	defer cleanup()
	RegisterNatsConn(conn)
	defer RegisterNatsConn(nil)

	// No subscriber — the request should time out.
	target := pki.KeyManager{SproutID: "no-such-sprout"}
	ping := apitypes.PingPong{Ping: false, Pong: false}

	start := time.Now()
	_, err := FPing(target, ping)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// The timeout is 15 seconds in FPing; verify we actually waited.
	if elapsed < 10*time.Second {
		t.Logf("elapsed: %v (expected ~15s timeout)", elapsed)
	}
}

func TestFPingInvalidResponse(t *testing.T) {
	conn, cleanup := startTestNATS(t)
	defer cleanup()
	RegisterNatsConn(conn)
	defer RegisterNatsConn(nil)

	target := pki.KeyManager{SproutID: "bad-sprout"}
	topic := "grlx.sprouts." + target.SproutID + ".test.ping"

	// Respond with invalid JSON.
	sub, err := conn.Subscribe(topic, func(msg *nats.Msg) {
		if err := msg.Respond([]byte("not json")); err != nil {
			t.Errorf("respond: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	ping := apitypes.PingPong{}
	_, err = FPing(target, ping)
	if err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}
}

func TestFPingSetsFieldsCorrectly(t *testing.T) {
	conn, cleanup := startTestNATS(t)
	defer cleanup()
	RegisterNatsConn(conn)
	defer RegisterNatsConn(nil)

	target := pki.KeyManager{SproutID: "field-check"}
	topic := "grlx.sprouts." + target.SproutID + ".test.ping"

	// Capture the request to verify FPing sets ping=true, pong=false.
	var received apitypes.PingPong
	sub, err := conn.Subscribe(topic, func(msg *nats.Msg) {
		json.Unmarshal(msg.Data, &received)
		resp, _ := json.Marshal(apitypes.PingPong{Ping: true, Pong: true})
		msg.Respond(resp)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	// Pass with Ping=false — FPing should override to Ping=true, Pong=false.
	ping := apitypes.PingPong{Ping: false, Pong: true}
	_, err = FPing(target, ping)
	if err != nil {
		t.Fatalf("FPing error: %v", err)
	}

	if !received.Ping {
		t.Error("FPing should set Ping=true before sending")
	}
	if received.Pong {
		t.Error("FPing should set Pong=false before sending")
	}
}

func TestFPingDifferentSproutIDs(t *testing.T) {
	conn, cleanup := startTestNATS(t)
	defer cleanup()
	RegisterNatsConn(conn)
	defer RegisterNatsConn(nil)

	ids := []string{"alpha", "beta-123", "sprout.with.dots"}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			target := pki.KeyManager{SproutID: id}
			topic := "grlx.sprouts." + id + ".test.ping"

			sub, err := conn.Subscribe(topic, func(msg *nats.Msg) {
				resp, _ := json.Marshal(apitypes.PingPong{Ping: true, Pong: true})
				msg.Respond(resp)
			})
			if err != nil {
				t.Fatalf("subscribe: %v", err)
			}
			defer sub.Unsubscribe()

			pong, err := FPing(target, apitypes.PingPong{})
			if err != nil {
				t.Fatalf("FPing(%q) error: %v", id, err)
			}
			if !pong.Pong {
				t.Errorf("FPing(%q): expected Pong true", id)
			}
		})
	}
}

// SPing tests.

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
	if pong.Ping {
		t.Error("expected Ping to remain false")
	}
}

func TestSPingAlwaysReturnsPongTrue(t *testing.T) {
	// Even if Pong is already true, SPing should still return Pong=true.
	ping := apitypes.PingPong{Ping: true, Pong: true}
	pong, err := SPing(ping)
	if err != nil {
		t.Fatalf("SPing error: %v", err)
	}
	if !pong.Pong {
		t.Error("expected Pong true")
	}
}

func TestSPingNoError(t *testing.T) {
	// SPing should never return an error.
	cases := []apitypes.PingPong{
		{Ping: true, Pong: false},
		{Ping: false, Pong: false},
		{Ping: true, Pong: true},
		{Ping: false, Pong: true},
	}
	for _, c := range cases {
		_, err := SPing(c)
		if err != nil {
			t.Errorf("SPing(%+v) returned error: %v", c, err)
		}
	}
}

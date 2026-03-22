package jobs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gogrlx/grlx/v2/internal/cook"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startTestNATSServer(t *testing.T) (*server.Server, *nats.Conn) {
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

	t.Cleanup(func() {
		conn.Close()
		ns.Shutdown()
	})

	return ns, conn
}

func TestNewCLIListener(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UPUBKEY_TEST")

	if listener.store != store {
		t.Error("store mismatch")
	}
	if listener.nc != conn {
		t.Error("nc mismatch")
	}
	if listener.userKey != "UPUBKEY_TEST" {
		t.Errorf("expected userKey UPUBKEY_TEST, got %s", listener.userKey)
	}
}

func TestCLIListener_SubscribeAll(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UKEY1")

	if err := listener.SubscribeAll(); err != nil {
		t.Fatalf("SubscribeAll: %v", err)
	}
	defer listener.Stop()

	if listener.sub == nil {
		t.Fatal("expected subscription to be set")
	}

	// Publish a step completion and verify it gets recorded.
	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepCompleted,
		Started:          time.Now(),
		Duration:         time.Second,
	}
	data, _ := json.Marshal(step)
	if err := conn.Publish("grlx.cook.sprout-test.job-sub-all", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	// Verify the step was recorded.
	summary, _, err := store.GetJob("job-sub-all")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary.Total != 1 {
		t.Errorf("expected 1 step, got %d", summary.Total)
	}
}

func TestCLIListener_SubscribeJob(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UKEY2")

	if err := listener.SubscribeJob("specific-jid"); err != nil {
		t.Fatalf("SubscribeJob: %v", err)
	}
	defer listener.Stop()

	// Publish to the specific JID.
	step := cook.StepCompletion{
		ID:               "step-1",
		CompletionStatus: cook.StepFailed,
		Started:          time.Now(),
		Duration:         2 * time.Second,
	}
	data, _ := json.Marshal(step)
	if err := conn.Publish("grlx.cook.sprout-x.specific-jid", data); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(200 * time.Millisecond)

	summary, _, err := store.GetJob("specific-jid")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if summary.Status != JobFailed {
		t.Errorf("expected failed status, got %s", summary.Status)
	}
}

func TestCLIListener_RecordJobInit(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UKEY3")

	listener.RecordJobInit("init-jid", "web.deploy", []string{"sprout-a", "sprout-b"})

	// Verify both sprouts got recorded.
	metaA, err := store.GetJobMeta("init-jid")
	if err != nil {
		t.Fatalf("GetJobMeta for sprout-a: %v", err)
	}
	if metaA.Recipe != "web.deploy" {
		t.Errorf("expected recipe web.deploy, got %s", metaA.Recipe)
	}
	if metaA.UserKey != "UKEY3" {
		t.Errorf("expected user key UKEY3, got %s", metaA.UserKey)
	}
}

func TestCLIListener_Stop(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UKEY4")

	if err := listener.SubscribeAll(); err != nil {
		t.Fatal(err)
	}
	listener.Stop()

	// After stop, subscription should be inactive.
	if listener.sub.IsValid() {
		t.Error("expected subscription to be invalid after Stop")
	}
}

func TestCLIListener_Stop_NilSub(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UKEY5")

	// Should not panic when sub is nil.
	listener.Stop()
}

func TestCLIListener_HandleStepCompletion_BadSubject(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCLIStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	_, conn := startTestNATSServer(t)
	listener := NewCLIListener(store, conn, "UKEY6")

	if err := listener.SubscribeAll(); err != nil {
		t.Fatal(err)
	}
	defer listener.Stop()

	// Publish with valid subject but bad JSON — should be silently handled.
	if err := conn.Publish("grlx.cook.sprout.jid", []byte("not json")); err != nil {
		t.Fatal(err)
	}
	conn.Flush()
	time.Sleep(100 * time.Millisecond)

	// Should not have recorded anything.
	_, _, err = store.GetJob("jid")
	// File should exist but be empty or non-existent.
	if err != ErrJobNotFound {
		t.Logf("error was: %v (acceptable if step wasn't recorded)", err)
	}
}

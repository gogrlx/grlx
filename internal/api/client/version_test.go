package client

import (
	"testing"

	"github.com/gogrlx/grlx/v2/internal/config"
)

func TestGetVersion_Success(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	want := config.Version{
		Arch:      "amd64",
		Compiler:  "gc",
		GitCommit: "abc1234",
		Tag:       "v2.1.0",
	}
	mockHandler(t, NatsConn, "grlx.api.version", want)

	got, err := GetVersion()
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Tag != "v2.1.0" {
		t.Fatalf("expected tag v2.1.0, got %q", got.Tag)
	}
	if got.Arch != "amd64" {
		t.Fatalf("expected arch amd64, got %q", got.Arch)
	}
	if got.GitCommit != "abc1234" {
		t.Fatalf("expected commit abc1234, got %q", got.GitCommit)
	}
}

func TestGetVersion_Error(t *testing.T) {
	cleanup := startTestNATS(t)
	defer cleanup()

	mockErrorHandler(t, NatsConn, "grlx.api.version", "farmer unavailable")

	_, err := GetVersion()
	if err == nil {
		t.Fatal("expected error")
	}
}

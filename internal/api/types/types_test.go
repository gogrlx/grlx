package apitypes

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestCmdCookJSON(t *testing.T) {
	cmd := CmdCook{
		Async:   true,
		Env:     "production",
		Recipe:  "deploy.nginx",
		Test:    false,
		Timeout: 30 * time.Second,
		JID:     "jid-001",
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded CmdCook
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Async != cmd.Async {
		t.Errorf("expected Async %v, got %v", cmd.Async, decoded.Async)
	}
	if decoded.Recipe != cmd.Recipe {
		t.Errorf("expected Recipe %q, got %q", cmd.Recipe, decoded.Recipe)
	}
	if decoded.JID != cmd.JID {
		t.Errorf("expected JID %q, got %q", cmd.JID, decoded.JID)
	}
}

func TestCmdRunJSON(t *testing.T) {
	cmd := CmdRun{
		Command:     "ls",
		Args:        []string{"-la", "/etc"},
		Path:        "/usr/bin/ls",
		CWD:         "/tmp",
		RunAs:       "root",
		Env:         EnvVar{"HOME": "/root"},
		Timeout:     10 * time.Second,
		StreamTopic: "grlx.stream.test",
		Stdout:      "output here",
		Stderr:      "",
		Duration:    500 * time.Millisecond,
		ErrCode:     0,
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded CmdRun
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Command != "ls" {
		t.Errorf("expected Command 'ls', got %q", decoded.Command)
	}
	if len(decoded.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(decoded.Args))
	}
	if decoded.Env["HOME"] != "/root" {
		t.Errorf("expected HOME=/root, got %q", decoded.Env["HOME"])
	}
	if decoded.StreamTopic != "grlx.stream.test" {
		t.Errorf("expected StreamTopic, got %q", decoded.StreamTopic)
	}
}

func TestPingPongJSON(t *testing.T) {
	pp := PingPong{Ping: true, Pong: false}
	b, err := json.Marshal(pp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded PingPong
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !decoded.Ping {
		t.Error("expected Ping true")
	}
	if decoded.Pong {
		t.Error("expected Pong false")
	}
}

func TestUserInfoJSON(t *testing.T) {
	ui := UserInfo{Pubkey: "UABC123", RoleName: "admin"}
	b, err := json.Marshal(ui)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded UserInfo
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Pubkey != "UABC123" {
		t.Errorf("expected Pubkey UABC123, got %q", decoded.Pubkey)
	}
	if decoded.RoleName != "admin" {
		t.Errorf("expected RoleName admin, got %q", decoded.RoleName)
	}
}

func TestExplainResponseJSON(t *testing.T) {
	resp := ExplainResponse{
		Pubkey:   "UABC123",
		RoleName: "operator",
		IsAdmin:  false,
		Actions: []ActionExplain{
			{Action: "cook", Scope: "*"},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ExplainResponse
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.IsAdmin {
		t.Error("expected IsAdmin false")
	}
	if len(decoded.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(decoded.Actions))
	}
}

func TestErrorSentinels(t *testing.T) {
	if !errors.Is(ErrAPIRouteNotFound, ErrAPIRouteNotFound) {
		t.Error("ErrAPIRouteNotFound should match itself")
	}
	if !errors.Is(ErrInvalidUserInput, ErrInvalidUserInput) {
		t.Error("ErrInvalidUserInput should match itself")
	}
}

func TestCmdRunStreamTopicOmitEmpty(t *testing.T) {
	cmd := CmdRun{Command: "echo"}
	b, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	// StreamTopic should be omitted when empty.
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if _, ok := raw["stream_topic"]; ok {
		t.Error("stream_topic should be omitted when empty")
	}
}

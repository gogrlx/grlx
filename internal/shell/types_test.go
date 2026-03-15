package shell

import (
	"testing"
)

func TestSubjectPrefix(t *testing.T) {
	sessionID := "abc-123"
	got := SubjectPrefix(sessionID)
	want := "grlx.shell.abc-123"
	if got != want {
		t.Errorf("SubjectPrefix(%q) = %q, want %q", sessionID, got, want)
	}
}

func TestSubjects(t *testing.T) {
	sessionID := "test-session-42"
	s := Subjects(sessionID)

	if s.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", s.SessionID, sessionID)
	}
	if s.InputSubject != "grlx.shell.test-session-42.input" {
		t.Errorf("InputSubject = %q", s.InputSubject)
	}
	if s.OutputSubject != "grlx.shell.test-session-42.output" {
		t.Errorf("OutputSubject = %q", s.OutputSubject)
	}
	if s.ResizeSubject != "grlx.shell.test-session-42.resize" {
		t.Errorf("ResizeSubject = %q", s.ResizeSubject)
	}
	if s.DoneSubject != "grlx.shell.test-session-42.done" {
		t.Errorf("DoneSubject = %q", s.DoneSubject)
	}
}

func TestCLIStartRequestFields(t *testing.T) {
	req := CLIStartRequest{
		SproutID: "my-sprout",
		Cols:     120,
		Rows:     40,
		Shell:    "/bin/bash",
	}
	if req.SproutID != "my-sprout" {
		t.Error("SproutID mismatch")
	}
	if req.Cols != 120 || req.Rows != 40 {
		t.Error("dimensions mismatch")
	}
	if req.Shell != "/bin/bash" {
		t.Error("shell mismatch")
	}
}

func TestStartRequestDefaults(t *testing.T) {
	req := StartRequest{}
	if req.Shell != "" {
		t.Errorf("zero-value Shell should be empty, got %q", req.Shell)
	}
	if req.SessionID != "" {
		t.Errorf("zero-value SessionID should be empty, got %q", req.SessionID)
	}
}

func TestDoneMessage(t *testing.T) {
	dm := DoneMessage{ExitCode: 127, Error: "command not found"}
	if dm.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127", dm.ExitCode)
	}
	if dm.Error != "command not found" {
		t.Errorf("Error = %q", dm.Error)
	}
}

func TestResizeMessage(t *testing.T) {
	rm := ResizeMessage{Cols: 200, Rows: 50}
	if rm.Cols != 200 || rm.Rows != 50 {
		t.Errorf("ResizeMessage = {%d, %d}, want {200, 50}", rm.Cols, rm.Rows)
	}
}

package sshpicker

import (
	"bytes"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInit(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b", "c"},
	}
	cmd := m.Init()
	if cmd != nil {
		t.Fatal("expected nil cmd from Init")
	}
}

func TestNavigateDown(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b", "c"},
		Cursor:  0,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)
	if m.Cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.Cursor)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)
	if m.Cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", m.Cursor)
	}

	// At bottom — should not move.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)
	if m.Cursor != 2 {
		t.Fatalf("expected cursor 2 at bottom, got %d", m.Cursor)
	}
}

func TestNavigateUp(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b", "c"},
		Cursor:  2,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = result.(Model)
	if m.Cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.Cursor)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = result.(Model)
	if m.Cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.Cursor)
	}

	// At top — should not move.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = result.(Model)
	if m.Cursor != 0 {
		t.Fatalf("expected cursor 0 at top, got %d", m.Cursor)
	}
}

func TestArrowKeys(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b"},
		Cursor:  0,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(Model)
	if m.Cursor != 1 {
		t.Fatalf("expected cursor 1 after down arrow, got %d", m.Cursor)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(Model)
	if m.Cursor != 0 {
		t.Fatalf("expected cursor 0 after up arrow, got %d", m.Cursor)
	}
}

func TestCancel_Q(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b"},
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	final := result.(Model)
	if !final.Cancelled {
		t.Fatal("expected cancelled after q")
	}
}

func TestCancel_Esc(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b"},
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	final := result.(Model)
	if !final.Cancelled {
		t.Fatal("expected cancelled after esc")
	}
}

func TestSelected(t *testing.T) {
	m := Model{
		Sprouts: []string{"alpha", "beta", "gamma"},
		Cursor:  1,
	}
	if got := m.Selected(); got != "beta" {
		t.Fatalf("expected beta, got %s", got)
	}

	m.Cancelled = true
	if got := m.Selected(); got != "" {
		t.Fatalf("expected empty when cancelled, got %s", got)
	}
}

func TestView_ContainsSprouts(t *testing.T) {
	m := Model{
		Cohort:  "test-cohort",
		Sprouts: []string{"alpha", "beta", "gamma"},
		Cursor:  1,
	}

	view := m.View()
	if !strings.Contains(view, "alpha") {
		t.Fatal("view should contain sprout name 'alpha'")
	}
	if !strings.Contains(view, "beta") {
		t.Fatal("view should contain sprout name 'beta'")
	}
	if !strings.Contains(view, "gamma") {
		t.Fatal("view should contain sprout name 'gamma'")
	}
	if !strings.Contains(view, "test-cohort") {
		t.Fatal("view should contain cohort name")
	}
}

func TestView_SelectedMarker(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b"},
		Cursor:  0,
	}

	view := m.View()
	if !strings.Contains(view, "▸") {
		t.Fatal("view should contain selected marker ▸")
	}
}

func TestCancel_CtrlC(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b"},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	final := result.(Model)
	if !final.Cancelled {
		t.Fatal("expected cancelled after ctrl+c")
	}
	if cmd == nil {
		t.Fatal("expected quit command after ctrl+c")
	}
}

func TestEnterSelects(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"alpha", "beta"},
		Cursor:  1,
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := result.(Model)
	if final.Cancelled {
		t.Fatal("enter should not cancel")
	}
	if final.Selected() != "beta" {
		t.Fatalf("expected 'beta' selected, got %q", final.Selected())
	}
	if cmd == nil {
		t.Fatal("expected quit command after enter")
	}
}

func TestSelected_CursorOutOfRange(t *testing.T) {
	m := Model{
		Sprouts: []string{"only"},
		Cursor:  5, // beyond range
	}
	if got := m.Selected(); got != "" {
		t.Fatalf("expected empty for out-of-range cursor, got %q", got)
	}
}

func TestView_EmptySprouts(t *testing.T) {
	m := Model{
		Cohort:  "empty-cohort",
		Sprouts: []string{},
		Cursor:  0,
	}

	view := m.View()
	if !strings.Contains(view, "empty-cohort") {
		t.Fatal("view should contain cohort name even with no sprouts")
	}
	if !strings.Contains(view, "0 sprouts") {
		t.Fatal("view should show '0 sprouts' count")
	}
}

func TestUnknownKey(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b"},
		Cursor:  0,
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	final := result.(Model)

	// Unknown key should not change state
	if final.Cursor != 0 {
		t.Errorf("expected cursor 0, got %d", final.Cursor)
	}
	if final.Cancelled {
		t.Error("should not be cancelled")
	}
	if cmd != nil {
		t.Error("expected nil cmd for unknown key")
	}
}

func TestNavigateMixed(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a", "b", "c", "d"},
		Cursor:  0,
	}

	// Navigate down with arrow, up with k
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(Model)
	if m.Cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", m.Cursor)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = result.(Model)
	if m.Cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.Cursor)
	}
}

func TestRunWithOptions_SelectFirst(t *testing.T) {
	// Simulate pressing Enter immediately (select first item).
	input := bytes.NewReader([]byte("\r"))
	output := io.Discard

	selected, err := RunWithOptions("web", []string{"alpha", "beta", "gamma"},
		tea.WithInput(input), tea.WithOutput(output))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected != "alpha" {
		t.Errorf("expected 'alpha', got %q", selected)
	}
}

func TestRunWithOptions_SelectSingle(t *testing.T) {
	// Single-item list — Enter selects the only option.
	input := bytes.NewReader([]byte("\r"))
	output := io.Discard

	selected, err := RunWithOptions("web", []string{"single-sprout"},
		tea.WithInput(input), tea.WithOutput(output))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected != "single-sprout" {
		t.Errorf("expected 'single-sprout', got %q", selected)
	}
}

func TestRunWithOptions_Cancel(t *testing.T) {
	// Press 'q' to cancel.
	input := bytes.NewReader([]byte("q"))
	output := io.Discard

	_, err := RunWithOptions("web", []string{"alpha", "beta"},
		tea.WithInput(input), tea.WithOutput(output))
	if err == nil {
		t.Fatal("expected error for cancelled selection")
	}
	if err.Error() != "cancelled" {
		t.Errorf("expected 'cancelled' error, got %q", err.Error())
	}
}

func TestRun_WrapsRunWithOptions(t *testing.T) {
	// We can't test Run directly (it opens a real terminal), but we verify
	// that RunWithOptions works identically.
	input := bytes.NewReader([]byte("\r"))
	output := io.Discard

	selected, err := RunWithOptions("test", []string{"only"},
		tea.WithInput(input), tea.WithOutput(output))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected != "only" {
		t.Errorf("expected 'only', got %q", selected)
	}
}

func TestView_HelpText(t *testing.T) {
	m := Model{
		Cohort:  "web",
		Sprouts: []string{"a"},
		Cursor:  0,
	}

	view := m.View()
	if !strings.Contains(view, "enter") {
		t.Fatal("view should contain help text with 'enter'")
	}
	if !strings.Contains(view, "cancel") {
		t.Fatal("view should contain help text with 'cancel'")
	}
}

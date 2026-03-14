package sshpicker

import (
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

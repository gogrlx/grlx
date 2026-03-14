// Package sshpicker provides an interactive sprout picker for the SSH command
// when targeting a cohort that resolves to multiple sprouts.
package sshpicker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleNormal   = lipgloss.NewStyle()
	styleHeader   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	styleHelp     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// Model is the bubbletea model for the sprout picker.
type Model struct {
	Cohort    string
	Sprouts   []string
	Cursor    int
	Cancelled bool
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.Cancelled = true
			return m, tea.Quit
		case "enter":
			return m, tea.Quit
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Sprouts)-1 {
				m.Cursor++
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	var sb strings.Builder
	sb.WriteString(styleHeader.Render(fmt.Sprintf("Cohort %q — %d sprouts", m.Cohort, len(m.Sprouts))))
	sb.WriteString("\n\n")

	for i, sprout := range m.Sprouts {
		if i == m.Cursor {
			sb.WriteString(styleSelected.Render(fmt.Sprintf("  ▸ %s", sprout)))
		} else {
			sb.WriteString(styleNormal.Render(fmt.Sprintf("    %s", sprout)))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(styleHelp.Render("↑/k up • ↓/j down • enter select • q/esc cancel"))
	sb.WriteString("\n")
	return sb.String()
}

// Selected returns the selected sprout ID, or empty string if cancelled.
func (m Model) Selected() string {
	if m.Cancelled || m.Cursor >= len(m.Sprouts) {
		return ""
	}
	return m.Sprouts[m.Cursor]
}

// Run launches the interactive picker and returns the selected sprout ID.
func Run(cohort string, sprouts []string) (string, error) {
	m := Model{
		Cohort:  cohort,
		Sprouts: sprouts,
	}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("picker: %w", err)
	}

	final := result.(Model)
	if final.Cancelled {
		return "", fmt.Errorf("cancelled")
	}

	return final.Selected(), nil
}

package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gogrlx/grlx/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffef"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#ced4da"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	helpStyle           = blurredStyle.Copy()
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ced4da"))

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "get started with a new grlx installation",
	Run: func(cmd *cobra.Command, _ []string) {
		auth.CreatePrivkey()
		pubKey, err := auth.GetPubkey()
		if err != nil {
			log.Println("Error: " + err.Error())
			os.Exit(1)
		}
		model, err := tea.NewProgram(initialModel()).Run()
		if err == nil {
			mData := model.(configModel)
			fInterface := mData.inputs[0].Value()
			fAPIPort := mData.inputs[1].Value()
			fBusPort := mData.inputs[2].Value()
			if fInterface != "" {
				viper.Set("FarmerInterface", fInterface)
			}
			if fAPIPort != "" {
				viper.Set("FarmerAPIPort", fAPIPort)
			}
			if fBusPort != "" {
				viper.Set("FarmerBusPort", fBusPort)
			}
			viper.WriteConfig()
			fmt.Printf("Public key: %s\n", pubKey)
		} else {
			fmt.Printf("Error: opening the configuration interface. Please manually edit %s\n", viper.ConfigFileUsed())
			fmt.Printf("Public key: %s\n", pubKey)
			os.Exit(1)
		}
	},
}

type configModel struct {
	focusIndex      int
	inputs          []textinput.Model
	farmerAPIPort   int
	farmerBusPort   int
	farmerInterface string
	cursorMode      cursor.Mode
}

func initialModel() configModel {
	m := configModel{
		inputs: make([]textinput.Model, 3),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64

		switch i {
		case 0:
			t.Placeholder = "Farmer Interface"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "Farmer API Port (default: 5405)"
			t.CharLimit = 5
		case 2:
			t.Placeholder = "Farmer Bus Port (default: 5406)"
			t.CharLimit = 5
		}
		m.inputs[i] = t
	}

	return m
}

func (m configModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		// Change cursor mode
		case "ctrl+r":
			m.cursorMode++
			if m.cursorMode > cursor.CursorHide {
				m.cursorMode = cursor.CursorBlink
			}
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				cmds[i] = m.inputs[i].Cursor.SetMode(m.cursorMode)
			}
			return m, tea.Batch(cmds...)

		// Set focus to next input
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, exit.
			if s == "enter" && m.focusIndex == len(m.inputs) {
				return m, tea.Quit
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			return m, tea.Batch(cmds...)
		}
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	return m, cmd
}

func (m *configModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m configModel) View() string {
	var b strings.Builder
	b.WriteString("\nWelcome to " + focusedStyle.Render("grlx") + "! Update your config defaults below.\n\n")
	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	fmt.Fprintf(&b, "\n\n%s\n\n", *button)

	return b.String()
}

func init() {
	rootCmd.AddCommand(initCmd)
}

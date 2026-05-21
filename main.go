package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ==============================================================================
// Constants & Styling
// ==============================================================================

const (
	bgCol      = lipgloss.Color("#0d1016")
	activeCol  = lipgloss.Color("#5ac1fe")
	titleCol   = lipgloss.Color("#4b4c4e")
	dashCol    = lipgloss.Color("#3f4043")
	sideBgCol  = lipgloss.Color("#131721") // Slight contrast for side panels
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(activeCol).
			Bold(true).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Background(bgCol).
			Height(1)

	footerLeftStyle = lipgloss.NewStyle().
			Foreground(activeCol).
			Padding(0, 1)

	footerRightStyle = lipgloss.NewStyle().
				Foreground(titleCol).
				Padding(0, 1)

	// Base panel style
	panelStyle = lipgloss.NewStyle().
			Background(bgCol)
)

// ==============================================================================
// Model
// = :============================================================================

type model struct {
	width  int
	height int
	active string // "files", "editor", "terminal"
}

func initialModel() model {
	return model{
		active: "editor",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+1":
			m.active = "files"
		case "ctrl+2":
			m.active = "editor"
		case "ctrl+3":
			m.active = "terminal"
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// ==============================================================================
// View Helpers
// ==============================================================================

func renderHeader(m model, title string, width int, isActive bool) string {
	var color lipgloss.Color
	if isActive {
		color = activeCol
	} else {
		color = titleCol
	}

	label := fmt.Sprintf(" %s ", strings.ToUpper(title))
	leftDashes := "──"
	
	// Calculate remaining space for right dashes
	// width - 2 (left dashes) - length of label
	rightDashCount := width - 2 - len(label)
	if rightDashCount < 0 {
		rightDashCount = 0
	}
	rightDashes := strings.Repeat("─", rightDashCount)

	return lipgloss.NewStyle().Foreground(dashCol).Render(leftDashes) +
		lipgloss.NewStyle().Foreground(color).Render(label) +
		lipgloss.NewStyle().Foreground(dashCol).Render(rightDashes)
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Dimensions
	headerH := 1
	footerH := 1
	mainH := m.height - headerH - footerH

	filesW := m.width / 5
	editorW := (m.width * 2) / 5 // 40%
	terminalW := m.width - filesW - editorW // Remaining 40%

	// 1. Header
	header := lipgloss.NewStyle().
		Width(m.width).
		Height(headerH).
		Background(bgCol).
		Render(headerStyle.Render("T R I X"))

	// 2. Main Panels
	filesHeader := renderHeader(m, "Files", filesW, m.active == "files")
	filesPanel := lipgloss.NewStyle().
		Width(filesW).
		Height(mainH).
		Background(sideBgCol).
		Render(filesHeader)

	editorHeader := renderHeader(m, "Editor", editorW, m.active == "editor")
	editorPanel := lipgloss.NewStyle().
		Width(editorW).
		Height(mainH).
		Background(bgCol).
		Render(editorHeader)

	terminalHeader := renderHeader(m, "Terminal", terminalW, m.active == "terminal")
	terminalPanel := lipgloss.NewStyle().
		Width(terminalW).
		Height(mainH).
		Background(sideBgCol).
		Render(terminalHeader)

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, editorPanel, terminalPanel)

	// 3. Footer
	fLeft := footerLeftStyle.Render("TRIX")
	fRight := footerRightStyle.Render("F1 Help")
	spacer := lipgloss.NewStyle().Width(m.width - lipgloss.Width(fLeft) - lipgloss.Width(fRight)).Render("")
	footer := footerStyle.Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Left, fLeft, spacer, fRight))

	return lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

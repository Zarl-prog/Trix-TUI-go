package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ==============================================================================
// Constants & Styling
// ==============================================================================

const (
	bgCol       = lipgloss.Color("#0a0b0f")
	headerBgCol = lipgloss.Color("#0d0e14")
	activeCol   = lipgloss.Color("#5ac1fe")
	inactiveCol = lipgloss.Color("#1e2130")
	titleCol    = lipgloss.Color("#bfbdb6")
	mutedCol    = lipgloss.Color("#4b4c4e")
	versionCol  = lipgloss.Color("#3f4043")
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(activeCol).
			Bold(true)

	folderStyle = lipgloss.NewStyle().
			Foreground(mutedCol)

	versionStyle = lipgloss.NewStyle().
			Foreground(versionCol)

	pillStyle = lipgloss.NewStyle().
			Background(inactiveCol).
			Foreground(activeCol).
			Padding(0, 1).
			MarginLeft(1)

	basePanelStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder())
)

// ==============================================================================
// Model
// ==============================================================================

type model struct {
	width         int
	height        int
	active        string // "files", "editor", "terminal"
	currentFolder string
}

func initialModel() model {
	cwd, _ := os.Getwd()
	return model{
		active:        "editor",
		currentFolder: filepath.Base(cwd),
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

func renderPanel(m model, title, icon string, width, height int, active bool) string {
	borderColor := inactiveCol
	titleColor := mutedCol
	if active {
		borderColor = activeCol
		titleColor = activeCol
	}

	// Create the title string: " 📁 Files "
	titleStr := fmt.Sprintf(" %s %s ", icon, title)
	
	// Create the style for the panel
	style := basePanelStyle.Copy().
		Width(width).
		Height(height).
		BorderForeground(borderColor)

	if active {
		style = style.Bold(true)
	}

	// Lip Gloss doesn't natively support "title in border" easily with just Border()
	// So we render the panel and then overlay/preprocess the top border.
	// However, we can use the BorderTop(true) and custom rendering.
	// For simplicity and "stunning" look, we'll manually construct the top line.
	
	content := style.Render("")
	lines := strings.Split(content, "\n")
	
	if len(lines) > 0 {
		// Replace the first line (top border) with our custom titled border
		// ╭─ 📁 Files ──────────╮
		topBorderRunes := []rune(lines[0])
		titleRunes := []rune(titleStr)
		
		// Style the title part
		styledTitle := lipgloss.NewStyle().Foreground(titleColor).Render(string(titleRunes))
		if active {
			styledTitle = lipgloss.NewStyle().Foreground(activeCol).Bold(true).Render(string(titleRunes))
		}

		// Reconstruct the top line
		// ╭─
		left := string(topBorderRunes[:2])
		// ╮
		right := string(topBorderRunes[len(topBorderRunes)-1:])
		
		// Fill remaining space with ─
		// Length of middle = total width - len(left) - len(right)
		// But we need to account for the styled title length.
		middleWidth := width - 2 - 1 // approximate
		
		// Construct the top line manually to be safe
		topLine := lipgloss.NewStyle().Foreground(borderColor).Render("╭─") +
			styledTitle +
			lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", width-lipgloss.Width(styledTitle)-3)) +
			lipgloss.NewStyle().Foreground(borderColor).Render("╮")
			
		lines[0] = topLine
	}

	return strings.Join(lines, "\n")
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Dimensions
	headerH := 1
	footerH := 1
	mainH := m.height - headerH - footerH
	gap := 1

	// Widths: 20%, 45%, 35%
	filesW := (m.width * 20) / 100
	editorW := (m.width * 45) / 100
	terminalW := m.width - filesW - editorW - (gap * 2)

	// 1. Header
	brand := headerStyle.PaddingLeft(1).Render("T  R  I  X")
	folder := folderStyle.Width(m.width - lipgloss.Width(brand) - 15).Align(lipgloss.Center).Render(m.currentFolder)
	version := versionStyle.PaddingRight(1).Render("v0.1.0")
	
	header := lipgloss.NewStyle().
		Width(m.width).
		Height(headerH).
		Background(headerBgCol).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, brand, folder, version))

	// 2. Main Area
	filesPanel := renderPanel(m, "Files", "📁", filesW, mainH, m.active == "files")
	editorPanel := renderPanel(m, "Editor", "📝", editorW, mainH, m.active == "editor")
	terminalPanel := renderPanel(m, "Terminal", "💻", terminalW, mainH, m.active == "terminal")

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top,
		filesPanel,
		lipgloss.NewStyle().Width(gap).Render(""),
		editorPanel,
		lipgloss.NewStyle().Width(gap).Render(""),
		terminalPanel,
	)

	// 3. Status Bar
	statusBrand := lipgloss.NewStyle().Foreground(activeCol).PaddingLeft(1).Render("TRIX")
	statusActive := lipgloss.NewStyle().Foreground(titleCol).PaddingLeft(2).Render(strings.ToUpper(m.active))
	
	p1 := pillStyle.Render("[^1 Files]")
	p2 := pillStyle.Render("[^2 Editor]")
	p3 := pillStyle.Render("[^3 Terminal]")
	pq := pillStyle.Render("[q Quit]")
	pills := lipgloss.JoinHorizontal(lipgloss.Left, p1, p2, p3, pq)

	statusSpacer := lipgloss.NewStyle().Width(m.width - lipgloss.Width(statusBrand) - lipgloss.Width(statusActive) - lipgloss.Width(pills) - 1).Render("")
	
	footer := lipgloss.NewStyle().
		Width(m.width).
		Height(footerH).
		Background(headerBgCol).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, statusBrand, statusActive, statusSpacer, pills))

	return lipgloss.NewStyle().Background(bgCol).Render(lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer))
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

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
			Bold(true).
			Background(headerBgCol)

	folderStyle = lipgloss.NewStyle().
			Foreground(mutedCol).
			Background(headerBgCol)

	versionStyle = lipgloss.NewStyle().
			Foreground(versionCol).
			Background(headerBgCol)

	pillStyle = lipgloss.NewStyle().
			Background(inactiveCol).
			Foreground(activeCol).
			Padding(0, 1).
			MarginLeft(1)

	basePanelStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(bgCol)
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

func renderPanel(title, icon string, width, height int, active bool) string {
	borderColor := inactiveCol
	titleColor := titleCol
	if active {
		borderColor = activeCol
		titleColor = activeCol
	}

	titleStr := fmt.Sprintf(" %s %s ", icon, title)
	
	// Create the main panel style
	style := basePanelStyle.Copy().
		Width(width - 2). // Border takes 2 cells
		Height(height - 2).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(borderColor)

	if active {
		style = style.Bold(true)
	}

	// Render the panel content (empty for now)
	content := style.Render("")
	lines := strings.Split(content, "\n")
	
	if len(lines) > 0 {
		// Style the title
		titleStyle := lipgloss.NewStyle().Foreground(titleColor)
		if active {
			titleStyle = titleStyle.Bold(true)
		}
		styledTitle := titleStyle.Render(titleStr)

		// Border components
		borderStyle := lipgloss.NewStyle().Foreground(borderColor)
		leftCorner := borderStyle.Render("╭─")
		rightCorner := borderStyle.Render("╮")
		
		// Fill dashes to match exact width
		// width - leftCorner(2) - styledTitle - rightCorner(1)
		visibleTitleLen := lipgloss.Width(styledTitle)
		dashCount := width - 2 - visibleTitleLen - 1
		if dashCount < 0 { dashCount = 0 }
		dashes := borderStyle.Render(strings.Repeat("─", dashCount))

		lines[0] = leftCorner + styledTitle + dashes + rightCorner
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

	// --- 1. HEADER ---
	brand := headerStyle.PaddingLeft(1).Render("T  R  I  X")
	version := versionStyle.PaddingRight(1).Render("v0.1.0")
	
	// Middle folder name
	remainingHeaderW := m.width - lipgloss.Width(brand) - lipgloss.Width(version)
	if remainingHeaderW < 0 { remainingHeaderW = 0 }
	folder := folderStyle.Width(remainingHeaderW).Align(lipgloss.Center).Render(m.currentFolder)
	
	header := lipgloss.NewStyle().
		Width(m.width).
		Height(headerH).
		Background(headerBgCol).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, brand, folder, version))

	// --- 2. MAIN AREA ---
	filesPanel := renderPanel("Files", "📁", filesW, mainH, m.active == "files")
	editorPanel := renderPanel("Editor", "📝", editorW, mainH, m.active == "editor")
	terminalPanel := renderPanel("Terminal", "💻", terminalW, mainH, m.active == "terminal")

	spacer := lipgloss.NewStyle().Width(gap).Background(bgCol).Render("")

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top,
		filesPanel,
		spacer,
		editorPanel,
		spacer,
		terminalPanel,
	)

	// --- 3. STATUS BAR ---
	statusBrand := lipgloss.NewStyle().Foreground(activeCol).PaddingLeft(1).Render("TRIX")
	statusActive := lipgloss.NewStyle().Foreground(titleCol).PaddingLeft(2).Render(strings.ToUpper(m.active))
	
	p1 := pillStyle.Render("[^1 Files]")
	p2 := pillStyle.Render("[^2 Editor]")
	p3 := pillStyle.Render("[^3 Terminal]")
	pq := pillStyle.Render("[q Quit]")
	pills := lipgloss.JoinHorizontal(lipgloss.Left, p1, p2, p3, pq)

	remainingStatusW := m.width - lipgloss.Width(statusBrand) - lipgloss.Width(statusActive) - lipgloss.Width(pills)
	if remainingStatusW < 0 { remainingStatusW = 0 }
	statusSpacer := lipgloss.NewStyle().Width(remainingStatusW).Render("")
	
	footer := lipgloss.NewStyle().
		Width(m.width).
		Height(footerH).
		Background(headerBgCol).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, statusBrand, statusActive, statusSpacer, pills))

	// --- FINAL ASSEMBLY ---
	finalView := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)
	
	// Force background and full size
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Background(bgCol).
		Render(finalView)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

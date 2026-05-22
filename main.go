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
	screenBgCol   = lipgloss.Color("#070709")
	filesBgCol    = lipgloss.Color("#0e0f15")
	editorBgCol   = lipgloss.Color("#0a0b10")
	termBgCol      = lipgloss.Color("#0e0f15")
	headerBgCol   = lipgloss.Color("#050507")
	statusBarBgCol = lipgloss.Color("#050507")

	activeCol     = lipgloss.Color("#5ac1fe")
	inactiveCol   = lipgloss.Color("#1a1b26")
	
	logoTCol      = lipgloss.Color("#5ac1fe")
	logoRCol      = lipgloss.Color("#4ba8e0")
	logoICol      = lipgloss.Color("#3d8fc2")
	logoXCol      = lipgloss.Color("#5ac1fe")
	
	sepCol        = lipgloss.Color("#1a1b26")
	folderCol     = lipgloss.Color("#3d4166")
	versionCol    = lipgloss.Color("#2a2b3d")
	
	titleActiveCol   = lipgloss.Color("#5ac1fe")
	titleInactiveCol = lipgloss.Color("#2a2b3d")
	
	pillBgCol     = lipgloss.Color("#0e0f15")
	pillKeyCol    = lipgloss.Color("#5ac1fe")
	pillDescCol   = lipgloss.Color("#3d4166")
)

var (
	// Header Styles
	logoTStyle = lipgloss.NewStyle().Foreground(logoTCol).Bold(true)
	logoRStyle = lipgloss.NewStyle().Foreground(logoRCol).Bold(true)
	logoIStyle = lipgloss.NewStyle().Foreground(logoICol).Bold(true)
	logoXStyle = lipgloss.NewStyle().Foreground(logoXCol).Bold(true)
	
	sepStyle    = lipgloss.NewStyle().Foreground(sepCol)
	folderStyle = lipgloss.NewStyle().Foreground(folderCol)
	versionStyle = lipgloss.NewStyle().Foreground(versionCol)

	// Panel Styles
	activePanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeCol).
			Padding(1)

	inactivePanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(inactiveCol).
			Padding(1)

	// Status Bar Styles
	statusBrandStyle = lipgloss.NewStyle().Foreground(activeCol).Bold(true)
	statusActiveStyle = lipgloss.NewStyle().Foreground(folderCol)
	
	pillStyle = lipgloss.NewStyle().
			Background(pillBgCol).
			Border(lipgloss.NormalBorder(), false, true, false, true).
			BorderForeground(inactiveCol).
			Padding(0, 1).
			MarginLeft(1)
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

func renderPanel(title string, width, height int, active bool, bg lipgloss.Color) string {
	borderColor := inactiveCol
	titleColor := titleInactiveCol
	if active {
		borderColor = activeCol
		titleColor = titleActiveCol
	}

	// Base panel style
	style := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(borderColor).
		Background(bg)

	// Placeholder content
	var content string
	switch title {
	case "Files":
		c1 := lipgloss.NewStyle().Foreground(titleInactiveCol).Render("No folder open")
		c2 := lipgloss.NewStyle().Foreground(titleInactiveCol).Render("Press Ctrl+O to open")
		content = lipgloss.JoinVertical(lipgloss.Center, c1, c2)
	case "Editor":
		c1 := lipgloss.NewStyle().Foreground(activeCol).Bold(true).Render("Welcome to TRIX")
		c2 := lipgloss.NewStyle().Foreground(titleInactiveCol).Render("Open a file to start editing")
		content = lipgloss.JoinVertical(lipgloss.Center, c1, c2)
	case "Terminal":
		c1 := lipgloss.NewStyle().Foreground(titleInactiveCol).Render("Terminal ready")
		c2 := lipgloss.NewStyle().Foreground(titleInactiveCol).Render("Press Ctrl+3 to focus")
		content = lipgloss.JoinVertical(lipgloss.Center, c1, c2)
	}

	// Center content within the panel
	content = lipgloss.Place(width-2, height-2, lipgloss.Center, lipgloss.Center, content)

	// Render the panel with content
	rendered := style.Render(content)
	lines := strings.Split(rendered, "\n")

	if len(lines) > 0 {
		// Style the title: "   Title   "
		titleStr := fmt.Sprintf("   %s   ", title)
		titleStyle := lipgloss.NewStyle().Foreground(titleColor)
		if active {
			titleStyle = titleStyle.Bold(true)
		}
		styledTitle := titleStyle.Render(titleStr)

		// Top border components
		borderStyle := lipgloss.NewStyle().Foreground(borderColor).Background(bg)
		
		// RoundedBorder top-left is ╭, horizontal is ─
		// We need to reconstruct the top line: ╭─ Title ─────────╮
		leftCorner := borderStyle.Render("╭─")
		rightCorner := borderStyle.Render("╮")
		
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
	headerH := 2 // Header + separator line
	footerH := 2 // Status bar + separator line
	mainH := m.height - headerH - footerH
	gap := 1

	// Widths: 20%, 45%, 35%
	filesW := (m.width * 20) / 100
	editorW := (m.width * 45) / 100
	terminalW := m.width - filesW - editorW - (gap * 2)

	// --- 1. HEADER ---
	// Giant spaced logo: T  R  I  X
	logoT := logoTStyle.Render(" T ")
	logoR := logoRStyle.Render(" R ")
	logoI := logoIStyle.Render(" I ")
	logoX := logoXStyle.Render(" X ")
	logo := lipgloss.JoinHorizontal(lipgloss.Left, logoT, logoR, logoI, logoX)
	
	sep := sepStyle.Render(" │ ")
	version := versionStyle.PaddingRight(1).Render("v0.1.0")
	
	// Middle folder name
	remainingHeaderW := m.width - lipgloss.Width(logo) - lipgloss.Width(sep) - lipgloss.Width(version)
	if remainingHeaderW < 0 { remainingHeaderW = 0 }
	folder := folderStyle.Width(remainingHeaderW).Align(lipgloss.Center).Render(strings.ToUpper(m.currentFolder))
	
	headerContent := lipgloss.NewStyle().
		Width(m.width).
		Height(1).
		Background(headerBgCol).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, logo, sep, folder, version))
	
	headerSep := sepStyle.Width(m.width).Render(strings.Repeat("─", m.width))
	header := lipgloss.JoinVertical(lipgloss.Left, headerContent, headerSep)

	// --- 2. MAIN AREA ---
	filesPanel := renderPanel("Files", filesW, mainH, m.active == "files", filesBgCol)
	editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", editorBgCol)
	terminalPanel := renderPanel("Terminal", terminalW, mainH, m.active == "terminal", termBgCol)

	spacer := lipgloss.NewStyle().Width(gap).Background(screenBgCol).Render("")

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top,
		filesPanel,
		spacer,
		editorPanel,
		spacer,
		terminalPanel,
	)

	// --- 3. STATUS BAR ---
	statusBrand := statusBrandStyle.PaddingLeft(1).Render("TRIX")
	statusSep := sepStyle.Render(" │ ")
	statusActive := statusActiveStyle.Render(strings.ToUpper(m.active))
	
	// Right side pills
	renderPill := func(key, desc string) string {
		k := lipgloss.NewStyle().Foreground(pillKeyCol).Render(key)
		d := lipgloss.NewStyle().Foreground(pillDescCol).Render(" " + desc)
		return pillStyle.Render(k + d)
	}

	p1 := renderPill("⌃1", "Files")
	p2 := renderPill("⌃2", "Editor")
	p3 := renderPill("⌃3", "Terminal")
	pq := renderPill("q", "Quit")
	pills := lipgloss.JoinHorizontal(lipgloss.Left, p1, p2, p3, pq)

	remainingStatusW := m.width - lipgloss.Width(statusBrand) - lipgloss.Width(statusSep) - lipgloss.Width(statusActive) - lipgloss.Width(pills)
	if remainingStatusW < 0 { remainingStatusW = 0 }
	statusSpacer := lipgloss.NewStyle().Width(remainingStatusW).Render("")
	
	footerContent := lipgloss.NewStyle().
		Width(m.width).
		Height(1).
		Background(statusBarBgCol).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, statusBrand, statusSep, statusActive, statusSpacer, pills))
	
	footerSep := sepStyle.Width(m.width).Render(strings.Repeat("─", m.width))
	footer := lipgloss.JoinVertical(lipgloss.Left, footerSep, footerContent)

	// --- FINAL ASSEMBLY ---
	finalView := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)
	
	// Force background and full size
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Background(screenBgCol).
		Render(finalView)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

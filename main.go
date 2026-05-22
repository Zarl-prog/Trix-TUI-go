package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ==============================================================================
// Config Persistence
// ==============================================================================

type Config struct {
	Theme string `json:"theme"`
}

func getConfigFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".trix", "config.json")
}

func loadConfig() Config {
	file := getConfigFile()
	data, err := os.ReadFile(file)
	if err != nil {
		return Config{Theme: "Ayu Dark"}
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(themeName string) {
	file := getConfigFile()
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0755)
	cfg := Config{Theme: themeName}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(file, data, 0644)
}

// ==============================================================================
// Model
// ==============================================================================

type model struct {
	width         int
	height        int
	active        string // "files", "editor", "terminal"
	currentFolder string
	currentTheme  Theme
}

func initialModel() model {
	cwd, _ := os.Getwd()
	cfg := loadConfig()
	
	theme := AyuDark
	for _, t := range Themes {
		if t.Name == cfg.Theme {
			theme = t
			break
		}
	}

	return model{
		active:        "editor",
		currentFolder: filepath.Base(cwd),
		currentTheme:  theme,
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
		case "ctrl+t":
			// Cycle themes
			idx := 0
			for i, t := range Themes {
				if t.Name == m.currentTheme.Name {
					idx = (i + 1) % len(Themes)
					break
				}
			}
			m.currentTheme = Themes[idx]
			saveConfig(m.currentTheme.Name)
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

func renderPanel(title string, width, height int, active bool, theme Theme, isEditor bool) string {
	bg := theme.SurfaceAlt
	if isEditor {
		bg = theme.Surface
	}

	borderColor := theme.Border
	titleColor := theme.TextMuted
	if active {
		borderColor = theme.BorderFocused
		titleColor = theme.Accent
	}

	// Base panel style
	style := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		Background(lipgloss.Color(bg))

	// Placeholder content
	var content string
	switch title {
	case "Files":
		c1 := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("No folder open")
		c2 := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("Press Ctrl+O to open")
		content = lipgloss.JoinVertical(lipgloss.Center, c1, c2)
	case "Editor":
		c1 := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true).Render("Welcome to TRIX")
		c2 := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("Open a file to start editing")
		content = lipgloss.JoinVertical(lipgloss.Center, c1, c2)
	case "Terminal":
		c1 := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("Terminal ready")
		c2 := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("Press Ctrl+3 to focus")
		content = lipgloss.JoinVertical(lipgloss.Center, c1, c2)
	}

	// Center content within the panel using lipgloss.Place
	content = lipgloss.Place(width-2, height-2, lipgloss.Center, lipgloss.Center, content)

	// Render the inner content
	inner := style.Render(content)
	
	// Construct the panel with borders
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor)).Background(lipgloss.Color(bg))
	
	// Top border with title: ╭─  Files ─────────╮
	titleStr := fmt.Sprintf("  %s ", title)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(titleColor))
	if active {
		titleStyle = titleStyle.Bold(true)
	}
	styledTitle := titleStyle.Render(titleStr)
	
	leftCorner := borderStyle.Render("╭─")
	rightCorner := borderStyle.Render("╮")
	
	visibleTitleLen := lipgloss.Width(styledTitle)
	dashCount := width - 2 - visibleTitleLen - 1
	if dashCount < 0 { dashCount = 0 }
	dashes := borderStyle.Render(strings.Repeat("─", dashCount))
	
	topLine := leftCorner + styledTitle + dashes + rightCorner

	// Middle lines: │ content │
	sideBorder := borderStyle.Render("│")
	lines := strings.Split(inner, "\n")
	var middleArea strings.Builder
	for _, line := range lines {
		middleArea.WriteString(sideBorder + line + sideBorder + "\n")
	}

	// Bottom line: ╰───────────╯
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")

	return topLine + "\n" + middleArea.String() + bottomLine
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	t := m.currentTheme

	// Dimensions
	headerH := 1 
	headerSepH := 1
	footerH := 1 
	footerSepH := 1
	mainH := m.height - headerH - headerSepH - footerH - footerSepH
	gap := 1

	// Widths: 20%, 45%, 35%
	filesW := (m.width * 20) / 100
	editorW := (m.width * 45) / 100
	terminalW := m.width - filesW - editorW - (gap * 2)

	// Header Styles
	logoTStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt))
	logoRStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt))
	logoIStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt))
	logoXStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt))
	
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Background(lipgloss.Color(t.SurfaceAlt))
	folderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Background(lipgloss.Color(t.SurfaceAlt))
	themeNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Background(lipgloss.Color(t.SurfaceAlt)).PaddingRight(1)
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Background(lipgloss.Color(t.SurfaceAlt)).PaddingRight(1)

	// --- 1. HEADER ---
	logoT := logoTStyle.Render(" T ")
	logoR := logoRStyle.Render(" R ")
	logoI := logoIStyle.Render(" I ")
	logoX := logoXStyle.Render(" X ")
	logo := lipgloss.JoinHorizontal(lipgloss.Left, logoT, logoR, logoI, logoX)
	
	sep := sepStyle.Render(" │ ")
	themeName := themeNameStyle.Render(t.Name)
	version := versionStyle.Render("v0.1.0")
	rightInfo := lipgloss.JoinHorizontal(lipgloss.Left, themeName, version)
	
	// Middle folder name
	remainingHeaderW := m.width - lipgloss.Width(logo) - lipgloss.Width(sep) - lipgloss.Width(rightInfo)
	if remainingHeaderW < 0 { remainingHeaderW = 0 }
	folder := folderStyle.Width(remainingHeaderW).Align(lipgloss.Center).Render(strings.ToUpper(m.currentFolder))
	
	headerContent := lipgloss.NewStyle().
		Width(m.width).
		Height(1).
		Background(lipgloss.Color(t.SurfaceAlt)).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, logo, sep, folder, rightInfo))
	
	headerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Background(lipgloss.Color(t.Background)).Width(m.width).Render(strings.Repeat("─", m.width))
	header := lipgloss.JoinVertical(lipgloss.Left, headerContent, headerSep)

	// --- 2. MAIN AREA ---
	filesPanel := renderPanel("Files", filesW, mainH, m.active == "files", t, false)
	editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", t, true)
	terminalPanel := renderPanel("Terminal", terminalW, mainH, m.active == "terminal", t, false)

	spacer := lipgloss.NewStyle().Width(gap).Background(lipgloss.Color(t.Background)).Render("")

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top,
		filesPanel,
		spacer,
		editorPanel,
		spacer,
		terminalPanel,
	)

	// --- 3. STATUS BAR ---
	statusBrandStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt))
	statusActiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Background(lipgloss.Color(t.SurfaceAlt))
	pillStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(t.Surface)).
		Border(lipgloss.NormalBorder(), false, true, false, true).
		BorderForeground(lipgloss.Color(t.Border)).
		Padding(0, 1).
		MarginLeft(1)

	statusBrand := statusBrandStyle.PaddingLeft(1).Render("TRIX")
	statusSep := sepStyle.Render(" │ ")
	statusActive := statusActiveStyle.Render(strings.ToUpper(m.active))
	
	// Right side pills
	renderPill := func(key, desc string) string {
		k := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Render(key)
		d := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Render(" " + desc)
		return pillStyle.Render(k + d)
	}

	p1 := renderPill("⌃1", "Files")
	p2 := renderPill("⌃2", "Editor")
	p3 := renderPill("⌃3", "Terminal")
	pq := renderPill("q", "Quit")
	pills := lipgloss.JoinHorizontal(lipgloss.Left, p1, p2, p3, pq)

	remainingStatusW := m.width - lipgloss.Width(statusBrand) - lipgloss.Width(statusSep) - lipgloss.Width(statusActive) - lipgloss.Width(pills)
	if remainingStatusW < 0 { remainingStatusW = 0 }
	statusSpacer := lipgloss.NewStyle().Width(remainingStatusW).Background(lipgloss.Color(t.SurfaceAlt)).Render("")
	
	footerContent := lipgloss.NewStyle().
		Width(m.width).
		Height(1).
		Background(lipgloss.Color(t.SurfaceAlt)).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, statusBrand, statusSep, statusActive, statusSpacer, pills))
	
	footerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Background(lipgloss.Color(t.Background)).Width(m.width).Render(strings.Repeat("─", m.width))
	footer := lipgloss.JoinVertical(lipgloss.Left, footerSep, footerContent)

	// --- FINAL ASSEMBLY ---
	finalView := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)
	
	// Force background and full size
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Background(lipgloss.Color(t.Background)).
		Render(finalView)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

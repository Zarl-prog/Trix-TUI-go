package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ==============================================================================
// Types & Messages
// ==============================================================================

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type FileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Path  string `json:"path"`
}

type listDirMsg struct {
	Entries []FileEntry
	Error   string
}

type readFileMsg struct {
	Content string
	Path    string
	Error   string
}

type writeFileMsg struct {
	Error string
}

type runCommandMsg struct {
	Output string
	Error  string
}

// ==============================================================================
// Model
// ==============================================================================

type model struct {
	width           int
	height          int
	active          string // "files", "editor", "terminal"
	currentFolder   string
	currentPath     string
	currentTheme    Theme
	
	// Editor
	textarea        textarea.Model
	
	// Files
	files           []FileEntry
	fileCursor      int
	
	// Terminal
	terminalBuf     *strings.Builder
	terminalInput   string
	terminalHistory []string
	terminalHistIdx int
	cursorVisible   bool
	
	// Overlays
	overlayMode     string // "", "open_folder"
	overlayInput    string
	
	// Status
	statusMsg       string
	isError         bool
	
	bridge          *Bridge
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

	b, _ := NewBridge("python") // Assume python is in PATH

	ta := textarea.New()
	ta.Placeholder = "Open a file to start editing..."
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color(theme.CursorLine))
	ta.ShowLineNumbers = true

	return model{
		active:          "files",
		currentFolder:   filepath.Base(cwd),
		currentTheme:    theme,
		textarea:        ta,
		terminalBuf:     &strings.Builder{},
		terminalHistIdx: -1,
		bridge:          b,
	}
}

// ==============================================================================
// Commands
// ==============================================================================

func listDir(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("list_dir", map[string]interface{}{"path": path})
		if err != nil {
			return listDirMsg{Error: err.Error()}
		}
		var data struct {
			Status  string      `json:"status"`
			Entries []FileEntry `json:"entries"`
			Message string      `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return listDirMsg{Error: data.Message}
		}
		return listDirMsg{Entries: data.Entries}
	}
}

func readFile(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("read_file", map[string]interface{}{"path": path})
		if err != nil {
			return readFileMsg{Error: err.Error()}
		}
		var data struct {
			Status  string `json:"status"`
			Content string `json:"content"`
			Message string `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return readFileMsg{Error: data.Message}
		}
		return readFileMsg{Content: data.Content, Path: path}
	}
}

func writeFile(b *Bridge, path, content string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("write_file", map[string]interface{}{"path": path, "content": content})
		if err != nil {
			return writeFileMsg{Error: err.Error()}
		}
		var data struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return writeFileMsg{Error: data.Message}
		}
		return writeFileMsg{}
	}
}

func runCommand(b *Bridge, command string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("run_command", map[string]interface{}{"command": command})
		if err != nil {
			return runCommandMsg{Error: err.Error()}
		}
		var data struct {
			Status  string `json:"status"`
			Output  string `json:"output"`
			Message string `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return runCommandMsg{Error: data.Message}
		}
		return runCommandMsg{Output: data.Output}
	}
}

func waitForEvent(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		return <-b.Events()
	}
}

func (m model) Init() tea.Cmd {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cwd = sanitizePath(cwd)
	
	return tea.Batch(
		tickCmd(),
		waitForEvent(m.bridge),
		listDir(m.bridge, cwd),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		m.cursorVisible = !m.cursorVisible
		return m, tickCmd()

	case listDirMsg:
		if msg.Error != "" {
			m.statusMsg = "ListDir error: " + msg.Error
			m.isError = true
		} else {
			m.files = msg.Entries
			m.fileCursor = 0
		}

	case readFileMsg:
		if msg.Error != "" {
			m.statusMsg = "Read error: " + msg.Error
			m.isError = true
		} else {
			m.currentPath = msg.Path
			m.textarea.SetValue(msg.Content)
			m.active = "editor"
			m.textarea.Focus()
			m.statusMsg = "Opened " + filepath.Base(msg.Path)
			m.isError = false
		}

	case writeFileMsg:
		if msg.Error != "" {
			m.statusMsg = "Save error: " + msg.Error
			m.isError = true
		} else {
			m.statusMsg = "Saved " + filepath.Base(m.currentPath)
			m.isError = false
		}

	case runCommandMsg:
		if msg.Error != "" {
			m.terminalBuf.WriteString("\nError: " + msg.Error + "\n")
		} else {
			m.terminalBuf.WriteString("\n" + msg.Output)
		}

	case RPCEvent:
		if msg.Event == "terminal_data" {
			var data struct {
				Data string `json:"data"`
			}
			json.Unmarshal(msg.Data, &data)
			m.terminalBuf.WriteString(data.Data)
		}
		return m, waitForEvent(m.bridge)

	case tea.KeyMsg:
		if m.overlayMode != "" {
			switch msg.String() {
			case "enter":
				if m.overlayMode == "open_folder" {
					path := m.overlayInput
					m.overlayMode = ""
					m.overlayInput = ""
					m.currentFolder = filepath.Base(path)
					return m, listDir(m.bridge, path)
				}
			case "esc":
				m.overlayMode = ""
				m.overlayInput = ""
				return m, nil
			case "backspace":
				if len(m.overlayInput) > 0 {
					m.overlayInput = m.overlayInput[:len(m.overlayInput)-1]
				}
				return m, nil
			}
			if len(msg.String()) == 1 {
				m.overlayInput += msg.String()
			}
			return m, nil
		}

		// Global bindings
		switch msg.String() {
		case "ctrl+c":
			if m.active != "terminal" {
				return m, tea.Quit
			}
		case "ctrl+q":
			return m, tea.Quit
		case "ctrl+1":
			m.active = "files"
			m.textarea.Blur()
		case "ctrl+2":
			m.active = "editor"
			m.textarea.Focus()
		case "ctrl+3":
			m.active = "terminal"
			m.textarea.Blur()
		case "ctrl+s":
			if m.currentPath != "" {
				return m, writeFile(m.bridge, m.currentPath, m.textarea.Value())
			}
		case "ctrl+o":
			m.overlayMode = "open_folder"
			m.overlayInput = ""
			return m, nil
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
			m.textarea.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color(m.currentTheme.CursorLine))
			saveConfig(m.currentTheme.Name)
		}

		case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && m.overlayMode == "" {
			filesW := (m.width * 20) / 100
			gap := 1
			editorW := (m.width * 45) / 100
			editorStart := filesW + gap
			termStart := editorStart + editorW + gap

			switch {
			case msg.X < filesW:
				m.active = "files"
				m.textarea.Blur()
			case msg.X >= editorStart && msg.X < termStart:
				m.active = "editor"
				m.textarea.Focus()
			case msg.X >= termStart:
				m.active = "terminal"
				m.textarea.Blur()
			}
		}

	// Panel specific bindings
		switch m.active {
		case "files":
			switch msg.String() {
			case "up", "k":
				if m.fileCursor > 0 {
					m.fileCursor--
				}
			case "down", "j":
				if m.fileCursor < len(m.files)-1 {
					m.fileCursor++
				}
			case "enter":
				if len(m.files) > 0 {
					entry := m.files[m.fileCursor]
					if entry.IsDir {
						return m, listDir(m.bridge, entry.Path)
					} else {
						return m, readFile(m.bridge, entry.Path)
					}
				}
			case "backspace":
				return m, listDir(m.bridge, "..")
			}

		case "editor":
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd

		case "terminal":
			switch msg.String() {
			case "enter":
				if m.terminalInput != "" {
					m.terminalHistory = append(m.terminalHistory, m.terminalInput)
					m.terminalBuf.WriteString("\n> " + m.terminalInput + "\n")
					cmd := runCommand(m.bridge, m.terminalInput)
					m.terminalInput = ""
					m.terminalHistIdx = -1
					return m, cmd
				}
			case "up":
				if len(m.terminalHistory) > 0 {
					if m.terminalHistIdx == -1 {
						m.terminalHistIdx = len(m.terminalHistory) - 1
					} else if m.terminalHistIdx > 0 {
						m.terminalHistIdx--
					}
					m.terminalInput = m.terminalHistory[m.terminalHistIdx]
				}
			case "down":
				if m.terminalHistIdx != -1 {
					if m.terminalHistIdx < len(m.terminalHistory)-1 {
						m.terminalHistIdx++
						m.terminalInput = m.terminalHistory[m.terminalHistIdx]
					} else {
						m.terminalHistIdx = -1
						m.terminalInput = ""
					}
				}
			case "backspace":
				if len(m.terminalInput) > 0 {
					m.terminalInput = m.terminalInput[:len(m.terminalInput)-1]
				}
			case "ctrl+l":
				m.terminalBuf.Reset()
			default:
				if len(msg.String()) == 1 {
					m.terminalInput += msg.String()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update panel widths for textarea resize
		editorW := (m.width * 45) / 100
		innerEditorW := editorW - 2
		mainH := m.height - 4 // header(2) + footer(2)
		m.textarea.SetWidth(innerEditorW)
		m.textarea.SetHeight(mainH - 2)
	}
	return m, tea.Batch(cmds...)
}

// ==============================================================================
// View Helpers
// ==============================================================================

func renderPanel(title string, width, height int, active bool, theme Theme, content string) string {
	bg := theme.SurfaceAlt
	if title == "Editor" {
		bg = theme.Surface
	}

	borderColor := theme.Border
	titleColor := theme.TextMuted
	if active {
		borderColor = theme.BorderFocused
		titleColor = theme.Accent
	}

	innerWidth := width - 2
	innerHeight := height - 2

	var styledContent string
	if content == "" {
		styledContent = lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, "...", lipgloss.WithWhitespaceBackground(lipgloss.Color(bg)))
	} else {
		// Truncate/pad lines to fit innerWidth/innerHeight
		lines := strings.Split(content, "\n")
		var processed []string
		for i := 0; i < innerHeight; i++ {
			if i < len(lines) {
				line := lines[i]
				if lipgloss.Width(line) > innerWidth {
					line = line[:innerWidth] // Simple truncation
				}
				processed = append(processed, line+strings.Repeat(" ", innerWidth-lipgloss.Width(line)))
			} else {
				processed = append(processed, strings.Repeat(" ", innerWidth))
			}
		}
		styledContent = strings.Join(processed, "\n")
	}

	// Border logic
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor)).Background(lipgloss.Color(bg))
	displayTitle := fmt.Sprintf("  %s ", title)
	styledTitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(titleColor)).
		Background(lipgloss.Color(bg)).
		Bold(active).
		Render(displayTitle)
	
	leftCorner := borderStyle.Render("╭─")
	rightCorner := borderStyle.Render("╮")
	dashCount := width - 2 - lipgloss.Width(styledTitle) - 1
	if dashCount < 0 { dashCount = 0 }
	topLine := leftCorner + styledTitle + borderStyle.Render(strings.Repeat("─", dashCount)) + rightCorner

	sideBorder := borderStyle.Render("│")
	contentLines := strings.Split(styledContent, "\n")
	var middleArea strings.Builder
	for i, line := range contentLines {
		if i >= innerHeight { break }
		middleArea.WriteString(sideBorder + line + sideBorder)
		if i < len(contentLines)-1 {
			middleArea.WriteString("\n")
		}
	}

	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")

	return topLine + "\n" + middleArea.String() + "\n" + bottomLine
}

func renderFiles(width, height int, active bool, theme Theme, files []FileEntry, cursor int) string {
	innerWidth := width - 2
	innerHeight := height - 2

	var lines []string
	for i, f := range files {
		if len(lines) >= innerHeight { break }
		
		prefix := "  "
		if f.IsDir {
			prefix = "📁 "
		}
		
		name := f.Name
		if lipgloss.Width(name) > innerWidth-4 {
			name = name[:innerWidth-7] + "..."
		}
		
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text))
		if i == cursor && active {
			style = style.Background(lipgloss.Color(theme.Accent)).Foreground(lipgloss.Color(theme.Background))
		} else if f.IsDir {
			style = style.Foreground(lipgloss.Color(theme.AccentAlt))
		}
		
		line := prefix + name
		lines = append(lines, style.Render(line+strings.Repeat(" ", innerWidth-lipgloss.Width(line))))
	}
	
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}
	
	return renderPanel("Files", width, height, active, theme, strings.Join(lines, "\n"))
}

func renderTerminal(width, height int, active bool, theme Theme, content string, input string, cursorVisible bool) string {
	innerWidth := width - 2
	innerHeight := height - 2

	outputHeight := innerHeight - 1
	rawLines := strings.Split(content, "\n")
	
	var lines []string
	if len(rawLines) > outputHeight {
		rawLines = rawLines[len(rawLines)-outputHeight:]
	}
	
	for _, line := range rawLines {
		if lipgloss.Width(line) > innerWidth {
			line = line[:innerWidth]
		}
		lines = append(lines, line+strings.Repeat(" ", innerWidth-lipgloss.Width(line)))
	}
	for len(lines) < outputHeight {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	// Input line
	prompt := "> "
	cursor := ""
	if cursorVisible && active {
		cursor = "█"
	}
	
	inputLine := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Render(prompt) + input + cursor
	if lipgloss.Width(inputLine) > innerWidth {
		inputLine = inputLine[lipgloss.Width(inputLine)-innerWidth:]
	}
	inputLine += strings.Repeat(" ", innerWidth-lipgloss.Width(inputLine))
	
	full := strings.Join(lines, "\n") + "\n" + inputLine
	return renderPanel("Terminal", width, height, active, theme, full)
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
	if mainH < 0 { mainH = 0 }
	gap := 1

	// Widths
	filesW := (m.width * 20) / 100
	editorW := (m.width * 45) / 100
	terminalW := m.width - filesW - editorW - (gap * 2)

	// --- 1. HEADER ---
	logoStyle := func(color string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt))
	}
	logo := lipgloss.JoinHorizontal(lipgloss.Left, 
		logoStyle(t.Accent).Render(" T "), 
		logoStyle(t.AccentAlt).Render(" R "), 
		logoStyle(t.AccentAlt).Render(" I "), 
		logoStyle(t.Accent).Render(" X "))
	
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" │ ")
	folder := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Background(lipgloss.Color(t.SurfaceAlt)).
		Width(m.width - lipgloss.Width(logo) - 20).Align(lipgloss.Center).Render(strings.ToUpper(m.currentFolder))
	
	headerContent := lipgloss.NewStyle().Width(m.width).Background(lipgloss.Color(t.SurfaceAlt)).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, logo, sep, folder))
	
	headerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(m.width).Render(strings.Repeat("─", m.width))
	header := lipgloss.JoinVertical(lipgloss.Left, headerContent, headerSep)

	// --- 2. MAIN AREA ---
	filesPanel := renderFiles(filesW, mainH, m.active == "files", t, m.files, m.fileCursor)
	
	// Editor Panel with Textarea
	editorView := m.textarea.View()
	editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", t, editorView)
	
	terminalPanel := renderTerminal(terminalW, mainH, m.active == "terminal", t, m.terminalBuf.String(), m.terminalInput, m.cursorVisible)

	spacer := lipgloss.NewStyle().Width(gap).Render("")
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, spacer, editorPanel, spacer, terminalPanel)

	// --- 3. STATUS BAR ---
	statusColor := t.Accent
	if m.isError {
		statusColor = t.Error
	}
	
	statusBrand := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt)).Padding(0, 1).Render("TRIX")
	statusText := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + m.statusMsg)
	
	remainingW := m.width - lipgloss.Width(statusBrand) - lipgloss.Width(statusText) - 30
	if remainingW < 0 { remainingW = 0 }
	statusSpacer := lipgloss.NewStyle().Width(remainingW).Background(lipgloss.Color(t.SurfaceAlt)).Render("")
	
	pills := lipgloss.NewStyle().Background(lipgloss.Color(t.Surface)).Foreground(lipgloss.Color(t.Accent)).Padding(0, 1).Render("⌃1 Files  ⌃2 Editor  ⌃3 Term  ⌃S Save  ⌃Q Quit")
	
	footerContent := lipgloss.NewStyle().Width(m.width).Background(lipgloss.Color(t.SurfaceAlt)).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, statusBrand, statusText, statusSpacer, pills))
	
	footerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(m.width).Render(strings.Repeat("─", m.width))
	footer := lipgloss.JoinVertical(lipgloss.Left, footerSep, footerContent)

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)

	if m.overlayMode != "" {
		overlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			renderOverlay(m.overlayMode, m.overlayInput, t),
		)
		return overlay
	}

	return finalView
}

func renderOverlay(mode, input string, theme Theme) string {
	width := 40
	title := "Open Folder"
	if mode == "open_folder" {
		title = "Open Folder Path:"
	}

	bg := theme.Surface
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Background(lipgloss.Color(bg)).Border(lipgloss.RoundedBorder())
	
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true).Padding(0, 1)
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text)).Background(lipgloss.Color(theme.Background)).Width(width - 4).Padding(0, 1)
	
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(title),
		"",
		inputStyle.Render(input+"█"),
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render(" [Enter] confirm   [Esc] cancel"),
	)
	
	return borderStyle.Render(content)
}

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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

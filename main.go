package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ==============================================================================
// Types & Constants
// ==============================================================================

var (
	langMap = map[string]string{
		".py": "Python", ".go": "Go", ".js": "JavaScript", ".ts": "TypeScript",
		".jsx": "JavaScript", ".tsx": "TypeScript", ".rs": "Rust", ".c": "C",
		".cpp": "C++", ".h": "C", ".hpp": "C++", ".java": "Java",
		".rb": "Ruby", ".php": "PHP", ".sh": "Bash", ".bash": "Bash",
		".json": "JSON", ".yaml": "YAML", ".yml": "YAML", ".toml": "TOML",
		".md": "Markdown", ".html": "HTML", ".htm": "HTML", ".css": "CSS",
		".sql": "SQL", ".xml": "XML", ".svg": "XML",
	}
)

type Theme struct {
	Name       string
	Background lipgloss.Color
	Border     lipgloss.Color
	Active     lipgloss.Color
	Inactive   lipgloss.Color
	Muted      lipgloss.Color
	File       lipgloss.Color
	Folder     lipgloss.Color
	Surface    lipgloss.Color
	Unsaved    lipgloss.Color
	BottomBar  lipgloss.Color
	DividerBg  lipgloss.Color
}

var Themes = []Theme{
	{
		Name: "Ayu Dark",
		Background: "#0d1016", Border: "#3f4043", Active: "#5ac1fe",
		Inactive: "#bfbdb6", Muted: "#8a8986", File: "#8a8986",
		Folder: "#bfbdb6", Surface: "#1f2127", Unsaved: "#feb454",
		BottomBar: "#131721", DividerBg: "#1a1d23",
	},
	{
		Name: "Ayu Light",
		Background: "#fafafa", Border: "#d9d8d7", Active: "#55b4d4",
		Inactive: "#5c6166", Muted: "#8a9199", File: "#8a9199",
		Folder: "#5c6166", Surface: "#f3f4f5", Unsaved: "#fa8d3e",
		BottomBar: "#e7e8e9", DividerBg: "#d9d8d7",
	},
	{
		Name: "Ayu Mirage",
		Background: "#1f2430", Border: "#3f4043", Active: "#5ccfe6",
		Inactive: "#cccac2", Muted: "#8a9199", File: "#8a9199",
		Folder: "#cccac2", Surface: "#2a2f3a", Unsaved: "#ffa759",
		BottomBar: "#171b24", DividerBg: "#242936",
	},
}

type FileNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Depth    int        `json:"depth"`
	Children []FileNode `json:"children,omitempty"`
}

type SearchMatch struct {
	Line     int
	ColStart int
	ColEnd   int
}

type GlobalResult struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type model struct {
	bridge        *Bridge
	err           error
	width         int
	height        int
	active        string // "files", "editor", "terminal"
	rootNode      FileNode
	cursor        int
	flatTree      []FileNode
	textarea      textarea.Model
	terminalLog   viewport.Model
	terminalBuf   *strings.Builder
	terminalInput string
	terminalHistory []string
	terminalHistIdx int
	currentPath   string
	currentLang   string
	hasChanges    bool
	expanded      map[string]bool
	gitBranch     string
	gitDirty      bool
	
	// Layout
	dragging      bool
	dragDivider   int // 1 = files|editor, 2 = editor|terminal
	dragStartX    int
	filesWidth    int
	editorWidth   int
	zenMode       bool
	fileTreeVisible bool

	// Overlays
	overlayMode   string // "", "new_file", "rename", "delete_confirm", "open_folder", "quit_confirm"
	overlayInput  string
	overlayTitle  string
	helpOpen      bool

	// Search
	searchOpen    bool
	searchQuery   string
	searchMatches []SearchMatch
	searchIdx     int
	globalSearchOpen    bool
	globalSearchQuery   string
	globalSearchResults []GlobalResult
	globalSearchIdx     int

	themeIdx      int
	cursorBlink   bool
}

func (m model) currentTheme() Theme {
	return Themes[m.themeIdx]
}

func flattenVisible(node FileNode, expanded map[string]bool, depth int) []FileNode {
	node.Depth = depth
	res := []FileNode{node}
	if node.IsDir && expanded[node.Path] {
		for _, child := range node.Children {
			res = append(res, flattenVisible(child, expanded, depth+1)...)
		}
	}
	return res
}

// ==============================================================================
// Bridge Commands
// ==============================================================================

type FileContent struct {
	Path    string
	Content string
}

type TerminalData struct {
	Data string `json:"data"`
}

type GitInfo struct {
	Status string `json:"status"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
}

type GlobalSearchResults struct {
	Results []GlobalResult `json:"results"`
}

func fetchFileTree(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("get_file_tree", map[string]string{"path": path})
		if err != nil { return err }
		var result struct {
			Status string   `json:"status"`
			Tree   FileNode `json:"tree"`
		}
		json.Unmarshal(res, &result)
		return result.Tree
	}
}

func readFile(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("read_file", map[string]string{"path": path})
		if err != nil { return err }
		var result struct {
			Status  string `json:"status"`
			Content string `json:"content"`
		}
		json.Unmarshal(res, &result)
		return FileContent{Path: path, Content: result.Content}
	}
}

func saveFile(b *Bridge, path, content string) tea.Cmd {
	return func() tea.Msg {
		_, err := b.Call("save_file", map[string]string{"path": path, "content": content})
		if err != nil { return err }
		return "saved"
	}
}

func createFile(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		b.Call("create_file", map[string]string{"path": path})
		return fetchFileTree(b, ".")()
	}
}

func deleteFile(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		b.Call("delete_file", map[string]string{"path": path})
		return fetchFileTree(b, ".")()
	}
}

func renameFile(b *Bridge, old, new string) tea.Cmd {
	return func() tea.Msg {
		b.Call("rename_file", map[string]string{"old_path": old, "new_path": new})
		return fetchFileTree(b, ".")()
	}
}

func fetchGitInfo(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		res, _ := b.Call("get_git_branch", map[string]string{"path": "."})
		var info GitInfo
		json.Unmarshal(res, &info)
		return info
	}
}

func globalSearch(b *Bridge, query string) tea.Cmd {
	return func() tea.Msg {
		res, _ := b.Call("search_files", map[string]string{"root": ".", "query": query})
		var result struct {
			Status  string         `json:"status"`
			Results []GlobalResult `json:"results"`
		}
		json.Unmarshal(res, &result)
		return GlobalSearchResults{Results: result.Results}
	}
}

func spawnTerminal(b *Bridge, rows, cols int) tea.Cmd {
	return func() tea.Msg {
		b.Call("terminal_spawn", map[string]int{"rows": rows, "cols": cols})
		return "terminal_spawned"
	}
}

func writeTerminal(b *Bridge, data string) tea.Cmd {
	return func() tea.Msg {
		b.Call("terminal_write", map[string]string{"data": data})
		return nil
	}
}

func waitForEvents(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		return <-b.Events()
	}
}

type blinkMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return blinkMsg{}
	})
}

// ==============================================================================
// Model Implementation
// ==============================================================================

func initialModel() model {
	b, _ := NewBridge("python")
	ta := textarea.New()
	ta.Placeholder = "No file open"
	
	vp := viewport.New(0, 0)
	
	return model{
		bridge:   b,
		active:   "terminal",
		textarea: ta,
		terminalLog: vp,
		terminalBuf: &strings.Builder{},
		terminalHistIdx: -1,
		expanded: map[string]bool{".": true},
		themeIdx: 0,
		fileTreeVisible: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		fetchFileTree(m.bridge, "."),
		spawnTerminal(m.bridge, 24, 80),
		fetchGitInfo(m.bridge),
		waitForEvents(m.bridge),
		textarea.Blink,
		tickCmd(),
	)
}

func findMatches(content, query string) []SearchMatch {
	var matches []SearchMatch
	if query == "" { return matches }
	lines := strings.Split(content, "\n")
	qLower := strings.ToLower(query)
	for row, line := range lines {
		lineLower := strings.ToLower(line)
		col := 0
		for {
			idx := strings.Index(lineLower[col:], qLower)
			if idx == -1 { break }
			abs := col + idx
			matches = append(matches, SearchMatch{row, abs, abs + len(query)})
			col = abs + 1
		}
	}
	return matches
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case blinkMsg:
		m.cursorBlink = !m.cursorBlink
		return m, tickCmd()
	case RPCEvent:
		if msg.Event == "terminal_data" {
			var td TerminalData
			if err := json.Unmarshal(msg.Data, &td); err == nil {
				m.terminalBuf.WriteString(td.Data)
				m.terminalLog.SetContent(m.terminalBuf.String())
				m.terminalLog.GotoBottom()
			}
		}
		return m, waitForEvents(m.bridge)
	case FileNode:
		m.rootNode = msg
		m.flatTree = flattenVisible(m.rootNode, m.expanded, 0)
	case FileContent:
		m.currentPath = msg.Path
		m.textarea.SetValue(msg.Content)
		m.hasChanges = false
		m.active = "editor"
		m.textarea.Focus()
		ext := filepath.Ext(m.currentPath)
		if lang, ok := langMap[ext]; ok {
			m.currentLang = lang
		} else {
			m.currentLang = "Plain Text"
		}
	case GitInfo:
		m.gitBranch = msg.Branch
		m.gitDirty = msg.Dirty
	case GlobalSearchResults:
		m.globalSearchResults = msg.Results
		m.globalSearchIdx = 0
	case string:
		if msg == "saved" {
			m.hasChanges = false
			return m, fetchGitInfo(m.bridge)
		}
	case error:
		m.err = msg
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		filesW := m.width / 5
		divW := 1
		remaining := m.width - filesW - (divW * 2)
		editorW := remaining / 2
		termW := remaining - editorW
		
		m.textarea.SetWidth(editorW)
		m.textarea.SetHeight(m.height - 3)
		m.terminalLog.Width = termW
		m.terminalLog.Height = m.height - 3
	case tea.MouseMsg:
		filesWidth := m.filesWidth
		if filesWidth == 0 { filesWidth = m.width / 5 }
		divW := 1
		remaining := m.width - filesWidth - (divW * 2)
		editorWidth := m.editorWidth
		if editorWidth == 0 { editorWidth = remaining / 2 }
		editorStart := filesWidth + divW
		termStart := editorStart + editorWidth + divW

		if msg.Type == tea.MouseLeft {
			if msg.Action == tea.MouseActionPress {
				// Focus panels
				if !m.zenMode {
					switch {
					case msg.X < filesWidth:
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
				
				// Divider dragging
				if !m.zenMode {
					if msg.X == filesWidth {
						m.dragging = true
						m.dragDivider = 1
						m.dragStartX = msg.X
					} else if msg.X == termStart-1 { // editor|terminal divider
						m.dragging = true
						m.dragDivider = 2
						m.dragStartX = msg.X
					}
				}
			} else if msg.Action == tea.MouseActionMotion && m.dragging {
				if m.dragDivider == 1 {
					m.filesWidth = msg.X
				} else {
					m.editorWidth = msg.X - editorStart
				}
			} else if msg.Action == tea.MouseActionRelease {
				m.dragging = false
			}
		}
	case tea.KeyMsg:
		// Overlays
		if m.helpOpen {
			if msg.String() == "f1" || msg.String() == "esc" {
				m.helpOpen = false
			}
			return m, nil
		}
		
		if m.overlayMode != "" {
			switch msg.String() {
			case "esc":
				m.overlayMode = ""
				m.overlayInput = ""
			case "enter":
				mode := m.overlayMode
				input := m.overlayInput
				m.overlayMode = ""
				m.overlayInput = ""
				switch mode {
				case "new_file": return m, createFile(m.bridge, input)
				case "rename": return m, renameFile(m.bridge, m.currentPath, input)
				case "delete_confirm":
					if input == "y" || input == "Y" {
						return m, deleteFile(m.bridge, m.flatTree[m.cursor].Path)
					}
				case "open_folder":
					return m, tea.Batch(
						fetchFileTree(m.bridge, input),
						func() tea.Msg { return "folder_opened" },
					)
				case "quit_confirm":
					if input == "y" || input == "Y" { return m, tea.Quit }
				}
			case "backspace":
				if len(m.overlayInput) > 0 { m.overlayInput = m.overlayInput[:len(m.overlayInput)-1] }
			default:
				if msg.Type == tea.KeyRunes || msg.String() == " " {
					m.overlayInput += msg.String()
				}
			}
			return m, nil
		}

		// Inline search
		if m.searchOpen {
			switch msg.String() {
			case "esc": m.searchOpen = false
			case "enter":
				if len(m.searchMatches) > 0 {
					m.searchIdx = (m.searchIdx + 1) % len(m.searchMatches)
				}
			case "backspace":
				if len(m.searchQuery) > 0 { m.searchQuery = m.searchQuery[:len(m.searchQuery)-1] }
				m.searchMatches = findMatches(m.textarea.Value(), m.searchQuery)
				m.searchIdx = 0
			default:
				if msg.Type == tea.KeyRunes || msg.String() == " " {
					m.searchQuery += msg.String()
					m.searchMatches = findMatches(m.textarea.Value(), m.searchQuery)
					m.searchIdx = 0
				}
			}
			return m, nil
		}
		
		// Global Search overlay
		if m.globalSearchOpen {
			switch msg.String() {
			case "esc": m.globalSearchOpen = false
			case "enter":
				if len(m.globalSearchResults) > 0 {
					res := m.globalSearchResults[m.globalSearchIdx]
					m.globalSearchOpen = false
					return m, readFile(m.bridge, res.File)
				}
			case "up":
				if m.globalSearchIdx > 0 { m.globalSearchIdx-- }
			case "down":
				if m.globalSearchIdx < len(m.globalSearchResults)-1 { m.globalSearchIdx++ }
			case "backspace":
				if len(m.globalSearchQuery) > 0 { m.globalSearchQuery = m.globalSearchQuery[:len(m.globalSearchQuery)-1] }
				return m, globalSearch(m.bridge, m.globalSearchQuery)
			default:
				if msg.Type == tea.KeyRunes || msg.String() == " " {
					m.globalSearchQuery += msg.String()
					return m, globalSearch(m.bridge, m.globalSearchQuery)
				}
			}
			return m, nil
		}

		// Global Shortcuts
		switch msg.String() {
		case "ctrl+q":
			if m.hasChanges {
				m.overlayMode = "quit_confirm"
				m.overlayTitle = "Unsaved changes. Quit anyway? (y/n)"
				return m, nil
			}
			return m, tea.Quit
		case "f1": m.helpOpen = true; return m, nil
		case "ctrl+t": m.themeIdx = (m.themeIdx + 1) % len(Themes); return m, nil
		case "ctrl+b": m.fileTreeVisible = !m.fileTreeVisible; return m, nil
		case "ctrl+\\": m.zenMode = !m.zenMode; return m, nil
		case "ctrl+o": m.overlayMode = "open_folder"; m.overlayTitle = "Open folder path:"; return m, nil
		case "ctrl+n": m.overlayMode = "new_file"; m.overlayTitle = "New file name:"; return m, nil
		case "ctrl+f": m.searchOpen = true; m.searchQuery = ""; m.searchMatches = nil; return m, nil
		case "ctrl+shift+f": m.globalSearchOpen = true; m.globalSearchQuery = ""; m.globalSearchResults = nil; return m, nil
		case "ctrl+s":
			if m.currentPath != "" { return m, saveFile(m.bridge, m.currentPath, m.textarea.Value()) }
		case "ctrl+r": return m, tea.Batch(fetchFileTree(m.bridge, "."), fetchGitInfo(m.bridge))
		case "ctrl+]":
			switch m.active {
			case "files": m.active = "editor"; m.textarea.Focus()
			case "editor": m.active = "terminal"; m.textarea.Blur()
			case "terminal": m.active = "files"
			}
			return m, nil
		}

		// Active panel updates
		if m.active == "terminal" {
			switch msg.String() {
			case "enter":
				m.terminalHistory = append(m.terminalHistory, m.terminalInput)
				m.terminalHistIdx = -1
				cmd := writeTerminal(m.bridge, m.terminalInput+"\r\n")
				m.terminalInput = ""
				return m, cmd
			case "up":
				if len(m.terminalHistory) > 0 {
					if m.terminalHistIdx == -1 { m.terminalHistIdx = len(m.terminalHistory) - 1 } else if m.terminalHistIdx > 0 { m.terminalHistIdx-- }
					m.terminalInput = m.terminalHistory[m.terminalHistIdx]
				}
				return m, nil
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
				return m, nil
			case "ctrl+l":
				m.terminalBuf.Reset()
				m.terminalLog.SetContent("")
				return m, writeTerminal(m.bridge, "clear\r\n")
			case "ctrl+c": return m, writeTerminal(m.bridge, "\x03")
			case "backspace":
				if len(m.terminalInput) > 0 { m.terminalInput = m.terminalInput[:len(m.terminalInput)-1] }
				return m, nil
			default:
				if msg.Type == tea.KeyRunes || msg.String() == " " {
					m.terminalInput += msg.String()
					return m, nil
				}
			}
		}

		if m.active == "files" {
			switch msg.String() {
			case "up": if m.cursor > 0 { m.cursor-- }
			case "down": if m.cursor < len(m.flatTree)-1 { m.cursor++ }
			case "enter":
				node := m.flatTree[m.cursor]
				if node.IsDir {
					m.expanded[node.Path] = !m.expanded[node.Path]
					m.flatTree = flattenVisible(m.rootNode, m.expanded, 0)
				} else {
					return m, readFile(m.bridge, node.Path)
				}
			case "f2":
				if len(m.flatTree) > 0 {
					m.overlayMode = "rename"
					m.overlayTitle = "Rename to:"
					m.overlayInput = m.flatTree[m.cursor].Name
				}
			case "delete":
				if len(m.flatTree) > 0 {
					m.overlayMode = "delete_confirm"
					m.overlayTitle = "Delete " + m.flatTree[m.cursor].Name + "? (y/n)"
				}
			}
		}

		if m.active == "editor" {
			oldVal := m.textarea.Value()
			m.textarea, cmd = m.textarea.Update(msg)
			if m.textarea.Value() != oldVal {
				m.hasChanges = true
			}
			cmds = append(cmds, cmd)
		}
	}

	if m.active == "terminal" {
		m.terminalLog, cmd = m.terminalLog.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func renderFileTree(m model, width, height int) string {
	var s strings.Builder
	theme := m.currentTheme()
	for i, fn := range m.flatTree {
		if i >= height { break }
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor && m.active == "files" {
			cursor = "> "
			style = style.Background(theme.Active).Foreground(theme.Background).Bold(true)
		} else {
			if fn.IsDir { style = style.Foreground(theme.Folder) } else { style = style.Foreground(theme.File) }
		}
		icon := "  "
		if fn.IsDir {
			if m.expanded[fn.Path] { icon = "▼ " } else { icon = "▶ " }
		} else { icon = "─ " }
		indent := strings.Repeat("  ", fn.Depth)
		line := fmt.Sprintf("%s%s%s%s", cursor, indent, icon, fn.Name)
		if len(line) > width { line = line[:width] }
		s.WriteString(style.Width(width).Render(line) + "\n")
	}
	return s.String()
}

func renderPanelHeader(m model, title string, width int, active bool) string {
	theme := m.currentTheme()
	textColor := theme.Inactive
	if active { textColor = theme.Active }
	label := fmt.Sprintf(" %s ", title)
	leftDashes := 2
	rightDashes := width - leftDashes - len(label)
	if rightDashes < 0 { rightDashes = 0 }
	return lipgloss.NewStyle().Foreground(theme.Border).Render("──") +
		lipgloss.NewStyle().Foreground(textColor).Render(label) +
		lipgloss.NewStyle().Foreground(theme.Border).Render(strings.Repeat("─", rightDashes))
}

func (m model) View() string {
	if m.err != nil { return fmt.Sprintf("Error: %v\n", m.err) }
	if m.width == 0 || m.height == 0 { return "Initializing..." }
	theme := m.currentTheme()

	if m.zenMode {
		m.textarea.SetWidth(m.width)
		m.textarea.SetHeight(m.height - 1)
		view := m.textarea.View()
		hint := lipgloss.NewStyle().Foreground(theme.Muted).Render("ctrl+\\ exit zen")
		return view + "\n" + lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(hint)
	}

	headerH, footerH := 1, 1
	mainH := m.height - headerH - footerH
	
	filesW := m.filesWidth
	if filesW == 0 { filesW = m.width / 5 }
	if !m.fileTreeVisible { filesW = 0 }
	
	divW := 1
	remaining := m.width - filesW - (divW * 2)
	if !m.fileTreeVisible { remaining = m.width - divW }
	
	editorW := m.editorWidth
	if editorW == 0 { editorW = remaining / 2 }
	termW := remaining - editorW

	// Styles for panels to ensure they take full space and have background
	panelStyle := lipgloss.NewStyle().Height(mainH).Background(theme.Background)
	dividerStyle := lipgloss.NewStyle().Width(divW).Height(mainH).Foreground(theme.Border).Background(theme.Background)

	// Header
	brand := lipgloss.NewStyle().Foreground(theme.Active).Bold(true).Render(" TRIX")
	folder := lipgloss.NewStyle().Foreground(theme.Inactive).Width(m.width - 25).Align(lipgloss.Center).Render(".")
	themeLabel := lipgloss.NewStyle().Foreground(theme.Inactive).Render(theme.Name + " ")
	header := lipgloss.NewStyle().Height(headerH).Width(m.width).Background(theme.Background).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, brand, folder, themeLabel))

	// Panels
	var panels []string
	if m.fileTreeVisible {
		filesHeader := renderPanelHeader(m, "Files", filesW, m.active == "files")
		filesContent := renderFileTree(m, filesW, mainH-1)
		panels = append(panels, panelStyle.Width(filesW).Render(lipgloss.JoinVertical(lipgloss.Left, filesHeader, filesContent)))
		panels = append(panels, dividerStyle.Render("│"))
	}
	
	// Editor
	m.textarea.SetWidth(editorW)
	m.textarea.SetHeight(mainH - 1)
	editorContent := m.textarea.View()
	if m.searchOpen {
		m.textarea.SetHeight(mainH - 2)
		matchInfo := fmt.Sprintf(" %d of %d ", m.searchIdx+1, len(m.searchMatches))
		if len(m.searchMatches) == 0 { matchInfo = " No matches " }
		searchBar := lipgloss.NewStyle().Background(theme.Surface).Width(editorW).Render(
			lipgloss.NewStyle().Foreground(theme.Active).Render(" Search: ") + m.searchQuery + "█" + 
			lipgloss.NewStyle().Width(editorW - len(m.searchQuery) - 30).Render("") + 
			lipgloss.NewStyle().Foreground(theme.Muted).Render(matchInfo + " [Enter] next [Esc] close"),
		)
		editorContent = lipgloss.JoinVertical(lipgloss.Left, searchBar, m.textarea.View())
	}
	editorHeader := renderPanelHeader(m, "Editor", editorW, m.active == "editor")
	panels = append(panels, panelStyle.Width(editorW).Render(lipgloss.JoinVertical(lipgloss.Left, editorHeader, editorContent)))
	
	panels = append(panels, dividerStyle.Render("│"))
	
	// Terminal
	m.terminalLog.Width, m.terminalLog.Height = termW, mainH - 2
	termCursor := ""
	if m.cursorBlink && m.active == "terminal" { termCursor = "█" }
	termInput := lipgloss.NewStyle().Foreground(theme.Active).Render("> ") + m.terminalInput + termCursor
	terminalHeader := renderPanelHeader(m, "Terminal", termW, m.active == "terminal")
	panels = append(panels, panelStyle.Width(termW).Render(lipgloss.JoinVertical(lipgloss.Left, terminalHeader, m.terminalLog.View(), termInput)))

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, panels...)

	// Footer (Status Bar)
	filename := m.currentPath
	if filename == "" { filename = "No file" }
	unsaved := ""
	if m.hasChanges { unsaved = lipgloss.NewStyle().Foreground(theme.Unsaved).Render(" *") }
	
	line, col := m.textarea.Line(), m.textarea.LineInfo().ColumnOffset
	cursorPos := fmt.Sprintf(" Ln %d, Col %d ", line+1, col+1)
	
	gitInfo := ""
	if m.gitBranch != "" {
		dot := ""
		if m.gitDirty { dot = lipgloss.NewStyle().Foreground(theme.Unsaved).Render(" ●") }
		gitInfo = fmt.Sprintf(" %s%s ", m.gitBranch, dot)
	}

	leftStatus := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Background(theme.Active).Foreground(theme.Background).Bold(true).Render(" TRIX "),
		lipgloss.NewStyle().Foreground(theme.Active).Render(" "+filename+unsaved+" "),
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(cursorPos),
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(gitInfo),
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(" "+m.currentLang+" "),
	)
	keyHints := lipgloss.NewStyle().Foreground(theme.Inactive).Render(" ^S Save  ^R Reload  ^] Cycle  ^C Quit ")
	spacerWidth := m.width - lipgloss.Width(leftStatus) - lipgloss.Width(keyHints)
	if spacerWidth < 0 { spacerWidth = 0 }
	spacer := lipgloss.NewStyle().Width(spacerWidth).Render("")
	footer := lipgloss.NewStyle().Background(theme.BottomBar).Width(m.width).Render(lipgloss.JoinHorizontal(lipgloss.Left, leftStatus, spacer, keyHints))

	view := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)

	// Overlays
	if m.globalSearchOpen {
		w, h := m.width-10, 15
		overlay := lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.RoundedBorder()).BorderForeground(theme.Active).Background(theme.Surface).Padding(0, 1).
			Render(lipgloss.JoinVertical(lipgloss.Left, 
				lipgloss.NewStyle().Foreground(theme.Active).Render(" Search in Files: ") + m.globalSearchQuery + "█",
				strings.Repeat("─", w-2),
				func() string {
					var lines []string
					for i, res := range m.globalSearchResults {
						if len(lines) >= h-4 { break }
						style := lipgloss.NewStyle().Foreground(theme.Inactive)
						if i == m.globalSearchIdx { style = style.Foreground(theme.Active).Bold(true).Background(theme.Background) }
						lines = append(lines, style.Render(fmt.Sprintf("%s:%d %s", res.File, res.Line, res.Text)))
					}
					return strings.Join(lines, "\n")
				}()))
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	} else if m.overlayMode != "" {
		w := 40
		overlay := lipgloss.NewStyle().Width(w).Border(lipgloss.RoundedBorder()).BorderForeground(theme.Active).Background(theme.Surface).Padding(0, 1).
			Render(lipgloss.JoinVertical(lipgloss.Left, 
				" " + m.overlayTitle,
				lipgloss.NewStyle().Foreground(theme.Active).Render(" > ") + m.overlayInput + "█",
				lipgloss.NewStyle().Foreground(theme.Muted).Render(" [Enter] confirm   [Esc] cancel")))
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	} else if m.helpOpen {
		helpText := ` Keyboard Shortcuts

 File
   Ctrl+N  New File         Ctrl+S  Save
   Ctrl+W  Close File       Ctrl+O  Open Folder
   F2      Rename File      Delete  Delete File

 Layout
   Ctrl+]  Cycle Panels     Click   Focus Panel
   Ctrl+B  Toggle Files     Ctrl+\  Zen Mode

 Editor
   Ctrl+Z  Undo             Ctrl+Y  Redo
   Ctrl+A  Select All       Ctrl+F  Search
   Ctrl+Shift+F Global Search

 Terminal
   Enter   Run command      Ctrl+C  Interrupt
   ↑ ↓     History          Ctrl+L  Clear

 General
   Ctrl+T  Cycle Theme      Ctrl+Q  Quit
   F1      Close this help`
		overlay := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.Active).Background(theme.Surface).Padding(0, 1).Render(helpText)
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	}

	return view
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

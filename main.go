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
	Path    string
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
	Output    string
	Error     string
	IsAICmd   bool // true if this was triggered from AI terminal
}

type gitBranchMsg struct {
	Branch string
	Dirty  bool
	Error  string
}

type searchResultsMsg struct {
	Results []GlobalResult
	Error   string
}

// ==============================================================================
// Model
// ==============================================================================

type GlobalResult struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type SearchMatch struct {
	Line     int
	ColStart int
	ColEnd   int
}

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Depth    int
	Children []FileNode
}

type model struct {
	width           int
	height          int
	active          string // "files", "editor", "terminal"
	currentFolder   string
	currentPath     string
	currentTheme    Theme
	themeIdx        int
	
	// Editor
	textarea        textarea.Model
	
	// Files
	rootNode        FileNode
	flatTree        []FileNode
	expanded        map[string]bool
	fileCursor      int
	
	// Terminal
	terminalBuf     *strings.Builder
	terminalInput   string
	terminalHistory []string
	terminalHistIdx int
	cursorVisible   bool
	
	// Overlays
	overlayMode     string // "", "open_folder", "new_file", "rename", "delete_confirm", "quit_confirm"
	overlayInput    string
	overlayTitle    string
	
	// Undo/Redo
	undoStack       []string
	redoStack       []string
	maxUndo         int
	
	// Status
	statusMsg       string
	isError         bool
	currentLang     string
	gitBranch       string
	gitDirty        bool
	cursorLine      int
	cursorCol       int
	hasChanges      bool
	
	// Search
	searchOpen      bool
	searchQuery     string
	searchMatches   []SearchMatch
	searchIdx       int
	
	// Global search
	globalSearchOpen    bool
	globalSearchQuery   string
	globalSearchResults []GlobalResult
	globalSearchIdx     int
	
	// Layout modes: 0=Classic (default), 1=AI Developer, 2=Focus, 3=Terminal Focus
	layoutMode        int
	
	// AI Terminal (right panel in AI Developer & Terminal Focus modes)
	aiTerminalBuf     *strings.Builder
	aiTerminalInput   string
	aiTerminalHistory []string
	aiTerminalHistIdx int
	aiAgentName       string // "Claude", "Codex", "Gemini", "Aider", "Custom", or ""
	aiAgentAnimTick   bool   // toggles for animated ● indicator
	
	// Toggles
	zenMode           bool
	fileTreeVisible   bool
	dragging          bool
	dragDivider       int // 1 = files|editor, 2 = editor|terminal, 3 = editor|ai-terminal
	dragStartX        int
	filesWidth        int // override, 0 = default
	editorWidth       int // override, 0 = default
	helpOpen          bool
	
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

	// Find theme index to match saved config
	themeIdx := 0
	for i, t := range Themes {
		if t.Name == cfg.Theme {
			themeIdx = i
			break
		}
	}

	layoutMode := cfg.Layout
	// If saved layout is AI Developer, Focus, or Terminal Focus, start with file tree hidden
	fileTreeVis := true
	if layoutMode != 0 {
		fileTreeVis = false
	}

	return model{
		active:            "files",
		currentFolder:     filepath.Base(cwd),
		currentTheme:      theme,
		themeIdx:          themeIdx,			textarea:          ta,
			undoStack:         []string{},
			redoStack:         []string{},
			maxUndo:           100,
		terminalBuf:       &strings.Builder{},
		terminalHistIdx:   -1,
		aiTerminalBuf:     &strings.Builder{},
		aiTerminalHistIdx: -1,
		layoutMode:        layoutMode,
		currentLang:       "Plain Text",
		expanded:          map[string]bool{".": true},
		fileTreeVisible:   fileTreeVis,
		bridge:            b,
	}
}

// ==============================================================================
// Commands
// ==============================================================================

func listDir(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("list_dir", map[string]interface{}{"path": path})
		if err != nil {
			return listDirMsg{Path: path, Error: err.Error()}
		}
		var data struct {
			Status  string      `json:"status"`
			Entries []FileEntry `json:"entries"`
			Message string      `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return listDirMsg{Path: path, Error: data.Message}
		}
		return listDirMsg{Path: path, Entries: data.Entries}
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

func runCommand(b *Bridge, command string, isAICmd bool) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("run_command", map[string]interface{}{"command": command})
		if err != nil {
			return runCommandMsg{Error: err.Error(), IsAICmd: isAICmd}
		}
		var data struct {
			Status  string `json:"status"`
			Output  string `json:"output"`
			Message string `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return runCommandMsg{Error: data.Message, IsAICmd: isAICmd}
		}
		return runCommandMsg{Output: data.Output, IsAICmd: isAICmd}
	}
}

func terminalWrite(b *Bridge, data string) tea.Cmd {
	return func() tea.Msg {
		_, err := b.Call("terminal_write", map[string]interface{}{"data": data})
		if err != nil {
			return runCommandMsg{Error: err.Error()}
		}
		return nil
	}
}

func startTerminalCmd(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		_, err := b.Call("start_terminal", nil)
		if err != nil {
			return runCommandMsg{Error: err.Error()}
		}
		return nil
	}
}

func waitForEvent(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		return <-b.Events()
	}
}

func fetchGitBranch(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("get_git_branch", map[string]interface{}{"path": path})
		if err != nil {
			return gitBranchMsg{Error: err.Error()}
		}
		var data struct {
			Status  string `json:"status"`
			Branch  string `json:"branch"`
			Dirty   bool   `json:"dirty"`
			Message string `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return gitBranchMsg{Error: data.Message}
		}
		return gitBranchMsg{Branch: data.Branch, Dirty: data.Dirty}
	}
}

func searchFiles(b *Bridge, root, query string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("search_files", map[string]interface{}{"root": root, "query": query})
		if err != nil {
			return searchResultsMsg{Error: err.Error()}
		}
		var data struct {
			Status  string         `json:"status"`
			Results []GlobalResult `json:"results"`
			Message string         `json:"message"`
		}
		json.Unmarshal(res, &data)
		if data.Status == "error" {
			return searchResultsMsg{Error: data.Message}
		}
		return searchResultsMsg{Results: data.Results}
	}
}

func findMatches(content, query string) []SearchMatch {
	var matches []SearchMatch
	if query == "" {
		return matches
	}
	lines := strings.Split(content, "\n")
	qLower := strings.ToLower(query)
	for row, line := range lines {
		lineLower := strings.ToLower(line)
		col := 0
		for {
			idx := strings.Index(lineLower[col:], qLower)
			if idx == -1 {
				break
			}
			abs := col + idx
			matches = append(matches, SearchMatch{row, abs, abs + len(query)})
			col = abs + 1
		}
	}
	return matches
}

// ==============================================================================
// Tree Helpers
// ==============================================================================

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

func findTreeNode(root *FileNode, path string) *FileNode {
	if root.Path == path {
		return root
	}
	for i := range root.Children {
		if found := findTreeNode(&root.Children[i], path); found != nil {
			return found
		}
	}
	return nil
}

func nodeFromEntries(entries []FileEntry) []FileNode {
	nodes := make([]FileNode, 0, len(entries))
	for _, e := range entries {
		nodes = append(nodes, FileNode{
			Name:  e.Name,
			Path:  e.Path,
			IsDir: e.IsDir,
		})
	}
	return nodes
}

func flattenTree(root FileNode, expanded map[string]bool) []FileNode {
	if !root.IsDir || !expanded[root.Path] {
		return nil
	}
	var result []FileNode
	for _, child := range root.Children {
		result = append(result, flattenVisible(child, expanded, 0)...)
	}
	return result
}

// ==============================================================================
// AI Terminal Helpers
// ==============================================================================

func (m *model) submitAITerminal() tea.Cmd {
	if m.aiTerminalInput == "" {
		return nil
	}
	m.aiTerminalHistory = append(m.aiTerminalHistory, m.aiTerminalInput)
	m.aiTerminalBuf.WriteString("\nAI ❯ " + m.aiTerminalInput + "\n")
	cmd := runCommand(m.bridge, m.aiTerminalInput, true)
	m.aiTerminalInput = ""
	m.aiTerminalHistIdx = -1
	m.active = "terminal"
	m.textarea.Blur()
	return cmd
}

func (m *model) injectContextFilePath() tea.Cmd {
	if m.currentPath == "" {
		m.statusMsg = "No file open to inject"
		m.isError = true
		return nil
	}
	contextText := "Here is my current file: " + m.currentPath
	if m.layoutMode == 0 {
		m.terminalInput = contextText
		m.statusMsg = "Injected file path into Terminal"
	} else {
		m.aiTerminalInput = contextText
		m.statusMsg = "Injected file path into AI Terminal"
	}
	m.isError = false
	m.active = "terminal"
	m.textarea.Blur()
	return nil
}

func (m *model) injectContextSelection() tea.Cmd {
	if m.currentPath == "" {
		m.statusMsg = "No file open to inject from"
		m.isError = true
		return nil
	}
	content := m.textarea.Value()
	fname := filepath.Base(m.currentPath)
	contextText := "Here is the current file (" + fname + "):"

	lines := strings.Split(content, "\n")
	cursorLine := m.cursorLine
	if cursorLine < len(lines) {
		start := cursorLine - 3
		if start < 0 { start = 0 }
		end := cursorLine + 4
		if end > len(lines) { end = len(lines) }
		snippet := strings.Join(lines[start:end], "\n")
		contextText += "\n" + snippet
	}

	if m.layoutMode == 0 {
		m.terminalInput = contextText
		m.statusMsg = "Injected context into Terminal"
	} else {
		m.aiTerminalInput = contextText
		m.statusMsg = "Injected context into AI Terminal"
	}
	m.isError = false
	m.active = "terminal"
	m.textarea.Blur()
	return nil
}

func (m *model) injectContextFullFile() tea.Cmd {
	if m.currentPath == "" {
		m.statusMsg = "No file open to inject"
		m.isError = true
		return nil
	}
	fname := filepath.Base(m.currentPath)
	content := m.textarea.Value()
	contextText := "Here is my current file (" + fname + "):\n" + content + "\n\nPlease help me with:"

	if m.layoutMode == 0 {
		m.terminalInput = contextText
		m.statusMsg = "Injected full file into Terminal"
	} else {
		m.aiTerminalInput = contextText
		m.statusMsg = "Injected full file into AI Terminal"
	}
	m.isError = false
	m.active = "terminal"
	m.textarea.Blur()
	return nil
}

// ==============================================================================
// Layout helpers
// ==============================================================================

func cycleLayout(m *model) {
	m.layoutMode = (m.layoutMode + 1) % 4
	switch m.layoutMode {
	case 0: // Classic
		m.fileTreeVisible = true
	case 1, 2, 3: // AI Developer, Focus, Terminal Focus
		m.fileTreeVisible = false
	}
	if m.layoutMode != 0 && m.active == "files" {
		m.active = "editor"
		m.textarea.Focus()
	}
	saveConfig(m.currentTheme.Name, m.layoutMode)
	layoutNames := []string{"Layout: Classic", "Layout: AI Developer", "Layout: Focus", "Layout: Terminal Focus"}
	m.statusMsg = layoutNames[m.layoutMode]
	m.isError = false
}

// ==============================================================================
// Init
// ==============================================================================

func (m model) Init() tea.Cmd {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cwd = sanitizePath(cwd)
	m.currentFolder = filepath.Base(cwd)
	
	return tea.Batch(
		tickCmd(),
		waitForEvent(m.bridge),
		listDir(m.bridge, cwd),
		fetchGitBranch(m.bridge, cwd),
		startTerminalCmd(m.bridge),
	)
}

// ==============================================================================
// Update
// ==============================================================================

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		m.cursorVisible = !m.cursorVisible
		m.aiAgentAnimTick = !m.aiAgentAnimTick
		return m, tickCmd()

	case listDirMsg:
		if msg.Error != "" {
			m.statusMsg = "ListDir error: " + msg.Error
			m.isError = true
		} else {
			node := findTreeNode(&m.rootNode, msg.Path)
			if node != nil {
				node.Children = nodeFromEntries(msg.Entries)
				m.expanded[node.Path] = true
			} else {
				m.rootNode = FileNode{
					Name:  ".",
					Path:  ".",
					IsDir: true,
					Children: nodeFromEntries(msg.Entries),
				}
				m.expanded["."] = true
			}
			m.flatTree = flattenTree(m.rootNode, m.expanded)
			if m.fileCursor >= len(m.flatTree) {
				m.fileCursor = 0
			}
		}

	case readFileMsg:
		if msg.Error != "" {
			m.statusMsg = "Read error: " + msg.Error
			m.isError = true
		} else {
			m.currentPath = msg.Path
			m.currentLang = detectLanguage(msg.Path)
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
			m.hasChanges = false
			cwd2, _ := os.Getwd()
			cmds = append(cmds, listDir(m.bridge, cwd2))
		}

	case runCommandMsg:
		output := "\n" + msg.Output
		if msg.Error != "" {
			output = "\nError: " + msg.Error + "\n"
		}
		if msg.IsAICmd {
			m.aiTerminalBuf.WriteString(output)
		} else {
			m.terminalBuf.WriteString(output)
		}

	case gitBranchMsg:
		if msg.Error == "" {
			m.gitBranch = msg.Branch
			m.gitDirty = msg.Dirty
		}

	case searchResultsMsg:
		if msg.Error == "" {
			m.globalSearchResults = msg.Results
			m.globalSearchIdx = 0
		}

	case RPCEvent:
		if msg.Event == "terminal_data" {
			var data struct {
				Data string `json:"data"`
			}
			json.Unmarshal(msg.Data, &data)
			// Write to both terminal buffers so streaming shows everywhere
			m.terminalBuf.WriteString(data.Data)
			m.aiTerminalBuf.WriteString(data.Data)
		}
		return m, waitForEvent(m.bridge)

	case tea.KeyMsg:
		// Help screen takes all input
		if m.helpOpen {
			if msg.String() == "f1" || msg.String() == "esc" {
				m.helpOpen = false
				return m, nil
			}
			return m, nil
		}

		// Zen mode - only ctrl+\ to exit, everything else passes through
		if m.zenMode {
			switch msg.String() {
			case "ctrl+\\":
				m.zenMode = false
				return m, nil
			default:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				if m.currentPath != "" {
					m.hasChanges = true
				}
				return m, cmd
			}
		}

		// Global search mode
		if m.globalSearchOpen {
			switch msg.String() {
			case "enter":
				if len(m.globalSearchResults) > 0 && m.globalSearchIdx < len(m.globalSearchResults) {
					result := m.globalSearchResults[m.globalSearchIdx]
					m.globalSearchOpen = false
					return m, readFile(m.bridge, result.File)
				}
			case "esc":
				m.globalSearchOpen = false
				m.globalSearchQuery = ""
				m.globalSearchResults = nil
				m.globalSearchIdx = 0
				return m, nil
			case "up", "ctrl+p":
				if m.globalSearchIdx > 0 {
					m.globalSearchIdx--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.globalSearchIdx < len(m.globalSearchResults)-1 {
					m.globalSearchIdx++
				}
				return m, nil
			case "backspace":
				if len(m.globalSearchQuery) > 0 {
					m.globalSearchQuery = m.globalSearchQuery[:len(m.globalSearchQuery)-1]
					cwd, _ := os.Getwd()
					if len(m.globalSearchQuery) >= 2 {
						return m, searchFiles(m.bridge, cwd, m.globalSearchQuery)
					} else {
						m.globalSearchResults = nil
					}
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.globalSearchQuery += msg.String()
					cwd, _ := os.Getwd()
					if len(m.globalSearchQuery) >= 2 {
						return m, searchFiles(m.bridge, cwd, m.globalSearchQuery)
					}
				}
				return m, nil
			}
		}

		// Inline search mode
		if m.searchOpen {
			switch msg.String() {
			case "enter":
				if len(m.searchMatches) > 0 {
					m.searchIdx = (m.searchIdx + 1) % len(m.searchMatches)
				}
				return m, nil
			case "shift+tab", "ctrl+p":
				if len(m.searchMatches) > 0 {
					m.searchIdx--
					if m.searchIdx < 0 {
						m.searchIdx = len(m.searchMatches) - 1
					}
				}
				return m, nil
			case "esc":
				m.searchOpen = false
				m.searchQuery = ""
				m.searchMatches = nil
				m.searchIdx = 0
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.searchMatches = findMatches(m.textarea.Value(), m.searchQuery)
					m.searchIdx = 0
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
					m.searchMatches = findMatches(m.textarea.Value(), m.searchQuery)
					m.searchIdx = 0
				}
				return m, nil
			}
		}

		// Overlay mode
		if m.overlayMode != "" {
			switch msg.String() {
			case "enter":
				path := m.overlayInput
				mode := m.overlayMode
				m.overlayMode = ""
				m.overlayInput = ""
				m.overlayTitle = ""
				switch mode {
				case "open_folder":
					m.currentFolder = filepath.Base(path)
					cwd2, _ := os.Getwd()
					return m, tea.Batch(listDir(m.bridge, path), fetchGitBranch(m.bridge, cwd2))
				case "new_file":
					cwd2, _ := os.Getwd()
					fullPath := filepath.Join(cwd2, path)
					return m, writeFile(m.bridge, fullPath, "")
				case "rename":
					if m.currentPath != "" && path != "" {
						dir := filepath.Dir(m.currentPath)
						newPath := filepath.Join(dir, path)
						res, err := m.bridge.Call("rename_file", map[string]interface{}{"old_path": m.currentPath, "new_path": newPath})
						if err != nil || res == nil {
							m.statusMsg = "Rename failed"
							m.isError = true
						} else {
							m.currentPath = newPath
							m.currentLang = detectLanguage(newPath)
							cwd2, _ := os.Getwd()
							return m, listDir(m.bridge, cwd2)
						}
					}
				case "delete_confirm":
					if strings.ToLower(strings.TrimSpace(path)) == "y" && m.fileCursor < len(m.flatTree) {
						node := m.flatTree[m.fileCursor]
						m.bridge.Call("delete_file", map[string]interface{}{"path": node.Path})
						cwd2, _ := os.Getwd()
						return m, listDir(m.bridge, cwd2)
					}
				case "quit_confirm":
					if strings.ToLower(strings.TrimSpace(path)) == "y" {
						return m, tea.Quit
					}
				}
				return m, nil
			case "esc":
				m.overlayMode = ""
				m.overlayInput = ""
				m.overlayTitle = ""
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
			if m.hasChanges {
				m.overlayMode = "quit_confirm"
				m.overlayInput = ""
				m.overlayTitle = "Unsaved changes. Quit anyway? (y/n)"
				return m, nil
			}
			return m, tea.Quit
		case "ctrl+1":
			if m.layoutMode == 0 {
				m.active = "files"
				m.textarea.Blur()
			} else {
				m.statusMsg = "Files panel hidden in current layout"
				m.isError = true
			}
		case "ctrl+2":
			m.active = "editor"
			m.textarea.Focus()
		case "ctrl+3":
			m.active = "terminal"
			m.textarea.Blur()
		case "ctrl+s":
			if m.currentPath != "" {
				m.hasChanges = false
				return m, writeFile(m.bridge, m.currentPath, m.textarea.Value())
			}
		case "ctrl+o":
			m.overlayMode = "open_folder"
			m.overlayInput = ""
			m.overlayTitle = "Open Folder Path:"
			return m, nil
		case "ctrl+n":
			m.overlayMode = "new_file"
			m.overlayInput = ""
			m.overlayTitle = "New file name:"
			return m, nil
		case "ctrl+w":
			m.currentPath = ""
			m.currentLang = "Plain Text"
			m.textarea.SetValue("")
			m.hasChanges = false
			m.statusMsg = "Closed file"
			m.isError = false
		case "ctrl+b":
			m.fileTreeVisible = !m.fileTreeVisible
		case "ctrl+z":
			if len(m.undoStack) > 0 {
				current := m.textarea.Value()
				m.redoStack = append(m.redoStack, current)
				prev := m.undoStack[len(m.undoStack)-1]
				m.undoStack = m.undoStack[:len(m.undoStack)-1]
				m.textarea.SetValue(prev)
				m.hasChanges = true
				m.statusMsg = "Undo"
				m.isError = false
			}
			return m, nil
		case "ctrl+y":
			if len(m.redoStack) > 0 {
				current := m.textarea.Value()
				m.undoStack = append(m.undoStack, current)
				next := m.redoStack[len(m.redoStack)-1]
				m.redoStack = m.redoStack[:len(m.redoStack)-1]
				m.textarea.SetValue(next)
				m.hasChanges = true
				m.statusMsg = "Redo"
				m.isError = false
			}
			return m, nil
		case "ctrl+\\":
			m.zenMode = !m.zenMode
		case "ctrl+f":
			if m.currentPath != "" {
				m.searchOpen = true
				m.searchQuery = ""
				m.searchMatches = nil
				m.searchIdx = 0
			}
		case "ctrl+shift+f":
			m.globalSearchOpen = true
			m.globalSearchQuery = ""
			m.globalSearchResults = nil
			m.globalSearchIdx = 0
		case "f1":
			m.helpOpen = !m.helpOpen
		case "f2":
			if m.currentPath != "" {
				m.overlayMode = "rename"
				m.overlayInput = filepath.Base(m.currentPath)
				m.overlayTitle = "Rename to:"
				return m, nil
			}
		case "ctrl+l":
			cycleLayout(&m)
			return m, nil
		case "ctrl+t":
			m.themeIdx = (m.themeIdx + 1) % len(Themes)
			m.currentTheme = Themes[m.themeIdx]
			m.textarea.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color(m.currentTheme.CursorLine))
			saveConfig(m.currentTheme.Name, m.layoutMode)
			return m, nil
		
		// --- AI Agent Launcher (Alt+1..5) ---
		case "alt+1":
			m.aiAgentName = "Claude"
			m.aiTerminalInput = "claude"
			return m, m.submitAITerminal()
		case "alt+2":
			m.aiAgentName = "Codex"
			m.aiTerminalInput = "codex"
			return m, m.submitAITerminal()
		case "alt+3":
			m.aiAgentName = "Gemini"
			m.aiTerminalInput = "gemini"
			return m, m.submitAITerminal()
		case "alt+4":
			m.aiAgentName = "Aider"
			m.aiTerminalInput = "aider"
			return m, m.submitAITerminal()
		case "alt+5":
			m.aiAgentName = "Custom"
			m.aiTerminalInput = ""
			m.statusMsg = "Custom AI agent selected. Type your command."
			m.isError = false
			m.active = "terminal"
			m.textarea.Blur()
			return m, nil
		
		// --- Context Injection ---
		case "alt+enter":
			return m, m.injectContextFilePath()
		case "alt+s":
			return m, m.injectContextSelection()
		case "alt+f":
			return m, m.injectContextFullFile()
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && m.overlayMode == "" && !m.helpOpen && !m.globalSearchOpen {
			// Calculate layout dimensions for mouse handling
			gap := 1
			filesW := m.width / 5
			if m.filesWidth > 0 { filesW = m.filesWidth }
			if !m.fileTreeVisible { filesW = 0 }
			
			editorW := m.width / 2
			if m.editorWidth > 0 { editorW = m.editorWidth }

			if m.layoutMode == 0 { // Classic
				editorW2 := (m.width - filesW - (gap * 2)) / 2
				if m.editorWidth > 0 { editorW2 = m.editorWidth }
				editorStart := filesW + gap
				termStart := editorStart + editorW2 + gap
				div1X := filesW
				div2X := termStart - gap

				switch {
				case msg.X == div1X || msg.X == div1X-1:
					m.dragging = true
					m.dragDivider = 1
					m.dragStartX = msg.X
				case msg.X == div2X || msg.X == div2X-1:
					m.dragging = true
					m.dragDivider = 2
					m.dragStartX = msg.X
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
			} else if m.layoutMode == 1 { // AI Developer
				editorW = (m.width * 60) / 100
				aiTermStart := editorW + gap
				divX := editorW

				switch {
				case msg.X == divX || msg.X == divX-1:
					m.dragging = true
					m.dragDivider = 3
					m.dragStartX = msg.X
				case msg.X < editorW:
					m.active = "editor"
					m.textarea.Focus()
				case msg.X >= aiTermStart:
					if msg.Y >= 2 && msg.Y <= 3 {
						pillX := aiTermStart + 2
						pills := []string{"Claude", "Codex", "Gemini", "Aider", "Custom"}
						keys := []string{"1", "2", "3", "4", "5"}
						for i, p := range pills {
							pillW := lipgloss.Width(fmt.Sprintf(" %s [Alt+%s] ", p, keys[i])) + 4
							if msg.X >= pillX && msg.X < pillX+pillW {
								m.aiAgentName = p
								if i < 4 {
									m.aiTerminalInput = strings.ToLower(p)
									return m, m.submitAITerminal()
								} else {
									m.aiTerminalInput = ""
									m.statusMsg = "Custom AI agent selected. Type your command."
									m.isError = false
									m.active = "terminal"
									m.textarea.Blur()
									return m, nil
								}
							}
							pillX += pillW + 1
						}
					}
					m.active = "terminal"
					m.textarea.Blur()
				}
			} else if m.layoutMode == 3 { // Terminal Focus
				editorW = (m.width * 40) / 100
				aiTermStart := editorW + gap
				divX := editorW

				switch {
				case msg.X == divX || msg.X == divX-1:
					m.dragging = true
					m.dragDivider = 3
					m.dragStartX = msg.X
				case msg.X < editorW:
					m.active = "editor"
					m.textarea.Focus()
				case msg.X >= aiTermStart:
					if msg.Y >= 2 && msg.Y <= 3 {
						pillX := aiTermStart + 2
						pills := []string{"Claude", "Codex", "Gemini", "Aider", "Custom"}
						keys := []string{"1", "2", "3", "4", "5"}
						for i, p := range pills {
							pillW := lipgloss.Width(fmt.Sprintf(" %s [Alt+%s] ", p, keys[i])) + 4
							if msg.X >= pillX && msg.X < pillX+pillW {
								m.aiAgentName = p
								if i < 4 {
									m.aiTerminalInput = strings.ToLower(p)
									return m, m.submitAITerminal()
								} else {
									m.aiTerminalInput = ""
									m.statusMsg = "Custom AI agent selected. Type your command."
									m.isError = false
									m.active = "terminal"
									m.textarea.Blur()
									return m, nil
								}
							}
							pillX += pillW + 1
						}
					}
					m.active = "terminal"
					m.textarea.Blur()
				}
			}
			// Focus mode (2) has no panels to click - just editor
		}
		if msg.Action == tea.MouseActionMotion && m.dragging {
			delta := msg.X - m.dragStartX
			if m.dragDivider == 1 {
				m.filesWidth += delta
				if m.filesWidth < 10 { m.filesWidth = 10 }
				if m.filesWidth > m.width/3 { m.filesWidth = m.width / 3 }
			} else if m.dragDivider == 2 {
				m.editorWidth += delta
				if m.editorWidth < 10 { m.editorWidth = 10 }
				if m.editorWidth > m.width/2 { m.editorWidth = m.width / 2 }
			} else if m.dragDivider == 3 {
				// In AI modes: editor|ai-terminal divider
				editorPct := float64(m.editorWidth+delta) / float64(m.width) * 100
				if editorPct < 20 { editorPct = 20 }
				if editorPct > 80 { editorPct = 80 }
				m.editorWidth = int(float64(m.width) * editorPct / 100)
			}
			m.dragStartX = msg.X
			return m, nil
		}
		if msg.Action == tea.MouseActionRelease && m.dragging {
			m.dragging = false
			m.dragDivider = 0
			return m, nil
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
				if m.fileCursor < len(m.flatTree)-1 {
					m.fileCursor++
				}
			case "enter":
				if len(m.flatTree) > 0 {
					node := m.flatTree[m.fileCursor]
					if node.IsDir {
						if !m.expanded[node.Path] && (len(node.Children) == 0) {
							return m, listDir(m.bridge, node.Path)
						}
						m.expanded[node.Path] = !m.expanded[node.Path]
						m.flatTree = flattenTree(m.rootNode, m.expanded)
					} else {
						return m, readFile(m.bridge, node.Path)
					}
				}
			case "backspace":
				if len(m.flatTree) > 0 && m.fileCursor < len(m.flatTree) {
					node := m.flatTree[m.fileCursor]
					if node.IsDir && m.expanded[node.Path] {
						m.expanded[node.Path] = false
						m.flatTree = flattenTree(m.rootNode, m.expanded)
					} else if m.fileCursor > 0 {
						m.fileCursor--
					}
				}
			case "delete":
				if len(m.flatTree) > 0 && m.fileCursor < len(m.flatTree) {
					node := m.flatTree[m.fileCursor]
					if !node.IsDir {
						m.overlayMode = "delete_confirm"
						m.overlayInput = ""
						m.overlayTitle = "Delete " + node.Name + "? (y/n)"
					}
				}
			}

		case "editor":
			// Save content for undo only on content-modifying keys
			switch msg.String() {
			case "ctrl+z", "ctrl+y":
				// Already handled globally, don't double-process
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				return m, cmd
			}
			prev := m.textarea.Value()
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			curr := m.textarea.Value()
			if curr != prev && m.currentPath != "" {
				m.undoStack = append(m.undoStack, prev)
				if len(m.undoStack) > m.maxUndo {
					m.undoStack = m.undoStack[len(m.undoStack)-m.maxUndo:]
				}
				m.redoStack = nil
				m.hasChanges = true
			}
			return m, cmd

		case "terminal":
			// Determine which terminal buffer/input to use based on layout mode
			isAITerminal := m.layoutMode == 1 || m.layoutMode == 3
			buf := m.terminalBuf
			input := m.terminalInput
			history := m.terminalHistory
			histIdx := m.terminalHistIdx
			if isAITerminal {
				buf = m.aiTerminalBuf
				input = m.aiTerminalInput
				history = m.aiTerminalHistory
				histIdx = m.aiTerminalHistIdx
			}

			switch msg.String() {
			case "enter":
				if input != "" {
					history = append(history, input)
					if isAITerminal {
						m.aiTerminalHistory = history
						m.aiTerminalBuf.WriteString("\nAI ❯ " + input + "\n")
						cmd := runCommand(m.bridge, input, true)
						m.aiTerminalInput = ""
						m.aiTerminalHistIdx = -1
						return m, cmd
					} else {
						m.terminalHistory = history
						// Show the command prompt in the buffer
						m.terminalBuf.WriteString("\n> " + input + "\n")
						// Send to persistent shell via streaming terminal_write
						cmd := terminalWrite(m.bridge, input+"\n")
						m.terminalInput = ""
						m.terminalHistIdx = -1
						return m, cmd
					}
				}
			case "up":
				if len(history) > 0 {
					if histIdx == -1 {
						histIdx = len(history) - 1
					} else if histIdx > 0 {
						histIdx--
					}
					if isAITerminal {
						m.aiTerminalHistIdx = histIdx
						m.aiTerminalInput = m.aiTerminalHistory[histIdx]
					} else {
						m.terminalHistIdx = histIdx
						m.terminalInput = m.terminalHistory[histIdx]
					}
				}
			case "down":
				if histIdx != -1 {
					if histIdx < len(history)-1 {
						histIdx++
						if isAITerminal {
							m.aiTerminalHistIdx = histIdx
							m.aiTerminalInput = m.aiTerminalHistory[histIdx]
						} else {
							m.terminalHistIdx = histIdx
							m.terminalInput = m.terminalHistory[histIdx]
						}
					} else {
						if isAITerminal {
							m.aiTerminalHistIdx = -1
							m.aiTerminalInput = ""
						} else {
							m.terminalHistIdx = -1
							m.terminalInput = ""
						}
					}
				}
			case "backspace":
				if isAITerminal {
					if len(m.aiTerminalInput) > 0 {
						m.aiTerminalInput = m.aiTerminalInput[:len(m.aiTerminalInput)-1]
					}
				} else {
					if len(m.terminalInput) > 0 {
						m.terminalInput = m.terminalInput[:len(m.terminalInput)-1]
					}
				}
			case "ctrl+c":
				if !isAITerminal {
					// Send Ctrl+C to streaming terminal
					m.terminalBuf.WriteString("^C\n")
					return m, terminalWrite(m.bridge, "\x03")
				}
			case "ctrl+l":
				buf.Reset()
				if isAITerminal {
					// For AI terminal, also reset the buffer on the remote side
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					if isAITerminal {
						m.aiTerminalInput += msg.String()
					} else {
						m.terminalInput += msg.String()
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		editorW := (m.width * 45) / 100
		innerEditorW := editorW - 2
		mainH := m.height - 4
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
		lines := strings.Split(content, "\n")
		var processed []string
		for i := 0; i < innerHeight; i++ {
			if i < len(lines) {
				line := lines[i]
				if lipgloss.Width(line) > innerWidth {
					line = line[:innerWidth]
				}
				processed = append(processed, line+strings.Repeat(" ", innerWidth-lipgloss.Width(line)))
			} else {
				processed = append(processed, strings.Repeat(" ", innerWidth))
			}
		}
		styledContent = strings.Join(processed, "\n")
	}

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

func renderFiles(width, height int, active bool, theme Theme, tree []FileNode, cursor int, expanded map[string]bool) string {
	innerWidth := width - 2
	innerHeight := height - 2

	var lines []string
	for i, node := range tree {
		if len(lines) >= innerHeight { break }
		
		indent := strings.Repeat("  ", node.Depth)
		
		var prefix string
		if node.IsDir {
			if expanded[node.Path] {
				prefix = "▼ "
			} else {
				prefix = "▶ "
			}
		} else {
			prefix = "  "
		}
		
		displayName := indent + prefix + node.Name
		if lipgloss.Width(displayName) > innerWidth-2 {
			displayName = displayName[:innerWidth-5] + "..."
		}
		
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text))
		if i == cursor && active {
			style = style.Background(lipgloss.Color(theme.Accent)).Foreground(lipgloss.Color(theme.Background))
		} else if node.IsDir {
			style = style.Foreground(lipgloss.Color(theme.AccentAlt))
		}
		
		line := displayName
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

// ==============================================================================
// AI Terminal Render
// ==============================================================================

func renderAITerminal(width, height int, active bool, theme Theme, content string, input string, cursorVisible bool, agentName string) string {
	innerWidth := width - 2
	innerHeight := height - 2

	// --- Agent Launcher Bar ---
	agentBarWidth := innerWidth
	agentBar := renderAgentBar(agentBarWidth, theme, agentName)

	// --- Separator ---
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Border))
	separator := sepStyle.Render(strings.Repeat("─", agentBarWidth))

	// Available height for terminal content + input
	barLines := strings.Count(agentBar, "\n") + 1 + 1 // bar + separator
	availHeight := innerHeight - barLines
	if availHeight < 2 { availHeight = 2 }

	outputHeight := availHeight - 1
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

	// Input line with AI prompt
	promptStr := "AI ❯ "
	cursor := ""
	if cursorVisible && active {
		cursor = "█"
	}
	
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	inputLine := promptStyle.Render(promptStr) + input + cursor
	if lipgloss.Width(inputLine) > innerWidth {
		inputLine = inputLine[lipgloss.Width(inputLine)-innerWidth:]
	}
	inputLine += strings.Repeat(" ", innerWidth-lipgloss.Width(inputLine))

	// Build full AI terminal content
	aiContent := agentBar + "\n" + separator + "\n" + strings.Join(lines, "\n") + "\n" + inputLine
	return renderPanel("AI Terminal", width, height, active, theme, aiContent)
}

func renderAgentBar(width int, theme Theme, activeAgent string) string {
	agents := []struct {
		Name string
		Key  string
	}{
		{"Claude", "1"},
		{"Codex", "2"},
		{"Gemini", "3"},
		{"Aider", "4"},
		{"Custom", "5"},
	}

	var pills []string
	for _, a := range agents {
		isActive := a.Name == activeAgent
		var pill string
		if isActive {
			pill = lipgloss.NewStyle().
				Background(lipgloss.Color(theme.Accent)).
				Foreground(lipgloss.Color(theme.Background)).
				Bold(true).
				Padding(0, 2).
				Render(fmt.Sprintf(" %s [Alt+%s] ", a.Name, a.Key))
		} else {
			pill = lipgloss.NewStyle().
				Background(lipgloss.Color(theme.Border)).
				Foreground(lipgloss.Color(theme.Text)).
				Padding(0, 2).
				Render(fmt.Sprintf(" %s [Alt+%s] ", a.Name, a.Key))
		}
		pills = append(pills, pill)
	}

	// Join pills horizontally
	pillRow := lipgloss.JoinHorizontal(lipgloss.Center, pills...)
	
	// Center the pills or left-align them
	if lipgloss.Width(pillRow) > width {
		pillRow = pillRow[:width]
	}

	// Wrap in a title bar
	barTitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Bold(true).
		Render(" AI Agents ")
	
	// Left-align: bar title + pills
	return barTitle + "\n" + pillRow
}

// ==============================================================================
// Help Screen
// ==============================================================================

func renderHelp(width, height int, theme Theme) string {
	helpWidth := 64
	helpHeight := 30
	if helpWidth > width-4 { helpWidth = width - 4 }
	if helpHeight > height-2 { helpHeight = height - 2 }

	lines := []string{
		"                    Keyboard Shortcuts",
		"",
		"  File                          Layout",
		"    Ctrl+N    New File             Ctrl+B    Toggle Files",
		"    Ctrl+S    Save File            Ctrl+\\    Zen Mode",
		"    Ctrl+W    Close File           F1        Help",
		"    Ctrl+O    Open Folder          Mouse     Click to Focus",
		"    F2        Rename File          Drag      Resize Panels",
		"    Del       Delete File",
		"",
		"  AI Developer Mode              Terminal",
		"    Ctrl+L    Cycle Layouts         Enter     Run Command",
		"    Alt+1     Launch Claude         Up/Down   History",
		"    Alt+2     Launch Codex          Ctrl+K    Clear (Reg)",
		"    Alt+3     Launch Gemini         Ctrl+K    Clear (AI)",
		"    Alt+4     Launch Aider          Ctrl+L    Cycle Layouts",
		"    Alt+5     Custom Agent",
		"",
		"  Context Injection              General",
		"    Alt+Enter Inject File Path     Ctrl+T    Cycle Theme",
		"    Alt+S     Inject Selection     Ctrl+Q    Quit",
		"    Alt+F     Inject Full File     Enter/Esc Confirm/Cancel",
		"",
		"  Navigation",
		"    Ctrl+1    Files Panel",
		"    Ctrl+2    Editor Panel",
		"    Ctrl+3    Terminal / AI Panel",
		"    Ctrl+F    Inline Search",
		"    Ctrl+Shift+F  Global Search",
		"",
		"  [F1/Esc] Close Help",
	}

	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	content = strings.TrimRight(content, "\n")

	return renderPanel("Keyboard Shortcuts", helpWidth, helpHeight, true, theme, content)
}

// ==============================================================================
// Search Overlay
// ==============================================================================

func renderGlobalSearch(width int, theme Theme, query string, results []GlobalResult, idx int) string {
	sHeight := 12
	if sHeight > width/3 { sHeight = width / 3 }

	innerW := width - 2
	var content string

	searchLine := "> " + query
	if len(searchLine) > innerW-2 {
		searchLine = searchLine[:innerW-5] + "..."
	}
	content += lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Render("Search: ") + query + "\n\n"

	if len(results) == 0 {
		if len(query) >= 2 {
			content += lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("  No results found") + "\n"
		} else {
			content += lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render("  Type at least 2 characters to search") + "\n"
		}
	} else {
		maxResults := sHeight - 4
		if maxResults > len(results) { maxResults = len(results) }
		start := 0
		if idx >= maxResults { start = idx - maxResults + 1 }

		for i := start; i < start+maxResults && i < len(results); i++ {
			r := results[i]
			line := fmt.Sprintf("  %s:%d  %s", r.File, r.Line, r.Text)
			if lipgloss.Width(line) > innerW-4 {
				line = line[:innerW-7] + "..."
			}
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text))
			if i == idx {
				style = style.Background(lipgloss.Color(theme.Accent)).Foreground(lipgloss.Color(theme.Background))
			}
			content += style.Render(line) + "\n"
		}
	}

	content += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TextMuted)).Render(" [Enter] open   [Esc] cancel   [↑/↓] navigate")

	return lipgloss.Place(width, sHeight, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(theme.Accent)).
			Background(lipgloss.Color(theme.Surface)).Width(width-4).Padding(0, 1).Render(content),
	)
}

// ==============================================================================
// Main View
// ==============================================================================

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	t := m.currentTheme

	// --- ZEN MODE ---
	if m.zenMode {
		editorView := m.textarea.View()
		hint := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Render(" ctrl+\\ exit zen ")
		return editorView + "\n" + strings.Repeat(" ", m.width-lipgloss.Width(hint)) + hint
	}

	// --- HELP SCREEN ---
	if m.helpOpen {
		helpPanel := renderHelp(m.width, m.height, t)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpPanel)
	}

	// Dimensions
	gap := 1
	mainH := m.height - 4 // header(2) + footer(2)
	if mainH < 0 { mainH = 0 }

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
	layoutLabel := []string{"CLASSIC", "AI DEV", "FOCUS", "TERM FOCUS"}[m.layoutMode]
	headerInfo := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + layoutLabel + " ")
	folder := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Background(lipgloss.Color(t.SurfaceAlt)).
		Width(m.width - lipgloss.Width(logo) - lipgloss.Width(sep) - lipgloss.Width(headerInfo) - 4).Align(lipgloss.Center).Render(strings.ToUpper(m.currentFolder))
	
	headerContent := lipgloss.NewStyle().Width(m.width).Background(lipgloss.Color(t.SurfaceAlt)).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, logo, sep, folder, sep, headerInfo))
	
	headerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(m.width).Render(strings.Repeat("─", m.width))
	header := lipgloss.JoinVertical(lipgloss.Left, headerContent, headerSep)

	// --- 2. MAIN AREA (per layout mode) ---
	var mainArea string

	switch m.layoutMode {
	case 0: // Classic: Files | Editor | Terminal
		filesW := (m.width * 20) / 100
		if m.filesWidth > 0 { filesW = m.filesWidth }
		if !m.fileTreeVisible { filesW = 0 }

		editorW := (m.width - filesW - (gap * 2)) / 2
		if m.editorWidth > 0 { editorW = m.editorWidth }

		terminalW := m.width - filesW - editorW - (gap * 2)
		if terminalW < 10 { terminalW = 10 }
		if !m.fileTreeVisible {
			editorW = m.width - terminalW - gap*2
		}

		if m.fileTreeVisible {
			filesPanel := renderFiles(filesW, mainH, m.active == "files", t, m.flatTree, m.fileCursor, m.expanded)
			divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(gap).Render("│")
			
			editorView := m.textarea.View()
			if m.searchOpen {
				matchInfo := ""
				if len(m.searchMatches) > 0 {
					matchInfo = fmt.Sprintf("  %d of %d", m.searchIdx+1, len(m.searchMatches))
				}
				searchBar := lipgloss.NewStyle().
					Foreground(lipgloss.Color(t.Accent)).
					Background(lipgloss.Color(t.Surface)).
					Width(editorW - 2).
					Render(fmt.Sprintf(" Search: %s%s    [Enter] next  [Esc] close", m.searchQuery, matchInfo))
				editorView = searchBar + "\n" + editorView
			}
			editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", t, editorView)
			divStyle2 := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(gap).Render("│")
			terminalPanel := renderTerminal(terminalW, mainH, m.active == "terminal", t, m.terminalBuf.String(), m.terminalInput, m.cursorVisible)
			mainArea = lipgloss.JoinHorizontal(lipgloss.Top, filesPanel, divStyle, editorPanel, divStyle2, terminalPanel)
		} else {
			editorView := m.textarea.View()
			if m.searchOpen {
				matchInfo := ""
				if len(m.searchMatches) > 0 {
					matchInfo = fmt.Sprintf("  %d of %d", m.searchIdx+1, len(m.searchMatches))
				}
				searchBar := lipgloss.NewStyle().
					Foreground(lipgloss.Color(t.Accent)).
					Background(lipgloss.Color(t.Surface)).
					Width(editorW - 2).
					Render(fmt.Sprintf(" Search: %s%s    [Enter] next  [Esc] close", m.searchQuery, matchInfo))
				editorView = searchBar + "\n" + editorView
			}
			editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", t, editorView)
			divStyle2 := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(gap).Render("│")
			terminalPanel := renderTerminal(terminalW, mainH, m.active == "terminal", t, m.terminalBuf.String(), m.terminalInput, m.cursorVisible)
			mainArea = lipgloss.JoinHorizontal(lipgloss.Top, editorPanel, divStyle2, terminalPanel)
		}

	case 1: // AI Developer: Editor (60%) | AI Terminal (40%)
		editorW := (m.width * 60) / 100
		aiTermW := m.width - editorW - gap

		editorView := m.textarea.View()
		if m.searchOpen {
			matchInfo := ""
			if len(m.searchMatches) > 0 {
				matchInfo = fmt.Sprintf("  %d of %d", m.searchIdx+1, len(m.searchMatches))
			}
			searchBar := lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.Accent)).
				Background(lipgloss.Color(t.Surface)).
				Width(editorW - 2).
				Render(fmt.Sprintf(" Search: %s%s    [Enter] next  [Esc] close", m.searchQuery, matchInfo))
			editorView = searchBar + "\n" + editorView
		}
		editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", t, editorView)
		divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(gap).Render("│")
		aiTermPanel := renderAITerminal(aiTermW, mainH, m.active == "terminal", t, m.aiTerminalBuf.String(), m.aiTerminalInput, m.cursorVisible, m.aiAgentName)
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, editorPanel, divStyle, aiTermPanel)

	case 2: // Focus: Just Editor (100%)
		editorView := m.textarea.View()
		if m.searchOpen {
			matchInfo := ""
			if len(m.searchMatches) > 0 {
				matchInfo = fmt.Sprintf("  %d of %d", m.searchIdx+1, len(m.searchMatches))
			}
			searchBar := lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.Accent)).
				Background(lipgloss.Color(t.Surface)).
				Width(m.width - 2).
				Render(fmt.Sprintf(" Search: %s%s    [Enter] next  [Esc] close", m.searchQuery, matchInfo))
			editorView = searchBar + "\n" + editorView
		}
		mainArea = renderPanel("Editor", m.width, mainH, true, t, editorView)

	case 3: // Terminal Focus: Editor (40%) | AI Terminal (60%)
		editorW := (m.width * 40) / 100
		aiTermW := m.width - editorW - gap

		editorView := m.textarea.View()
		if m.searchOpen {
			matchInfo := ""
			if len(m.searchMatches) > 0 {
				matchInfo = fmt.Sprintf("  %d of %d", m.searchIdx+1, len(m.searchMatches))
			}
			searchBar := lipgloss.NewStyle().
				Foreground(lipgloss.Color(t.Accent)).
				Background(lipgloss.Color(t.Surface)).
				Width(editorW - 2).
				Render(fmt.Sprintf(" Search: %s%s    [Enter] next  [Esc] close", m.searchQuery, matchInfo))
			editorView = searchBar + "\n" + editorView
		}
		editorPanel := renderPanel("Editor", editorW, mainH, m.active == "editor", t, editorView)
		divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(gap).Render("│")
		aiTermPanel := renderAITerminal(aiTermW, mainH, m.active == "terminal", t, m.aiTerminalBuf.String(), m.aiTerminalInput, m.cursorVisible, m.aiAgentName)
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, editorPanel, divStyle, aiTermPanel)
	}

	// --- 3. STATUS BAR ---
	statusBrand := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true).Background(lipgloss.Color(t.SurfaceAlt)).Padding(0, 1).Render("TRIX")
	
	// Layout name in status bar
	layoutNames := []string{"Layout: Classic", "Layout: AI Dev", "Layout: Focus", "Layout: Term Focus"}
	layoutInfo := lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + layoutNames[m.layoutMode] + " ")
	
	// Active AI agent indicator
	agentInfo := ""
	if m.aiAgentName != "" && (m.layoutMode == 1 || m.layoutMode == 3) {
		animIndicator := ""
		if m.aiAgentAnimTick {
			animIndicator = " ●"
		}
		agentInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" Agent: " + m.aiAgentName + animIndicator + " ")
	}
	
	// File info
	fileInfo := ""
	if m.currentPath != "" {
		fname := filepath.Base(m.currentPath)
		if m.hasChanges {
			fname += " *"
			fileInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Warning)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + fname + " ")
		} else {
			fileInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + fname + " ")
		}
	}
	
	// Git info
	gitInfo := ""
	if m.gitBranch != "" {
		gitDirtyMark := ""
		if m.gitDirty {
			gitDirtyMark = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Warning)).Background(lipgloss.Color(t.SurfaceAlt)).Render("●")
		}
		gitInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Background(lipgloss.Color(t.SurfaceAlt)).Render(fmt.Sprintf(" %s %s ", m.gitBranch, gitDirtyMark))
	}
	
	// Language
	langInfo := lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + m.currentLang + " ")
	
	// Right: status message
	var statusRight string
	if m.isError {
		statusRight = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + m.statusMsg + "  ")
	} else if m.statusMsg != "" {
		statusRight = lipgloss.NewStyle().Foreground(lipgloss.Color(t.TextMuted)).Background(lipgloss.Color(t.SurfaceAlt)).Render(" " + m.statusMsg + "  ")
	}
	
	// Right: keyboard hints
	var pills string
	if m.layoutMode == 0 {
		pills = lipgloss.NewStyle().Background(lipgloss.Color(t.Surface)).Foreground(lipgloss.Color(t.Accent)).Padding(0, 1).Render("^1 Files  ^2 Editor  ^3 Term  ^S Save  ^Q Quit")
	} else if m.layoutMode == 1 || m.layoutMode == 3 {
		pills = lipgloss.NewStyle().Background(lipgloss.Color(t.Surface)).Foreground(lipgloss.Color(t.Accent)).Padding(0, 1).Render("Alt+Enter File  Alt+F Full  Alt+1-5 Agents  ^L Layout")
	} else {
		pills = lipgloss.NewStyle().Background(lipgloss.Color(t.Surface)).Foreground(lipgloss.Color(t.Accent)).Padding(0, 1).Render("^2 Editor  ^L  Layout  ^S Save  ^Q Quit")
	}
	
	leftWidth := lipgloss.Width(statusBrand) + lipgloss.Width(layoutInfo) + lipgloss.Width(agentInfo) + lipgloss.Width(fileInfo) + lipgloss.Width(gitInfo) + lipgloss.Width(langInfo)
	rightWidth := lipgloss.Width(statusRight) + lipgloss.Width(pills) + 4
	statusCenter := m.width - leftWidth - rightWidth
	if statusCenter < 2 { statusCenter = 2 }
	
	leftContent := statusBrand + layoutInfo + agentInfo + fileInfo + gitInfo + langInfo
	
	footerContent := lipgloss.NewStyle().Width(m.width).Background(lipgloss.Color(t.SurfaceAlt)).
		Render(lipgloss.JoinHorizontal(lipgloss.Left,
			leftContent,
			lipgloss.NewStyle().Width(statusCenter).Background(lipgloss.Color(t.SurfaceAlt)).Render(""),
			statusRight,
			pills,
		))
	
	footerSep := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)).Width(m.width).Render(strings.Repeat("─", m.width))
	footer := lipgloss.JoinVertical(lipgloss.Left, footerSep, footerContent)

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)

	// Overlays
	if m.overlayMode != "" {
		overlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			renderOverlay(m.overlayMode, m.overlayInput, m.overlayTitle, t),
		)
		return overlay
	}

	// Global search overlay
	if m.globalSearchOpen {
		return renderGlobalSearch(m.width, t, m.globalSearchQuery, m.globalSearchResults, m.globalSearchIdx)
	}

	return finalView
}

func renderOverlay(mode, input, title string, theme Theme) string {
	width := 44
	if title == "" {
		title = "Enter value:"
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
// Language Detection
// ==============================================================================

var langMap = map[string]string{
	".py": "Python", ".go": "Go", ".js": "JavaScript", ".ts": "TypeScript",
	".jsx": "JavaScript", ".tsx": "TypeScript", ".rs": "Rust", ".c": "C",
	".cpp": "C++", ".h": "C", ".hpp": "C++", ".java": "Java",
	".rb": "Ruby", ".php": "PHP", ".sh": "Bash", ".bash": "Bash",
	".json": "JSON", ".yaml": "YAML", ".yml": "YAML", ".toml": "TOML",
	".md": "Markdown", ".html": "HTML", ".htm": "HTML", ".css": "CSS",
	".sql": "SQL", ".xml": "XML", ".svg": "XML",
	".ps1": "PowerShell", ".bat": "Batch", ".cmd": "Batch",
	".zig": "Zig", ".swift": "Swift", ".kt": "Kotlin", ".kts": "Kotlin",
	".r": "R", ".lua": "Lua", ".pl": "Perl", ".pm": "Perl",
	".hs": "Haskell", ".ex": "Elixir", ".exs": "Elixir",
	".vue": "Vue", ".svelte": "Svelte", ".astro": "Astro",
	".dart": "Dart", ".scala": "Scala", ".clj": "Clojure",
	".tex": "LaTeX", ".graphql": "GraphQL", ".gql": "GraphQL",
	".csv": "CSV", ".env": "Env", ".gitignore": "Git",
	".makefile": "Makefile", ".mk": "Makefile",
}

func detectLanguage(path string) string {
	ext := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	base := filepath.Base(path)
	if lang, ok := langMap[strings.ToLower(base)]; ok {
		return lang
	}
	return "Plain Text"
}

// ==============================================================================
// Config Persistence
// ==============================================================================

type Config struct {
	Theme  string `json:"theme"`
	Layout int    `json:"layout"`
}

func getConfigFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".trix", "config.json")
}

func loadConfig() Config {
	file := getConfigFile()
	data, err := os.ReadFile(file)
	if err != nil {
		return Config{Theme: "Ayu Dark", Layout: 0}
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(themeName string, layoutMode int) {
	file := getConfigFile()
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0755)
	cfg := Config{Theme: themeName, Layout: layoutMode}
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

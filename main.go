package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	backgroundColor   = lipgloss.Color("#0d1016")
	borderColor       = lipgloss.Color("#3f4043")
	activeColor        = lipgloss.Color("#5ac1fe")
	inactiveColor      = lipgloss.Color("#bfbdb6")
	headerFolderColor = lipgloss.Color("#4b4c4e")
	bottomBarBg       = lipgloss.Color("#131721")
	dividerBg          = lipgloss.Color("#1a1d23")
	fileColor         = lipgloss.Color("#8a8986")
	folderColor       = lipgloss.Color("#bfbdb6")
	unsavedColor      = lipgloss.Color("#feb454")
	terminalOutputCol = lipgloss.Color("#8a8986")
)

type FileNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Children []FileNode `json:"children,omitempty"`
}

type FileContent struct {
	Path    string
	Content string
}

type TerminalData struct {
	Data string `json:"data"`
}

type SearchResult struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type Theme struct {
	Name        string
	Background  lipgloss.Color
	Border      lipgloss.Color
	Active      lipgloss.Color
	Inactive    lipgloss.Color
	File        lipgloss.Color
	Folder      lipgloss.Color
	BottomBar   lipgloss.Color
}

var (
	AyuDark = Theme{
		Name:       "Ayu Dark",
		Background: lipgloss.Color("#0d1016"),
		Border:     lipgloss.Color("#3f4043"),
		Active:     lipgloss.Color("#5ac1fe"),
		Inactive:   lipgloss.Color("#bfbdb6"),
		File:       lipgloss.Color("#8a8986"),
		Folder:     lipgloss.Color("#bfbdb6"),
		BottomBar:  lipgloss.Color("#131721"),
	}
	AyuLight = Theme{
		Name:       "Ayu Light",
		Background: lipgloss.Color("#fafafa"),
		Border:     lipgloss.Color("#d9d9d9"),
		Active:     lipgloss.Color("#ff9940"),
		Inactive:   lipgloss.Color("#5c6773"),
		File:       lipgloss.Color("#5c6773"),
		Folder:     lipgloss.Color("#36a3d9"),
		BottomBar:  lipgloss.Color("#f0f0f0"),
	}
	AyuMirage = Theme{
		Name:       "Ayu Mirage",
		Background: lipgloss.Color("#1f2430"),
		Border:     lipgloss.Color("#33415e"),
		Active:     lipgloss.Color("#ffcc66"),
		Inactive:   lipgloss.Color("#707a8c"),
		File:       lipgloss.Color("#cbccc6"),
		Folder:     lipgloss.Color("#ffa759"),
		BottomBar:  lipgloss.Color("#191e2a"),
	}
)

type model struct {
	bridge        *Bridge
	err           error
	width         int
	height        int
	active        string // "files", "editor", "terminal"
	rootNode      FileNode
	cursor        int
	flatTree      []FlatNode
	textarea      textarea.Model
	terminalLog   viewport.Model
	terminalBuf   strings.Builder
	terminalInput string
	currentPath   string
	hasChanges    bool
	expanded      map[string]bool
	gitBranch     string
	gitDirty      bool
	promptMode    string // "none", "new", "delete", "rename", "search"
	promptValue   string
	searchResults []SearchResult
	searchCursor  int
	themeIdx      int
	themes        []Theme
}

func (m model) currentTheme() Theme {
	return m.themes[m.themeIdx]
}

type FlatNode struct {
	FileNode
	Depth int
}

func flattenTree(node FileNode, depth int, expanded map[string]bool) []FlatNode {
	res := []FlatNode{{FileNode: node, Depth: depth}}
	if node.IsDir && expanded[node.Path] {
		for _, child := range node.Children {
			res = append(res, flattenTree(child, depth+1, expanded)...)
		}
	}
	return res
}

type GitBranch struct {
	Branch string `json:"branch"`
}

type GitStatus struct {
	Dirty bool `json:"dirty"`
}

type SearchResults struct {
	Results []SearchResult `json:"results"`
}

func fetchFileTree(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("get_file_tree", map[string]string{"path": path})
		if err != nil {
			return err
		}
		var result struct {
			Status string   `json:"status"`
			Tree   FileNode `json:"tree"`
		}
		if err := json.Unmarshal(res, &result); err != nil {
			return err
		}
		return result.Tree
	}
}

func readFile(b *Bridge, path string) tea.Cmd {
	return func() tea.Msg {
		res, err := b.Call("read_file", map[string]string{"path": path})
		if err != nil {
			return err
		}
		var result struct {
			Status  string `json:"status"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(res, &result); err != nil {
			return err
		}
		return FileContent{Path: path, Content: result.Content}
	}
}

func saveFile(b *Bridge, path, content string) tea.Cmd {
	return func() tea.Msg {
		_, err := b.Call("save_file", map[string]string{"path": path, "content": content})
		if err != nil {
			return err
		}
		return "saved"
	}
}

func waitForEvents(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		return <-b.Events()
	}
}

func spawnTerminal(b *Bridge, rows, cols int) tea.Cmd {
	return func() tea.Msg {
		_, err := b.Call("terminal_spawn", map[string]int{"rows": rows, "cols": cols})
		if err != nil {
			return err
		}
		return "terminal_spawned"
	}
}

func writeTerminal(b *Bridge, data string) tea.Cmd {
	return func() tea.Msg {
		_, err := b.Call("terminal_write", map[string]string{"data": data})
		if err != nil {
			return err
		}
		return nil
	}
}

func fetchGitInfo(b *Bridge) tea.Cmd {
	return func() tea.Msg {
		res, _ := b.Call("get_git_branch", map[string]string{"path": "."})
		var branch GitBranch
		json.Unmarshal(res, &branch)

		res, _ = b.Call("get_git_status", map[string]string{"path": "."})
		var status GitStatus
		json.Unmarshal(res, &status)

		return tea.Batch(
			func() tea.Msg { return branch },
			func() tea.Msg { return status },
		)()
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
		b.Call("rename_file", map[string]string{"old": old, "new": new})
		return fetchFileTree(b, ".")()
	}
}

func searchFiles(b *Bridge, query string) tea.Cmd {
	return func() tea.Msg {
		res, _ := b.Call("search_files", map[string]string{"root": ".", "query": query})
		var result struct {
			Results []SearchResult `json:"results"`
		}
		json.Unmarshal(res, &result)
		return SearchResults{Results: result.Results}
	}
}

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
		expanded: make(map[string]bool),
		themes:   []Theme{AyuDark, AyuLight, AyuMirage},
		themeIdx: 0,
	}
}

func (m model) Init() tea.Cmd {
	m.expanded["."] = true
	return tea.Batch(
		fetchFileTree(m.bridge, "."),
		spawnTerminal(m.bridge, 24, 80),
		fetchGitInfo(m.bridge),
		waitForEvents(m.bridge),
		textarea.Blink,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
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
		m.flatTree = flattenTree(m.rootNode, 0, m.expanded)
	case FileContent:
		m.currentPath = msg.Path
		m.textarea.SetValue(msg.Content)
		m.hasChanges = false
		m.active = "editor"
		m.textarea.Focus()
	case GitBranch:
		m.gitBranch = msg.Branch
	case GitStatus:
		m.gitDirty = msg.Dirty
	case SearchResults:
		m.searchResults = msg.Results
		m.searchCursor = 0
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
		m.textarea.SetWidth(m.width / 2) // Approximate, View will handle precision
		m.textarea.SetHeight(m.height - 3)
		m.terminalLog.Width = m.width / 2
		m.terminalLog.Height = m.height - 3
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			filesWidth := m.width / 5
			editorStart := filesWidth + 1
			editorWidth := (m.width - filesWidth - 2) / 2
			terminalStart := editorStart + editorWidth + 1

			if msg.X < filesWidth {
				m.active = "files"
				m.textarea.Blur()
			} else if msg.X >= editorStart && msg.X < terminalStart {
				m.active = "editor"
				m.textarea.Focus()
			} else if msg.X >= terminalStart {
				m.active = "terminal"
				m.textarea.Blur()
			}
		}
	case tea.KeyMsg:
		// Prompt handling
		if m.promptMode != "none" {
			switch msg.String() {
			case "esc":
				m.promptMode = "none"
				m.promptValue = ""
			case "enter":
				mode := m.promptMode
				val := m.promptValue
				m.promptMode = "none"
				m.promptValue = ""
				switch mode {
				case "new":
					return m, createFile(m.bridge, val)
				case "delete":
					if val == "y" || val == "Y" {
						node := m.flatTree[m.cursor]
						return m, deleteFile(m.bridge, node.Path)
					}
				case "rename":
					node := m.flatTree[m.cursor]
					return m, renameFile(m.bridge, node.Path, val)
				case "search":
					if len(m.searchResults) > 0 {
						res := m.searchResults[m.searchCursor]
						return m, readFile(m.bridge, res.File)
					}
				}
			case "backspace":
				if len(m.promptValue) > 0 {
					m.promptValue = m.promptValue[:len(m.promptValue)-1]
				}
				if m.promptMode == "search" {
					return m, searchFiles(m.bridge, m.promptValue)
				}
			case "up":
				if m.promptMode == "search" && m.searchCursor > 0 {
					m.searchCursor--
				}
			case "down":
				if m.promptMode == "search" && m.searchCursor < len(m.searchResults)-1 {
					m.searchCursor++
				}
			default:
				if msg.Type == tea.KeyRunes || msg.String() == " " {
					m.promptValue += msg.String()
					if m.promptMode == "search" {
						return m, searchFiles(m.bridge, m.promptValue)
					}
				}
			}
			return m, nil
		}

		// Global keys
		switch msg.String() {
		case "ctrl+c":
			if m.active == "terminal" {
				return m, writeTerminal(m.bridge, "\x03")
			}
			if m.bridge != nil {
				m.bridge.Close()
			}
			return m, tea.Quit
		case "ctrl+t":
			m.themeIdx = (m.themeIdx + 1) % len(m.themes)
			return m, nil
		case "ctrl+f":
			m.promptMode = "search"
			m.searchResults = nil
			return m, nil
		case "ctrl+n":
			m.promptMode = "new"
			return m, nil
		case "f2":
			if m.active == "files" && len(m.flatTree) > 0 {
				m.promptMode = "rename"
				m.promptValue = m.flatTree[m.cursor].Name
			}
			return m, nil
		case "delete":
			if m.active == "files" && len(m.flatTree) > 0 {
				m.promptMode = "delete"
				m.promptValue = "" // Reset for confirm
			}
			return m, nil
		case "ctrl+s":
			if m.currentPath != "" {
				return m, saveFile(m.bridge, m.currentPath, m.textarea.Value())
			}
		case "ctrl+r":
			return m, tea.Batch(fetchFileTree(m.bridge, "."), fetchGitInfo(m.bridge))
		case "ctrl+]":
			switch m.active {
			case "files":
				m.active = "editor"
				m.textarea.Focus()
			case "editor":
				m.active = "terminal"
				m.textarea.Blur()
			case "terminal":
				m.active = "files"
			}
			return m, nil
		}

		// Terminal input
		if m.active == "terminal" {
			switch msg.String() {
			case "enter":
				input := m.terminalInput + "\r\n"
				m.terminalInput = ""
				return m, writeTerminal(m.bridge, input)
			case "backspace":
				if len(m.terminalInput) > 0 {
					m.terminalInput = m.terminalInput[:len(m.terminalInput)-1]
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes || msg.String() == " " {
					m.terminalInput += msg.String()
					return m, nil
				}
			}
		}

		// Panel specific keys
		switch m.active {
		case "files":
			switch msg.String() {
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.flatTree)-1 {
					m.cursor++
				}
			case "enter":
				if len(m.flatTree) > 0 {
					node := m.flatTree[m.cursor]
					if node.IsDir {
						m.expanded[node.Path] = !m.expanded[node.Path]
						m.flatTree = flattenTree(m.rootNode, 0, m.expanded)
					} else {
						return m, readFile(m.bridge, node.Path)
					}
				}
			}
		}
	}

	if m.active == "editor" {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.active == "terminal" {
		m.terminalLog, cmd = m.terminalLog.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func renderFileTree(m model, width, height int) string {
	var s strings.Builder
	theme := m.currentTheme()
	
	for i, fn := range m.flatTree {
		if i >= height {
			break
		}

		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor && m.active == "files" {
			cursor = "> "
			style = style.Background(theme.Active).Foreground(theme.Background).Bold(true)
		} else {
			if fn.IsDir {
				style = style.Foreground(theme.Folder)
			} else {
				style = style.Foreground(theme.File)
			}
		}

		icon := "─ "
		if fn.IsDir {
			if m.expanded[fn.Path] {
				icon = "▼ "
			} else {
				icon = "▶ "
			}
		}

		indent := strings.Repeat("  ", fn.Depth)
		line := fmt.Sprintf("%s%s%s%s", cursor, indent, icon, fn.Name)
		if len(line) > width {
			line = line[:width]
		}
		s.WriteString(style.Width(width).Render(line) + "\n")
	}
	return s.String()
}

func renderPanelHeader(m model, title string, width int, active bool) string {
	theme := m.currentTheme()
	textColor := theme.Inactive
	if active {
		textColor = theme.Active
	}

	label := fmt.Sprintf(" %s ", title)
	leftDashes := 2
	rightDashes := width - leftDashes - len(label)
	if rightDashes < 0 {
		rightDashes = 0
	}

	header := lipgloss.NewStyle().Foreground(theme.Border).Render("──") +
		lipgloss.NewStyle().Foreground(textColor).Render(label) +
		lipgloss.NewStyle().Foreground(theme.Border).Render(strings.Repeat("─", rightDashes))

	return header
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	theme := m.currentTheme()
	headerHeight := 1
	footerHeight := 1
	mainHeight := m.height - headerHeight - footerHeight
	
	filesWidth := m.width / 5
	dividerWidth := 1
	remainingWidth := m.width - filesWidth - (dividerWidth * 2)
	editorWidth := remainingWidth / 2
	terminalWidth := remainingWidth - editorWidth

	headerStyle := lipgloss.NewStyle().
		Height(headerHeight).
		Width(m.width).
		Background(theme.Background)
	
	brand := lipgloss.NewStyle().Foreground(theme.Active).Bold(true).Render(" TRIX")
	folder := lipgloss.NewStyle().Foreground(theme.Inactive).Width(m.width - 25).Align(lipgloss.Center).Render(".")
	themeName := lipgloss.NewStyle().Foreground(theme.Inactive).Render(theme.Name + " ")
	header := headerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, brand, folder, themeName))

	panelStyle := lipgloss.NewStyle().Height(mainHeight).Background(theme.Background)
	dividerStyle := lipgloss.NewStyle().Width(dividerWidth).Height(mainHeight).Foreground(theme.Border).Background(theme.Background)
	divider := dividerStyle.Render("│")

	filesHeader := renderPanelHeader(m, "Files", filesWidth, m.active == "files")
	filesContent := renderFileTree(m, filesWidth, mainHeight-1)
	filesPanel := panelStyle.Width(filesWidth).Render(lipgloss.JoinVertical(lipgloss.Left, filesHeader, filesContent))

	editorHeader := renderPanelHeader(m, "Editor", editorWidth, m.active == "editor")
	m.textarea.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(theme.BottomBar)
	m.textarea.SetWidth(editorWidth)
	m.textarea.SetHeight(mainHeight - 1)
	editorContent := m.textarea.View()
	editorPanel := panelStyle.Width(editorWidth).Render(lipgloss.JoinVertical(lipgloss.Left, editorHeader, editorContent))

	terminalHeader := renderPanelHeader(m, "Terminal", terminalWidth, m.active == "terminal")
	m.terminalLog.Width = terminalWidth
	m.terminalLog.Height = mainHeight - 2
	terminalContent := m.terminalLog.View()
	
	terminalInput := lipgloss.NewStyle().Foreground(theme.Active).Render("> ") + 
		m.terminalInput + lipgloss.NewStyle().Foreground(theme.Active).Blink(true).Render("█")
	
	terminalPanel := panelStyle.Width(terminalWidth).Render(lipgloss.JoinVertical(lipgloss.Left, 
		terminalHeader, 
		terminalContent,
		terminalInput,
	))

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top,
		filesPanel,
		divider,
		editorPanel,
		divider,
		terminalPanel,
	)

	footerStyle := lipgloss.NewStyle().
		Height(footerHeight).
		Width(m.width).
		Background(theme.BottomBar)
	
	filename := m.currentPath
	if filename == "" {
		filename = "No file"
	}
	
	dirtyIcon := ""
	if m.gitDirty {
		dirtyIcon = lipgloss.NewStyle().Foreground(theme.Active).Render(" ●")
	}
	
	gitInfo := ""
	if m.gitBranch != "" {
		gitInfo = fmt.Sprintf("  %s%s", m.gitBranch, dirtyIcon)
	}

	line, col := m.textarea.Line(), m.textarea.LineInfo().ColumnOffset
	cursorPos := fmt.Sprintf(" Ln %d, Col %d ", line+1, col+1)

	leftStatus := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Background(theme.Active).Foreground(theme.Background).Bold(true).Render(" TRIX "),
		lipgloss.NewStyle().Foreground(theme.Active).Render(" "+filename+" "),
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(cursorPos),
		lipgloss.NewStyle().Foreground(theme.Inactive).Render(gitInfo),
	)

	keybindings := lipgloss.NewStyle().Foreground(theme.Inactive).Render(" ^S Save  ^R Reload  ^] Cycle  ^C Quit ")
	spacerWidth := m.width - lipgloss.Width(leftStatus) - lipgloss.Width(keybindings)
	if spacerWidth < 0 { spacerWidth = 0 }
	spacer := lipgloss.NewStyle().Width(spacerWidth).Render("")
	
	footer := footerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, leftStatus, spacer, keybindings))

	view := lipgloss.JoinVertical(lipgloss.Left, header, mainArea, footer)

	if m.promptMode == "search" {
		overlayWidth := m.width - 10
		overlayHeight := 10
		overlayStyle := lipgloss.NewStyle().
			Width(overlayWidth).
			Height(overlayHeight).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Active).
			Background(theme.Background).
			Padding(0, 1)

		var searchLines []string
		searchLines = append(searchLines, lipgloss.NewStyle().Foreground(theme.Active).Render("Search: ")+m.promptValue+"█")
		searchLines = append(searchLines, strings.Repeat("─", overlayWidth-2))
		
		for i, res := range m.searchResults {
			if i >= overlayHeight-3 {
				break
			}
			style := lipgloss.NewStyle().Foreground(theme.Inactive)
			if i == m.searchCursor {
				style = style.Foreground(theme.Active).Bold(true).Background(theme.BottomBar)
			}
			line := fmt.Sprintf("%s:%d: %s", res.File, res.Line, res.Text)
			if len(line) > overlayWidth-4 {
				line = line[:overlayWidth-7] + "..."
			}
			searchLines = append(searchLines, style.Render(line))
		}

		overlay := overlayStyle.Render(strings.Join(searchLines, "\n"))
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	} else if m.promptMode != "none" {
		promptText := ""
		switch m.promptMode {
		case "new": promptText = "New file name: "
		case "delete": 
			nodeName := ""
			if len(m.flatTree) > m.cursor { nodeName = m.flatTree[m.cursor].Name }
			promptText = fmt.Sprintf("Delete %s? (y/n): ", nodeName)
		case "rename": promptText = "Rename to: "
		}
		
		prompt := lipgloss.NewStyle().
			Background(theme.Active).
			Foreground(theme.Background).
			Bold(true).
			Render(" " + promptText + m.promptValue + "█ ")
		
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Bottom, prompt)
	}

	return view
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

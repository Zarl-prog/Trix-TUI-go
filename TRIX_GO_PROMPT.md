# Trix-TUI-Go — Complete Feature Migration Prompt

You are completing **Trix**, a terminal IDE with a Go (Bubble Tea + Lip Gloss) UI
layer and a Python backend (`core_bridge.py`) connected via JSON-RPC over stdin/stdout.

The existing Go repo has: `main.go`, `bridge.go`, `core_bridge.py`
The Python reference repo has all features already working.

Your job is to port EVERY feature from the Python version into Go with better UI.
`bridge.go` is complete — do NOT touch it.

All Python functions must only write to stdout via `send_response()` or `send_event()`
— never bare `print()` or the bridge breaks.

---

## PART 1 — FIX THE CRASH (main.go)

`strings.Builder` cannot be copied by value inside a Bubble Tea model struct.

```go
// In the model struct, change:
terminalBuf strings.Builder
// To:
terminalBuf *strings.Builder

// In initialModel(), change:
terminalBuf: &strings.Builder{},

// In Update(), the usage is the same — no other changes needed
```

---

## PART 2 — TERMINAL INPUT BAR (main.go)

The terminal panel has no way to type commands. Add a proper input bar.

Add to `model` struct:
```go
terminalInput string
terminalHistory []string
terminalHistIdx int
```

In `Update()`, when `m.active == "terminal"`, handle `tea.KeyMsg`:
- Printable single characters → append to `m.terminalInput`
- `backspace` → remove last character from `m.terminalInput`
- `up` → cycle history backwards (like a real shell)
- `down` → cycle history forwards
- `enter` → append to `m.terminalHistory`, send `m.terminalInput + "\r\n"` via
  `writeTerminal()`, clear `m.terminalInput`, reset `m.terminalHistIdx = -1`
- `ctrl+c` → send `"\x03"` via `writeTerminal()`
- `ctrl+l` → send `"clear\r\n"` via `writeTerminal()`, clear `m.terminalBuf`

In `View()`, render terminal panel as two rows:
```
┌─ Terminal ────────────────────────────┐
│ [viewport showing terminal output]   │
│ > terminalInput█                     │  ← input line at bottom
└──────────────────────────────────────┘
```

Style the input line:
- `"> "` in activeColor (`#5ac1fe`)
- typed text in white (`#ffffff`)
- cursor block `"█"` blinking (toggle with a `tickCmd` every 500ms)

---

## PART 3 — FOLDER EXPAND / COLLAPSE (main.go)

Currently all folders dump everything into `flatTree`. Add proper tree navigation.

Add to `model` struct:
```go
expanded map[string]bool
```

In `initialModel()`:
```go
expanded: map[string]bool{".": true},
```

Change `flattenTree` to respect `expanded`:
```go
type FileNode struct {
    Name     string
    Path     string
    IsDir    bool
    Depth    int        // ADD THIS
    Children []FileNode
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
```

In `Update()`, when `active == "files"` and key is `enter`:
```go
node := m.flatTree[m.cursor]
if node.IsDir {
    m.expanded[node.Path] = !m.expanded[node.Path]
    m.flatTree = flattenVisible(m.rootNode, m.expanded, 0)
} else {
    return m, readFile(m.bridge, node.Path)
}
```

In `renderFileTree()`:
- Indent: `strings.Repeat("  ", node.Depth)`
- Dirs: `"▶ "` when collapsed, `"▼ "` when expanded
- Files: `"  "` prefix (no icon needed, or use `"─ "`)
- Highlight current cursor row with `activeColor` background

---

## PART 4 — STATUS BAR (main.go + core_bridge.py)

Replace the static footer with a real status bar.

**Python side** — add to `core_bridge.py`:
```python
def get_git_branch(path):
    try:
        r = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            cwd=path, capture_output=True, text=True, timeout=2
        )
        branch = r.stdout.strip()
        dirty_r = subprocess.run(
            ["git", "status", "--short"],
            cwd=path, capture_output=True, text=True, timeout=2
        )
        dirty = bool(dirty_r.stdout.strip())
        return {"status": "ok", "branch": branch, "dirty": dirty}
    except Exception as e:
        return {"status": "error", "message": str(e)}
```

Add to dispatcher in `main()`:
```python
elif method == "get_git_branch":
    res = get_git_branch(params.get("path", "."))
```

**Go side** — add to `model` struct:
```go
gitBranch   string
gitDirty    bool
cursorLine  int
cursorCol   int
```

Add `fetchGitBranch` command (same pattern as `fetchFileTree`).
Call it in `Init()`.
Handle the response in `Update()` to set `m.gitBranch` and `m.gitDirty`.

When a file is opened, update `m.cursorLine` and `m.cursorCol`.

**Status bar layout** (replace footer in `View()`):
```
 TRIX  filename.py *    Ln 12, Col 4     main ●    Python   F1 Help
 ^^^^  ^^^^^^^^^^^       ^^^^^^^^^^^     ^^^^^^^^^  ^^^^^^
 brand  file+unsaved      cursor pos     git        lang
```

Styles:
- Brand `TRIX`: activeColor + bold
- Filename: white, `*` suffix in `unsavedColor` (`#feb454`) if dirty
- Cursor pos: `inactiveColor`
- Git branch: `inactiveColor`, `●` in warning color (`#e6b450`) if dirty
- Language: `inactiveColor`
- Right-aligned hints: `^S Save  ^R Reload  ^] Cycle  ^C Quit`

---

## PART 5 — MOUSE CLICK FOCUS (main.go)

Add `tea.WithMouseCellMotion()` to `tea.NewProgram()` options.

Handle `tea.MouseMsg` in `Update()`:
```go
case tea.MouseMsg:
    if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
        filesWidth    := m.width / 5
        divW          := 1
        remaining     := m.width - filesWidth - (divW * 2)
        editorWidth   := remaining / 2
        editorStart   := filesWidth + divW
        termStart     := editorStart + editorWidth + divW

        switch {
        case msg.X < filesWidth:
            m.active = "files"
        case msg.X >= editorStart && msg.X < termStart:
            m.active = "editor"
            m.textarea.Focus()
        case msg.X >= termStart:
            m.active = "terminal"
            m.textarea.Blur()
        }
    }
```

---

## PART 6 — DRAGGABLE DIVIDERS (main.go)

The Python version has draggable dividers. Port this to Go.

Add to `model` struct:
```go
dragging      bool
dragDivider   int    // 1 = files|editor divider, 2 = editor|terminal divider
dragStartX    int
filesWidth    int    // override, 0 = use default (width/5)
editorWidth   int    // override, 0 = use default
```

In `Update()`, handle `tea.MouseMsg`:
- `MouseActionPress` on divider column → set `m.dragging = true`, record which divider
- `MouseActionMotion` while dragging → update `m.filesWidth` or `m.editorWidth`
- `MouseActionRelease` → set `m.dragging = false`

In `View()`, use `m.filesWidth` if non-zero, else `m.width / 5`.
Render dividers as `"│"` (U+2502), colored `#3f4043`, highlight blue `#5ac1fe` when hovered.

---

## PART 7 — FILE OPERATIONS (main.go + core_bridge.py)

**Python side** — add to `core_bridge.py`:
```python
def create_file(path):
    try:
        p = Path(path)
        p.parent.mkdir(parents=True, exist_ok=True)
        p.touch()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def delete_file(path):
    try:
        Path(path).unlink()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def rename_file(old_path, new_path):
    try:
        Path(old_path).rename(new_path)
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}
```

Add to dispatcher in `main()`:
```python
elif method == "create_file":
    res = create_file(params.get("path"))
elif method == "delete_file":
    res = delete_file(params.get("path"))
elif method == "rename_file":
    res = rename_file(params.get("old_path"), params.get("new_path"))
```

**Go side** — add an `overlayMode` string to `model`:
```go
overlayMode   string   // "", "new_file", "rename", "delete_confirm", "open_folder"
overlayInput  string   // current text typed in overlay
overlayTitle  string   // prompt label shown in overlay
```

Handle these keybindings anywhere (not just in active panel):
- `ctrl+n` → `m.overlayMode = "new_file"`, `m.overlayTitle = "New file name:"`
- `ctrl+w` → clear editor, `m.currentPath = ""`
- `ctrl+o` → `m.overlayMode = "open_folder"`, `m.overlayTitle = "Open folder path:"`
- `f2` → only if file is open: `m.overlayMode = "rename"`, pre-fill `m.overlayInput`
  with current filename
- `delete` → only if `m.active == "files"` and file selected:
  `m.overlayMode = "delete_confirm"`,
  `m.overlayTitle = "Delete " + filename + "? (y/n)"`

When `m.overlayMode != ""`:
- All key presses go to overlay input, not panels
- `enter` → execute the action, close overlay, refresh file tree
- `escape` → cancel, close overlay

Render overlay as a centered floating box in `View()`:
```
╭─────────────────────────────────────╮
│  New file name:                     │
│  > myfile.go█                       │
│  [Enter] confirm   [Esc] cancel     │
╰─────────────────────────────────────╯
```

Style: border in `activeColor`, background `#1f2127`, dim backdrop.

---

## PART 8 — INLINE EDITOR SEARCH (main.go)

Add to `model` struct:
```go
searchOpen    bool
searchQuery   string
searchMatches []SearchMatch   // {line, colStart, colEnd int}
searchIdx     int
```

```go
type SearchMatch struct {
    Line     int
    ColStart int
    ColEnd   int
}
```

`ctrl+f` → `m.searchOpen = true`, clear query and matches.

When `m.searchOpen`:
- Key input goes to `m.searchQuery`
- `enter` → advance to next match
- `shift+tab` or `ctrl+p` → go to previous match
- `escape` → close search bar

Search logic (run on every keypress in search mode):
```go
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
```

Render search bar docked at top of editor panel:
```
 Search: query█    3 of 12    [Enter] next  [Esc] close
```

Highlight current match line in the editor viewport using a background color.

---

## PART 9 — GLOBAL FILE SEARCH (main.go + core_bridge.py)

**Python side** — add to `core_bridge.py`:
```python
def search_files(root, query):
    results = []
    try:
        for fpath in Path(root).rglob("*"):
            if not fpath.is_file():
                continue
            if any(p in str(fpath) for p in [".git", "__pycache__", "node_modules"]):
                continue
            try:
                lines = fpath.read_text(encoding="utf-8", errors="ignore").splitlines()
            except Exception:
                continue
            for i, line in enumerate(lines):
                if query.lower() in line.lower():
                    results.append({
                        "file": str(fpath),
                        "line": i + 1,
                        "text": line.strip()[:120]
                    })
                    if len(results) >= 200:
                        return {"status": "ok", "results": results}
    except Exception as e:
        return {"status": "error", "message": str(e)}
    return {"status": "ok", "results": results}
```

Add to dispatcher:
```python
elif method == "search_files":
    res = search_files(params.get("root", "."), params.get("query", ""))
```

**Go side** — add to `model`:
```go
globalSearchOpen    bool
globalSearchQuery   string
globalSearchResults []GlobalResult
globalSearchIdx     int
```

```go
type GlobalResult struct {
    File string
    Line int
    Text string
}
```

`ctrl+shift+f` → `m.globalSearchOpen = true`

Render a full-width overlay panel at the bottom half of the screen:
```
╭─ Search in Files ───────────────────────────────────╮
│ > query█                                            │
├─────────────────────────────────────────────────────┤
│ src/main.go:42    func initialModel() model {       │
│ src/bridge.go:11  type Bridge struct {              │  ← selected
│ ...                                                 │
╰─────────────────────────────────────────────────────╯
```

On `enter` → open the selected file at the correct line, close overlay.

---

## PART 10 — THEME SWITCHER (main.go)

Define 3 themes as Go structs:

```go
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
```

Add `themeIdx int` to `model`.
`ctrl+t` → `m.themeIdx = (m.themeIdx + 1) % len(Themes)`

Use `Themes[m.themeIdx]` everywhere instead of hardcoded color constants.
Update the header theme label on every cycle.

---

## PART 11 — ZEN MODE (main.go)

Add `zenMode bool` to `model`.

`ctrl+\` → toggle `m.zenMode`

In `View()`, when `m.zenMode == true`:
- Render ONLY the editor textarea, full width and full height
- Hide: header, files panel, terminal panel, both dividers, status bar
- Show a tiny dim hint in the bottom-right corner: `"ctrl+\\ exit zen"`

---

## PART 12 — TOGGLE FILE TREE (main.go)

Add `fileTreeVisible bool` to `model` (default `true`).

`ctrl+b` → toggle `m.fileTreeVisible`

In `View()`, when `m.fileTreeVisible == false`:
- Skip rendering the files panel and first divider
- Give the freed width to the editor panel

---

## PART 13 — OPEN FOLDER (main.go)

`ctrl+o` → open the folder overlay (Part 7 overlay system).

On confirm:
- Call `fetchFileTree(m.bridge, enteredPath)` to reload the tree
- Update the header folder label
- Clear the editor
- Reset `m.currentPath = ""`

---

## PART 14 — HELP SCREEN (main.go)

`f1` → toggle `m.helpOpen bool`

When `m.helpOpen == true`, render a centered overlay showing:

```
╭─────────────────── Keyboard Shortcuts ───────────────────╮
│                                                          │
│  File                                                    │
│    Ctrl+N    New File         Ctrl+S    Save             │
│    Ctrl+W    Close File       Ctrl+O    Open Folder      │
│    F2        Rename File      Delete    Delete File      │
│                                                          │
│  Layout                                                  │
│    Ctrl+]    Cycle Panels     Click     Focus Panel      │
│    Ctrl+B    Toggle Files     Ctrl+\    Zen Mode         │
│                                                          │
│  Editor                                                  │
│    Ctrl+Z    Undo             Ctrl+Y    Redo             │
│    Ctrl+A    Select All       Ctrl+_    Toggle Comment   │
│    Ctrl+D    Duplicate Line   Ctrl+F    Search           │
│    Ctrl+Shift+F  Global Search                           │
│                                                          │
│  Terminal                                                │
│    Enter     Run command      Ctrl+C    Interrupt        │
│    ↑ ↓       History          Ctrl+L    Clear            │
│                                                          │
│  General                                                 │
│    Ctrl+T    Cycle Theme      Ctrl+Q    Quit             │
│    F1        Close this help                             │
│                                                          │
╰──────────────────────────────────────────────────────────╯
```

`escape` or `f1` → close help.

---

## PART 15 — LANGUAGE DETECTION + SYNTAX HINTS (main.go)

When a file is opened, detect language from extension:

```go
var langMap = map[string]string{
    ".py": "Python", ".go": "Go", ".js": "JavaScript", ".ts": "TypeScript",
    ".jsx": "JavaScript", ".tsx": "TypeScript", ".rs": "Rust", ".c": "C",
    ".cpp": "C++", ".h": "C", ".hpp": "C++", ".java": "Java",
    ".rb": "Ruby", ".php": "PHP", ".sh": "Bash", ".bash": "Bash",
    ".json": "JSON", ".yaml": "YAML", ".yml": "YAML", ".toml": "TOML",
    ".md": "Markdown", ".html": "HTML", ".htm": "HTML", ".css": "CSS",
    ".sql": "SQL", ".xml": "XML", ".svg": "XML",
}
```

Store detected language in `m.currentLang string`.
Show it in the status bar (Part 4).

---

## PART 16 — UNSAVED CHANGES GUARD (main.go)

Add `hasChanges bool` to `model`.

- Set `m.hasChanges = true` on every keystroke in the editor textarea
- Set `m.hasChanges = false` after a successful save or file open
- On `ctrl+q`: if `m.hasChanges == true`, show confirm overlay:
  `"Unsaved changes. Quit anyway? (y/n)"`
  Only quit if user confirms

---

## FINAL WIRING — main() (main.go)

```go
func main() {
    p := tea.NewProgram(
        initialModel(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),   // ← required for Part 5
    )
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}
```

---

## SUMMARY — Feature Parity Checklist

| Feature                        | Python ✓ | Go Target |
|-------------------------------|----------|-----------|
| 3-panel layout                | ✓        | Part 1    |
| Terminal input bar            | ✓        | Part 2    |
| Folder expand/collapse        | ✓        | Part 3    |
| Status bar + git branch       | ✓        | Part 4    |
| Mouse click focus panels      | ✓        | Part 5    |
| Draggable dividers            | ✓        | Part 6    |
| New / Delete / Rename file    | ✓        | Part 7    |
| Inline editor search (Ctrl+F) | ✓        | Part 8    |
| Global file search            | ✓        | Part 9    |
| Theme switcher (3 themes)     | ✓        | Part 10   |
| Zen mode                      | ✓        | Part 11   |
| Toggle file tree              | ✓        | Part 12   |
| Open folder                   | ✓        | Part 13   |
| Help screen (F1)              | ✓        | Part 14   |
| Language detection            | ✓        | Part 15   |
| Unsaved changes guard         | ✓        | Part 16   |

Build after each part: `go build . && ./trix.exe`
Do NOT change `bridge.go`.

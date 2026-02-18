# mull-tui Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `mull-tui`, a Bubble Tea TUI binary for browsing matters, docket, and generating prompts.

**Architecture:** Separate binary (`cmd/mull-tui/main.go`) in the same Go module, sharing `internal/model` and `internal/storage`. TUI code lives in `internal/tui/`. Uses Bubble Tea v1 (stable) with Lip Gloss styling and fsnotify for live reload.

**Tech Stack:** Go, Bubble Tea v1, Bubbles (table, viewport, textinput), Lip Gloss, fsnotify

**Design doc:** `docs/plans/2026-02-18-mull-tui-design.md`

---

### Task 1: Add dependencies and entry point

**Files:**
- Create: `cmd/mull-tui/main.go`
- Modify: `Makefile`
- Modify: `go.mod` (via go get)

**Step 1: Add Bubble Tea dependencies**

Run:
```bash
cd /Users/jeff/dev/kit/mull
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/fsnotify/fsnotify@latest
```

**Step 2: Create the entry point**

Create `cmd/mull-tui/main.go`:
```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"mull/internal/storage"
	"mull/internal/tui"
)

func main() {
	store, err := storage.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.NewApp(store), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Update the Makefile**

Add a `build-tui` target and update `install` and `clean`:
```makefile
TUI_BINARY := mull-tui

build-tui:
	go build -o $(TUI_BINARY) ./cmd/mull-tui

install:
	go install .
	go install ./cmd/mull-tui
	@echo "Installed $(BINARY_NAME) and $(TUI_BINARY) to $(GOBIN)"
	@echo "Make sure $(GOBIN) is in your PATH"

clean:
	rm -f $(BINARY_NAME) $(TUI_BINARY)
	rm -f coverage.out coverage.html
```

Also add `build-tui` to the `help` target output.

**Step 4: Create minimal app stub so it compiles**

Create `internal/tui/app.go` with a minimal model that just shows "mull-tui" and quits on `q`. This will be replaced in subsequent tasks but lets us verify the build pipeline works.

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"mull/internal/storage"
)

type App struct {
	store *storage.Store
}

func NewApp(store *storage.Store) App {
	return App{store: store}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a App) View() string {
	return "mull-tui - press q to quit\n"
}
```

**Step 5: Build and verify**

Run:
```bash
make build-tui
./mull-tui
```
Expected: TUI launches in alt screen, shows stub text, `q` exits cleanly.

**Step 6: Commit**

```bash
git add cmd/mull-tui/ internal/tui/ Makefile go.mod go.sum
git commit -m "Scaffold mull-tui binary with Bubble Tea stub"
```

---

### Task 2: Styles and keybinding definitions

**Files:**
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keys.go`

**Step 1: Define styles**

Create `internal/tui/styles.go` with Lip Gloss styles for:
- Tab bar (active tab, inactive tab)
- Table header row
- Selected row highlight
- Status badge colors (raw=gray, refined=yellow, planned=blue, done=green, dropped=red)
- Status bar at the bottom (flash messages, filter indicator)
- Detail view heading and body
- Help overlay

Use `lipgloss.NewStyle()` and keep colors restrained — the terminal may be light or dark. Use adaptive colors where possible (`lipgloss.AdaptiveColor`).

**Step 2: Define keybindings**

Create `internal/tui/keys.go` using `bubbles/key` for binding definitions:

```go
package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Esc     key.Binding
	Quit    key.Binding
	Help    key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Open    key.Binding
	Closed  key.Binding
	All     key.Binding
	Status  key.Binding
	Epic    key.Binding
	Search  key.Binding
	Prompt  key.Binding
	Docket  key.Binding
	Refresh key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
		Esc:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "matters")),
		Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "docket")),
		Open:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open only")),
		Closed:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "closed only")),
		All:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")),
		Status:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle status")),
		Epic:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "filter epic")),
		Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Prompt:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "copy prompt")),
		Docket:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "toggle docket")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}
```

**Step 3: Commit**

```bash
git add internal/tui/styles.go internal/tui/keys.go
git commit -m "Add TUI styles and keybinding definitions"
```

---

### Task 3: Data loading and the App model

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Define the full App model**

Replace the stub `App` with the real model. It needs:

```go
type viewMode int

const (
	viewMatters viewMode = iota
	viewDocket
	viewDetail
)

type filterMode int

const (
	filterOpen filterMode = iota    // raw, refined, planned
	filterClosed                     // done, dropped
	filterAll
	filterStatus                     // single status
)

type App struct {
	store   *storage.Store
	keys    keyMap

	// Data
	matters []*model.Matter
	docket  []model.DocketEntry
	docketSet map[string]bool  // quick lookup

	// View state
	view       viewMode
	prevView   viewMode
	cursor     int
	docketCursor int

	// Filter state
	filter       filterMode
	statusFilter string          // when filterStatus, which one
	epicFilter   string
	searchQuery  string
	searching    bool            // search input active

	// UI components
	searchInput textinput.Model
	viewport    viewport.Model   // for detail view

	// Detail
	detailMatter *model.Matter

	// Flash message
	flash     string
	flashTimer *time.Timer

	// Dimensions
	width  int
	height int

	// Help
	showHelp bool
}
```

**Step 2: Implement data loading**

Add a `loadData` method that calls `store.ListMatters(nil)` and `store.LoadDocket()`, then stores results. Also build a `docketSet` map for O(1) membership checks.

Add a `filteredMatters()` method that applies the current filter/search state and returns the slice to display.

Add a `docketMatters()` method that joins docket entries with matter data, preserving docket order.

**Step 3: Implement Init**

`Init()` should return a `tea.Batch` of:
- A command that loads data and returns a `dataLoadedMsg`
- A command that starts the file watcher (Task 6)

For now, just the data load command. Watcher comes in Task 6.

**Step 4: Build and verify**

```bash
make build-tui
```
Expected: Compiles. Not yet runnable because Update/View are stubs.

**Step 5: Commit**

```bash
git add internal/tui/app.go
git commit -m "Define full App model with data loading and filter state"
```

---

### Task 4: Matters list view

**Files:**
- Create: `internal/tui/matters.go`
- Modify: `internal/tui/app.go` (wire into Update/View)

**Step 1: Implement the matters list renderer**

Create `internal/tui/matters.go` with a function `renderMatters(a *App) string` that:
- Renders a tab bar at the top showing `[1 Matters]  2 Docket` with the active tab styled
- Renders a filter indicator line: e.g., `Filter: open` or `Filter: epic=dashboard` or `Search: "dark"`
- Renders a table with columns: ID, Title, Status, Epic, Effort
- Highlights the current cursor row
- Uses status badge colors from styles
- Truncates title to fit terminal width
- Shows count: "12 matters" in the status bar

**Step 2: Wire matters view into App.View()**

In `app.go`, update `View()` to dispatch based on `a.view`:
- `viewMatters` → `renderMatters(a)`
- `viewDocket` → placeholder string for now
- `viewDetail` → placeholder string for now

**Step 3: Wire navigation into App.Update()**

Handle in `Update()`:
- `j`/`k`/arrows → move cursor within `filteredMatters()` bounds
- `1` → switch to viewMatters
- `2` → switch to viewDocket (placeholder)
- `Enter` → switch to viewDetail, set `detailMatter`
- `q`/`ctrl+c` → quit
- `tea.WindowSizeMsg` → update width/height

**Step 4: Wire filter keys into Update()**

Handle:
- `o` → `filter = filterOpen`, reset cursor
- `c` → `filter = filterClosed`, reset cursor
- `a` → `filter = filterAll`, reset cursor
- `s` → cycle `statusFilter` through raw→refined→planned→done→dropped→(back to filterOpen)
- `/` → activate search input, focus it

For search: when `searching` is true, forward key events to `searchInput.Update()`. On Enter or Esc, deactivate searching. The search query filters `filteredMatters()` by case-insensitive title substring match.

**Step 5: Build and run**

```bash
make build-tui && ./mull-tui
```
Expected: Shows matters list with filtering. Navigation works. (Detail and docket are placeholders.)

**Step 6: Commit**

```bash
git add internal/tui/matters.go internal/tui/app.go
git commit -m "Implement matters list view with filtering and navigation"
```

---

### Task 5: Docket view

**Files:**
- Create: `internal/tui/docket.go`
- Modify: `internal/tui/app.go` (wire in)

**Step 1: Implement the docket list renderer**

Create `internal/tui/docket.go` with `renderDocket(a *App) string` that:
- Renders tab bar with `1 Matters  [2 Docket]`
- Renders a table with columns: #, ID, Title, Status, Epic, Note
- The `#` column is the 1-indexed position (priority order from docket.yml)
- Highlights cursor row
- Shows count: "5 items on docket"

**Step 2: Wire into App**

- `viewDocket` in `View()` calls `renderDocket(a)`
- Navigation (j/k/enter) works the same but uses `docketCursor` and `docketMatters()`
- Pressing `2` switches view and resets docket cursor
- Filter keys (`o`, `c`, `a`, `s`, `e`) are disabled in docket view (docket has its own ordering)

**Step 3: Build and verify**

```bash
make build-tui && ./mull-tui
```
Expected: Press `2` to see docket, `1` to go back. Navigation works in both.

**Step 4: Commit**

```bash
git add internal/tui/docket.go internal/tui/app.go
git commit -m "Implement docket view with priority ordering"
```

---

### Task 6: Detail view

**Files:**
- Create: `internal/tui/detail.go`
- Modify: `internal/tui/app.go` (wire in)

**Step 1: Implement detail renderer**

Create `internal/tui/detail.go` with `renderDetail(a *App) string` that:
- Shows matter title as a heading
- Shows metadata in a key-value layout:
  ```
  Status:  planned     Epic:    dashboard
  Effort:  medium      Created: 2026-02-15
  Tags:    feature, ui
  Relates: ab3f, c7d1  Blocks:  d040
  Needs:   a1b2        Parent:  xyz1
  Docs:    docs/plans/design.md
  ```
- Shows body text below, rendered in a viewport for scrolling
- Shows "Esc to go back" in the status bar

**Step 2: Wire into App**

- `viewDetail` in `View()` calls `renderDetail(a)`
- `Enter` in list views sets `detailMatter` from the current cursor position and switches to `viewDetail`
- `Esc` in detail returns to `prevView`
- `j`/`k` in detail scrolls the viewport

**Step 3: Build and verify**

```bash
make build-tui && ./mull-tui
```
Expected: Navigate to a matter, press Enter, see detail. Esc goes back. Body scrolls.

**Step 4: Commit**

```bash
git add internal/tui/detail.go internal/tui/app.go
git commit -m "Implement matter detail view with scrollable body"
```

---

### Task 7: File watcher

**Files:**
- Create: `internal/tui/watcher.go`
- Modify: `internal/tui/app.go` (integrate)

**Step 1: Implement the watcher**

Create `internal/tui/watcher.go`:

```go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

type dataReloadMsg struct{}

func watchFiles(mullDir string) tea.Cmd {
	return func() tea.Msg {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		// Watch the matters directory and docket file
		watcher.Add(mullDir + "/matters")
		watcher.Add(mullDir)

		// Debounce: wait for 100ms of quiet before sending reload
		var timer *time.Timer
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return nil
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(100*time.Millisecond, func() {})
				// Wait for the timer
				<-timer.C
				return dataReloadMsg{}
			case _, ok := <-watcher.Errors:
				if !ok {
					return nil
				}
			}
		}
	}
}
```

Note: Since Bubble Tea commands return a single `tea.Msg`, the watcher command fires once per batch of changes. After handling `dataReloadMsg` in Update, re-issue the watch command to keep listening.

**Step 2: Integrate into App**

- In `Init()`, add `watchFiles(store.Root())` to the batch
- In `Update()`, handle `dataReloadMsg`: reload data, preserve cursor position (clamp if list shrank), re-issue watch command
- Handle `r` key the same way: reload data immediately

**Step 3: Test manually**

1. Run `./mull-tui` in one terminal
2. In another terminal, run `mull add "Test live reload" --status raw`
3. The TUI should update to show the new matter within ~200ms

**Step 4: Commit**

```bash
git add internal/tui/watcher.go internal/tui/app.go
git commit -m "Add fsnotify file watcher for live data reload"
```

---

### Task 8: Prompt generation and clipboard

**Files:**
- Modify: `internal/tui/app.go` (handle `p` key)

**Step 1: Implement prompt generation**

Add a function in `app.go`:

```go
func generatePrompt(m *model.Matter) string {
	switch m.Status {
	case "raw":
		return fmt.Sprintf("Explore matter %s (%s) with the user", m.ID, m.Title)
	case "refined":
		return fmt.Sprintf("Help plan matter %s (%s)", m.ID, m.Title)
	case "planned":
		return fmt.Sprintf("Implement matter %s (%s)", m.ID, m.Title)
	case "done":
		return fmt.Sprintf("Review matter %s (%s)", m.ID, m.Title)
	case "dropped":
		return fmt.Sprintf("Revisit matter %s (%s)", m.ID, m.Title)
	default:
		return fmt.Sprintf("Work on matter %s (%s)", m.ID, m.Title)
	}
}
```

**Step 2: Implement clipboard copy**

```go
func copyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
```

**Step 3: Wire into Update**

On `p` key (when not searching, not in help):
1. Get the currently highlighted matter
2. Generate prompt
3. Copy to clipboard
4. Set `flash = "Copied to clipboard"` and start a 2-second timer that sends a `clearFlashMsg`

Add `clearFlashMsg` type and handle it in Update to clear the flash string.

**Step 4: Update View to show flash**

In the status bar area (bottom of screen), show `a.flash` when non-empty, styled distinctly.

**Step 5: Build and test**

```bash
make build-tui && ./mull-tui
```
Expected: Highlight a matter, press `p`, see "Copied to clipboard" flash, paste into another app to verify.

**Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "Add prompt generation with clipboard copy"
```

---

### Task 9: Docket toggle action

**Files:**
- Modify: `internal/tui/app.go` (handle `d` key)

**Step 1: Implement docket toggle**

On `d` key:
1. Get the currently highlighted matter
2. Check `docketSet[matter.ID]`
3. If in docket: call `store.DocketRemove(id)`, flash "Removed from docket"
4. If not in docket: call `store.DocketAdd(id, "", "")`, flash "Added to docket"
5. Reload data (the watcher will also catch it, but immediate feedback is better)

**Step 2: Build and test**

```bash
make build-tui && ./mull-tui
```
Expected: Press `d` to toggle, flash confirms, docket view updates.

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "Add docket toggle action"
```

---

### Task 10: Epic filter

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Implement epic picker**

On `e` key:
1. Collect unique epics from all matters
2. If only one epic exists, toggle it on/off
3. If multiple, cycle through them: epic1 → epic2 → ... → (no filter)
4. Update `epicFilter`, reset cursor
5. Show current epic filter in the filter indicator line

**Step 2: Build and test**

```bash
make build-tui && ./mull-tui
```
Expected: Press `e` to cycle through epics. Filter indicator updates.

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "Add epic filter cycling"
```

---

### Task 11: Help overlay

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Implement help overlay**

On `?` key, toggle `showHelp`. When true, `View()` renders a centered overlay box listing all keybindings from the keyMap, grouped by category:

```
Navigation          Filtering           Actions
─────────           ─────────           ───────
j/↑  up             o  open only        p  copy prompt
k/↓  down           c  closed only      d  toggle docket
enter  detail       a  all              r  refresh
esc    back         s  cycle status
1  matters          e  cycle epic
2  docket           /  search

Press ? to close
```

The overlay should be rendered on top of the current view (render the view underneath dimmed, overlay the help box centered).

**Step 2: Build and test**

```bash
make build-tui && ./mull-tui
```
Expected: Press `?` to toggle help overlay.

**Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "Add help overlay"
```

---

### Task 12: Final polish and install

**Files:**
- Modify: `internal/tui/app.go` (any remaining edges)
- Modify: `CLAUDE.md` (document the new binary)

**Step 1: Edge cases**

- Empty state: if no matters exist, show "No matters found. Run `mull add` to create one."
- Empty docket: show "Docket is empty. Press `d` on a matter to add it."
- Clamp cursors on data reload
- Handle narrow terminals gracefully (truncate columns)

**Step 2: Update CLAUDE.md**

Add to the Build Commands section:
```
make build-tui     # Build TUI binary (outputs ./mull-tui)
```

Note in Architecture section that `internal/tui/` contains the Bubble Tea TUI and `cmd/mull-tui/` is its entry point.

**Step 3: Full build and test**

```bash
make all && make build-tui && make install
```
Expected: Both binaries build and install cleanly.

**Step 4: Commit**

```bash
git add -A
git commit -m "Polish mull-tui and update documentation"
```

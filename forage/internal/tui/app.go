package tui

import (
	"strings"
	"time"

	"forage/internal/model"
	"forage/internal/storage"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	viewList viewMode = iota
	viewDetail
)

type filterMode int

const (
	filterNonTerminal filterMode = iota // wishlist, reading, paused, read
	filterWishlist
	filterReading
	filterPaused
	filterRead
	filterAll
)

var filterCycle = []filterMode{filterNonTerminal, filterWishlist, filterReading, filterPaused, filterRead, filterAll}
var filterNames = map[filterMode]string{
	filterNonTerminal: "active",
	filterWishlist:    "wishlist",
	filterReading:     "reading",
	filterPaused:      "paused",
	filterRead:        "read",
	filterAll:         "all",
}

type dataLoadedMsg struct {
	books []model.Book
}

type clearFlashMsg struct{}

type App struct {
	store *storage.Store
	keys  keyMap

	books []model.Book

	view   viewMode
	cursor int

	filter      filterMode
	searchQuery string
	searching   bool

	searchInput textinput.Model
	viewport    viewport.Model

	detailBook *model.Book

	flash    string
	width    int
	height   int
	showHelp bool
}

func NewApp(store *storage.Store) App {
	si := textinput.New()
	si.Prompt = "/ "
	si.CharLimit = 64

	return App{
		store:       store,
		keys:        newKeyMap(),
		searchInput: si,
	}
}

func (a App) Init() tea.Cmd {
	return a.loadDataCmd()
}

func (a App) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		books, _ := a.store.ListBooks(nil)
		return dataLoadedMsg{books: books}
	}
}

func (a *App) filteredBooks() []model.Book {
	var result []model.Book
	for _, b := range a.books {
		if !a.matchesFilter(b) {
			continue
		}
		if a.searchQuery != "" {
			q := strings.ToLower(a.searchQuery)
			hay := strings.ToLower(b.Title + " " + b.Author + " " + strings.Join(b.Tags, " "))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		result = append(result, b)
	}
	return result
}

func (a *App) matchesFilter(b model.Book) bool {
	switch a.filter {
	case filterNonTerminal:
		return !model.IsTerminal(b.Status)
	case filterWishlist:
		return b.Status == "wishlist"
	case filterReading:
		return b.Status == "reading"
	case filterPaused:
		return b.Status == "paused"
	case filterRead:
		return b.Status == "read"
	case filterAll:
		return true
	}
	return true
}

func (a *App) clampCursor() {
	fb := a.filteredBooks()
	if a.cursor >= len(fb) {
		a.cursor = max(0, len(fb)-1)
	}
}

func (a *App) currentBook() *model.Book {
	fb := a.filteredBooks()
	if a.cursor < len(fb) {
		b := fb[a.cursor]
		return &b
	}
	return nil
}

func matchKey(msg tea.KeyMsg, binding key.Binding) bool {
	for _, k := range binding.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

func (a *App) setFlash(msg string) tea.Cmd {
	a.flash = msg
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return clearFlashMsg{}
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case dataLoadedMsg:
		a.books = msg.books
		a.clampCursor()
		return a, nil

	case clearFlashMsg:
		a.flash = ""
		return a, nil

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.viewport.Width = msg.Width
		a.viewport.Height = msg.Height - 8
		return a, nil

	case tea.KeyMsg:
		if a.searching {
			return a.updateSearchInput(msg)
		}
		if a.showHelp {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				a.showHelp = false
			}
			return a, nil
		}
		if a.view == viewDetail {
			return a.updateDetail(msg)
		}
		return a.updateList(msg)
	}
	return a, nil
}

func (a App) updateSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		a.searchQuery = a.searchInput.Value()
		a.searching = false
		a.cursor = 0
		return a, nil
	case "esc":
		a.searching = false
		a.searchInput.SetValue(a.searchQuery)
		return a, nil
	}
	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	return a, cmd
}

func (a App) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.view = viewList
		return a, nil
	case "q", "ctrl+c":
		return a, tea.Quit
	case "j", "down":
		a.viewport.ScrollDown(1)
		return a, nil
	case "k", "up":
		a.viewport.ScrollUp(1)
		return a, nil
	}
	return a, nil
}

func (a App) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, a.keys.Quit):
		return a, tea.Quit

	case matchKey(msg, a.keys.Esc):
		if a.searchQuery != "" {
			a.searchQuery = ""
			a.searchInput.SetValue("")
			a.cursor = 0
			return a, nil
		}
		return a, nil

	case matchKey(msg, a.keys.Help):
		a.showHelp = true
		return a, nil

	case matchKey(msg, a.keys.Down):
		fb := a.filteredBooks()
		if a.cursor < len(fb)-1 {
			a.cursor++
		}
		return a, nil

	case matchKey(msg, a.keys.Up):
		if a.cursor > 0 {
			a.cursor--
		}
		return a, nil

	case matchKey(msg, a.keys.Enter):
		b := a.currentBook()
		if b != nil {
			a.detailBook = b
			a.view = viewDetail
			a.viewport.SetContent(b.Body)
			a.viewport.GotoTop()
		}
		return a, nil

	case matchKey(msg, a.keys.Refresh):
		return a, a.loadDataCmd()

	case matchKey(msg, a.keys.Search):
		a.searching = true
		a.searchInput.SetValue("")
		a.searchInput.Focus()
		return a, textinput.Blink

	case matchKey(msg, a.keys.SetStatus):
		b := a.currentBook()
		if b != nil {
			return a.cycleBookStatus(b)
		}
		return a, nil

	case matchKey(msg, a.keys.MarkRead):
		b := a.currentBook()
		if b != nil {
			return a.markRead(b)
		}
		return a, nil

	case matchKey(msg, a.keys.Drop):
		b := a.currentBook()
		if b != nil {
			return a.dropBook(b)
		}
		return a, nil

	case matchKey(msg, a.keys.Rate):
		b := a.currentBook()
		if b != nil {
			return a.rateBook(b, msg.String())
		}
		return a, nil

	case matchKey(msg, a.keys.CycleFilt):
		a.cycleFilter()
		return a, nil

	case matchKey(msg, a.keys.FilterAll):
		a.filter = filterAll
		a.cursor = 0
		return a, nil

	case matchKey(msg, a.keys.FilterWL):
		a.filter = filterWishlist
		a.cursor = 0
		return a, nil

	case matchKey(msg, a.keys.FilterPa):
		a.filter = filterPaused
		a.cursor = 0
		return a, nil

	case matchKey(msg, a.keys.FilterRd):
		a.filter = filterRead
		a.cursor = 0
		return a, nil
	}
	return a, nil
}

var statusProgression = []string{"wishlist", "reading", "paused", "read"}

func (a App) cycleBookStatus(b *model.Book) (tea.Model, tea.Cmd) {
	for i, s := range statusProgression {
		if s == b.Status {
			next := statusProgression[(i+1)%len(statusProgression)]
			if _, err := a.store.UpdateBook(b.ID, "status", next); err != nil {
				return a, a.setFlash("Error: " + err.Error())
			}
			return a, tea.Batch(a.setFlash(b.ID+" → "+next), a.loadDataCmd())
		}
	}
	return a, a.setFlash("Can't advance from " + b.Status)
}

func (a App) markRead(b *model.Book) (tea.Model, tea.Cmd) {
	today := time.Now().Format("2006-01-02")
	if _, err := a.store.UpdateBook(b.ID, "status", "read"); err != nil {
		return a, a.setFlash("Error: " + err.Error())
	}
	a.store.UpdateBook(b.ID, "date_read", today)
	return a, tea.Batch(a.setFlash(b.ID+" → read"), a.loadDataCmd())
}

func (a App) dropBook(b *model.Book) (tea.Model, tea.Cmd) {
	if _, err := a.store.UpdateBook(b.ID, "status", "dropped"); err != nil {
		return a, a.setFlash("Error: " + err.Error())
	}
	return a, tea.Batch(a.setFlash(b.ID+" → dropped"), a.loadDataCmd())
}

func (a App) rateBook(b *model.Book, digit string) (tea.Model, tea.Cmd) {
	if _, err := a.store.UpdateBook(b.ID, "rating", digit); err != nil {
		return a, a.setFlash("Error: " + err.Error())
	}
	var flash string
	if digit == "0" {
		flash = b.ID + " rating cleared"
	} else {
		n := int(digit[0] - '0')
		flash = b.ID + " " + strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
	}
	return a, tea.Batch(a.setFlash(flash), a.loadDataCmd())
}

func (a *App) cycleFilter() {
	for i, f := range filterCycle {
		if f == a.filter {
			a.filter = filterCycle[(i+1)%len(filterCycle)]
			a.cursor = 0
			return
		}
	}
	a.filter = filterNonTerminal
	a.cursor = 0
}

func (a App) View() string {
	var content string
	switch a.view {
	case viewList:
		content = renderList(&a)
	case viewDetail:
		content = renderDetail(&a)
	}

	if a.showHelp {
		content = renderHelpOverlay(&a, content)
	}

	return content
}

func renderHelpOverlay(a *App, bg string) string {
	help := `Navigation          Filtering           Actions
─────────           ─────────           ───────
j/↓  down           tab  cycle filter   S    cycle status
k/↑  up             a    all            D    mark read
enter  detail       w    wishlist       X    drop
esc    back/clear   p    paused         0-5  rate
/  search           d    read           r    refresh

Press ? to close`

	box := helpBox.Render(help)
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)

	x := (a.width - boxW) / 2
	if x < 0 {
		x = 0
	}
	y := (a.height - boxH) / 2
	if y < 0 {
		y = 0
	}

	return placeOverlay(x, y, box, bg)
}

func placeOverlay(x, y int, fg, bg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	needed := y + len(fgLines)
	for len(bgLines) < needed {
		bgLines = append(bgLines, "")
	}

	for i := range bgLines {
		bgLines[i] = dimOverlay.Render(stripANSI(bgLines[i]))
	}

	for i, fgLine := range fgLines {
		bgIdx := y + i
		if bgIdx >= 0 && bgIdx < len(bgLines) {
			line := bgLines[bgIdx]
			visW := lipgloss.Width(line)
			if visW < x {
				line += strings.Repeat(" ", x-visW)
			}
			bgLines[bgIdx] = truncateVisual(line, x) + fgLine
		}
	}

	return strings.Join(bgLines, "\n")
}

func stripANSI(s string) string {
	var result []rune
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

func truncateVisual(s string, n int) string {
	var result []rune
	visWidth := 0
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			result = append(result, r)
			continue
		}
		if inEscape {
			result = append(result, r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		if visWidth >= n {
			break
		}
		result = append(result, r)
		visWidth++
	}
	return string(result)
}

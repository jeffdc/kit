package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"mull/internal/model"
	"mull/internal/storage"
)

type viewMode int

const (
	viewMatters viewMode = iota
	viewDocket
	viewDetail
)

type filterMode int

const (
	filterOpen filterMode = iota // raw, refined, planned
	filterClosed                 // done, dropped
	filterAll
	filterStatus // single status
)

type dataLoadedMsg struct {
	matters []*model.Matter
	docket  []model.DocketEntry
}

type clearFlashMsg struct{}

type App struct {
	store *storage.Store
	keys  keyMap

	// Data
	matters   []*model.Matter
	docket    []model.DocketEntry
	docketSet map[string]bool

	// View state
	view         viewMode
	prevView     viewMode
	cursor       int
	docketCursor int

	// Filter state
	filter       filterMode
	statusFilter string
	epicFilter   string
	searchQuery  string
	searching    bool

	// UI components
	searchInput textinput.Model
	viewport    viewport.Model

	// Detail
	detailMatter *model.Matter

	// Flash message
	flash string

	// Dimensions
	width  int
	height int

	// Help
	showHelp bool
}

func NewApp(store *storage.Store) App {
	si := textinput.New()
	si.Prompt = "/ "
	si.CharLimit = 64

	return App{
		store:     store,
		keys:      newKeyMap(),
		docketSet: make(map[string]bool),
		searchInput: si,
	}
}

func (a App) Init() tea.Cmd {
	return a.loadDataCmd()
}

func (a App) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		matters, _ := a.store.ListMatters(nil)
		docket, _ := a.store.LoadDocket()
		return dataLoadedMsg{matters: matters, docket: docket}
	}
}

func (a *App) applyDataLoaded(msg dataLoadedMsg) {
	a.matters = msg.matters
	a.docket = msg.docket
	a.docketSet = make(map[string]bool)
	for _, e := range a.docket {
		a.docketSet[e.ID] = true
	}
}

func (a *App) filteredMatters() []*model.Matter {
	var result []*model.Matter
	for _, m := range a.matters {
		if !a.matchesFilter(m) {
			continue
		}
		if a.searchQuery != "" {
			if !strings.Contains(strings.ToLower(m.Title), strings.ToLower(a.searchQuery)) {
				continue
			}
		}
		if a.epicFilter != "" && m.Epic != a.epicFilter {
			continue
		}
		result = append(result, m)
	}
	return result
}

func (a *App) matchesFilter(m *model.Matter) bool {
	switch a.filter {
	case filterOpen:
		return !m.IsTerminal()
	case filterClosed:
		return m.IsTerminal()
	case filterAll:
		return true
	case filterStatus:
		return m.Status == a.statusFilter
	}
	return true
}

func (a *App) docketMatters() []*model.Matter {
	lookup := make(map[string]*model.Matter)
	for _, m := range a.matters {
		lookup[m.ID] = m
	}
	var result []*model.Matter
	for _, e := range a.docket {
		if m, ok := lookup[e.ID]; ok {
			result = append(result, m)
		}
	}
	return result
}

func (a *App) clampCursors() {
	fm := a.filteredMatters()
	if a.cursor >= len(fm) {
		a.cursor = max(0, len(fm)-1)
	}
	dm := a.docketMatters()
	if a.docketCursor >= len(dm) {
		a.docketCursor = max(0, len(dm)-1)
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case dataLoadedMsg:
		a.applyDataLoaded(msg)
		a.clampCursors()
		return a, nil

	case clearFlashMsg:
		a.flash = ""
		return a, nil

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.viewport.Width = msg.Width
		a.viewport.Height = msg.Height - 6
		return a, nil

	case tea.KeyMsg:
		// Search input mode
		if a.searching {
			return a.updateSearchInput(msg)
		}

		// Help overlay
		if a.showHelp {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				a.showHelp = false
			}
			return a, nil
		}

		// Detail view
		if a.view == viewDetail {
			return a.updateDetail(msg)
		}

		// List views
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
		a.view = a.prevView
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

	case matchKey(msg, a.keys.Help):
		a.showHelp = true
		return a, nil

	case matchKey(msg, a.keys.Tab1):
		a.view = viewMatters
		return a, nil

	case matchKey(msg, a.keys.Tab2):
		a.view = viewDocket
		a.docketCursor = 0
		return a, nil

	case matchKey(msg, a.keys.Down):
		a.moveCursor(1)
		return a, nil

	case matchKey(msg, a.keys.Up):
		a.moveCursor(-1)
		return a, nil

	case matchKey(msg, a.keys.Enter):
		m := a.currentMatter()
		if m != nil {
			a.detailMatter = m
			a.prevView = a.view
			a.view = viewDetail
			a.viewport.SetContent(m.Body)
			a.viewport.GotoTop()
		}
		return a, nil

	case matchKey(msg, a.keys.Refresh):
		return a, a.loadDataCmd()

	case matchKey(msg, a.keys.Search):
		if a.view == viewMatters {
			a.searching = true
			a.searchInput.SetValue("")
			a.searchInput.Focus()
			return a, textinput.Blink
		}
		return a, nil
	}

	// Filter keys only in matters view
	if a.view == viewMatters {
		return a.updateFilter(msg)
	}
	return a, nil
}

func (a App) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, a.keys.Open):
		a.filter = filterOpen
		a.cursor = 0
	case matchKey(msg, a.keys.Closed):
		a.filter = filterClosed
		a.cursor = 0
	case matchKey(msg, a.keys.All):
		a.filter = filterAll
		a.cursor = 0
	case matchKey(msg, a.keys.Status):
		a.cycleStatus()
	case matchKey(msg, a.keys.Epic):
		a.cycleEpic()
	}
	return a, nil
}

var statusCycle = []string{"raw", "refined", "planned", "done", "dropped"}

func (a *App) cycleStatus() {
	if a.filter != filterStatus {
		a.filter = filterStatus
		a.statusFilter = statusCycle[0]
		a.cursor = 0
		return
	}
	for i, s := range statusCycle {
		if s == a.statusFilter {
			if i+1 < len(statusCycle) {
				a.statusFilter = statusCycle[i+1]
			} else {
				a.filter = filterOpen
				a.statusFilter = ""
			}
			a.cursor = 0
			return
		}
	}
	a.filter = filterOpen
	a.statusFilter = ""
	a.cursor = 0
}

func (a *App) cycleEpic() {
	epics := a.uniqueEpics()
	if len(epics) == 0 {
		return
	}
	if a.epicFilter == "" {
		a.epicFilter = epics[0]
		a.cursor = 0
		return
	}
	for i, e := range epics {
		if e == a.epicFilter {
			if i+1 < len(epics) {
				a.epicFilter = epics[i+1]
			} else {
				a.epicFilter = ""
			}
			a.cursor = 0
			return
		}
	}
	a.epicFilter = ""
	a.cursor = 0
}

func (a *App) uniqueEpics() []string {
	seen := make(map[string]bool)
	var epics []string
	for _, m := range a.matters {
		if m.Epic != "" && !seen[m.Epic] {
			seen[m.Epic] = true
			epics = append(epics, m.Epic)
		}
	}
	return epics
}

func (a *App) moveCursor(delta int) {
	if a.view == viewDocket {
		dm := a.docketMatters()
		a.docketCursor += delta
		if a.docketCursor < 0 {
			a.docketCursor = 0
		}
		if a.docketCursor >= len(dm) {
			a.docketCursor = max(0, len(dm)-1)
		}
		return
	}
	fm := a.filteredMatters()
	a.cursor += delta
	if a.cursor < 0 {
		a.cursor = 0
	}
	if a.cursor >= len(fm) {
		a.cursor = max(0, len(fm)-1)
	}
}

func (a *App) currentMatter() *model.Matter {
	if a.view == viewDocket {
		dm := a.docketMatters()
		if a.docketCursor < len(dm) {
			return dm[a.docketCursor]
		}
		return nil
	}
	fm := a.filteredMatters()
	if a.cursor < len(fm) {
		return fm[a.cursor]
	}
	return nil
}

func (a *App) setFlash(msg string) tea.Cmd {
	a.flash = msg
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return clearFlashMsg{}
	})
}

func matchKey(msg tea.KeyMsg, binding key.Binding) bool {
	for _, k := range binding.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

func (a App) View() string {
	return "mull-tui - press q to quit\n"
}

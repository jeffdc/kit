# mull-tui Design

Separate binary in the mull repo providing a human-friendly TUI for browsing and acting on matters.

## Binary & Packaging

- Binary name: `mull-tui`
- Lives in `cmd/mull-tui/main.go`, same Go module as mull
- Imports `internal/model` and `internal/storage` directly — no duplication
- Makefile gets `build-tui` and `install` targets for both binaries
- TUI-only deps (Bubble Tea, fsnotify) don't affect the mull CLI binary

## Package Layout

```
cmd/mull-tui/main.go    # entry point
internal/tui/
  app.go                 # top-level model, view routing
  matters.go             # filterable matters list view
  docket.go              # docket view (priority-ordered)
  detail.go              # matter detail pane
  keys.go                # keybinding definitions
  styles.go              # Lip Gloss styles
  watcher.go             # fsnotify → tea.Msg
```

## Views

Two main views, switchable with `1` and `2`:

**Matters view (default):** Filterable table of all matters showing ID, title, status, epic, effort. Default filter: open only (raw, refined, planned).

**Docket view:** Docket items in priority order showing position, ID, title, status, epic, note.

**Detail pane:** `Enter` on any matter opens full metadata + body. `Esc` returns to list.

## Keybindings

### Navigation
- `j`/`k` or arrows — move up/down
- `1`/`2` — switch views
- `Enter` — open detail
- `Esc` — back to list
- `q` — quit
- `?` — help overlay

### Filtering (matters view)
- `o` — open matters only (raw, refined, planned) — default
- `c` — closed matters only (done, dropped)
- `a` — all matters
- `s` — cycle individual status filter
- `e` — filter by epic (picker)
- `/` — fuzzy search by title

### Actions
- `p` — generate prompt to clipboard
- `d` — toggle docket membership
- `r` — manual refresh

## Prompt Generation

Press `p` to copy a status-aware prompt to clipboard:

| Status   | Prompt                                       |
|----------|----------------------------------------------|
| raw      | Explore matter {id} ({title}) with the user  |
| refined  | Help plan matter {id} ({title})              |
| planned  | Implement matter {id} ({title})              |
| done     | Review matter {id} ({title})                 |
| dropped  | Revisit matter {id} ({title})                |

Flash message "Copied to clipboard" in status bar for ~2s. Uses `pbcopy` on macOS.

## File Watching

fsnotify watches the `.mull/` directory tree. On change:

1. fsnotify event fires
2. Debounce 100ms to batch rapid changes
3. Send `dataReloadMsg` into Bubble Tea event loop
4. Reload all matters + docket from Store
5. Re-render preserving cursor position and active filters

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/charmbracelet/bubbles` — table, viewport, textinput
- `github.com/fsnotify/fsnotify` — file watching

Clipboard via `exec.Command("pbcopy")`, no extra dependency.

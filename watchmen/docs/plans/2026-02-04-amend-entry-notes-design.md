# Amend Entry Notes Design

**Date:** 2026-02-04
**Purpose:** Add ability to edit notes on any historical time entry

## Overview

Implements `watchmen amend` command to modify the note field on completed time entries. Designed for both interactive human use and efficient LLM agent automation.

## Command Structure

### Three Operating Modes

1. **Interactive Mode** (no arguments)
   ```bash
   watchmen amend
   ```
   - Lists last 20 completed entries
   - Shows: index, date/time, project, duration, current note
   - Prompts for entry selection by number
   - Prompts for new note text

2. **Index Mode** (with entry number)
   ```bash
   watchmen amend 1
   watchmen amend 5
   ```
   - Direct selection by reverse chronological index (1=most recent)
   - Shows selected entry details
   - Prompts for new note (unless `--note` flag provided)

3. **Direct Mode** (with flag)
   ```bash
   watchmen amend 1 --note "Fixed authentication bug"
   watchmen amend 2 --clear
   ```
   - Updates entry immediately without prompts
   - Ideal for automation and LLM agents

### Flags

- `--note, -n <text>`: Provide new note text directly (skips prompt)
- `--clear`: Remove note entirely (sets to empty string)

### Entry Selection Rules

- Only completed entries can be amended
- Entries numbered in reverse chronological order (1=newest)
- Running or paused entries must use `stop --note` instead
- Invalid index returns clear error message

## User Interface

### Interactive List Display

```
Recent time entries:

  1. 2026-02-04 14:23  ProjectX  2.5h  "Implemented user auth"
  2. 2026-02-04 11:45  ProjectY  1.2h  "Bug fixes"
  3. 2026-02-03 16:30  ProjectX  3.0h
  4. 2026-02-03 13:15  ProjectZ  0.5h  "Team meeting"
  ...

Enter entry number to amend (or 'q' to quit):
```

### Entry Detail Display

When an entry is selected (before prompting for new note):

```
Entry #1:
  Project: ProjectX
  Date: 2026-02-04 14:23
  Duration: 2.5h
  Current note: "Implemented user auth"

Enter new note (or press Enter to keep current):
```

### Success Confirmation

```
Updated entry #1:
  Project: ProjectX
  Duration: 2.5h
  Note: "Implemented user authentication system"
```

## Implementation Details

### Command File Structure

**Location:** `cmd/amend.go`

**Key Functions:**
- `runAmendInteractive()` - Lists entries and prompts for selection
- `runAmendDirect(index, note)` - Updates entry directly
- `displayEntryList(entries)` - Formats entry list for display
- `promptForNote(currentNote)` - Handles note input with current note shown

### Storage Operations

**New Method:** `Store.AmendEntry(index int, note string) (*Entry, error)`

- Validates index is within range of completed entries
- Retrieves completed entries in reverse chronological order
- Updates entry note field
- Calls existing `SaveData()` for persistence
- Returns updated entry for confirmation display

### Data Model

No changes to `internal/model/model.go` - the `Entry.Note` field already exists and supports arbitrary text.

### Error Handling

- **No completed entries:** "No completed entries to amend"
- **Invalid index:** "Entry #5 not found (only 3 completed entries)"
- **Running entry:** "Cannot amend running entry - use 'watchmen stop --note' instead"
- **Paused entry:** "Cannot amend paused entry - use 'watchmen stop --note' instead"
- **Storage errors:** Pass through with context

## Testing Strategy

### Manual Testing
1. Create several test entries with `start`/`stop`
2. Test interactive mode - select entry, update note
3. Test index mode - `amend 1`, `amend 2`, verify correct entry updated
4. Test direct mode - `amend 1 --note "new"`, verify no prompts
5. Test `--clear` flag - verify note removed
6. Test error cases - invalid index, no entries, etc.

### Edge Cases
- Amending entry that already has no note
- Amending entry with very long note (ensure no truncation)
- Multiple sequential amends to same entry
- Amending while another entry is running (should work fine)

## LLM Agent Usage Patterns

Primary use case for agents:

```bash
# After reviewing work log, fix typo in recent entry
watchmen amend 1 --note "Implemented user authentication (fixed typo)"

# Bulk update multiple entries
watchmen amend 1 --note "Sprint 23: Auth implementation"
watchmen amend 2 --note "Sprint 23: Bug fixes"
watchmen amend 3 --note "Sprint 23: Code review"

# Clear note from entry
watchmen amend 5 --clear
```

Agents should prefer direct mode with `--note` flag for efficiency and avoid interactive prompts.

## Future Enhancements (Not in Scope)

- Filter entries by project: `watchmen amend --project ProjectX`
- Search entries by date range: `watchmen amend --since "2 days ago"`
- Bulk amend: `watchmen amend 1-5 --note "Sprint 23"`
- Amend other fields (project reassignment, time adjustment)
- Undo/history of amendments

## Implementation Checklist

- [ ] Create `cmd/amend.go` with command structure
- [ ] Implement `Store.AmendEntry()` method
- [ ] Add interactive mode with entry list display
- [ ] Add index mode with direct selection
- [ ] Add `--note` and `--clear` flags
- [ ] Implement note prompting with current note display
- [ ] Add error handling for all edge cases
- [ ] Test all three modes manually
- [ ] Update README or docs with command usage
- [ ] Commit and test with `make install`

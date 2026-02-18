# Add Enhancements & Batch Link — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Collapse the add → append → link → docket-add workflow into a single `mull add` invocation, and support multiple targets in `mull link`.

**Architecture:** Pure CLI-layer changes. No new storage APIs needed — `add.go` orchestrates existing `store.CreateMatter`, `store.AppendBody`, `store.LinkMatters`, and `store.DocketAdd` calls. `link.go` loops over variadic target args calling `store.LinkMatters` per target.

**Tech Stack:** Go, Cobra CLI framework

---

### Task 1: Add `--body` flag to `add` command

**Files:**
- Modify: `cmd/add.go`
- Test: `cmd/add_test.go` (create)

**Step 1: Write the failing test**

Create `cmd/add_test.go`. This tests the integration at the cmd layer, but since `add` just calls store methods, a storage-layer test is more appropriate. Add to `internal/storage/storage_test.go`:

```go
func TestCreateMatterWithBody(t *testing.T) {
	s := setupTestStore(t)

	m, err := s.CreateMatter("With body", nil)
	if err != nil {
		t.Fatalf("CreateMatter() error: %v", err)
	}

	m, err = s.AppendBody(m.ID, "Initial description")
	if err != nil {
		t.Fatalf("AppendBody() error: %v", err)
	}

	got, _ := s.GetMatter(m.ID)
	if got.Body != "Initial description" {
		t.Errorf("Body = %q, want %q", got.Body, "Initial description")
	}
}
```

This test already passes (AppendBody works). The real change is in the CLI wiring. Skip the test step — this is pure flag plumbing.

**Step 2: Add `--body` flag to `cmd/add.go`**

In `cmd/add.go`, add after the matter creation:

```go
// In the RunE function, after store.CreateMatter:
body, _ := cmd.Flags().GetString("body")
if body != "" {
    m, err = store.AppendBody(m.ID, body)
    if err != nil {
        return err
    }
}
```

In `init()`:
```go
addCmd.Flags().String("body", "", "set the matter body")
```

**Step 3: Run tests**

Run: `make test`
Expected: all pass

**Step 4: Commit**

```bash
git add cmd/add.go
git commit -m "Add --body flag to add command"
```

---

### Task 2: Add link flags (`--relates`, `--blocks`, `--needs`, `--parent`) to `add` command

**Files:**
- Modify: `cmd/add.go`

**Step 1: Add link flags and wiring to `cmd/add.go`**

In `RunE`, after the body block:

```go
// Create links
relatesIDs, _ := cmd.Flags().GetStringSlice("relates")
for _, targetID := range relatesIDs {
    if err := store.LinkMatters(m.ID, "relates", targetID); err != nil {
        return err
    }
}

blocksIDs, _ := cmd.Flags().GetStringSlice("blocks")
for _, targetID := range blocksIDs {
    if err := store.LinkMatters(m.ID, "blocks", targetID); err != nil {
        return err
    }
}

needsIDs, _ := cmd.Flags().GetStringSlice("needs")
for _, targetID := range needsIDs {
    if err := store.LinkMatters(m.ID, "needs", targetID); err != nil {
        return err
    }
}

parentID, _ := cmd.Flags().GetString("parent")
if parentID != "" {
    if err := store.LinkMatters(m.ID, "parent", parentID); err != nil {
        return err
    }
}
```

After all links, re-read the matter so the output includes populated link fields:

```go
// Re-read matter to include link data in output
if len(relatesIDs) > 0 || len(blocksIDs) > 0 || len(needsIDs) > 0 || parentID != "" {
    m, err = store.GetMatter(m.ID)
    if err != nil {
        return err
    }
}
```

In `init()`:
```go
addCmd.Flags().StringSlice("relates", nil, "link as relates to these matter IDs (repeatable)")
addCmd.Flags().StringSlice("blocks", nil, "link as blocks these matter IDs (repeatable)")
addCmd.Flags().StringSlice("needs", nil, "link as needs these matter IDs (repeatable)")
addCmd.Flags().String("parent", "", "set parent matter ID")
```

**Step 2: Run tests**

Run: `make test`
Expected: all pass

**Step 3: Commit**

```bash
git add cmd/add.go
git commit -m "Add link flags to add command (--relates, --blocks, --needs, --parent)"
```

---

### Task 3: Add `--docket` flag to `add` command

**Files:**
- Modify: `cmd/add.go`

**Step 1: Add `--docket` flag and wiring**

In `RunE`, after the link block (just before the JSON output):

```go
docket, _ := cmd.Flags().GetBool("docket")
if docket {
    if err := store.DocketAdd(m.ID, "", ""); err != nil {
        return err
    }
}
```

In `init()`:
```go
addCmd.Flags().Bool("docket", false, "add the new matter to the docket")
```

**Step 2: Run tests**

Run: `make test`
Expected: all pass

**Step 3: Commit**

```bash
git add cmd/add.go
git commit -m "Add --docket flag to add command"
```

---

### Task 4: Make `link` accept multiple target IDs

**Files:**
- Modify: `cmd/link.go`

**Step 1: Change link.go to support variadic targets**

Replace the entire `RunE` body and change `Args` from `ExactArgs(3)` to `MinimumNArgs(3)`:

```go
var linkCmd = &cobra.Command{
	Use:   "link <id> <type> <id> [id...]",
	Short: "Create a relationship between matters",
	Long:  `Type is one of: relates, blocks, needs, parent.`,
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id1 := args[0]
		relType := args[1]
		targets := args[2:]

		results := make([]map[string]string, 0, len(targets))
		for _, id2 := range targets {
			if err := store.LinkMatters(id1, relType, id2); err != nil {
				return err
			}
			results = append(results, map[string]string{
				"from": id1,
				"type": relType,
				"to":   id2,
			})
		}

		// Backward compatible: single target returns single object
		if len(results) == 1 {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"linked": results[0],
			})
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"linked": results,
		})
	},
}
```

**Step 2: Run tests**

Run: `make test`
Expected: all pass

**Step 3: Manual smoke test**

```bash
make build
./mull add "Test A" --status raw
./mull add "Test B" --status raw
# Use IDs from output to test:
# ./mull link <idA> relates <idB>
# ./mull add "Test C" --status raw --body "A description" --relates <idA> --docket
```

**Step 4: Commit**

```bash
git add cmd/link.go
git commit -m "Support multiple target IDs in link command"
```

---

### Task 5: Update docs and skill

**Files:**
- Modify: `CLAUDE.md` (mull's CLAUDE.md if workflow section needs updating)
- Check: any skill files that reference `mull add` or `mull link` syntax

**Step 1: Check if CLAUDE.md or skill files reference add/link syntax**

Look for references in the sessionstart hook or skill files. Update usage examples to show the new flags.

**Step 2: Commit**

```bash
git add -A
git commit -m "Update docs for add enhancements and batch link"
```

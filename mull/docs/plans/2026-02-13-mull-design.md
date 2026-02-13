# mull - Design Document

A CLI tool for tracking ideas and features ("matters") for solo projects. LLMs are the primary consumer via Claude Code skills. Humans interact through the LLM layer, not the CLI directly.

## Storage

All data lives in `.mull/` at the repo root. Version controlled alongside the project.

```
.mull/
  matters/
    ab3f-add-an-rss-feed.md
    c7d1-dark-mode.md
  docket.yml
  config.yml    # optional: default tags, custom statuses, etc.
```

### Matter files

Each matter is a markdown file: `<4-char-hash>-<slugified-title>.md`

The 4-char hash is derived from title + timestamp. The slug makes files human-browsable.

```markdown
---
status: idea
tags: [content, low-effort]
effort: small
created: 2026-02-13
updated: 2026-02-13
---

# Add an RSS feed

Let people subscribe to new posts. Could use the Atom format
since it's cleaner. Should auto-generate from the existing
post metadata.
```

YAML frontmatter has no fixed schema beyond `status` and `created`. Any key-value pair is valid and queryable. The body is freeform markdown.

### Matter lifecycle

```
raw → refined → planned → done
                   ↓
            docs/plans/YYYY-MM-DD-<slug>.md
```

- **raw** - just captured, rough
- **refined** - fleshed out, clear enough to act on
- **planned** - has a plan doc in `docs/plans/`, ready for execution
- **done** - shipped
- **dropped** - decided against

When status is `planned`, the matter should include a `plan` metadata field linking to the plan doc path.

### Relationships

Stored as metadata in YAML frontmatter using matter IDs:

```yaml
---
relates: [c7d1, e2a9]
blocks: [f1b2]
needs: [a4c8]
parent: d3e5
---
```

Three relationship types plus grouping:

- **`relates`** - loose association ("these are connected")
- **`blocks` / `needs`** - directional dependency ("this blocks that" / "this needs that first")
- **`parent`** - grouping ("this is part of a bigger matter")

Relationships are always kept in sync on both sides. `mull link ab3f blocks c7d1` writes `blocks: [c7d1]` in ab3f AND `needs: [ab3f]` in c7d1.

### Docket

The docket is a sequenced list of matter IDs representing prioritized work order. Stored in `.mull/docket.yml`:

```yaml
- id: a4c8
  note: "do this first, unblocks RSS and tags"
- id: ab3f
- id: e2a9
- id: c7d1
  note: "stretch goal"
```

Matters not on the docket exist but aren't prioritized.

## CLI

All output is JSON to stdout. Errors go to stderr as JSON: `{"error": "matter not found", "id": "zzzz"}`. No interactive prompts. No color. Designed for LLM callers.

### Commands

```bash
# CRUD
mull add <title> [--key value ...]       # create a matter, returns id + file path
mull show <id>                            # full matter: metadata + body
mull list [--status X] [--tag X] [...]    # filter/list matters
mull search <query>                       # full-text search across titles + bodies
mull set <id> <key> <value>               # set any metadata field
mull append <id> <text>                   # append text to body
mull drop <id>                            # set status to dropped
mull rm <id>                              # delete the file

# Relationships
mull link <id> <type> <id>                # create relationship (relates|blocks|needs|parent)
mull unlink <id> <type> <id>              # remove relationship

# Docket
mull docket                               # show docket with matter summaries
mull docket add <id> [--after <id>] [--note "..."]
mull docket rm <id>
mull docket move <id> --after <id>

# Graph
mull graph                                # dependency graph of all docket matters
mull graph <id>                           # graph centered on one matter

# Context
mull prime                                # compact dump for LLM context injection
```

### `mull prime` output

Token-efficient summary. Excludes `done` and `dropped`. Bodies omitted.

```json
{
  "matters": [
    {"id": "ab3f", "title": "Add RSS feed", "status": "raw", "tags": ["content"]},
    {"id": "c7d1", "title": "Dark mode", "status": "refined", "tags": ["design"], "needs": ["a4c8"]}
  ],
  "docket": ["a4c8", "ab3f", "e2a9", "c7d1"],
  "counts": {"raw": 3, "refined": 2, "planned": 1}
}
```

### `mull graph` output

Structured JSON representing nodes and edges:

```json
{
  "nodes": [
    {"id": "a4c8", "title": "Restructure post metadata", "status": "refined"},
    {"id": "ab3f", "title": "Add RSS feed", "status": "raw"}
  ],
  "edges": [
    {"from": "a4c8", "to": "ab3f", "type": "blocks"}
  ]
}
```

## Implementation

**Language:** Go. Single binary. Fast startup (important for repeated LLM invocations in a session).

**Dependencies:** `gopkg.in/yaml.v3` for YAML frontmatter parsing. Everything else is stdlib.

**ID generation:** First 4 hex chars of SHA-256 of title + RFC3339 timestamp. Regenerate on collision.

**Initialization:** Running any `mull` command in a repo without `.mull/` creates the directory structure automatically. No `init` command needed.

**Backlink sync:** All relationship mutations update both sides atomically. If writing the second file fails, the first write is rolled back.

## Skills (separate effort)

Claude Code skills wrap the CLI to enable two conversational workflows:

**Workflow 1 - Work on an existing matter:**
1. User mentions an existing idea or asks what's open
2. Skill calls `mull prime` or `mull search` to find the matter
3. Pulls full content with `mull show <id>`
4. Conversational refinement: skill calls `mull append` and `mull set` as the matter evolves
5. Exit: execute immediately if small, or generate plan doc in `docs/plans/` and set status to `planned`

**Workflow 2 - Create a new matter:**
1. User starts discussing something new
2. Skill calls `mull add` to capture it
3. Flesh out conversationally, skill appends details and sets metadata
4. Same exit paths as workflow 1

**Docket consultation:**
- "What should I work on next?" → skill reads docket, checks dependency graph, suggests first unblocked matter
- "Show me the roadmap" → skill calls `mull docket`, renders readable summary

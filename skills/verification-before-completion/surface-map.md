# Surface Map Reference

## What Are Surfaces?

A **surface** is any place where the same data, behavior, or concept is represented. When one surface changes and its parallels don't, they drift silently.

## Running the Audit

### 1. Get the full diff

Against a base branch:
```bash
git diff --name-only $(git merge-base HEAD main)..HEAD
```

For single-commit work:
```bash
git diff --name-only HEAD~1..HEAD
```

Use the base branch version for multi-commit sessions — checking only the last commit misses accumulated drift.

### 2. Categorize changed files and check mapped surfaces

For each changed file, find its category in the surface map. Then check each mapped surface:

- **Code surfaces** — check whether the file appears in the diff. Absence means it wasn't touched.
- **Documentation surfaces** (`*.md` in `docs/`, `runbooks/`, project root, etc.) — **read the file** and check whether its statements about the changed behavior are still factually correct. A doc's absence from the diff tells you nothing; the problem is precisely that it *didn't* change when it should have.

### 3. Ask the user about each gap

Frame questions specifically:

> "You changed `lib/app/contexts/galls.ex` to add a `generation` field. The API controller `lib/app_web/controllers/api/gall_controller.ex` was not modified — does the API need this field too?"

> "You updated `assets/js/components/FilterPanel.tsx`. The component inventory in `CLAUDE.md` was not updated — does it need to reflect this change?"

Don't batch gaps into a single generic question. Each gap gets its own specific question so the user can decide individually.

## Default Surface Mapping Rules

These are pattern-based defaults. Projects override them via their CLAUDE.md `## Surface Map` section.

| When you change... | Check whether these were also updated |
|---|---|
| Domain/context code (`**/contexts/**`, `**/models/**`, `**/schemas/**`) | API controllers exposing the same entities, API response schemas/serializers, API tests asserting response shape |
| UI views/templates/LiveView (`**/live/**`, `**/templates/**`, `**/views/**`) | API controllers for the same entities (UI often gets data the API doesn't expose yet) |
| Frontend components (`**/components/**`) | LLM context docs that inventory components (CLAUDE.md, etc.) |
| Schema/migration files (`**/migrations/**`, `**/migrate/**`) | Domain model documentation, entity relationship docs |
| API controllers or serializers | OpenAPI/Swagger schema definitions, API test assertions, API documentation |
| Dev tooling (Makefile, mix.exs, package.json, config files, scripts) | Developer documentation ("Development Commands", "Getting Started" sections) |
| Primary LLM instruction file (CLAUDE.md) | Parallel instruction files (copilot-instructions.md, .cursorrules, etc.) |
| Admin UI (`**/admin/**`, `**/dashboard/**`) | Admin-facing documentation, onboarding docs |
| Infrastructure config (Dockerfile, docker-compose, deploy configs, CI) | Operational runbooks, deployment documentation |
| Test fixtures or factories | Other tests that create the same entities (may need the new fields) |

## Project-Specific Configuration

Projects define their own surface map in their `CLAUDE.md` under a `## Surface Map` heading. When present, this **supplements** the defaults above (it doesn't replace them).

Format — each line is a glob pattern mapping:

```markdown
## Surface Map

- lib/**/contexts/** → lib/**/controllers/api/**, priv/static/swagger.json, test/**/controllers/api/**
- lib/**/live/** → lib/**/controllers/api/**
- assets/js/components/** → CLAUDE.md (Component Inventory)
- priv/repo/migrations/** → docs/domain-model.md
- CLAUDE.md → .github/copilot-instructions.md
- mix.exs, Makefile → CLAUDE.md (Development Commands)
```

The `→` separates the trigger pattern (left) from the surfaces to check (right). Parenthetical notes after a path indicate a specific section within that file.

If no `## Surface Map` section exists in the project CLAUDE.md, use only the default rules above.

## Documentation Surfaces Require Reading

Documentation is a special surface type. For code files, the diff check works: if a mapped controller isn't in the diff, flag it. For documentation files, absence from the diff is the *expected* failure mode — stale docs don't announce themselves.

**The failure mode:** You scan the diff, see no doc files changed, and conclude "no docs need updating." But the reason no docs changed is that nobody updated them. The doc may describe the old behavior in detail — a range model that was replaced, API fields that were renamed, a workflow that no longer exists. You can only know by reading it.

**When a mapped surface is a doc file** (`.md` in `docs/`, `runbooks/`, admin guides, onboarding docs, `CLAUDE.md` sections, etc.):

1. **Open the file.** Diff-checking is insufficient.
2. **Search for statements about the changed behavior.** Look for descriptions of the feature, data model, workflow, or API shape you modified.
3. **Check whether those statements are still factually correct.** If they describe the old behavior, flag them — the doc needs updating.

This applies to every documentation surface in the mapping table: domain model docs, API documentation, developer docs, admin-facing docs, operational runbooks, and component inventories.

## Example Audit Output

```
Surface audit — 4 files changed:

  lib/app/contexts/galls.ex ............ changed
  lib/app_web/live/gall_live/show.ex ... changed
  lib/app_web/controllers/api/gall_controller.ex ... NOT in diff
  priv/static/swagger.json ............. NOT in diff

Questions for you:
1. Galls context added `generation` and `taxonomy_path` fields.
   The API controller (gall_controller.ex) was not updated — does the API need these fields?
2. The OpenAPI schema (swagger.json) was not updated — does it need the new fields?
```

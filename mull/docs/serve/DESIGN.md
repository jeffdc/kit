---
name: mull serve
description: The human's local view of a mull project, drawn as an issued drawing set.
colors:
  ground: "#FFFFFF"
  panel: "#EEF2F6"
  ink: "#121417"
  mut: "#565F6B"
  faint: "#69717C"
  line: "#8DB0D6"
  red: "#D2232A"
  sel: "#DCE6F1"
  blue: "#2E58C2"
  blue-soft: "#E3EBFA"
  green: "#2B7A48"
  hl: "#FFE14D"
  hl-ink: "#121417"
  rail: "#EEF3FB"
  rail-sel: "#DDE7F8"
  ground-dark: "#101215"
  panel-dark: "#181C21"
  ink-dark: "#E6E9ED"
  mut-dark: "#A2ABB6"
  faint-dark: "#828B97"
  line-dark: "#34506F"
  red-dark: "#FF5D5D"
  sel-dark: "#1F2A37"
  blue-dark: "#86A8F5"
  blue-soft-dark: "#1B2740"
  green-dark: "#6CC48B"
  hl-dark: "#F0CF45"
  hl-ink-dark: "#101215"
  rail-dark: "#131A25"
  rail-sel-dark: "#1E2B42"
typography:
  display:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "clamp(24px, 2.6vw, 34px)"
    fontWeight: 800
    lineHeight: 1.05
    letterSpacing: "-0.015em"
    fontVariation: "'wdth' 118"
  brand:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "22px"
    fontWeight: 800
    letterSpacing: "-0.01em"
    fontVariation: "'wdth' 112"
  headline:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "19px"
    fontWeight: 750
    letterSpacing: "-0.01em"
    fontVariation: "'wdth' 95"
  title:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "15px"
    fontWeight: 700
  value:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "14.5px"
    fontWeight: 550
  body:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "13.5px"
    fontWeight: 400
    lineHeight: 1.45
  prose:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.66
  label:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "11px"
    fontWeight: 650
    letterSpacing: "0.12em"
    fontVariation: "'wdth' 75"
  status:
    fontFamily: "Archivo, Helvetica Neue, Arial, sans-serif"
    fontSize: "11px"
    fontWeight: 650
    letterSpacing: "0.09em"
    fontVariation: "'wdth' 72"
  mono:
    fontFamily: "Red Hat Mono, ui-monospace, Menlo, monospace"
    fontSize: "12px"
    fontWeight: 500
    fontFeature: "'tnum'"
rounded:
  none: "0px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "22px"
  xl: "34px"
  gutter: "40px"
  page-x: "clamp(16px, 2.4vw, 32px)"
components:
  button:
    backgroundColor: "{colors.ground}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "0 12px"
    height: "28px"
  button-hover:
    backgroundColor: "{colors.panel}"
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    rounded: "{rounded.none}"
    padding: "0 12px"
    height: "28px"
  button-small:
    padding: "0 10px"
    height: "24px"
  tag-active:
    backgroundColor: "{colors.blue}"
    textColor: "{colors.ground}"
    typography: "{typography.status}"
    rounded: "{rounded.none}"
    padding: "1px 5px"
  tag-done:
    textColor: "{colors.green}"
    typography: "{typography.status}"
    rounded: "{rounded.none}"
    padding: "1px 5px"
  stamp-blocking:
    backgroundColor: "{colors.hl}"
    textColor: "{colors.hl-ink}"
    typography: "{typography.status}"
    rounded: "{rounded.none}"
    padding: "2px 5px"
  tab:
    textColor: "{colors.mut}"
    height: "44px"
  tab-active:
    textColor: "{colors.ink}"
  tree-node:
    backgroundColor: "{colors.rail}"
    textColor: "{colors.ink}"
    height: "29px"
  tree-node-current:
    backgroundColor: "{colors.rail-sel}"
    textColor: "{colors.blue}"
    height: "29px"
  tier-header:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink}"
    height: "32px"
  toast:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    padding: "10px 14px"
---

# Design System: mull serve

The build in `mull/internal/serve/web/` is the source of truth (`style.css` holds every token and component). This file records what shipped; the direction contract is `direction.md`, the approved comp is `prototype.html`, and where the comp and the build differ this file describes the build.

## Overview

**Creative North Star: "The Issued Drawing Set"**

A project is a set of drawings issued for review. The left rail is the sheet index, every page opens with a title block of boxed fields, lists are schedules (ruled tables and numbered tiers), and whatever changed since the human last looked is ringed by a red revision cloud. The look is white bond paper with ink linework (deep graphite with light linework in dark), pale blue construction lines for minor rules, square corners, and no elevation.

It's dense in the way a drawing sheet is dense. Small type sits in a tight grid, every region is boxed or ruled, and colour is rare enough that each hue reads as a checker's mark with one job. The UI is read-first: every action copies a mull command or an agent prompt to the clipboard and says so in a toast. Nothing in the UI writes to the database yet, so actions must never look like they save anything.

Rejected outright by the contract: soft cards on grey, status pills everywhere, cream-and-serif "AI" styling, delta triangles and per-row change counts.

**Key Characteristics:**
- Square corners everywhere. Ink rules at 1 and 1.5px, blue construction lines at 1px.
- One job per hue: revision red, highlighter yellow, blueline blue, checker's green.
- Archivo across its width axis for everything except IDs and figures, which use Red Hat Mono.
- Revision clouds (scalloped red outlines) are the only "changed" signal.
- Light and dark follow the system, with a `data-theme` override on the root.

## Colors

The palette is ink on paper plus four marker hues, each restricted to one meaning. Dark mode is the same set of roles on graphite. Every colour is a CSS custom property on `:root`, and the `-dark` keys in the frontmatter are the values those same properties take in dark mode.

### Primary
- **Blueline Blue** (`--blue`): now, active, and the selected path. Used for the active stage tag (filled), ready and now tags (outlined), the current sheet-index row's text, the "Now" group header in schedules, the active cell in cell bars, the underline on doc ID references, and the focus ring.
- **Blueline Wash** (`--blue-soft`): the fill behind the active cell in a cell bar and behind a "now" group row in a table.

### Secondary
- **Revision Red** (`--red`): revision clouds and nothing else.
- **Highlighter Yellow** (`--hl`, with **Highlighter Ink** `--hl-ink` on top): blocking work only. Used for the blocking stamp (a yellow box with a 1px ink outline) and the highlighter swipe behind the "Blocking work" tier title.

### Tertiary
- **Checker's Green** (`--green`): done, accepted and passed. Used for done tags, done cells in cell bars, the passed check's 1px border, and the filled PASSED tag.

### Neutral
- **Bond** (`--ground`): the page ground, and the fill of outline buttons.
- **Panel Grey** (`--panel`): tier headers, grouped-row headers, hover fill on rows and buttons, and code and table-header fills in docs.
- **Ink** (`--ink`): text, the 1.5px structural rules, button borders, the primary button fill, the toast.
- **Muted** (`--mut`): labels, secondary lines, inactive tabs, chevrons.
- **Faint** (`--faint`): row numbers, waiting rows in the tree, dropped work.
- **Construction Line** (`--line`): minor 1px rules between rows and fields, tree guides, link underlines, the scrollbar.
- **Selection** (`--sel`): text selection only.
- **Blueline Paper** (`--rail`, with **Rail Select** `--rail-sel`): the sheet-index rail and its hovered or current row.

### Named Rules
**The One Mark, One Job Rule.** Red means changed since you last looked, and nothing else in the UI is red. That includes errors, destructive actions and warnings. Yellow means blocking, blue means now, active or selected, and green means done or passed. A new state takes ink, grey or a line style (dashed, dotted, double, struck through) before it takes a hue.

**The Cloud Alone Rule.** Change is shown only by a cloud. Don't add per-row change counts, delta triangles or "new" badges. The title block's Revision field and the revision schedule are the only places that count changes.

## Typography

**Display Font:** Archivo (with Helvetica Neue, Arial)
**Body Font:** Archivo
**Label/Mono Font:** Red Hat Mono (with ui-monospace, Menlo), for IDs and figures only

**Character:** One grotesque used across its width axis. Wide and heavy for the names of sheets, condensed and tracked in caps for field labels and status words. A mono with tabular figures makes IDs and numbers read like drawing numbers.

Both families are self-hosted woff2 under `/fonts/`: Archivo variable (wdth 62–125, wght 100–900) and Red Hat Mono (wght 300–700), in latin and latin-ext subsets, OFL licensed (see `internal/serve/web/fonts/README.md`). The latin files are preloaded. Don't load either family from a font CDN.

### Hierarchy
- **Display** (800, clamp(24px, 2.6vw, 34px), line-height 1.05, width 118%): the sheet name in the title block. Long names step down. Plan, phase and item titles take a smaller face at width 112% (`.long`), and titles over 48 characters drop to 18px at width 100% (`.xlong`).
- **Brand** (800, 22px, width 112%): "mull" at the top of the rail.
- **Headline** (750, 19px, width 95%): section heads ("Needs you", "Plans", "Revision 9"). Each sits on a 1.5px ink rule.
- **Title** (700, 15px): plan names in the Plans column. Tier titles use 700 at 12.5px.
- **Value** (550, 14.5px): the value in a title-block field.
- **Body** (400, 13.5px, line-height 1.45): the app default. Row text in schedules is 550. Table text is 13px.
- **Prose** (400, 16px, line-height 1.66, measure 68ch): rendered plan, phase and item docs. Doc h1 and h2 are 21px at 750, h3 is 16.5px. Notes and review packets use 15px.
- **Label** (650, 11px, letter-spacing 0.12em, width 75%, uppercase, muted): title-block field labels, table headers, the right-hand captions on section heads, and labels in the Plans definition list.
- **Status** (650, 11px, letter-spacing 0.09em, width 72%, uppercase): stage tags. Stamps use the same face at 700 with 0.08em tracking.
- **Mono** (500, 12px, tabular figures): IDs, branch names, clocks, counts and ratios. It's 11px in the tree and in tier numbers.

### Named Rules
**The Mono Is For Figures Rule.** Red Hat Mono sets IDs, hashes, branch names, clock times, counts and ratios. Never use it for sentences, headings or labels.

**The Width Axis Rule.** Hierarchy comes from Archivo's width and weight, not from new families. Names are wide and heavy (112–118%), labels are condensed and tracked (72–78%), and everything else sits near 100%.

## Layout

**Shell.** A two-column grid: a 312px sheet-index rail and the main sheet. The rail is sticky and full height on blueline paper, with a 1.5px ink rule on its right. It holds the brand row (56px, with the Live/Offline indicator), the scrolling tree, and a footer listing repos that aren't on mull yet.

**Sheet.** Each main sheet stacks the same way: title block, then the tabs row (project and plan sheets), then the page. The page has horizontal padding of `page-x`, 22px top and 64px bottom padding, and a 34px gap between sections.

**Title block.** A grid of boxed fields. The name box spans two rows on the left (minmax 280px, 1.5fr, or 3fr on `.wide` sheets for long titles), followed by three columns of fields in two rows of three. Fields are divided by 1px construction lines, and the whole block closes on a 1.5px ink rule.

**Project overview body.** Two columns: the Needs you schedule (fluid) and the Plans column (384px), 40px apart. Doc sheets use a 68ch prose column plus a status margin (200–270px), 44px apart. Review packets split into two equal columns 40px apart.

**Rhythm.** Rows are compact: tree rows are 29px, buttons 28px (24px small), tabs 44px, tier headers 32px minimum, and schedule rows have 10px vertical padding. Spacing steps in 4 / 8 / 12 / 22 / 34 / 40.

**Routes.** Hash routes: `#/` for all projects, `#/<project>` for its overview, `#/<project>/unplanned`, `#/<project>/backlog`, and `#/<project>/plans|phases|items/<id>`. A plan has tabs for Status, `/doc` and `/backlog`.

**Breakpoints.**
- **≤1180px.** The title block reflows to three columns, with the name box spanning the full width on top. Needs you and Plans stack, and so do doc blocks and their margin and the review packet's two columns. The tabs become a grid, and the actions move to their own row under a construction line.
- **≤860px.** The rail collapses into a top bar. The brand row shows a "Sheet index" toggle, and the tree (capped at 60vh) and the rail footer appear only when it's open. The title block goes to two columns. Needs you rows put their action under the text. Tier hints hide. Table cells marked `.w` (wide-only) hide. The last visible column must never be a `.w` column, because the last cell closes a clouded row. A cell marked `.nl` draws the cloud's right edge once the `.w` columns after it are hidden.
- **Phone (about 400px)** runs on the ≤860 layout. Tables keep a horizontal scroll wrapper as a fallback.

## Elevation & Depth

Flat. There are no drop shadows and nothing floats except the toast. Depth comes from rules and fills: 1.5px ink rules mark structure (the rail edge, the title block base, section heads, table header base), 1px ink outlines box controls, 1px construction lines divide rows and fields, and Panel Grey fills header bands. Hover means a Panel Grey or Rail Select fill, never a lift.

`box-shadow` appears only inset, as linework. It draws the blocking stamp's 1px outline, the current tree row's 3px blue left bar, the underline of the active cell in a cell bar, and the hollow Offline dot.

### Named Rules
**The Linework Not Lift Rule.** Separate regions with a rule or a fill, never with a shadow, blur or raised card. An inset `box-shadow` is allowed only as a line.

## Shapes

Square corners throughout (`rounded.none`), and `select` resets its radius to 0. Fields and controls are boxed with 1px ink. The only round shape is the 7px live dot. Stage words are distinguished by line style instead of colour: captured and shaping are dashed, waits is dotted, in review is a 3px double ink border, and dropped is struck through in faint ink. Cell bars are rows of 8×9px cells inside a 1px ink box, with done cells green, in-review cells hatched with ink at 135°, and active cells on Blueline Wash with a blue base.

## Components

### Buttons
- **Shape:** square, 28px tall, 1px ink border, 600 weight at 12px.
- **Default:** outline on Bond. **Hover** fills Panel Grey. **Active** presses down 1px.
- **Primary (`.p`):** ink fill, Bond text. Hover drops opacity to 0.88. There's one primary per action group, and the "Answer" button is primary only in the Blocking tier.
- **Small (`.s`):** 24px, 11.5px text, for row actions.
- **Disabled:** opacity 0.45, with a title explaining why (for example, "Mark revision seen" when nothing is new).
- **Behaviour:** read-first. A button copies a mull command or agent prompt and confirms in the toast ("Copied `mull accept …`: run it in a terminal"). "Mark revision seen" is the single exception: it posts the seen revision and fades the clouds.

### Stage tags and stamps
- **Tag:** status face in a 1px currentColor box, square. Colour follows the One Mark, One Job Rule: ready and now are blue, active is a solid blue fill, done is green, passed is a solid green fill, and committed and captured are muted.
- **Stamp:** a yellow box with a 1px ink outline that carries a blocking count ("2 BLOCKING", or just "2" in tight rows). It's for blocking only.

### Segmented filter, select, checkbox
- **Segmented control:** buttons joined in a 1px ink box. The pressed one (`aria-pressed`) is filled with ink.
- **Select:** 28px with a square ink border, drawn with a CSS chevron. Hover fills Panel Grey.
- **Checkbox:** a 15px ink square whose checked state is a 7px ink square that scales in over 0.12s.

### Navigation: the sheet index
- A tree drilling project → plan → phase → item, with Unplanned and Backlog under each project. Rows are 29px and indent 14px per level, with vertical blue construction-line guides.
- A rotating 10px SVG chevron (0.15s ease-out) toggles a row, and leaf rows hide it. Meta sits right-aligned in condensed muted text: stage words, counts, stamps, Go live tags.
- **Hover:** Rail Select fill. **Current:** Rail Select fill, bold blue text, and a 3px blue inset bar on the left. That bar comes from the approved prototype, and the human confirmed it (2026-11-11) because the tint alone is too faint: it's the one sanctioned left stripe, so don't add others. **Waiting:** faint text. Top-level project rows are bold.

### Tabs
- 44px links in muted 550 type. The active tab is ink, bold, with a 2px ink underline that overlaps the row's 1px ink base. Counts follow in condensed tabular figures. Actions sit at the right end of the row (they take their own row at ≤1180px).

### Title block
- The name box holds breadcrumbs and the display-size name. Each field box has a label over a value. Project sheets show integration branch, ahead count, status (Live / Reconnecting), last change, agents working, and revision ("Rev 9" plus a muted "9 changes since 07:16 PM").

### Needs you schedule (signature)
- Numbered tiers 01–06 in a `details` list, ranked by what answering unblocks. The ranking comes from the API.
- **Tier header:** a Panel Grey band (32px) with mono rank, chevron, bold title, muted mono count, and a right-aligned muted hint.
- **Rows:** a grid of rank number (1.1, 1.2…), mono ID, text (550) with a muted "where" line of underlined links, and a small action button.
- **Blocking tier:** its title gets a yellow highlighter swipe (a gradient covering 12–88% of the line height), and its row actions are primary.
- **Open by default:** the Blocking, Review and Now tiers. Open state persists across live re-renders.
- **Decision tier:** groups sources with "Approve all" (which copies `mull accept`) or "Review" when a review carries the group.
- Tiers holding more than one entry offer "Copy all N for an agent".

### Plans column
- Each plan gets a title and stage tag, then a definition list (76px label column) with Built / Now / Next / Go live / Agents. Rows hold name, stamp, cell bar and a mono ratio, and are separated by construction lines.

### Tables (schedules)
- Full width with a 1.5px ink header rule. Headers use the label face. Cells have 9px × 10px padding over 1px construction lines.
- Numeric columns are right-aligned with tabular figures. Titles are 600 with an optional muted meta line below.
- Group header rows use Panel Grey, and the "now" group uses Blueline Wash with blue text.
- Whole rows link via `data-href` and fill Panel Grey on hover. The title link stays the keyboard target.

### Docs
- Markdown is rendered with vendored marked 16.1.1. Raw HTML in a doc is shown as escaped text. Only `http(s):`, `mailto:` and `#` links stay links, and anything else becomes a dotted-underline span. Images become `[alt]` text.
- A four-hex ID that matches a known phase or item becomes a reference link: mono ID, blue 1px underline, and the stage as a tiny condensed superscript.
- Each section has a status margin of its own: the referenced phases and items with tag, cell bar and counts.
- Inline code and pre blocks sit on Panel Grey with no radius. Blockquotes use a 1px ink left rule in muted text. Tables are fully ruled in construction lines with a Panel Grey header.

### Revision clouds (signature)
- A scalloped red outline built from four radial-gradient arc strips: 12px scallops, a 1px stroke (5.1–6.1px radius), and a 7px band, drawn in a `::before` that sits 6px × 10px outside the element and doesn't take pointer events.
- **What gets clouded:** a tree row (when it's collapsed and anything inside changed, or open and the change is in none of its open children), a closed Needs you tier (at its header), a Needs you row or decision, Plans rows, tabs (Unplanned, Backlog, Plan doc), and the sheet name.
- **Table rows:** each cell draws its share of the cloud, so a clouded row's top and bottom edges run across all its cells while the first and last cells close the sides. Consecutive clouded rows share one cloud through `.cl-cont` and `.cl-open`.
- **Motion:** a fresh cloud (one not present on the previous fetch) draws in left to right over 0.7s `cubic-bezier(.16, 1, .3, 1)` via clip-path, and stays fresh for 750ms. "Mark revision seen" fades clouds out over 0.35s ease-out, and the tree's clouds clear in step.

### The model (help sheet)
- `#/-/model` ("-" is reserved for sheets that belong to no project), linked from the sheet index's foot on every page as "The model: how mull works", marked current while open.
- Static, in `model.js`: the hierarchy as a mono tree diagram (bold record names, muted asides; it scrolls sideways on a phone), the item stages as stage tags joined by arrows, the six Needs you tiers as a schedule table (rank, title and hint, what lands there), the rules and the commands the human runs as two-column definition lists (`.rules`; one column at ≤860px).
- It holds no status and no live data. When the model or a tier's wording changes, change it here too (tier titles and hints match `internal/mull/needsyou.go`).

### Toast
- An ink block with Bond text, fixed 18px from the bottom right (it respects the safe-area inset). It's at most 440px wide and lasts 5s. It's the only floating element.

### Motion
- The live dot pulses to 0.35 opacity every 2.4s, and the Offline dot is a hollow muted ring that doesn't move. Chevrons rotate in 0.15s ease-out.
- `prefers-reduced-motion: reduce` turns off every animation and transition.

## Do's and Don'ts

### Do:
- **Do** use the CSS custom properties (`--ink`, `--line`, `--blue`, and so on) and keep the light and dark blocks in step. A new token needs a value in the light `:root`, the `prefers-color-scheme: dark` block and the `[data-theme="dark"]` block.
- **Do** build structure from rules: 1.5px ink for structural edges, 1px construction line for minor dividers, 1px ink for control boxes.
- **Do** keep each hue to its job: red for clouds, yellow for blocking, blue for now, active and selected, green for done and passed.
- **Do** set IDs, counts and times in Red Hat Mono with tabular figures, and labels in condensed tracked Archivo caps.
- **Do** make every new action copy a mull command or agent prompt and confirm in the toast until writes exist.
- **Do** mark a table's wide-only columns `.w` and give the last always-visible cell `.nl` when `.w` columns follow it. The final visible column must not be `.w`.
- **Do** let any element that can hold a change take the cloud class from `Changes.cls(...)` rather than inventing its own marker.

### Don't:
- **Don't** use red for anything except revision clouds. That includes errors, destructive buttons and warnings.
- **Don't** add per-row change counts, delta triangles or "new" badges. The cloud alone means changed.
- **Don't** round corners, use pills for status, or put soft cards on grey.
- **Don't** add drop shadows, blur or raised surfaces. Inset `box-shadow` is allowed only as a line.
- **Don't** use yellow for anything but blocking, or blue for decoration.
- **Don't** load fonts from a CDN or add a third family.
- **Don't** render a doc's raw HTML or follow non-web links from docs.
- **Don't** show a raw API error code (such as `not_initialized`) for an expected state: say it in words. An unexpected code may sit beside the words, for diagnosis.

## Known gaps

- The FORM seed in `direction.md` couldn't be verified on this machine.
- The finish review judged the build "ship after fixes" against `direction.md` and the prototype. Its fixes are applied: tabs at ≤1180px, the current row stays blue, no raw `not_initialized` code, and phone-width tables.

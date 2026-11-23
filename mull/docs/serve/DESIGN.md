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
  vi: "#5B3F98"
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
  vi-dark: "#B6A2EC"
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
  copy-id:
    textColor: "{colors.mut}"
    rounded: "{rounded.none}"
    width: "18px"
    height: "18px"
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

# mull serve: the design system

**North star: "The Issued Drawing Set."** A project is a set of drawings issued for review: white bond paper, ink linework, pale blue construction lines, square corners, no elevation. Dense the way a drawing sheet is dense, with colour rare enough that each hue reads as a checker's mark with one job. The build in `mull/internal/ui/web/` is the source of truth: `style.css` holds every token and component, and the tokens above are its values (the `-dark` keys are dark mode's).

## Colours: one job per hue

- **Ink on paper** (`--ink`, `--ground`, `--panel`, `--mut`, `--faint`) for everything, and **construction lines** (`--line`) for minor rules.
- **Blueline blue** (`--blue`, `--blue-soft`): active and selected, and the focus ring.
- **Checker's green** (`--green`): done, passed, landed.
- **Checker's violet** (`--vi`): the left edge of a call that needs the human, and nothing else.
- A failure to save is an ink block (NOT SAVED) with the reason, not a colour. The highlighter and revision red tokens are reserved and unused.
- Light and dark follow the system; `data-theme` on the root overrides it.

## Type

Archivo across its width axis for text; Red Hat Mono for IDs, branches, counts and paths. Both are self-hosted woff2 in `/fonts/` (OFL); never load them from a CDN.

## Pages

- **Projects:** one row each: its calls, active and queued counts, and open plans. An unreadable repository says why.
- **Surface** (a project's page): Needs you first, each call on one row with its actions; then Trending worse, only when there's something; then Plans, each a two-sided hill (uphill: being worked out; downhill: executing; waiting and done quiet); then Flow, folded. Mark seen moves the last look.
- **Rules, Queue, Plan, Item:** the rulebook with each rule's check or a "no check" tag; the queue, newest first, with a one-line form to queue a bug or idea; a plan's doc, hill and items; an item and its latest report.

## Writes

Every action is the human's own step and saves as them, on the first click (closing a plan asks once). A failed write says NOT SAVED with the reason, in place; a saved one shows a toast and redraws. The page offers only the ops `uiOps` in `internal/ui/server.go` lists.

## Don't

- Soft cards on grey, status pills everywhere, shadows, rounded corners, cream-and-serif "AI" styling.
- A second meaning for a hue.
- Long text: the human reads a line per call, and opens the report when they want more.

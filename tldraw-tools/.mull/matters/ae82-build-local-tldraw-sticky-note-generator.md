---
status: planned
created: 2026-09-14
updated: 2026-09-14
---

# Build local tldraw sticky-note generator

## Implementation Plan

**Goal:** Build a local React application that lays out a configured list as tldraw sticky notes and exports a portable `.tldr` document.

**Architecture:** A Vite React TypeScript app hosts one full-window `Tldraw` editor. `src/App.tsx` owns the static item list and deterministic grid layout, creates note shapes once when the editor mounts, then uses tldraw's built-in file menu for export.

**Tech Stack:** Vite, React, TypeScript, tldraw.

### Task 1: Scaffold the local editor

**Files:**
- Create: `package.json`
- Create: `vite.config.ts`
- Create: `tsconfig.json`, `tsconfig.app.json`, `tsconfig.node.json`
- Create: `index.html`
- Create: `src/main.tsx`

**Behavior:** `npm run dev` serves a full-page React application; `npm run build` type-checks and creates a production bundle.

**Testing:** Verify the production build succeeds and the development URL loads without runtime errors.

### Task 2: Add the sticky-note grid

**Files:**
- Create: `src/App.tsx`
- Modify: `src/index.css` (remove starter styling that constrains the editor)
- Modify: `package.json` (add `tldraw`)

**Behavior:** On first editor mount, create one yellow note for each configured item. Place notes in a five-column, 200-pixel grid with 40-pixel gaps, convert each label to rich text, and zoom the viewport to fit all notes.

**Testing:** In the browser, verify the four provided items appear as four separate notes and that remounting does not duplicate them.

### Task 3: Verify portable export

**Files:**
- No source changes expected.

**Behavior:** Use tldraw's built-in File > Save a copy command to download a `.tldr` document suitable for importing into tldraw.com.

**Testing:** Export through the running UI and inspect the downloaded document to confirm it contains the configured note records.

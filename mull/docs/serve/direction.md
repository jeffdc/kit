# mull serve: the human's view

Scope: every route of the local UI (all projects, project overview with Overview / Unplanned / Backlog tabs, plan, phase, item and review, the model page). Mode: Operate (Read for plan and phase docs).

Audience and job: one solo developer, glancing between agent sessions, doing sit-down reviews, working beside an agent, and checking in at the start and end of the day, on a large monitor and a laptop. Job: answer "where are we" and "what's next, and what do I need to do", review work, and read the plans.

Constraints: light and dark, following the system. Not generic SaaS, not the AI look (no cream and serif). Not too sparse or too dense, not decorative. The ranked Needs you tiers stay. Plan docs must render as well-formatted, readable markdown. Status must be visible, but chips are optional. Read-first; actions arrive later, so leave room for them.

Unresolved: the production front-end stack; the name "phase" or "epic"; how answers flow back into docs.

## Direction contract

THESIS: The project is an issued drawing set. The tree is its sheet index, the header is a title block, and whatever changed since you last looked is clouded in red. This refuses the category default of soft cards on grey, and status pills everywhere.

OWN-WORLD: White bond ground (deep graphite in dark), ink linework at 1 and 1.5px, blue construction lines for minor rules. Colour follows a checker's marks on a check print, one job each: revision red for change clouds and deltas only; highlighter yellow for blocking work; blueline blue for now, active and the selected path; checker's green for done, accepted and passed. The sheet-index rail sits on pale blueline paper. Archivo across widths (condensed tracked caps for field labels and status words, wide heavy for page names) and Red Hat Mono for IDs and figures only. The real server self-hosts both as woff2; the artifact prototype loads them from Google Fonts because that is the only font host its sandbox allows. Square corners and boxed fields; no shadows.

STORY: You open a project and read the title block (branch, ahead count, live state, revision), then the numbered Needs you schedule ranked by what it unblocks, then the plans' Now, Next and Go live lines. Red clouds show at once what moved since your last visit, and "Mark revision seen" clears them.

FIRST VIEWPORT: A 312px sheet-index tree rail on the left (widened from 280px so item titles stay legible next to their stage and delta marks; behind a "Sheet index" toggle below 860px), drilling project → plan → phase → item, with Unplanned and Backlog under the project. On the right: the title block across the top (project name large, six boxed fields), the tabs row with the actions, then a two-column body: the Needs you schedule (numbered tiers 01–06, rows with ID, text, where, and an action) and the Plans column (Built / Now / Next / Go live / Agents as a labelled definition list with cell bars). Signature interaction: revision clouds on changed rows, cleared by "Mark revision seen", with the tree's clouds clearing in step. No delta triangles or counts: the cloud alone means changed.

FORM: Architectural drawing set (sheet index, title block, revision clouds), position 1 on the ordered grounded list, chosen as the pick. seed:efd5bfe58c3d5900

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

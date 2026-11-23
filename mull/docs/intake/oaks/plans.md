# Plans (draft for oaks)

Each plan doc below replaces today's (the old ones stay in `mull plan doc <slug> --log`). Appetites are proposals: Jeff sets them (D2). Phase "outline" lines are what gets cut into items when the phase starts.

## compendium: Compendium rebuild (active, target plan/compendium)

**Goal.** The reference for telling oaks and their hybrids apart, in Jeff's words, every claim traceable to its source. Done when taxa live in the new model linked to iNat; Jeff's material is in the database, private; facts are sourced and reviewable with Jeff's leading; Jeff writes and publishes entries in the site; the public site runs on the new design (taxon pages, comparisons, ID tool); every data change goes through the app or the import endpoint.

**Appetite.** About 6 weeks from now, scope cut rather than slipped. At most 3 agents, on disjoint areas; Taxonomy and Material can run side by side.

**Rules it adds.** R7 (contexts; its check comes with the first item), R9, R21, R26, R27.

**Settled (don't reopen).** SQLite. No audit log, no undo. iNat is the backbone; North America first. Varieties are taxa. Agents work outside the app; the app makes no LLM calls. Facts from Jeff's writing are proposed with the quote and count only when he accepts. The editor is in the site. Entries: key sentence, account, historical context, more later. No bulk iNat exports. One taxon template; comparisons at `?vs=`, the reader at `?read=`. Facts aren't copyrightable; Jeff's prose is. Any source can supply range data. ID mode starts as a feature list plus a link. The book is out of scope. No API key rotation for now. Design references: PRODUCT.md, DESIGN.md, design/prototype/, .impeccable/surfaces/; UI work uses the impeccable skill at medium and large size.

**Phases**, in order:

1. **Foundations** (54e3), **Site shell** (6b12): done.
2. **Polish** (224c), 3 days. Done when: Jeff calls the site acceptable on desktop and phone; check-ui passes; R17's ratchet is at most 12. Left: the 12 reviews (see queue-and-asks.md).
3. **Sources and facts core** (9ce7), 3 days. Starts now, so its 5 items stay: 64ed, 0f0b, a5a3, aabd, c245, plus a new first item, Boundary (R7's check, the map). Done when: a fact stores with its sources, evidence and standing; Jeff's fact leads its slot in a test; a disagreement raises a conflict in a test; a read model returns a taxon's facts per slot; `mix compile` enforces Boundary.
4. **Taxonomy** (5ba9), 5 days. Done when: every species row is a taxon with names, loaded through the import after Jeff read the dry-run report; hybrids have sourced parentage; varieties and subspecies are child taxa with a stored rank; taxa carry iNat ids; today's pages and v1 API tests pass through the adapter. Outline: taxa, names and ancestry (port gallformers' Ancestry, tree validator, NameNormalizer); hybrids and parentage; migrate species (fixing bugs b558, cb93, a946); iNat sync; editorial fields; adapter. Idea ed7a.
5. **Material and observations** (909b), 3 days, beside Taxonomy. Done when: compendium-notes is loaded after Jeff's sample check; a test shows no public route returns materials; duplicates are grouped; Jeff can capture one iNat comment or observation by URL. Outline: materials schema; load compendium-notes; duplicate grouping; observations and capture.
6. **Traits and fact review** (4210), 5 days. Done when: Jeff approved the vocabulary; counts and other sources' values are facts; an agent can propose a fact with its quote through the import; Jeff can accept, edit or decline it and resolve conflicts (prototypes ed-facts, ed-conflicts). Outline: vocabulary; load measures and source facts; proposals; review screen; conflicts screen.
7. **Authoring editor** (0cc1), 5 days. Done when: Jeff opens a taxon, sees text marked by provenance, inserts material verbatim, marks sections written and publishes them; a test shows the import refuses entry text (R21 gets its check). Outline: entry structure (ask da8b); entries, versions and provenance; editor screen; load the live short text for review; publishing (ask 7570); a section across taxa.
8. **Taxon pages** (cfc7), 5 days. Done when: species, hybrid, variety and higher taxa pages use one template on real data; `?vs=` and `?read=` work; Impeccable's finish review passes. Outline: first, the components the page needs, in the library (ideas 334d, 45f6); taxon page; comparison mode; hybrid and infraspecific pages; browse, species list and sources pages; API caching plug. Queue: d2a4, 180f. Ideas a41d, 4533.
9. **ID tool** (a1fe), 5 days. Done when: the trait filter and hybrid helper work on real North American facts with range overlap and hedging; ID mode copies a comment for an observation. Outline: matching engine; trait filter; hybrid helper; ID mode. Ideas c948, a903.
10. **Ranges and maps** (0ad5), 3 days. Done when: taxa have sourced range facts per place and the taxon page draws a map from them. Outline: boundary data; range facts; map component (in the library); map on the page.

**Back to outlines (dropped, 34 items).** Taxonomy: c3c8, eb79, a854, bba0, 4cdc, 58c9. Material: 0614, ffac, 260a, 18e8. Traits: 27ae, d6db, f7e1, e67f, 73ed. Editor: 1d51, 9125, 2211, 0337, c2ef, 8489. Taxon pages: 8142, cdc7, af27, 61c7, 8425. ID tool: 2b2c, e948, f3c3, 264f. Ranges: 50da, 1f5c, f812, 3601. Their notes stay readable in history.

## gallformers-parity: Gallformers parity (active, lands on next)

**Goal.** The two remaining ports from gallformers: the code-review command (a518) and structured JSON logging in prod (ba39). **Appetite.** 1 day. **Rules it adds.** None. **Phase** Ports (c3b2), done when both are accepted; then close the plan.

## search: Search rethink (shaping, target plan/search)

**Goal.** Search covers names, published entry sections and facts, and stays fast as content grows. **Appetite.** 1 week, after Taxon pages. **Rules it adds.** None new (R10 holds).

**To settle when shaping** (these replace questions e821 and 3bff): coverage is names, published entry sections and facts, through SQLite FTS5; results stay grouped by kind, with ranking inside a group only if FTS5's bm25 is enough. First, bug 4742 (search broken): confirm it still reproduces. Idea fa45 (say which common name matched) joins. Phases are cut when Jeff agrees the plan.

## audit-2026-11: Codebase audit follow-ups (close)

Its only open entry is question 948c (has the CSP been checked in a browser on prod?). That's a task an agent can do, so it becomes an unplanned item and the plan closes.

---
target: the whole Forage app (PWA)
total_score: 26
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/Users/jeff/dev/kit/.worktrees/forage-owned/forage/internal/pwa/assets/index.html"
target_fingerprint: "sha256:603cf657cf6270d05b8fa370d7c0b4714bfc0e74525f216f79e76296db21c720"
target_path: /Users/jeff/dev/kit/.worktrees/forage-owned/forage/internal/pwa/assets/index.html
timestamp: 2026-11-11T01-08-00Z
slug: internal-pwa-assets-index-html
---
Method: dual-agent (A: design review · B: detector and browser). A captured phone/light only; the parent captured desktop 1280/1440 and dark to verify.

## Design Health Score

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 3 | Sync state is honest; offline only discovered after pressing Find in Add |
| 2 | Match System / Real World | 3 | Cover metaphor works; "Got it" ambiguous |
| 3 | User Control and Freedom | 2 | No undo for status, finish, owned, rating clear or delete |
| 4 | Consistency and Standards | 3 | Strong system; Add's status segment unlabeled, Tags shown in Edit though hidden elsewhere |
| 5 | Error Prevention | 2 | Live stars on unread books in the scroll path; re-tap clears rating silently |
| 6 | Recognition Rather Than Recall | 3 | Ownership glyph cryptic; undated reads only "at the end of History" |
| 7 | Flexibility and Efficiency | 2 | Only / and Esc; no filters, no list keyboard nav, no desktop density |
| 8 | Aesthetic and Minimalist Design | 3 | Calm, but empty meta columns and long covers |
| 9 | Error Recovery | 3 | Refused ops explained in Settings with reasons |
| 10 | Help and Documentation | 2 | Good empty states; no hints for shortcuts or undated reads |
| **Total** | | **26/40** | **Acceptable** |

## Design Specificity Verdict
Authored, not generic, on phone: colour means status and nothing else, one face, square ink grammar, and the three-band cover with its sweep is unmistakably this product. Generic in the lists (a hairline row with a mostly empty right column) and on desktop, which is the phone column centred in a laptop window. The new ownership glyph reads as letters ("Ilz") at 18px.
Detector: index.html clean; directory scan 10 advisory design-system-font-size notes in style.css, about 7 are sizes DESIGN.md prose allows but its YAML ramp omits (20 sub-head, 19 cover author, 16 buttons/toast/own-toggle, 12 match); real drift: .unsynced 12px, .empty-title 20px, .sync-line 18px. Overlay: no anti-patterns on Wishlist, detail, Add (phone and desktop); History inconclusive (1,889 rows). Touch targets under 44px: sync button 67x36, sort select x36. Contrast passes AA everywhere measured.

## Priority Issues
1. [P1] Desktop is a stretched phone. 720px column centred (50% of a 1440 screen empty), phone tab bar at the bottom, sort as a select, book opens as a modal 480px drawer over a scrim that covers the list's right columns; any click outside only closes it, so browsing costs two clicks per book. Fix: from ~1100px a left rail of shelves with counts, a dense sortable table (Title, Author, Year, Status, Last read, Rating, Owned), and a persistent non-modal cover pane following the selection; keyboard j/k, Enter, f, o, 1-5, e, n. 760-1100: list + pane, tabs on top. Phone unchanged. Commands: layout, adapt.
2. [P1] Ownership mark is unreadable and in the wrong places. Pictogram reads as letters; Wishlist reserves an always-empty column; History marks ~every row (wallpaper); search, where "do I own this?" is asked, buries it beside tag/date/stars. Fix: words in the existing tag grammar (an ink-outlined OWNED tag), shown where it informs (search, Reading/To Read, Add matches); desktop Owned column; an Owned filter. Commands: clarify, typeset.
3. [P1] Fixing missing read dates is a dead end. 1,345 undated reads at the bottom of ~1,889 History rows; Settings mentions them without a link; each fix is scroll, open, scroll, date, back. Fix: an Undated view (link from History head and Settings) with inline year/month/day entry that saves on change; a "Set date" primary and "Next undated" in the open book. Commands: shape, layout.
4. [P2] No undo. Status buttons, finish, owned toggle, rating clear and delete all commit instantly with a 3s toast. Fix: Undo on every write toast (inverse op, or drop the unsent op), toasts with actions stay 6s+ and pause on focus/hover; armed delete-read becomes a labelled text button. Command: harden.
5. [P2] Field use costs taps. Long titles fill the phone's first screen; interactive stars on wishlist/to-read books; Find a copy below the fold with 10 sellers; in Add the status/owned controls sit between input and results, so results land behind the keyboard; Settings and Edit pop the keyboard on open. Fix: clamp cover title, stars only once started, top 3 sellers + More, results directly under the input with "Add as" moved to the result, autofocus only in Add. Commands: distill, layout.

## Persona Red Flags
Alex (power user): only / and Esc; no filters (owned, undated, unrated, year); clicking another row with the drawer open just closes it; no bulk date or ownership edit.
Casey (one-handed in a bookstore, bad signal): search at the top of the screen; 18px glyph unreadable at arm's length; offline only detected after Find; sync tap opens Settings with the keyboard up; stars in the scroll path with no undo; 3s toasts.
Sam (accessibility): no focus trap in aria-modal panels (Tab walks the dimmed page on desktop); row names run together with no status announced on tab lists; armed delete-read changes silently; timed toast actions; on-band unfilled stars at .55 opacity.

## Minor Observations
Reading head counts 14 while the tab holds 61; To Read buried under 43 Paused. Search matches inside words ("guin" hits "arguing"). Reading cover's top band is empty on the right. Edit's Save below the fold on phone. Candidate "Add" doesn't say where. Long tracked label "LINK, ISBN, OR TITLE AND AUTHOR" strains the Tracked Label Rule. Wishlist orange and alert red are close hues. DESIGN.md's YAML type ramp is missing sizes its prose uses.

## Questions to Consider
- On desktop, why is the open book modal at all? What if the cover were always on screen, a catalogue card beside a ledger?
- Is History a shelf you browse or an archive you query? Would a "Needs a date" queue earn more than year bars?
- Should the store moment answer in words: READ · OWNED · WISHLIST?
- Should the app open on the last shelf, or on search?

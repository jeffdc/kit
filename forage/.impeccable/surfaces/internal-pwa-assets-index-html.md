---
version: 1
slug: "internal-pwa-assets-index-html"
primary_target: "internal/pwa/assets/index.html"
related_targets: ["internal/pwa/assets/app.js","internal/pwa/assets/style.css"]
---

# Surface brief: Forage PWA

Mode: Operate. iPhone first (390 px), desktop occasional. Opens to Wishlist.

Jobs: search everything ("have I read this?"), browse the wishlist in a store (sorted by author surname), log a finish (date and stars), re-reads, add from a link (resolver) or by hand, edit. Sorting stays (title, author, rating, date added; date read in History). Tags and notes are hidden from the main UI.

States: offline, pending sync, rejected op, empty search, undated reads (1,345), the first sync of 2,100 books.

## Direction contract

THESIS: Status is the book's colour, the way a mid-century paperback series told its kind by its band. It refuses the default grey iOS list with one blue accent, and the cream-and-serif "bookish" app.

OWN-WORLD: Gill Sans (system on iOS; humanist-sans fallbacks). Ground #FBFBF9, ink #1C1C1A, neutrals from one fixed six-step ramp. Flat ink band colours per status: wishlist orange, owned cerise, reading green, paused ochre, read blue, dropped grey. Zero radius, no shadows, gradients or glass, 1px hairlines between rows, square 1.5px-outlined inputs, solid ink primary buttons, tracked small caps for labels.

STORY: Opening to the Wishlist, the owner reads 9 or 10 books per screen, types three letters, and sees every match with its status. Tapping one opens the book as a cover, and one tap logs a finish.

FIRST VIEWPORT: Top line: FORAGE wordmark in small caps at left, sync dot at right. Then a full-width square search field. Then the shelf heading "Wishlist" with its orange band swatch, count and sort control. Then hairline rows: a 5px status edge, bold title, author, and year or owned tag at right. The bottom tab bar holds Wishlist, Reading and History (the active tab topped by a 4px band in its colour) and a square ink "+" Add button.

FORM: Mid-century colour-coded paperback series covers, candidate 3 of 7 (assigned); seed:03ef4c984740eff7. Signature move: the detail sheet is a three-band cover, with status-colour bands top and bottom and title and author centred in the white middle band. The primary action sits in the bottom band, and a status change slides the new colour across the bands.

Raises (from declined challengers): one fixed tonal ramp (Darkroom). History year bars drawn to scale (Labanotation). Unsynced rows keep a mark until they land (Gate board). Flat colour, no effects (Zoo map). Hairlines, not boxes (Design annual). Bands at full strength only on the open book (Catalog sleeve).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

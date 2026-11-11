---
name: Forage
description: A private book library where each status is a colour, set like a mid-century paperback series.
colors:
  ground: "#FBFBF9"
  paper: "#FFFFFF"
  ink: "#1C1C1A"
  n1: "#3A3A37"
  n2: "#5E5E59"
  n3: "#8C8C86"
  n4: "#D9D9D4"
  n5: "#EFEFEB"
  band-wishlist: "#C4471B"
  on-wishlist: "#FFFFFF"
  band-to-read: "#A62B5C"
  on-to-read: "#FFFFFF"
  band-reading: "#2B6B43"
  on-reading: "#FFFFFF"
  band-paused: "#D4A82A"
  on-paused: "#1C1C1A"
  band-read: "#24488A"
  on-read: "#FFFFFF"
  band-dropped: "#686864"
  on-dropped: "#FFFFFF"
  alert: "#B3261E"
  ground-dark: "#141413"
  paper-dark: "#1C1C1A"
  ink-dark: "#ECECE8"
  n1-dark: "#C8C8C2"
  n2-dark: "#A3A39D"
  n3-dark: "#77776F"
  n4-dark: "#33332F"
  n5-dark: "#1F1F1D"
  band-wishlist-dark: "#E4743F"
  band-to-read-dark: "#D6649A"
  band-reading-dark: "#5BAE78"
  band-paused-dark: "#D9B341"
  band-read-dark: "#7A9FE0"
  band-dropped-dark: "#A3A39D"
  on-band-dark: "#141413"
  alert-dark: "#F28B82"
typography:
  display:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "31px"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-0.01em"
  display-long:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.15
  headline:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "28px"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "22px"
    fontWeight: 700
  title-sub:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "20px"
    fontWeight: 700
  cover-author:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "19px"
    fontWeight: 400
  title-row:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "17px"
    fontWeight: 700
    lineHeight: 1.2
  body:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "17px"
    fontWeight: 400
    lineHeight: 1.35
  body-secondary:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    fontFeature: "tnum"
  label-band:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 700
    letterSpacing: "0.26em"
  label-section:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 700
    letterSpacing: "0.2em"
  label-field:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 700
    letterSpacing: "0.14em"
  label-tab:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 600
    letterSpacing: "0.04em"
  label-button:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 700
  label-match:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 700
    letterSpacing: "0.08em"
  label-tag:
    fontFamily: "Gill Sans, Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui, sans-serif"
    fontSize: "11px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "0.14em"
rounded:
  none: "0px"
spacing:
  xs: "6px"
  sm: "10px"
  md: "12px"
  gutter: "16px"
  section: "26px"
  empty: "56px"
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    rounded: "{rounded.none}"
    padding: "0 16px"
    height: "46px"
  button-outline:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "0 16px"
    height: "46px"
  button-danger:
    backgroundColor: "transparent"
    textColor: "{colors.alert}"
    rounded: "{rounded.none}"
    padding: "0 16px"
    height: "46px"
  button-danger-armed:
    backgroundColor: "{colors.alert}"
    textColor: "{colors.ground}"
    rounded: "{rounded.none}"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    height: "46px"
    padding: "0"
  band-button:
    backgroundColor: "transparent"
    textColor: "{colors.on-read}"
    rounded: "{rounded.none}"
    padding: "0 12px"
    height: "50px"
  band-button-primary:
    backgroundColor: "{colors.on-read}"
    textColor: "{colors.band-read}"
    rounded: "{rounded.none}"
    padding: "0 12px"
    height: "50px"
  add-button:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    rounded: "{rounded.none}"
    width: "58px"
    height: "48px"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "10px 12px"
    height: "48px"
  search-field:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0 4px 0 12px"
    height: "46px"
  book-row:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.title-row}"
    height: "62px"
  book-row-pressed:
    backgroundColor: "{colors.n5}"
  status-tag:
    backgroundColor: "{colors.band-to-read}"
    textColor: "{colors.on-to-read}"
    typography: "{typography.label-tag}"
    rounded: "{rounded.none}"
    padding: "3px 6px 2px"
  tab:
    backgroundColor: "{colors.ground}"
    textColor: "{colors.n2}"
    typography: "{typography.label-tab}"
  tab-active:
    textColor: "{colors.ink}"
  segmented-pressed:
    backgroundColor: "{colors.band-wishlist}"
    textColor: "{colors.on-wishlist}"
    height: "42px"
  cover-mid:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.display}"
    padding: "30px 20px 20px"
  toast:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.ground}"
    rounded: "{rounded.none}"
    padding: "12px 12px 12px 16px"
  sync-dot:
    backgroundColor: "{colors.band-reading}"
    size: "8px"
    rounded: "{rounded.none}"
---

# Design System: Forage

## Overview

**Creative North Star: "The Paperback Series Shelf"**

Forage looks like a run of mid-century paperbacks: one humanist sans, flat ink, and a coloured band that tells you what kind of book it is. Here the band tells you the book's status. Wishlist is orange, to read cerise, reading green, paused ochre, read blue, and dropped grey. Status is the only thing colour encodes. Everything else is ink on a near-white ground, with a single fixed neutral ramp.

The interface is dense and plain because the job is speed. Nine or ten hairline rows fit on a phone screen. The search field is a square ink-outlined box at the top of every shelf. Lists stay quiet: colour shows only as a 5px edge, a swatch or a small tag. Colour fills a large area only when a book is opened. The detail view is drawn as a cover with three bands: a status band, a white title band and a status band. A status change sweeps the old colour off the cover to reveal the new one. That sweep is the system's one bit of theatre.

The world rejects two defaults by name: the grey iOS list with one blue accent, and the cream-and-serif "bookish" app. There are no shadows, gradients, glass or rounded corners anywhere.

**Key Characteristics:**
- Status-coded flat band colours, each paired with a fixed on-band text colour.
- One fixed neutral ramp (ground, paper, ink, n1 to n5) for light and dark.
- One humanist sans (Gill Sans, then Seravek) at every size.
- Zero radius. Hairlines (1px) separate. Ink outlines (1.5px) mark anything you can act on.
- A three-band cover for the open book, with a status sweep on change.
- A square icon grammar: 1.75 stroke, square caps, mitred joins.

## Colors

The palette is flat ink and flat paper. Each status gets one saturated band, and the neutrals come from one ramp. In dark mode the bands get lighter and every on-band colour becomes the dark ground.

### Primary (status bands)
- **Paperback Orange** (band-wishlist): the wishlist. Used for the wishlist tab band, the wishlist swatch and row edge, the wishlist cover, and the sync dot while sending.
- **Cerise** (band-to-read): books to read, not started. Used for the To Read tab band, its row edge and swatch, and the to-read cover. (It was "owned" until ownership became its own field.)
- **Reading Green** (band-reading): currently reading. Used for the Reading tab band, the reading cover, and the sync dot when synced.
- **Ochre** (band-paused): paused. This is the only light-scheme band that takes ink text (on-paused) instead of white.
- **Series Blue** (band-read): read books and History. Used for the History tab band, the year bars, the read cover and text selection.
- **Slate** (band-dropped): dropped books. It is close to the neutrals on purpose, so dropped books recede.

Each band has an **on-band** partner (on-wishlist, on-to-read, and so on) for any text, outline or star drawn on it. In dark mode every on-band colour is the dark ground (on-band-dark).

### Secondary
- **Alert Red** (alert): errors, the "Not synced" mark on rows, the sync dot on failure, and the armed delete button. It is never used as a status.

### Neutral
- **Ground** (ground): the page, header, tab bar and panels.
- **Paper** (paper): raised writing surfaces with no shadow: inputs, the search field, the sort select, and the white middle band of the cover.
- **Ink** (ink): text, outlines on interactive elements, primary buttons, the add button and the toast.
- **n1**: secondary text weight (authors on rows and the cover).
- **n2**: muted text (counts, meta, field and section labels, placeholders, inactive tabs, icon tint).
- **n3**: idle marks (unfilled stars, idle sync dot, scrollbar).
- **n4**: hairlines (row dividers, header and tab bar rules, cover side rules, drawer edge).
- **n5**: pressed and hover row fill, and the alphabet letter strip.

### Named Rules
**The Band Means Status Rule.** A band colour always names a status. Don't use one as a decorative accent. The only exceptions are the sync dot and text selection, which borrow existing inks as signals. Don't add a new hue to the palette.

**The Full Band Rule.** Colour fills a large area only on the open book's cover. In lists it shrinks to a 5px edge, a 24x8 swatch, a tag, or the 4px band over a tab.

**The One Ramp Rule.** Every grey comes from ground, paper, ink and n1 to n5. Nothing else is grey.

**The On-Band Pairing Rule.** Anything drawn on a band, whether text, an outline, a star or a date field border, uses that band's on-band colour. Don't put ink on a band unless ink is its on-band.

## Typography

**Display Font:** Gill Sans (with Gill Sans MT, Gill Sans Nova, Seravek, Cabin, system-ui)
**Body Font:** the same face
**Label/Mono Font:** the same face. Numbers use tabular figures.

**Character:** Gill Sans is a mid-century, humanist and slightly bookish sans that never turns into a serif. One face covers the whole hierarchy. Weight (700 against 400) and tracked capitals carry the structure. Forage has one user, who reads it on an iPhone and a Mac, so it relies on the installed Gill Sans and Seravek rather than loading a web font.

### Hierarchy
- **Display** (700, 31px, 1.1, -0.01em, balanced wrap): the book title on the cover. Used only there. Titles longer than 48 characters step down to **Display Long** (24px, 1.15) so the cover doesn't fill a phone screen.
- **Headline** (700, 28px, 1.1, -0.01em): shelf titles (Wishlist, To Read, Reading, History).
- **Title** (700, 22px): sheet titles, History year headings and empty-state titles. **Title Sub** (20px) is for the Paused sub-head.
- **Title Row** (700, 17px, 1.2, clamped to two lines): the book title in a row.
- **Body** (400, 17px, 1.35): the base size. Inputs are also 17px, which stops iOS zooming on focus. Notes are capped at 65ch.
- **Body Secondary** (400, 15px, tabular figures): authors, counts, row meta and cover facts. Authors on the cover are 19px.
- **Label Band** (700, 14px, 0.26em, uppercase): the status name in the cover's top band. The FORAGE wordmark uses the same tracking at 600.
- **Label Section** (700, 14px, 0.2em, uppercase, n2): detail sections (Reads, Find a copy, Details), set over a hairline.
- **Label Field** (700, 13px, 0.14em, uppercase, n2): form field labels.
- **Label Tab** (600, 13px, 0.04em): tab bar labels.
- **Label Button** (700, 16px): button labels, the toast, and row titles in the desk table.
- **Label Tag** (700, 11px, 0.14em, uppercase): status and owned tags in rows. **Label Match** (12px at 0.08em) is the candidate match tag.

### Named Rules
**The One Face Rule.** Gill Sans at every level. Build hierarchy from weight, size and tracking. Never add a second family.

**The Tracked Label Rule.** Tracked capitals are only for short labels that name something: the wordmark, a status, a section, a field, a tag, an alphabet letter. Never set titles, sentences or buttons in tracked capitals.

**The Optical Dot Rule.** Inline facts are joined with a middle dot inside a span with margins of 0.48em left and 0.22em right. Gill Sans draws the dot right of centre, so a plain " · " looks lopsided. Never type a literal spaced dot.

## Layout

The layout is a single column, capped at 720px and centred. It is built for the phone first. There is a 16px gutter throughout. The sticky header holds the wordmark and sync on one line, with the 46px search field underneath. A fixed 64px tab bar sits at the bottom, plus the safe-area inset. Rows have no gaps and are separated by hairlines. Rows are at least 62px high, so about ten fit on a phone screen. Section and year headings get 26px of space above and 10px below. Shelf headings are a two-column grid: the title and count on the left, the sort control on the right. The count is indented 34px to line up past the swatch.

The book detail and the sheets (Add, Edit, Settings) are full-screen panels that slide up from the bottom on a phone. **From 760px** they become a 480px drawer that slides in from the right edge. The drawer has a 1px n4 left rule, and a 28% dark scrim covers the page behind it. In the drawer the back chevron turns into a close X.

The phone layout is for the field (a bookstore, adding from a link). A laptop is for long jobs (browsing History, filling in dates), so wide screens get a workspace instead of a stretched phone:

- **Split (900px and wider):** the list on the left and the open book as a **pane** on the right (clamp(380px, 34vw, 480px), 1px n4 left rule). The pane is always on screen: picking a row shows that book in it, with no scrim, no slide and no history entry, and the selected row is filled n5 with a 1.5px ink inset outline. With nothing picked the pane teaches the keys. The list scrolls inside its own column; the header, search and shelves stay put. The shelves become a strip of tabs above the list, with counts, and Add becomes a labelled ink button at its right end. Sheets stay drawers over everything.
- **Desk (1280px and wider):** a 208px **rail** on the left holds the wordmark and sync, an "Add a book" button, and the shelves with counts; the active shelf has a 4px band on its left edge and an n5 fill. "Needs a date" sits under History. The list becomes a **table**: each row is one 44px line of columns (Title, Author, Year or Read date, Rating, Owned; search adds Status), with sticky column headings that sort when clicked.

Spacing comes from a short set of steps: 6, 10, 12, 16 (the gutter), 26 (section) and 56 (empty states). Touch targets are 44px or more. Icon buttons are 44px, boxed icon buttons 48px, band buttons 50px.

## Elevation & Depth

Forage is completely flat. Nothing casts a shadow, nothing has a gradient, and nothing is blurred or glassy. Depth comes from position and from lines. Panels slide over the page and cover it fully, so they never float above it. A 1px n4 hairline marks every edge between regions. The paper fill is one step brighter than the ground, which separates writing surfaces without lifting them. The drawer scrim (rgb(20 20 19 / .28)) is the one translucent layer in the system.

### Named Rules
**The Flat Ink Rule.** No box-shadow, gradient, backdrop-filter or translucency anywhere, except the drawer scrim. If something needs separating, use a hairline or a change of fill.

**The Hairlines, Not Boxes Rule.** Rows, sections and headers are divided by 1px n4 rules. Don't wrap a list item in a card.

## Shapes

Every corner is square (rounded.none). That includes inputs, buttons, the select, tags, the toast, the 8px sync dot, the 24x8 swatch and the year bars. There are two line weights. A 1px n4 hairline divides content. A 1.5px ink outline (or on-band on a band) marks anything you can type in or press. Status shows up as rectangles: a 5px row edge, a 4px band across the top of the active tab, a 40x2 rule under the cover title, and the full-width cover bands.

Icons follow the same square grammar. They are drawn on a 24px grid with a 1.75 stroke, square caps and mitred joins, coloured with currentColor and rendered as inline SVG. In buttons they drop to 20px, and the seller link icon is 18px. The star is the only filled glyph: it uses a 1.5 stroke and a mitre join, filled when on and drawn as an n3 outline when off.

### Named Rules
**The Square Rule.** Radius is zero everywhere, the dot included. Nothing in the UI is a circle or a pill. The only round shape is the magnifier's lens inside the search icon.

## Components

### Buttons
Square and inked, with confident 700-weight labels.
- **Shape:** square corners (0px), 1.5px ink outline, at least 46px high, 0 16px padding, 16px/700 label, 20px icon with an 8px gap.
- **Primary:** solid ink with ground-coloured text ("Find", "Add", "Save").
- **Outline:** transparent with ink outline and ink text. This is the default for secondary actions.
- **Danger:** an alert-red outline and text. Tapping it once **arms** it: it fills alert red and the label changes to "Tap again to delete".
- **Quiet:** no box. An ink 600-weight label, underlined at 1px with a 4px offset, with a leading icon ("Add an earlier read", "Enter it by hand").
- **Disabled:** 50% opacity.
- **Focus:** a 2px ink outline at a 2px offset, applied globally.

### Status Tag
- **Style:** a solid band fill with on-band text, 11px/700, 0.14em uppercase, padding 3px 6px 2px. It sits in the row meta column (every status in search results). The candidate **match** tag is the same idea at 12px and can include on-band stars.

### Book Row
The core element of the app.
- **Structure:** a full-width button with a 5px status edge on the left, then the title (17px/700, up to two lines) above the author (15px, n1, one line with an ellipsis), then a right-aligned meta column (14px, n2, tabular) of two lines: the tags (status tag in search, then the owned tag), then the year or date with the stars. The alert-coloured "Not synced" mark goes last.
- **Desk table:** the same cells laid out as one line of grid columns (see Layout). The owned cell is an ink check under an OWNED heading, since a column of words would be wallpaper; the word stays for screen readers.

### Owned Tag
- **Ownership is words, in ink, never a band.** Ownership isn't a status, so it never takes a band colour (The Band Means Status Rule). In rows it's an **outlined tag**: OWNED in Label Tag style, a 1.5px ink outline, no fill, so it reads at arm's length in a shop and sits beside a filled status tag without being mistaken for one.
- In the open book it's the "I own a copy" switch under the rating: a 26px square 1.5px ink box that fills with ink and shows a check when on. The add sheets carry the same switch under "Add as".

### Needs a Date
- The queue for reads with no finish date, reached from History, Settings and the desk rail. Each row is the book (title over author, opening it) and its own date field (172px); a date saves when it's complete and focus moves to the next row's field. A date still being typed (year 0002, 0020, 0202) is never saved.
- On an open book with an undated read, the bottom band's primary becomes "Set finish date", which jumps to that read's date field.

### Keys (wide screens)
- j/k or the arrows move the selection, / searches, f finishes a reading or paused book, o toggles owned, 1–5 rate a started book, e edits, n adds, Esc closes or clears. Keys are drawn as **kbd**: 600 14px ink on paper with a 1px n3 outline.
- **Divider:** a 1px n4 bottom hairline. There is no card or fill.
- **Pressed / Hover:** fills n5.
- **Alphabet strip:** when a list is sorted by author or title, an n5 strip carries the letter at 13px/600, 0.22em tracking.

### Inputs / Fields
- **Style:** square, 1.5px ink outline, paper fill, at least 48px high, 10px 12px padding, 17px text, n2 placeholder. Native appearance is removed.
- **Search field:** 46px high, a square ink box with an n2 magnifier and a 40px clear button. On focus it gets a 2px ink outline at a 1px offset.
- **Select:** a 1px n4 outline (lighter than inputs), paper fill, and a square-capped chevron drawn inline.
- **On a band:** date inputs inside a band switch to an on-band outline.
- **Error:** alert-red text below the field.

### Segmented Control
- **Style:** a 1.5px ink frame split by 1px ink dividers, with buttons at least 42px high in 15px/600. The pressed segment fills with **its own status band** and on-band text. This is how "Add as" previews the colour the book will get.

### Navigation (Tab Bar)
- **Style:** fixed to the bottom on a ground fill with a 1px n4 top rule. Each of the four tabs (Wishlist, To Read, Reading, History, in the order a book moves through them) stacks a 24px icon over a 13px/600 label in n2. To Read's icon is a closed book.
- **Active:** ink text and a **4px band along the top edge** in the shelf's colour (orange for Wishlist, cerise for To Read, green for Reading, blue for History).
- **Split strip (900–1279px):** the tabs drop their icons and show counts; Add is a square "+" until 1100px, then gains its label.
- **Add:** a square 58x48 solid ink button with a ground-coloured plus, at the right end.

### Sync Dot
- **Style:** an **8px square**, never round. It is n3 when idle, reading green when synced, wishlist orange when changes are waiting or sending (with a 1s opacity pulse while syncing), and alert red on error. It sits beside 15px n2 text in the header and in Settings.

### Toast
- **Style:** a solid ink bar with ground-coloured text, 16px, sitting 12px above the tab bar (24px from the bottom of the list column on wide screens) and at most 560px wide. Its action is a 1.5px ground-outlined button. It enters by fading in (200ms) and rising 12px (300ms, on the system ease).
- **Undo:** every change made from the open book (status, Got it, owned, clearing a rating, logging a finish, adding or deleting a read) says what it did with an **Undo** button, which queues the inverse change. A toast with a button stays 6s and waits while the pointer or focus is on it. Deleting a book has a two-tap confirm instead.

### The Cover (signature component)
The open book, drawn as a paperback cover.
- **Top band:** the status fill, at least 58px high. The status name is on the left in Label Band style. On the right are read count and last-read date, or the date added, in 15px tabular figures.
- **Middle band:** paper fill with 1px n4 side rules and 30px 20px 20px padding, all centred. It holds the Display title, then a **40x2 rule in the band colour** (14px above, 12px below), then the author (19px, n1), the facts (15px, n2), and a row of five 44px star buttons with 26px stars.
- **Bottom band:** the status fill again, padded 14px 16px 16px. It holds one or two 50px band buttons. The primary is filled with on-band and has band-coloured text. The secondary is an on-band outline. The finish form (date, stars, Log it) replaces the buttons in the same band.
- **Status sweep:** when the status changes, each band shows the old colour on top and shrinks it to the right (scaleX 1 to 0, origin right) over **550ms** on the expo ease-out cubic-bezier(.16, 1, .3, 1). The bottom band starts **70ms** after the top. The band contents rise 3px from 60% opacity over 450ms. A re-render waits 640ms so the sweep isn't cut off. With reduced motion there is no sweep.

### Panels
- **Motion:** panels slide in over 340ms on the same expo ease-out, up from the bottom on a phone and in from the right at 760px and wider. With reduced motion the transition is turned off.
- **Header:** a sticky bar on the ground fill with 44px icon buttons. Sheets add a 22px/700 title and an n4 rule below.
- **Focus:** Add and manual entry open with the keyboard up; other sheets focus their close button. Tab stays inside an open sheet.
- **Not started:** a wishlist or to-read book shows no stars (nothing to rate yet) and puts "Find a copy" before Reads: three sellers, then "More places (n)".

## Do's and Don'ts

### Do:
- **Do** set every surface from the token ramp: ground for pages, paper for anything you write in, ink for anything you press.
- **Do** carry status as a 5px edge, a 24x8 swatch, a tag or a 4px tab band in lists, and save full-width colour for the cover.
- **Do** pair every band with its on-band colour for text, outlines, stars and fields drawn on it.
- **Do** separate rows and sections with 1px n4 hairlines, and outline interactive elements in 1.5px ink.
- **Do** draw new icons on the 24px grid with a 1.75 stroke, square caps and mitred joins, coloured with currentColor.
- **Do** join inline facts with the optically spaced middle-dot span (margins 0.48em / 0.22em).
- **Do** use tabular figures for counts, dates, years and ISBNs.
- **Do** keep touch targets at 44px or more and inputs at 17px.

### Don't:
- **Don't** round any corner, including dots, tags and buttons (0px everywhere).
- **Don't** add box-shadows, gradients, blur or glass. Only the drawer scrim is translucent.
- **Don't** introduce a new hue, or use a band colour to decorate something that isn't a status.
- **Don't** drift toward the grey iOS list with one blue accent, or the cream-and-serif "bookish" app.
- **Don't** add a second typeface, or set titles and sentences in tracked capitals.
- **Don't** wrap rows or sections in cards.
- **Don't** use round line caps or joins in icons, or use emoji or Unicode symbols as icons.
- **Don't** fill a large area with a status colour anywhere except the open book's cover.

# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

One person, the owner. No accounts, sharing, or other users.

They use it mostly on an iPhone, as a home-screen web app, in three situations:

- **Reading online:** they find a book recommended on a website or in a Bluesky post and want it on the wishlist in a few seconds.
- **In a bookstore:** often with poor or no signal. They browse the wishlist, check whether they've already read something, and add finds to the wishlist.
- **After finishing a book:** they log it with the date finished and a rating.

A laptop browser is used occasionally.

## Product Purpose

A personal book library: what they've read, what they're reading, and what they want to read.

Success means:

- Every book they finish is recorded with its date and rating, including re-reads.
- Any book can be found by search in a moment, offline.
- Adding a book from a shared link or in a store is quick enough that it actually happens.

## Positioning

A private, single-user tool built around the owner's own jobs: a quick wishlist add from a shared link, bookstore lookups, and an accurate reading history. It has none of the social, recommendation, or catalog features of Goodreads-style services.

## Operating Context

- iPhone first, installed to the home screen. A desktop browser now and then.
- Offline in stores. All reading and searching work from local data. Changes queue and sync when back online.
- Links arrive from Safari, Bluesky and other apps through the iOS share sheet, via an iOS Shortcut that calls the server API.
- The owner prefers buying used. Bookseller links (Better World Books, Biblio, Bookshop.org, ThriftBooks, AbeBooks, WorldCat, Goodreads, BookFinder) are used to find copies. Amazon is kept only as a price reference.

## Capabilities and Constraints

- About 2,100 books: roughly 1,900 read, 150 on the wishlist, plus reading, paused, owned and dropped.
- Statuses: wishlist, owned, reading, paused, read, dropped (shown as "Abandoned"; Goodreads called it abandoned). All six stay. Owned books sit on the Reading tab ("On your shelf, not started"), not the wishlist.
- Reads: a book can be read many times, and each read has a date finished (day precision). 1,345 older reads have no date (imported history).
- Rating (0–5 stars) belongs to the book, not to each read.
- Open Library supplies book metadata. A server-side resolver turns a URL, ISBN, Bluesky post or text into candidate books and flags ones already in the library.
- Tags exist in the data but are hidden in the UI for now. They may return later.
- Notes exist but are secondary.
- No covers. Barcode scanning was considered and rejected.
- Stack: a Go server serving a plain-JS PWA (no build step, no framework), with IndexedDB on the client and SQLite on the server.

## Evidence on Hand

The real library, about 2,100 books, is in the production database (local backups: `forage-backup-*.db`). Use it for realistic content. Don't invent sample books.

## Product Principles

1. **Task first, fast.** Every common job is one or two taps from opening the app. Speed beats decoration.
2. **Works offline.** Everything except adding from a link must work with no signal.
3. **History is sacred.** Never lose a read, a date, or a rating. Make it easy to fill in missing dates.
4. **One person, no ceremony.** No accounts, onboarding, social features, or engagement mechanics.
5. **Search is the front door.** "Have I read this?" must be answerable instantly from anywhere in the app.

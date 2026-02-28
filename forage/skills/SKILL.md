---
name: forage
description: Use when discussing books to read, managing a reading list, getting book recommendations, or browsing the book library
---

# forage

Conversational wrapper around the `forage` CLI for managing a personal book library.

## Orientation (always first)

1. Run `forage prime` to load the library
2. If `$ARGUMENTS`: `forage search <args>` — match → work on book, no match → offer to add
3. No arguments → present library overview, ask what the user wants to do

## Recommending Books

When the user asks for book recommendations:

1. `forage prime` to understand their library — what they've read, their ratings, their tags
2. Ask about their mood, what they're looking for, or what they've enjoyed recently
3. Make recommendations based on their reading history and preferences
4. Offer to add recommendations to their wishlist: `forage add "<title>" --author "<name>" --tag <genre>`

## Working with Books

- `forage show <id>` — full details including notes
- `forage set <id> <key> <value>` — update metadata (title, author, status, rating, tags, date_read)
- `forage read <id>` — mark as read with today's date
- `forage drop <id>` — mark as dropped

## Adding Books

```
forage add "<title>" --author "<name>" [--tag <tag>] [--rating <1-5>] [--status <status>] [--body "<notes>"]
```

Tags are repeatable: `--tag sci-fi --tag classic`

Default status is `wishlist`.

## Querying

- `forage list` — all non-dropped books
- `forage list --status wishlist` — just the wishlist
- `forage list --tag <tag>` — filter by tag
- `forage list --author "<name>"` — filter by author
- `forage search <query>` — full-text search across title, author, tags, notes
- `forage prime` — compact JSON snapshot (for orientation, not display)

## Statuses

Valid statuses: `wishlist`, `reading`, `read`, `dropped`. No others accepted.

## Portable View

- `forage export` — generates a self-contained HTML file for phone/offline use
- `forage export -o ~/Desktop/books.html` — custom output path

## Principles

- **Match the user's energy** — a quick "add Dune" is not a book report
- **Recommend thoughtfully** — use their reading history, ratings, and tags to understand taste
- **Don't over-organize** — only add tags/ratings if the user cares about them

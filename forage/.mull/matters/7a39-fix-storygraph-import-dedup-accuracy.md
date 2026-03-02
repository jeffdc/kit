---
status: raw
tags: [import, forage]
created: 2026-03-01
updated: 2026-03-01
---

# Fix StoryGraph import dedup accuracy

The Goodreads/StoryGraph CSV import dedup still reports too many "new" books from StoryGraph after a Goodreads import. Title normalization was improved (stripping subtitles, series parentheticals) but 371 new from StoryGraph still seemed high.

## What's needed

- Investigate why so many StoryGraph books aren't matching Goodreads entries
- May need fuzzy matching or author normalization in addition to title normalization
- Run the actual StoryGraph import once dedup is trustworthy
- Consider a dry-run diff view that makes it easy to spot false negatives

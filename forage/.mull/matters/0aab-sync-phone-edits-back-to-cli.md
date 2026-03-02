---
status: raw
tags: [sync, forage]
created: 2026-03-01
updated: 2026-03-01
needs: [f424]
---

# Sync phone edits back to CLI

Design and implement a way to sync edits made on the phone (in the PWA) back to the CLI's SQLite database.

## Ideas discussed

- Export the .sqlite file from the browser and diff/merge with the CLI's copy
- Track changes as a log of mutations that can be replayed
- Keep it simple — the CLI is the source of truth, phone edits are secondary

## Dependencies

Depends on the PWA being built first. The sync strategy should be informed by how the PWA stores and modifies data.

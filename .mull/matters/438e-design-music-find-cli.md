---
status: raw
tags: [design]
created: 2026-08-20
updated: 2026-08-20
---

# Design music-find CLI

# music-find design

## Goal
Provide a fast CLI answer to whether an artist or album is present in beets, the organized NAS library, or the unorganized NAS archive.

## Command and query semantics

    music-find QUERY

Accept exactly one non-empty positional query. The shell handles quoting; the program treats the resulting string as a Unicode-aware, case-insensitive literal substring. Regex, glob, beets-query, and shell metacharacters have no meaning inside the program. `--help` is supported; extra positional arguments are rejected.

## Configuration
Read `$XDG_CONFIG_HOME/music-find/config.toml`, defaulting to `~/.config/music-find/config.toml`:

    library_root = "/Volumes/music/library"
    archive_root = "/Volumes/music"

`archive_root` is also the required NAS mount point. Before any search, require `os.path.ismount(archive_root)` and require `library_root` to be an accessible directory beneath it. If either check fails, print an explicit mount/retry error and exit 2 without invoking beets or starting a partial search.

## Search flow
1. Search beets and `library_root` concurrently.
2. Invoke `beet list --album` with a controlled output format, enumerate album artist/title/path, and filter in Python. Never pass the user query into beets' query parser.
3. Recursively inspect directory names under `library_root`. Match each directory's name with `casefold()` literal substring semantics. Do not follow symlinks.
4. For each matching beets album, require its path to exist and resolve beneath `library_root`. Report missing or out-of-root records as invalid.
5. If neither beets nor the organized-library scan matches, walk `archive_root`, pruning the entire `library_root` subtree. Otherwise skip the archive scan.
6. Match NAS directory names only, but report relative paths for context. Never enumerate or report individual track files.

## Output
Group all results under `BEETS`, `NAS LIBRARY`, and `NAS ARCHIVE`. Print all matching beets albums and all matching NAS directories. A beets entry includes artist, album, and one album path. Each section reports `FOUND`, `NOT FOUND`, `SKIPPED`, `INVALID`, or `ERROR` as applicable. A complete miss ends with a concise not-found message.

Exit codes:
- 0: at least one valid beets or NAS match
- 1: all searches completed and nothing matched
- 2: mount, configuration, beets, traversal, or beets-path-integrity failure

Independent source failures should not suppress safely obtainable results after preflight, but any partial/integrity failure takes exit-code precedence.

## Performance
Use only Python 3.12 standard-library facilities plus the installed beets 2.8 CLI. Run beets and organized NAS searches concurrently. Walk directories without following symlinks; prune the organized library from archive fallback. Add no cache or index initially. Benchmark a known hit and full miss on the real mount. Consider `fd` only after measurement; if archive misses remain slow, prefer a periodically refreshed directory index over increasingly complex live traversal.

## Verification
Use temporary directory trees and a fake `beet` executable for deterministic tests covering literal special characters, Unicode/case-insensitive matching, archive pruning and fallback, mount/config errors, stale or out-of-root beets paths, source failures, and exit codes. Final smoke tests use the installed `beet` and mounted configured roots without modifying either.


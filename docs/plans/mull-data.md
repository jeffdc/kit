mull's reads stay fast as the record grows. Today every request re-reads and re-parses the whole mull branch, done work is loaded and shown everywhere, and what changed since the human last looked costs more the longer they were away. This plan makes cost follow open work and change, not history.

Done when mull serve's pages answer well under 100ms on a repository many times kit's size, done work ages out of the way without leaving the record, and the human can find any record by ID or title.

## What we measured (2026-11-11, kit: 432 files, 306 commits)

- Reading the whole branch with git: 35ms. Pages in-process: tree 54ms, overview 108-176ms. The same pages from the installed server: 250ms to 1.2s.
- The installed server runs as a macOS Background process (fa47), which throttles its CPU and I/O and its git children. That's most of the gap.
- Nothing is cached: every request re-runs ls-tree, cat-file and a YAML parse of every file, though the mull ref's commit (written by compare-and-swap) would be an exact cache key.
- Done and dropped records are parsed and shown everywhere (1f0b).
- Revision loads two full states and walks the git log between them, so it grows with the time since the last look.
- First load fires the project list (a summary of every repository), the unmulled scan, the overview, the tree and revision at once, each doing its own full load.

## Foundations

- **The commit is the key.** Anything derived from the branch (parsed records, indexes, summaries, search) is computed once per mull commit and reused until the ref moves. A request costs one rev-parse when nothing changed.
- **State lives on the server.** The cache is mull serve's in-memory state, not files on disk or on the branch. The CLI keeps loading directly; it's one load per process and already fast.
- **Measure first, change storage last.** The on-branch format changes only if the numbers at scale say the cache and aging out aren't enough. A format change means a migration.

## Phases, and why this order

1. **Measure.** A seeded large repository and a benchmark of every page, in-process and over HTTP, with git calls counted. First so every later phase shows its gain.
2. **Read cache.** Fix the Background throttling, cache the parsed state and its indexes per commit, and share the project summaries and the unmulled scan. The base the rest builds on.
3. **Done work ages out.** Done and dropped work ages out of the views and of what's loaded by default, still in the record and reachable.
4. **Cheap revision.** What changed since the last look, computed from cached states instead of two full loads and a log walk.
5. **Search.** An index over the cached state, behind the s palette.

Out of scope: UI state (tree expansion, filters, scroll; 28c1, 18d6), and the backlog view's tools (mull-backlog-ui).

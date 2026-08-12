# cd2beets Design

## Purpose

`cd2beets` is a foreground macOS command-line tool for processing a one-time backlog of roughly 200 audio CDs. The operator starts it manually. It waits for a CD, identifies the release, checks whether the complete album is already in the configured beets library, and either ejects the disc or rips, imports, verifies, cleans up, and ejects it. It then waits for the next disc until interrupted.

The implementation will live in `~/dev/kit/cd2beets`.

## Requirements

For each inserted audio CD, the tool must:

1. Detect the disc without requiring a per-disc command.
2. Read its table of contents and look up MusicBrainz metadata.
3. Determine whether that release and all expected tracks are already present in beets and on the configured library filesystem.
4. Ask the operator whenever that determination is uncertain.
5. Rip absent discs to constant 320 kbps MP3 files.
6. Import with beets using copy semantics, even when the global beets configuration requests move semantics.
7. Verify the complete album in beets and every corresponding file on the configured NAS library root.
8. Delete only MP3 files created by this tool and only after verification succeeds.
9. Eject after a confirmed existing-album skip or a successful import and cleanup.
10. Before cleanup begins, preserve all generated MP3s and keep the disc loaded after an operational failure; if cleanup itself fails, preserve every remaining generated MP3 and keep the disc loaded.
11. Continue waiting for the next disc after a successful eject.

The tool must never invoke a beets deletion operation. It must never delete a directory, journal, log, intermediate non-MP3 file, unrelated file, or MP3 it did not create.

## Selected Approach

Use a Python 3.12 orchestrator with narrow external dependencies:

- macOS `drutil` for drive status and ejection;
- Homebrew `libdiscid` plus Python `discid` for MusicBrainz disc identification;
- the public MusicBrainz service at `https://musicbrainz.org` for release metadata;
- the beets Python API for read-only library inspection;
- Homebrew `libcdio-paranoia` (`cd-paranoia`) for verified audio extraction;
- LAME for constant 320 kbps MP3 encoding;
- the `beet` CLI for one fixed, copy-only import operation.

This provides explicit state transitions and safety gates while delegating CD extraction, encoding, metadata, and library management to established tools.

### Rejected Alternatives

- **`abcde` wrapper:** less custom rip orchestration, but Homebrew marks it deprecated and unmaintained. Its combined lookup/rip/tag pipeline makes pre-rip library checks and post-import safety proof harder to control.
- **XLD automation:** a mature macOS ripper, but GUI/profile automation and output parsing are less deterministic for a 200-disc foreground batch and do not simplify beets verification.

## Architecture

The package is divided into small adapters coordinated by a workflow state machine:

- **Drive adapter:** reports no media, audio media, or an unusable disc; reads the physical drive identifier; ejects only when requested by a successful or operator-approved transition.
- **Disc metadata adapter:** reads the disc TOC and MusicBrainz disc ID, queries `https://musicbrainz.org`, and normalizes release candidates and expected tracks.
- **Library adapter:** reads the active beets configuration and library, classifies candidates, and verifies post-import state. It exposes no mutation method.
- **Ripper adapter:** streams one selected audio track from `cd-paranoia` into LAME and writes one exclusively created MP3.
- **Importer adapter:** invokes only the fixed beets import command described below.
- **Filesystem safety adapter:** owns generated-path registration, local staging checks, NAS path checks, and the narrowly guarded MP3 unlink operation.
- **Journal:** appends structured lifecycle events without truncating or deleting previous events.
- **Workflow coordinator:** implements preflight, waiting, selection, classification, ripping, import, verification, cleanup, failure pause, and eject transitions.
- **Terminal UI:** prints the current phase, evidence, and conservative prompts.

External commands are executed as argument vectors without a shell.

## macOS Music Insertion Handling

Before the first batch run, the operator must open **Music > Settings > General** and set **When a CD is inserted** to **Show CD**, using the non-importing option available in this macOS version and documented in the [Music General settings guide](https://support.apple.com/guide/music/change-general-settings-mus1719/mac). The operator must not select **Import CD** or **Import CD and Eject**.

`cd2beets` must not incorporate Music's importer. That would introduce a second managed library, Music-controlled encoding and destinations, and no reliable copy-verification boundary. The tool also must not write undocumented Music preference keys, quit Music, or terminate it automatically.

Startup prints the one-time **Show CD** prerequisite. Music may open and display the inserted disc, but it must not play, import, or eject it. If disc identification or extraction reports that the drive is busy and the Music process is running, the tool pauses, explains the likely contention, and asks the operator to close Music manually or abort. It does not change Music or retry automatically.

## Foreground Workflow

### 1. Preflight

Preflight runs at startup and again before processing each detected disc.

It must verify:

- required binaries and Python modules are available;
- beets is version-compatible with the library API used by the tool;
- the active beets configuration uses `musicbrainz.host: musicbrainz.org`, and the public MusicBrainz service is available before metadata lookup;
- the configured beets library database is readable;
- the configured library directory exists, is writable, and is on a mounted filesystem distinct from the Mac root filesystem;
- the local staging root is not within the beets library root;
- active beets plugins and import configuration cannot delete, move, or replace existing library files through the tool's import invocation;
- exactly one optical drive is selected.

The current beets configuration has `directory: /Volumes/music/library` and `import.move: yes`. The one-time setup must set `musicbrainz.host: musicbrainz.org`, `musicbrainz.ratelimit: 1`, and `musicbrainz.ratelimit_interval: 1` so both direct metadata lookup and `beet import --search-id` use the public service without exceeding one request per second. The tool must discover the library and import values dynamically and reject any non-public MusicBrainz host. It must stop before ripping while `/Volumes/music/library` is unavailable. The import operation must explicitly override `import.move: yes` with copy semantics.

### 2. Wait for a Disc

The process polls `drutil status` at a short fixed interval. No-media status is normal and causes no mutation. After a successful eject, the coordinator waits until the drive reports no media before accepting another insertion event.

Normal mode loops until interrupted. `--once` processes at most one detected disc and then exits, which supports troubleshooting and acceptance testing.

### 3. Identify and Select Metadata

For an audio CD, the tool reads the TOC, computes the MusicBrainz disc ID, and queries the public MusicBrainz service over HTTPS.

- One complete, exact release candidate is selected automatically.
- Multiple release candidates require an operator selection.
- Missing or incomplete release metadata requires an operator decision; the tool never invents metadata.

The terminal shows release ID, album artist, album, date or edition details, medium position, and expected audio track count. The selected MusicBrainz release ID and expected track identities become the contract for library classification, import, and verification.

### 4. Classify the Existing Library

Classification has exactly three outcomes.

#### Definitely present

All of the following must be true:

- an album has the exact selected MusicBrainz release ID;
- it contains one unique item for every expected track on the selected medium;
- track identities and track count match the metadata contract;
- each beets item has a file path;
- each path exists and resolves beneath the configured library root.

The tool reports the evidence, ejects the CD, and returns to waiting.

#### Definitely absent

There is no exact release match and no plausible album candidate based on normalized album artist, album title, track count, and available MusicBrainz identifiers. The tool may proceed to ripping without an additional presence prompt.

#### Uncertain

Any other state is uncertain, including:

- an exact release with missing or duplicate tracks;
- missing files for otherwise matching beets items;
- a matching artist/title with absent MusicBrainz IDs;
- a plausible different edition or release ID;
- split albums, inconsistent track totals, or conflicting candidates.

The tool displays the exact evidence and asks the operator to choose:

- `skip/eject`;
- `proceed` with the selected release;
- `abort/keep loaded`.

The conservative default is `skip/eject`. No uncertain state is silently treated as absent or allowed to trigger a rip without an explicit operator decision.

### 5. Rip to Local MP3s

Each rip has a unique run ID. Generated tracks are placed beneath the configurable local staging root, which defaults to `~/Music/cd2beets-staging`. The staging root is private to the current user, and each created track's device and inode are recorded as part of its ownership evidence.

For each expected audio track:

1. Reserve a unique destination path with exclusive-create semantics so existing content can never be overwritten.
2. Record that exact path as owned by the current run in memory and in the append-only journal.
3. Stream the selected track from `cd-paranoia` in full verification mode with abort-on-skip into LAME.
4. Encode constant bitrate MP3 audio with an explicit 320 kbps setting and write the selected release's title, album artist, album title, year when known, and track number/total as ID3v2 metadata so beets can evaluate the fixed release match non-interactively.
5. Treat any extractor or encoder failure as a hard workflow failure.

No WAV intermediate is created. A failed or interrupted track remains preserved as a generated MP3 artifact and is never considered importable until a later explicit recovery flow verifies it.

### 6. Import with Beets

The importer constructs a fixed command equivalent in intent to:

```text
beet import --copy --quiet --quiet-fallback skip --noresume --noincremental --search-id <release-id> <run-staging-path>
```

Exact spellings will be confirmed against the installed beets 2.8.0 CLI during implementation. The selected release ID has already been confirmed by the operator or metadata rules. Non-interactive import prevents a beets duplicate menu from exposing destructive choices.

The importer must not expose an arbitrary beets subcommand or arbitrary additional arguments. It must never invoke `remove`, destructive duplicate actions, move import, or any plugin operation that deletes or replaces existing files.

A successful process exit is necessary but insufficient. The tool proceeds to independent verification.

### 7. Verify Beets and NAS State

Post-import verification reopens or refreshes the beets library and requires:

- exactly one complete logical album for the selected release and medium;
- one unique item for every expected track identity;
- no missing or duplicate expected track;
- every MP3 registered to the current run still present in local staging immediately after import, proving beets copied rather than moved the source;
- all paths resolving under the configured beets library root, never under staging;
- every destination path present as a regular, non-empty file;
- every destination recognized as MP3 with a bitrate consistent with 320 kbps;
- no expected track represented only by a local staging path.

The tool records pre-import library evidence and post-import evidence so a skipped or ineffective import cannot be mistaken for a verified copy. Any mismatch fails closed.

### 8. Clean Up and Eject

Cleanup is allowed only after the complete verification gate passes.

For each cleanup candidate, all conditions must hold at deletion time:

- the exact path belongs to the current run's generated-track set;
- the path is beneath the configured local staging root;
- the path has an `.mp3` suffix;
- `lstat` identifies a regular file, not a symlink;
- its device and inode match the values recorded when the tool exclusively created it;
- the complete run verification remains successful.

Before the first unlink, the tool validates every cleanup candidate; one failed guard means nothing is deleted. It then unlinks each MP3 and immediately journals the result. If an operating-system error causes partial cleanup, already removed MP3s have verified NAS copies, all remaining MP3s are preserved, and the workflow pauses with the disc loaded. The tool does not remove now-empty directories. After complete cleanup, it ejects with `drutil`, waits for no media, and resumes the foreground loop.

## Failure and Interruption Behavior

Ripping, metadata, import, verification, cleanup, or eject errors stop the current workflow. Before cleanup starts, the tool:

- preserves every generated MP3;
- preserves journals and logs;
- keeps the disc loaded unless the operator explicitly selects eject;
- shows the failing command or invariant without hiding stderr;
- offers only narrowly scoped recovery actions such as retrying verification/import, explicitly ejecting, or quitting.

If cleanup has already started, the partial-cleanup behavior defined above applies.

`Ctrl-C` preserves staging and leaves the disc loaded. There is no automatic retry. A later recovery must re-establish ownership from the append-only journal and pass the same verification gate before deleting any generated MP3.

## Journal

The journal is append-only JSON Lines under the staging root. Events include:

- run and disc IDs;
- release candidates and selected release;
- library classification and supporting evidence;
- each reserved/generated MP3 path;
- rip command outcome per track;
- import command and outcome;
- post-import verification evidence;
- each permitted MP3 cleanup result;
- eject outcome.

The tool never truncates, rotates, rewrites, or deletes the journal. It contains no credentials.

## Command-Line Interface

The primary interface is:

```text
.venv/bin/cd2beets [--staging-dir PATH] [--poll-interval SECONDS] [--once]
```

Defaults:

- foreground continuous mode;
- staging at `~/Music/cd2beets-staging`;
- a short, non-busy polling interval;
- active beets library/import configuration, with the public MusicBrainz endpoint fixed as described above;
- conservative terminal prompt defaults.

The implementation must not include a general-purpose cleanup command, arbitrary beets passthrough, daemon installation, launchd agent, GUI, cover-art workflow, lossless archive, or multi-drive orchestration.

## Dependencies and Setup

The implementation may install and use:

- Homebrew `libcdio-paranoia`;
- Homebrew `libdiscid`;
- the already installed LAME and beets CLI;
- a project-local Python virtual environment containing a beets version compatible with 2.8.0 and `python-discid`.

Setup is explicit and one-time. It includes selecting **Show CD** in Music as described above and setting `musicbrainz.host: musicbrainz.org`, `musicbrainz.ratelimit: 1`, and `musicbrainz.ratelimit_interval: 1` in the active beets configuration. Runtime preflight reports missing dependencies, rejects a different MusicBrainz host, and reminds the operator about the Music setting, but it does not install packages or change application preferences automatically.

## Test and Verification Strategy

### Automated tests

Use fake adapters to test observable workflow behavior without a physical CD, NAS mutation, or real beets mutation:

- no-media waiting and `--once` behavior;
- exact, partial, fuzzy, conflicting, and missing library classification;
- metadata candidate selection and ambiguity prompts;
- extractor-to-encoder streaming arguments and constant 320 kbps configuration;
- fixed copy-only beets command construction;
- import failure and nonzero exit behavior;
- post-import item, path, format, bitrate, and track completeness checks;
- generated-path ownership, staging-root, suffix, regular-file, and symlink deletion guards;
- failure and interruption transitions preserving artifacts and keeping media loaded;
- successful present-skip and import paths ejecting and returning to the wait state.

Contract tests cover parsing against captured, sanitized outputs from the installed command versions. Tests must not validate source text or implementation plumbing.

### Local no-media smoke test

With no CD inserted, start the foreground watcher, observe the waiting state, and interrupt it to prove:

- preflight results are accurate;
- the HP USB drive is detected;
- no-media status causes no mutation;
- no beets import or file cleanup operation runs.

### Hardware smoke test

Insert one test CD and prove:

- `drutil`, `libdiscid`, and `cd-paranoia` can address the current HP CDDVDW TS-L633R drive;
- Music may open and display the inserted CD but does not play, import, or eject it;
- `cd2beets` retains access to the drive without terminating Music;
- TOC and MusicBrainz disc ID agree on track count;
- metadata candidates are displayed correctly;
- every selected track streams through LAME to a playable 320 kbps MP3.

`drutil list` currently reports the drive support level as `Unsupported`, although `drutil status` detects the drive. The hardware smoke test is therefore mandatory before claiming the ripping path works.

### End-to-end NAS acceptance

With `/Volumes/music/library` mounted and a CD known to be absent:

1. Record the selected MusicBrainz release and all generated staging paths.
2. Rip and import through the normal tool workflow.
3. Query beets for the selected release and expected tracks.
4. Confirm every destination exists beneath `/Volumes/music/library` as a non-empty 320 kbps MP3.
5. Confirm only the current run's generated local MP3s were removed.
6. Confirm journals and unrelated files remain.
7. Confirm the CD ejected and the process returned to waiting or exited under `--once`.

A second insertion of the same CD must classify it as definitely present, perform no rip or import, and eject it.

## Acceptance Criteria

The design is complete when implementation evidence demonstrates:

- automatic foreground detection for successive inserted CDs;
- MusicBrainz lookup and explicit handling of ambiguous metadata;
- fail-closed beets classification with operator input for every uncertain presence result;
- direct 320 kbps MP3 ripping without non-MP3 intermediates;
- explicit copy-only beets import despite global move configuration;
- complete post-import beets and NAS verification;
- deletion limited to verified, regular `.mp3` files owned by the current tool run;
- artifact preservation and loaded-disc pause on failure;
- automatic eject after confirmed skip or fully verified success;
- a successful real-disc, real-NAS end-to-end run and a subsequent already-present skip run.

# cd2beets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a foreground macOS CLI that detects inserted audio CDs, resolves MusicBrainz metadata, safely checks a beets library, rips missing discs to 320 kbps MP3, imports by copy to the NAS, verifies the result, deletes only tool-owned MP3s, and ejects.

**Architecture:** A small Python package coordinates narrow adapters for subprocesses, `drutil`, libdiscid, MusicBrainz HTTP, read-only beets access, staging/journaling, ripping, import, verification, and terminal decisions. Pure domain functions own classification and verification; side effects sit behind injectable adapters so the complete workflow can be tested without a CD, NAS, or real beets mutation.

**Tech Stack:** Python 3.12, beets 2.8.0, python-discid 1.4.2/libdiscid, `libcdio-paranoia`, LAME, macOS `drutil`, stdlib `urllib`, pytest.

## Global Constraints

- Implementation root is `~/dev/kit/cd2beets`.
- Runtime is macOS with Python `>=3.12,<3.13`.
- The operator manually sets **Music > Settings > General > When a CD is inserted > Show CD**; the tool never changes Music preferences or terminates Music, and Music must not play, import, or eject the disc.
- The tool runs in the foreground and supports continuous processing plus `--once`.
- Default staging is `~/Music/cd2beets-staging`; staging must not be beneath the beets library root.
- The active beets library/import configuration is discovered dynamically; both direct lookup and beets import must use the public `musicbrainz.org` service, with beets limited to one request per second.
- The configured NAS library root must exist, be writable, and be on a mounted filesystem distinct from `/` before ripping begins.
- The current beets configuration has `import.move: yes`; every tool import must explicitly use `--copy`.
- The only mutating beets command is the fixed copy-only `beet import`; no arbitrary beets command or arguments are accepted.
- Presence is exactly `PRESENT`, `ABSENT`, or `UNCERTAIN`; every uncertain result requires an operator decision and defaults to `skip/eject`.
- Audio extraction streams directly from verified `cd-paranoia` into LAME; no WAV intermediate is created.
- Encoding is constant 320 kbps MP3.
- The tool never deletes a directory, journal, log, non-MP3, unrelated file, beets item, or MP3 it did not exclusively create and record.
- Cleanup requires complete post-import beets/NAS verification plus path, suffix, regular-file, device, and inode checks for every candidate before the first unlink.
- Any pre-cleanup failure preserves all generated MP3s and keeps the disc loaded. A partial-cleanup failure preserves all remaining MP3s and keeps the disc loaded.
- Successful existing-album skips and successful verified imports eject the disc; continuous mode then waits for the next insertion.
- Do not add daemon installation, launchd support, a GUI, arbitrary cleanup, cover art, lossless output, or multi-drive orchestration.

## File Map

- `pyproject.toml` — package metadata, pinned runtime dependencies, pytest setup, console entry point.
- `.gitignore` — project-local Python and virtual-environment artifacts.
- `src/cd2beets/__init__.py` — package version.
- `src/cd2beets/models.py` — immutable domain values and enums shared across adapters.
- `src/cd2beets/process.py` — shell-free command and two-process pipeline execution.
- `src/cd2beets/drive.py` — `drutil` parsing, status, selected drive, and eject.
- `src/cd2beets/metadata.py` — libdiscid TOC reader, MusicBrainz HTTP lookup, release normalization.
- `src/cd2beets/library.py` — beets configuration loading, read-only catalog snapshots, presence classification.
- `src/cd2beets/journal.py` — append-only JSONL lifecycle journal.
- `src/cd2beets/staging.py` — private run directories, exclusive MP3 reservation, ownership checks, guarded cleanup.
- `src/cd2beets/ripper.py` — verified cd-paranoia-to-LAME track streaming.
- `src/cd2beets/importer.py` — fixed copy-only beets import command.
- `src/cd2beets/verification.py` — post-import beets, staging, format, bitrate, path, and copy verification.
- `src/cd2beets/ui.py` — terminal rendering and conservative decisions.
- `src/cd2beets/workflow.py` — one-disc and continuous state transitions.
- `src/cd2beets/preflight.py` — dependency, configuration, NAS mount, plugin, drive, and MusicBrainz checks.
- `src/cd2beets/cli.py` — argument parsing, composition root, exit codes, and interrupt handling.
- `tests/` — one focused test module per production module plus workflow tests.
- `tests/fixtures/` — sanitized `drutil` and MusicBrainz contract responses.

---

### Task 1: Package Foundation and Domain Contracts

**Files:**
- Create: `pyproject.toml`
- Create: `.gitignore`
- Create: `src/cd2beets/__init__.py`
- Create: `src/cd2beets/models.py`
- Create: `tests/test_models.py`

**Interfaces:**
- Consumes: none.
- Produces: `MediaState`, `Presence`, `OperatorDecision`, `FailureDecision`, `DiscToc`, `TrackMetadata`, `ReleaseMetadata`, `LibraryItem`, `LibrarySnapshot`, `Classification`, `OwnedMp3`, `VerificationReport`, and `normalize_text()`.

- [ ] **Step 1: Install machine prerequisites and create the project environment**

Run:

```bash
brew install libcdio-paranoia libdiscid
python3 -m venv .venv
```

Expected: Homebrew reports `libcdio-paranoia` and `libdiscid` installed; `.venv/bin/python --version` reports Python 3.12.x. Do not install anything automatically from runtime code.

- [ ] **Step 2: Create package metadata and ignores**

Create `pyproject.toml`:

```toml
[build-system]
requires = ["setuptools>=75"]
build-backend = "setuptools.build_meta"

[project]
name = "cd2beets"
version = "0.1.0"
description = "Safely rip audio CDs into a beets library"
requires-python = ">=3.12,<3.13"
dependencies = [
  "beets==2.8.0",
  "discid==1.4.2",
]

[project.optional-dependencies]
test = ["pytest>=8,<10"]

[project.scripts]
cd2beets = "cd2beets.cli:main"

[tool.setuptools.packages.find]
where = ["src"]

[tool.pytest.ini_options]
addopts = "-ra"
testpaths = ["tests"]
```

Create `.gitignore`:

```gitignore
.venv/
.pytest_cache/
__pycache__/
*.py[cod]
```

Create `src/cd2beets/__init__.py`:

```python
__version__ = "0.1.0"
```

Install the editable package:

```bash
.venv/bin/pip install -e '.[test]'
```

Expected: exit 0; `.venv/bin/python -c 'import beets, discid, cd2beets'` exits 0.

- [ ] **Step 3: Write failing domain-model tests**

Create `tests/test_models.py` with tests that require normalization, track lookup, and immutable verification values:

```python
from pathlib import Path

from cd2beets.models import (
    ReleaseMetadata,
    TrackMetadata,
    VerificationReport,
    normalize_text,
)


def test_normalize_text_is_case_accent_and_punctuation_insensitive():
    assert normalize_text("Beyoncé — Live!") == "beyonce live"


def test_release_exposes_expected_recording_ids():
    release = ReleaseMetadata(
        release_id="release-1",
        albumartist="Artist",
        album="Album",
        date="2001",
        country="US",
        disambiguation="",
        medium=1,
        tracks=(
            TrackMetadata(1, "track-1", "One", 120_000),
            TrackMetadata(2, "track-2", "Two", 180_000),
        ),
    )
    assert release.recording_ids == ("track-1", "track-2")


def test_verification_report_is_immutable_and_boolean_by_errors():
    report = VerificationReport(errors=("missing track",), destination_paths=(Path("/nas/a.mp3"),))
    assert report.ok is False
```

- [ ] **Step 4: Run the tests to verify they fail**

Run:

```bash
.venv/bin/pytest tests/test_models.py -v
```

Expected: FAIL during import because `cd2beets.models` does not exist.

- [ ] **Step 5: Implement the domain values**

Create `src/cd2beets/models.py` with frozen dataclasses and string enums. Use these exact public fields:

```python
from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
import re
import unicodedata


class MediaState(StrEnum):
    NO_MEDIA = "no_media"
    AUDIO = "audio"
    UNUSABLE = "unusable"


class Presence(StrEnum):
    PRESENT = "present"
    ABSENT = "absent"
    UNCERTAIN = "uncertain"


class OperatorDecision(StrEnum):
    SKIP_EJECT = "skip_eject"
    PROCEED = "proceed"
    ABORT_KEEP_LOADED = "abort_keep_loaded"


class FailureDecision(StrEnum):
    RETRY_IMPORT = "retry_import"
    RETRY_VERIFY = "retry_verify"
    EJECT = "eject"
    QUIT = "quit"


def normalize_text(value: str) -> str:
    decomposed = unicodedata.normalize("NFKD", value.casefold())
    ascii_text = "".join(char for char in decomposed if not unicodedata.combining(char))
    return " ".join(re.findall(r"[a-z0-9]+", ascii_text))


@dataclass(frozen=True)
class DiscToc:
    disc_id: str
    device: str
    first_track: int
    last_track: int
    sectors: int

    @property
    def track_count(self) -> int:
        return self.last_track - self.first_track + 1


@dataclass(frozen=True)
class TrackMetadata:
    position: int
    recording_id: str
    title: str
    length_ms: int | None


@dataclass(frozen=True)
class ReleaseMetadata:
    release_id: str
    albumartist: str
    album: str
    date: str
    country: str
    disambiguation: str
    medium: int
    tracks: tuple[TrackMetadata, ...]

    @property
    def recording_ids(self) -> tuple[str, ...]:
        return tuple(track.recording_id for track in self.tracks)


@dataclass(frozen=True)
class LibraryItem:
    item_id: int
    release_id: str
    recording_id: str
    albumartist: str
    album: str
    medium: int
    track: int
    tracktotal: int
    path: Path
    format: str
    bitrate: int


@dataclass(frozen=True)
class LibrarySnapshot:
    exact_items: tuple[LibraryItem, ...]
    plausible_items: tuple[LibraryItem, ...]


@dataclass(frozen=True)
class Classification:
    presence: Presence
    reasons: tuple[str, ...]
    snapshot: LibrarySnapshot


@dataclass(frozen=True)
class OwnedMp3:
    path: Path
    device: int
    inode: int
    track_position: int


@dataclass(frozen=True)
class VerificationReport:
    errors: tuple[str, ...]
    destination_paths: tuple[Path, ...]

    @property
    def ok(self) -> bool:
        return not self.errors
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
.venv/bin/pytest tests/test_models.py -v
```

Expected: 3 passed.

Commit:

```bash
git add cd2beets/.gitignore cd2beets/pyproject.toml cd2beets/src/cd2beets/__init__.py cd2beets/src/cd2beets/models.py cd2beets/tests/test_models.py
git commit -m "Add cd2beets domain foundation"
```

---

### Task 2: Shell-Free Process Execution

**Files:**
- Create: `src/cd2beets/process.py`
- Create: `tests/test_process.py`

**Interfaces:**
- Consumes: standard `Sequence[str]` argument vectors and a binary destination handle.
- Produces: `CommandResult`, `PipelineResult`, `ProcessRunner.run(args, timeout=None)`, and `ProcessRunner.pipeline(source_args, sink_args, destination)`.

- [ ] **Step 1: Write failing process-runner tests**

Create `tests/test_process.py`:

```python
from pathlib import Path

from cd2beets.process import ProcessRunner


def test_run_captures_output_without_a_shell():
    result = ProcessRunner().run(["/usr/bin/printf", "%s", "hello"])
    assert result.returncode == 0
    assert result.stdout == "hello"
    assert result.stderr == ""


def test_pipeline_streams_source_through_sink(tmp_path: Path):
    destination = tmp_path / "pipeline.mp3"
    with destination.open("xb") as output:
        result = ProcessRunner().pipeline(
            ["/usr/bin/printf", "abc"],
            ["/usr/bin/tr", "a-z", "A-Z"],
            output,
        )
    assert result.source_returncode == 0
    assert result.sink_returncode == 0
    assert destination.read_bytes() == b"ABC"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `.venv/bin/pytest tests/test_process.py -v`

Expected: FAIL because `cd2beets.process` does not exist.

- [ ] **Step 3: Implement process execution**

Create `src/cd2beets/process.py`. `run()` must call `subprocess.run` with `shell=False`, text capture, and no implicit check. `pipeline()` must start both processes with argument lists, connect source stdout to sink stdin, direct sink stdout to the caller-owned binary handle, close the parent copy of source stdout, drain both stderr pipes concurrently, wait for both children, and return decoded stderr without raising:

```python
from __future__ import annotations

from dataclasses import dataclass
from subprocess import PIPE, Popen, run
from threading import Thread
from typing import BinaryIO, Sequence


@dataclass(frozen=True)
class CommandResult:
    returncode: int
    stdout: str
    stderr: str


@dataclass(frozen=True)
class PipelineResult:
    source_returncode: int
    sink_returncode: int
    source_stderr: str
    sink_stderr: str

    @property
    def ok(self) -> bool:
        return self.source_returncode == 0 and self.sink_returncode == 0


class ProcessRunner:
    def run(self, args: Sequence[str], timeout: float | None = None) -> CommandResult:
        completed = run(
            list(args),
            capture_output=True,
            text=True,
            timeout=timeout,
            shell=False,
            check=False,
        )
        return CommandResult(completed.returncode, completed.stdout, completed.stderr)

    def pipeline(
        self,
        source_args: Sequence[str],
        sink_args: Sequence[str],
        destination: BinaryIO,
    ) -> PipelineResult:
        source = Popen(list(source_args), stdout=PIPE, stderr=PIPE, shell=False)
        if source.stdout is None or source.stderr is None:
            raise RuntimeError("source process pipes unavailable")
        sink = Popen(
            list(sink_args),
            stdin=source.stdout,
            stdout=destination,
            stderr=PIPE,
            shell=False,
        )
        source.stdout.close()
        if sink.stderr is None:
            raise RuntimeError("sink process stderr unavailable")
        captured: dict[str, bytes] = {}

        def drain(name: str, pipe: BinaryIO) -> None:
            captured[name] = pipe.read()

        source_thread = Thread(target=drain, args=("source", source.stderr), daemon=True)
        sink_thread = Thread(target=drain, args=("sink", sink.stderr), daemon=True)
        source_thread.start()
        sink_thread.start()
        sink_returncode = sink.wait()
        source_returncode = source.wait()
        source_thread.join()
        sink_thread.join()
        return PipelineResult(
            source_returncode,
            sink_returncode,
            captured["source"].decode(errors="replace"),
            captured["sink"].decode(errors="replace"),
        )
```

- [ ] **Step 4: Run tests and commit**

Run: `.venv/bin/pytest tests/test_process.py -v`

Expected: 2 passed.

Commit:

```bash
git add cd2beets/src/cd2beets/process.py cd2beets/tests/test_process.py
git commit -m "Add shell-free process runner"
```

---

### Task 3: Optical Drive Adapter

**Files:**
- Create: `src/cd2beets/drive.py`
- Create: `tests/test_drive.py`
- Create: `tests/fixtures/drutil_list_one_drive.txt`
- Create: `tests/fixtures/drutil_status_no_media.txt`
- Create: `tests/fixtures/drutil_status_audio.txt`

**Interfaces:**
- Consumes: `ProcessRunner.run()`.
- Produces: `DriveInfo`, `parse_drive_list()`, `parse_media_state()`, and `DrutilDrive.list_drives()`, `.status()`, `.eject()`.

- [ ] **Step 1: Add captured fixtures**

Use the observed one-drive and no-media outputs:

```text
   Vendor   Product           Rev   Bus       SupportLevel
1  hp       CDDVDW TS-L633R   0200  USB       Unsupported
```

```text
 Vendor   Product           Rev 
 hp       CDDVDW TS-L633R   0200

           Type: No Media Inserted
```

Add a sanitized audio-state fixture whose decisive line is `Type: Audio CD`; Task 11 will replace or extend it only if the real inserted-disc output differs.

- [ ] **Step 2: Write failing parser and adapter tests**

Tests must assert:

```python
assert parse_drive_list(list_text) == (
    DriveInfo(1, "hp", "CDDVDW TS-L633R", "0200", "USB", "Unsupported"),
)
assert parse_media_state(no_media_text) is MediaState.NO_MEDIA
assert parse_media_state(audio_text) is MediaState.AUDIO
```

Use a fake runner to assert `.status()` executes `['drutil', 'status']`, `.eject()` executes `['drutil', 'eject']`, and nonzero eject raises `DriveError` with stderr preserved.

- [ ] **Step 3: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_drive.py -v`

Expected: FAIL because `cd2beets.drive` does not exist.

- [ ] **Step 4: Implement `drive.py`**

Implement:

```python
@dataclass(frozen=True)
class DriveInfo:
    index: int
    vendor: str
    product: str
    revision: str
    bus: str
    support_level: str

class DriveError(RuntimeError):
    pass
```

`parse_drive_list()` must parse only numbered rows and split the current fixed columns without treating the product's embedded space as a second field. `parse_media_state()` returns `NO_MEDIA` for `No Media Inserted`, `AUDIO` for `Type: Audio CD`, and `UNUSABLE` otherwise. `DrutilDrive` accepts a `ProcessRunner`, rejects zero or multiple selected drives, and never calls eject from `status()`.

- [ ] **Step 5: Run tests and commit**

Run: `.venv/bin/pytest tests/test_drive.py -v`

Expected: all drive tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/drive.py cd2beets/tests/test_drive.py cd2beets/tests/fixtures/drutil_list_one_drive.txt cd2beets/tests/fixtures/drutil_status_no_media.txt cd2beets/tests/fixtures/drutil_status_audio.txt
git commit -m "Add macOS optical drive adapter"
```

---

### Task 4: Disc ID and MusicBrainz Metadata

**Files:**
- Create: `src/cd2beets/metadata.py`
- Create: `tests/test_metadata.py`
- Create: `tests/fixtures/musicbrainz_disc.json`

**Interfaces:**
- Consumes: `DiscToc`, `TrackMetadata`, `ReleaseMetadata`, python-discid `discid.get_default_device()`/`discid.read(device)`, and the fixed public endpoint `https://musicbrainz.org`.
- Produces: `DiscidReader.default_device() -> str`, `DiscidReader.read(device) -> DiscToc`, `MusicBrainzClient.ping()`, `MusicBrainzClient.lookup(toc) -> tuple[ReleaseMetadata, ...]`, `MusicBrainzClient.parse_lookup_response(toc, payload)`, and `MetadataError`.

- [ ] **Step 1: Create a minimal sanitized MusicBrainz fixture and failing tests**

The fixture must contain two releases, but only one medium with a `discs[].id` matching `disc-1`. Its matching medium must contain two tracks with recording IDs, titles, positions, and lengths. Tests must assert:

```python
releases = client.parse_lookup_response(toc, payload)
assert len(releases) == 1
assert releases[0].release_id == "release-1"
assert releases[0].medium == 2
assert releases[0].recording_ids == ("recording-1", "recording-2")
```

Also test that a medium with missing recording IDs raises `MetadataError` rather than inventing metadata, that an explicit HTTPS endpoint uses TLS port 443, and that a host without a scheme remains compatible with HTTP mirrors.

- [ ] **Step 2: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_metadata.py -v`

Expected: FAIL because `cd2beets.metadata` does not exist.

- [ ] **Step 3: Implement default-device discovery and libdiscid reading**

`DiscidReader.default_device()` must call `discid.get_default_device()`, require a non-empty device string, and wrap libdiscid exceptions as `MetadataError`. `DiscidReader.read(device)` calls `discid.read(device)`, converts `.id`, `.first_track_num`, `.last_track_num`, and `.sectors` into `DiscToc`, and preserves the selected device in that value. `cd-paranoia` independently auto-detects the same sole optical drive; the matching TOC and track count are verified during hardware acceptance.

- [ ] **Step 4: Implement deterministic public MusicBrainz HTTPS lookup**

Use stdlib `urllib.request` with no write methods. `ping()` opens a TCP connection to `musicbrainz.org:443` with a two-second timeout. `lookup()` performs:

```text
GET https://musicbrainz.org/ws/2/discid/<urlquoted-disc-id>?inc=artist-credits+recordings&fmt=json
User-Agent: cd2beets/0.1 (local batch ripper)
Accept: application/json
```

Parse only media whose `discs` includes the queried disc ID. Join `artist-credit` entries using each `joinphrase`. Sort tracks by integer position and releases by `(albumartist, album, date, country, release_id, medium)`. Deduplicate by `(release_id, medium)`. Missing release IDs, recording IDs, track positions, titles, or an empty matching medium are errors presented to the operator, never defaults.

- [ ] **Step 5: Run tests and commit**

Run: `.venv/bin/pytest tests/test_metadata.py -v`

Expected: all metadata tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/metadata.py cd2beets/tests/test_metadata.py cd2beets/tests/fixtures/musicbrainz_disc.json
git commit -m "Add MusicBrainz disc metadata lookup"
```

---

### Task 5: Read-Only Beets Catalog and Presence Classification

**Files:**
- Create: `src/cd2beets/library.py`
- Create: `tests/test_library.py`

**Interfaces:**
- Consumes: beets `config`, `ui.load_config`, `Library`, `util.syspath`, `ReleaseMetadata`, and `LibraryItem`.
- Produces: `BeetsSettings.load()`, `BeetsLibrary.refresh()`, `BeetsLibrary.snapshot(release)`, and pure `classify_presence(release, snapshot, library_root)`.

- [ ] **Step 1: Write failing pure classification tests**

Build item factories and cover these exact contracts:

1. Exact release, one unique item per expected recording, matching medium, existing paths under root => `PRESENT`.
2. No exact items and no normalized artist/title candidate => `ABSENT`.
3. Exact release missing a recording => `UNCERTAIN`.
4. Exact release with a duplicate recording => `UNCERTAIN`.
5. Exact release whose file is missing or outside root => `UNCERTAIN`.
6. Same normalized artist/title but blank or different MusicBrainz release ID => `UNCERTAIN`.
7. Split or inconsistent `tracktotal` => `UNCERTAIN`.

Every uncertain assertion must also check a human-readable reason naming the failed invariant.

- [ ] **Step 2: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_library.py -v`

Expected: FAIL because `cd2beets.library` does not exist.

- [ ] **Step 3: Implement settings and a cached read-only catalog**

`BeetsSettings` is frozen and contains:

```python
@dataclass(frozen=True)
class BeetsSettings:
    library_path: Path
    library_root: Path
    musicbrainz_host: str
    plugins: tuple[str, ...]
    duplicate_action: str
    duplicates_delete: bool
    duplicates_remove: bool
    duplicates_move: str
```

`load()` calls `beets.ui.load_config()`, reads paths with `.as_filename()`, scalar values with `.get(type)`, and plugin names with `.as_str_seq()`. `BeetsLibrary` opens `Library(str(settings.library_path))` and never calls `store`, `add`, `remove`, `move`, or a mutating model method.

`refresh()` reads albums/items into immutable `LibraryItem` values. Convert beets byte paths using `Path(util.syspath(item.path))`. Build two indexes: exact release ID and `(normalize_text(albumartist), normalize_text(album))`. Refresh only at startup and after import, not for every classification comparison.

- [ ] **Step 4: Implement fail-closed classification**

`classify_presence()` must compare expected recording IDs as a multiset, require expected medium and track totals, use `Path.resolve(strict=True)` for destination existence/root containment, and return:

```python
Classification(Presence.PRESENT, (), snapshot)
Classification(Presence.ABSENT, (), snapshot)
Classification(Presence.UNCERTAIN, tuple(sorted(reasons)), snapshot)
```

An exact release with any failed invariant is always uncertain. A plausible normalized artist/title candidate is always uncertain. Only no exact and no plausible candidates is absent.

- [ ] **Step 5: Run tests and commit**

Run: `.venv/bin/pytest tests/test_library.py -v`

Expected: all classification and adapter tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/library.py cd2beets/tests/test_library.py
git commit -m "Add fail-closed beets classification"
```

---

### Task 6: Append-Only Journal and Owned-MP3 Staging

**Files:**
- Create: `src/cd2beets/journal.py`
- Create: `src/cd2beets/staging.py`
- Create: `tests/test_journal.py`
- Create: `tests/test_staging.py`

**Interfaces:**
- Consumes: `OwnedMp3`, `VerificationReport`, a staging root, and a run ID.
- Produces: `Journal.append(event, **fields)`, `StagingRun.run_dir: Path`, `StagingRun.reserve(track_position) -> (OwnedMp3, BinaryIO)`, `StagingRun.validate_cleanup(tracks, report)`, and `StagingRun.cleanup(tracks, report)`.

- [ ] **Step 1: Write failing journal and staging tests**

Tests must prove:

- two journal appends create two JSONL records without rewriting the first;
- staging root and run directory modes are `0700`;
- reserved files use `O_CREAT | O_EXCL | O_WRONLY | O_NOFOLLOW` and mode `0600`;
- a second reservation of the same path fails without overwrite;
- ownership records exact `st_dev` and `st_ino` from `fstat`;
- cleanup rejects an unverified report, a path outside staging, non-`.mp3`, symlink, non-regular file, mismatched inode, and unowned path;
- if any candidate fails validation, no candidate is unlinked;
- successful cleanup unlinks only owned MP3s and leaves the run directory and journal present;
- a later unlink error stops cleanup and leaves remaining MP3s present.

Use monkeypatch spies around `Path.unlink` to prove rejected paths never reach unlink.

- [ ] **Step 2: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_journal.py tests/test_staging.py -v`

Expected: FAIL because journal/staging modules do not exist.

- [ ] **Step 3: Implement append-only journaling**

`Journal.append()` opens only with text append mode, serializes one compact JSON object containing UTC timestamp, event, and supplied fields, flushes, and calls `os.fsync`. It never opens with `w`, truncates, rotates, renames, or unlinks. Reject field values that are not JSON serializable before opening the file.

- [ ] **Step 4: Implement exclusive staging and cleanup prevalidation**

Use `uuid.uuid4().hex` run IDs by default. Create directories with `mode=0o700, exist_ok=True` and then enforce `chmod(0o700)` only on tool-created staging directories. Reserve `<run-id>-track<position:02d>.mp3`, journal `track_reserved`, call `os.open` with exclusive/no-follow flags, `fstat` the descriptor, construct `OwnedMp3`, journal `track_created`, and return `os.fdopen(fd, 'wb', closefd=True)`.

`validate_cleanup()` first requires `report.ok`, then validates every candidate against the in-memory ownership dictionary, normalized staging parent, `.mp3`, `lstat`, `stat.S_ISREG`, device, and inode. It returns the complete ordered tuple or raises `CleanupRefused` before any unlink.

`cleanup()` calls `validate_cleanup()` once, journals `track_cleanup_started` before each unlink, unlinks only that exact path, then journals `track_cleanup_succeeded`. On `OSError`, journal `track_cleanup_failed` and raise `PartialCleanupError(removed, remaining, cause)`.

- [ ] **Step 5: Run tests and commit**

Run: `.venv/bin/pytest tests/test_journal.py tests/test_staging.py -v`

Expected: all journal and staging tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/journal.py cd2beets/src/cd2beets/staging.py cd2beets/tests/test_journal.py cd2beets/tests/test_staging.py
git commit -m "Add guarded MP3 staging cleanup"
```

---

### Task 7: Track Ripping and Fixed Beets Import

**Files:**
- Create: `src/cd2beets/ripper.py`
- Create: `src/cd2beets/importer.py`
- Create: `tests/test_ripper.py`
- Create: `tests/test_importer.py`

**Interfaces:**
- Consumes: `ProcessRunner.pipeline`, `ProcessRunner.run`, `DiscToc`, `TrackMetadata`, `StagingRun.reserve()`, `StagingRun.run_dir`, and a selected MusicBrainz release ID.
- Produces: `Ripper.rip_track(toc, track, staging) -> OwnedMp3`, `BeetsImporter.import_release(release_id, run_dir)`, `RipError`, and `BeetsImportError`.

- [ ] **Step 1: Confirm installed command contracts without media**

Run:

```bash
cd-paranoia --help
lame --help
beet import --help
```

Expected cd-paranoia help contains `-q`, `-e`, `-X`, and `-p`; LAME help accepts raw PCM input/output `-`, `-r`, and `-b`; beets help contains `--copy`, `--quiet-fallback`, `--noresume`, `--noincremental`, and `--search-id`. Do not introduce shell redirection or a WAV file.

- [ ] **Step 2: Write failing exact-command tests**

The ripper test uses fake staging and runner objects and requires:

```python
source_args == [
    "cd-paranoia", "-q", "-e", "-X", "-p", "3", "-",
]
sink_args == ["lame", "--silent", "-r", "-b", "320", "--id3v2-only", "--tt", "Track Three", "--ta", "Album Artist", "--tl", "Album Title", "--ty", "1992", "--tn", "3/10", "-", "-"]
```

It must assert the reserved binary handle is the pipeline destination, both process statuses must be zero, stderr appears in `RipError`, and a failed rip leaves the owned MP3 registered/present.

The importer test requires exactly:

```python
[
    "beet", "import", "--copy", "--quiet", "--quiet-fallback=skip",
    "--noresume", "--noincremental",
    "--search-id=11111111-1111-1111-1111-111111111111",
    "/local/staging/run-1",
]
```

It must prove no caller-supplied extra beets argument is accepted and nonzero exit raises `BeetsImportError` with stdout/stderr.

- [ ] **Step 3: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_ripper.py tests/test_importer.py -v`

Expected: FAIL because ripper/importer modules do not exist.

- [ ] **Step 4: Implement ripping and import**

`Ripper.rip_track()` reserves the MP3 before launching processes, streams `cd-paranoia` host-endian raw PCM into LAME, supplies the selected track/release title, album artist, year when known, and track number/total as ID3v2 fields, passes the open MP3 handle to the pipeline, closes it in `finally`, journals source/sink return codes and stderr, and returns the `OwnedMp3` only when both return codes are zero and stderr contains no exact `cd-paranoia` skip event. The tags are required because the generated staging names intentionally carry no release identity and beets' fixed non-interactive release match otherwise skips them. Full paranoia verification plus `-X` aborts on an uncorrected skip, but `cd-paranoia` returns zero after that abort; the ripper therefore treats its machine-readable `##: 6 [skip] @ <sector>` diagnostic as a hard failure. It never removes failed output.

`BeetsImporter.import_release()` validates the release ID against UUID syntax, builds only the fixed vector above, runs it with no shell, journals start/success/failure, and returns the `CommandResult` on zero. It has no generic subcommand or extra-argument parameter.

- [ ] **Step 5: Run tests and commit**

Run: `.venv/bin/pytest tests/test_ripper.py tests/test_importer.py -v`

Expected: all ripper/importer tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/ripper.py cd2beets/src/cd2beets/importer.py cd2beets/tests/test_ripper.py cd2beets/tests/test_importer.py
git commit -m "Add 320 kbps ripping and copy import"
```

---

### Task 8: Independent Post-Import Verification

**Files:**
- Create: `src/cd2beets/verification.py`
- Create: `tests/test_verification.py`

**Interfaces:**
- Consumes: selected `ReleaseMetadata`, pre/post `LibrarySnapshot`, current `OwnedMp3` values, staging root, and library root.
- Produces: pure `verify_import(release, pre_snapshot, post_snapshot, owned, staging_root, library_root) -> VerificationReport`.

- [ ] **Step 1: Write failing verification tests**

Create one complete success case and one test for each failure:

- source MP3 missing after import, proving move rather than copy;
- source inode/device changed;
- missing/duplicate expected recording ID;
- post item still under staging;
- destination outside library root;
- destination missing, symlink, empty, or not regular;
- destination format not MP3;
- bitrate outside `319_000 <= bitrate <= 321_000`;
- an expected recording absent before import but not represented by a newly created post item ID;
- unexpected release ID or medium.

Each test asserts a specific human-readable error and `report.ok is False`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_verification.py -v`

Expected: FAIL because `cd2beets.verification` does not exist.

- [ ] **Step 3: Implement fail-closed verification**

`verify_import()` must accumulate all errors, never mutate beets or files, and sort destination paths by expected track position. It must:

1. Re-`lstat` every owned source and require regular file plus recorded device/inode.
2. Index post items by recording ID and require exactly one per expected track on the selected release/medium.
3. Require each destination to resolve strictly beneath the library root and not beneath staging.
4. Require regular, non-symlink, non-empty MP3 destinations with bitrate in the explicit range.
5. Compare pre/post item IDs: every expected recording absent from the pre-snapshot must have a post item ID absent from the pre-snapshot. Existing partial tracks may retain their prior IDs.
6. Return every failure in `VerificationReport.errors`; return an empty tuple only on complete success.

- [ ] **Step 4: Run tests and commit**

Run: `.venv/bin/pytest tests/test_verification.py -v`

Expected: all verification tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/verification.py cd2beets/tests/test_verification.py
git commit -m "Add beets and NAS import verification"
```

---

### Task 9: Terminal Decisions and Workflow State Machine

**Files:**
- Create: `src/cd2beets/ui.py`
- Create: `src/cd2beets/workflow.py`
- Create: `tests/test_ui.py`
- Create: `tests/test_workflow.py`

**Interfaces:**
- Consumes: drive, metadata, library, journal, ripper, importer, staging, verifier, and terminal adapters defined in Tasks 3–8.
- Produces: `TerminalUI.select_release()`, `.decide_uncertain()`, `.decide_failure()`, `DiscWorkflow.process_disc()`, and `DiscWorkflow.run(once, poll_interval)`.

- [ ] **Step 1: Write failing terminal tests**

Use `io.StringIO` for input/output. Require candidate displays to include release ID, artist, album, date, country, disambiguation, medium, and track count. Require:

- one complete candidate is selected without a prompt;
- multiple candidates require a numeric selection;
- empty response at an uncertain-presence prompt returns `SKIP_EJECT`;
- explicit `p` returns `PROCEED`;
- explicit `a` returns `ABORT_KEEP_LOADED`;
- invalid input is re-prompted without taking action;
- failure choices expose only stage-valid retry, eject, and quit actions.

- [ ] **Step 2: Write failing workflow tests with fakes**

Cover observable sequences:

1. `PRESENT` => no staging/rip/import/cleanup, one eject.
2. `ABSENT` => rip every track in order, import once, refresh, verify, cleanup, eject.
3. `UNCERTAIN + default skip` => no rip/import, one eject.
4. `UNCERTAIN + proceed` => normal rip/import path.
5. `UNCERTAIN + abort` => no eject and return held state.
6. metadata/rip/import/verification failure => generated MP3s preserved and no automatic eject.
7. import retry invokes only import again; verification retry invokes only refresh/verify again.
8. cleanup partial failure => no eject and remaining MP3s preserved.
9. continuous mode waits for no media after eject before accepting another disc.
10. `--once` exits after one processed disc.
11. `KeyboardInterrupt` never calls cleanup or eject.


The ordered fake log must also require append-only journal events for disc ID, candidate releases, selected release, classification and reasons, generated tracks, import outcome, verification report, each cleanup result, failure/held state, and eject outcome.
Assert ordered fake call logs, not source implementation details.

- [ ] **Step 3: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_ui.py tests/test_workflow.py -v`

Expected: FAIL because UI/workflow modules do not exist.

- [ ] **Step 4: Implement terminal UI**

`TerminalUI` owns all `input()` calls. Presence prompt text is `[s]kip/eject (default), [p]roceed, [a]bort/keep loaded`. Failure prompts are built from an explicit tuple of `FailureDecision` values supplied by workflow; there is no arbitrary command entry. It prints stderr and invariant failures verbatim but never credentials.

- [ ] **Step 5: Implement workflow transitions**

Define `DiscOutcome` as `EJECTED`, `HELD`, or `QUIT`. `process_disc()` gets the one device string from `DiscidReader.default_device()`, passes it to `DiscidReader.read(device)`, and performs identify → select → snapshot/classify → decision → stage/rip → import using `staging.run_dir` → refresh/verify → guarded cleanup → eject. It captures the pre-import snapshot before any rip and appends the lifecycle events required by the journal contract at each completed transition. It never calls cleanup unless `report.ok`. It never ejects failure/abort paths without explicit `FailureDecision.EJECT`.

`run()` polls `drive.status()`; `NO_MEDIA` sleeps, `UNUSABLE` reports and sleeps, `AUDIO` processes once. After an eject, it polls until `NO_MEDIA`. `once=True` returns after one processed disc, not merely one poll. Catch `KeyboardInterrupt` only at the outer loop, print preservation guidance, and return without cleanup/eject.

If disc access reports busy and `pgrep -x Music` succeeds, display the manual-close guidance and offer quit/eject only; never send a signal to Music.

- [ ] **Step 6: Run tests and commit**

Run: `.venv/bin/pytest tests/test_ui.py tests/test_workflow.py -v`

Expected: all UI/workflow tests pass.

Commit:

```bash
git add cd2beets/src/cd2beets/ui.py cd2beets/src/cd2beets/workflow.py cd2beets/tests/test_ui.py cd2beets/tests/test_workflow.py
git commit -m "Add conservative CD processing workflow"
```

---

### Task 10: Preflight and CLI Composition

**Files:**
- Create: `src/cd2beets/preflight.py`
- Create: `src/cd2beets/cli.py`
- Create: `tests/test_preflight.py`
- Create: `tests/test_cli.py`

**Interfaces:**
- Consumes: every concrete adapter, `BeetsSettings`, CLI arguments, filesystem/socket/process probes.
- Produces: `Preflight.run() -> PreflightReport`, `build_parser()`, `main(argv=None) -> int`, and installed `.venv/bin/cd2beets`.

- [ ] **Step 1: Write failing preflight tests**

Use injected probes and require one named error for each failure:

- missing `drutil`, `cd-paranoia`, `lame`, `beet`, or `pgrep`;
- beets CLI version not exactly 2.8.0;
- unreadable library database;
- NAS root absent, unwritable, or same `st_dev` as `/`;
- staging nested under library root;
- zero or multiple optical drives;
- unknown active plugin outside `{musicbrainz, web, duplicates, missing}`;
- `duplicate_action != 'skip'`;
- duplicates `delete`, `remove`, or `move` configured;
- MusicBrainz TCP ping failure.

A successful report includes resolved library/staging paths and the selected drive. Verify preflight performs no import, eject, unlink, Music preference change, or process termination.

- [ ] **Step 2: Write failing CLI tests**

Require parser defaults:

```python
staging_dir == Path("~/Music/cd2beets-staging").expanduser()
poll_interval == 2.0
once is False
```

Require `--staging-dir`, `--poll-interval`, and `--once`. Reject non-positive poll intervals. `main()` prints the **Show CD** reminder before preflight, returns 2 for preflight errors, 1 for held/failed workflows, 0 for clean `--once` completion or user interrupt, and does not expose arbitrary beets or cleanup arguments.

- [ ] **Step 3: Run tests to verify they fail**

Run: `.venv/bin/pytest tests/test_preflight.py tests/test_cli.py -v`

Expected: FAIL because preflight/CLI modules do not exist.

- [ ] **Step 4: Implement preflight**

`Preflight.run()` uses `shutil.which`, `beet version`, `Path.stat`, `os.access`, drive listing, explicit plugin/config checks, and `MusicBrainzClient.ping`. It returns all failures together rather than stopping at the first. Compare the resolved NAS root's `st_dev` to `/` to prevent silently writing to an unmounted `/Volumes/music` directory. Do not create a writable test file because the tool cannot delete it afterward.

- [ ] **Step 5: Implement CLI composition**

`main()` loads settings, resolves staging, builds journal and all concrete adapters, runs preflight, then constructs `DiscWorkflow`. It prints exact preflight failures and exits before waiting/ripping on any error. Signal/interrupt handling leaves media and generated paths untouched. Add no environment mutation, installer, preference command, daemon, or cleanup subcommand.

- [ ] **Step 6: Run focused and full automated tests**

Run:

```bash
.venv/bin/pytest tests/test_preflight.py tests/test_cli.py -v
.venv/bin/pytest -v
```

Expected: focused tests pass; full suite passes with zero failures.

- [ ] **Step 7: Commit**

```bash
git add cd2beets/src/cd2beets/preflight.py cd2beets/src/cd2beets/cli.py cd2beets/tests/test_preflight.py cd2beets/tests/test_cli.py
git commit -m "Add cd2beets preflight and CLI"
```

---

### Task 11: Contract Checks and Real-System Acceptance

**Files:**
- Modify only if real output differs from sanitized contracts: `tests/fixtures/drutil_status_audio.txt`
- Modify only if command parsing needs a source-level correction proven by the real tool: the directly affected adapter and its test.
- No new cleanup script or acceptance-only deletion command.

**Interfaces:**
- Consumes: installed CLI, HP CDDVDW TS-L633R drive, one audio CD, active beets config limited to one public MusicBrainz request per second, and mounted `/Volumes/music/library`.
- Produces: observed no-media, hardware-rip, NAS-copy, cleanup, eject, and second-insertion evidence.

- [ ] **Step 1: Run static package verification**

Run:

```bash
.venv/bin/python -m compileall -q src tests
.venv/bin/pytest -v
.venv/bin/cd2beets --help
```

Expected: compile exits 0; all tests pass; help lists only `--staging-dir`, `--poll-interval`, `--once`, and standard help.

- [ ] **Step 2: Verify the no-media path on the actual Mac**

With no CD inserted and Music set to **Show CD**, run:

```bash
.venv/bin/cd2beets
```

Observe: preflight names any unavailable prerequisite, the HP USB drive is detected, the process enters the no-media waiting state, no `beet import`, cleanup, or eject occurs, and `Ctrl-C` exits without changing the drive. The NAS must be mounted first because preflight intentionally fails closed when `/Volumes/music/library` is unavailable.

- [ ] **Step 3: Verify real disc identification before a full rip**

Insert a test audio CD and run `--once`. Observe and record in the existing append-only journal:

- Music may open and display the CD but does not play, import, or eject it;
- `drutil`, libdiscid, and `cd-paranoia` can access the HP drive;
- TOC and selected MusicBrainz medium have the same track count;
- candidate display has complete release/medium data;
- uncertain library evidence requires an operator decision.

If real `drutil status` differs from the sanitized audio fixture, add the captured non-personal status shape, write a failing parser test, make the smallest parser correction, rerun the drive/workflow tests, and commit only those exact files.

- [ ] **Step 4: Execute one absent-disc end-to-end acceptance run**

Mount the NAS and verify the configured root is `/Volumes/music/library`. Choose a CD the tool classifies absent, then proceed through normal `--once` operation. Observe:

1. Every expected track produces one local MP3 and no WAV/INF sidecar.
2. LAME reports/produces constant 320 kbps output.
3. Beets is invoked with copy mode and selected release ID.
4. All staging MP3s still exist immediately after import.
5. Post-import beets items exactly match expected recording IDs.
6. Every destination is a non-empty MP3 beneath `/Volumes/music/library` at 319–321 kbps.
7. Only current-run staging MP3s disappear after verification; journal and run directory remain.
8. The disc ejects.

Any failed invariant must leave the disc loaded and every not-yet-cleaned MP3 present. Do not manually delete test artifacts; use the product's guarded path only after its normal verification succeeds.

- [ ] **Step 5: Verify the already-present fast path**

Reinsert the same CD and run:

```bash
.venv/bin/cd2beets --once
```

Expected: exact release and all tracks classify `PRESENT`; no ripper, importer, staging cleanup, or beets mutation runs; the tool ejects.

- [ ] **Step 6: Run final regression verification**

Run:

```bash
.venv/bin/pytest -v
```

Expected: zero failures after any real-output contract correction. Report the exact test count plus the hardware/NAS observations; do not claim hardware completion from automated fakes alone.

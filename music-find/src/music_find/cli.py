from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
import time
import tomllib
import unicodedata
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable, Sequence

FIELD_SEPARATOR = "\x1f"
RECORD_SEPARATOR = "\x1e"
INDEX_VERSION = 1
BEET_FORMAT = (
    f"{RECORD_SEPARATOR}$albumartist{FIELD_SEPARATOR}"
    f"$album{FIELD_SEPARATOR}$path"
)


class MusicFindError(Exception):
    """An operational error that prevents a complete search."""


@dataclass(frozen=True)
class Config:
    library_root: Path
    archive_root: Path


@dataclass(frozen=True)
class Album:
    artist: str
    title: str
    path: Path


@dataclass(frozen=True)
class InvalidAlbum:
    album: Album
    reason: str


@dataclass(frozen=True)
class DirectoryEntry:
    source: str
    relative_path: str


@dataclass(frozen=True)
class DirectoryIndex:
    generated_at: datetime
    library_root: Path
    archive_root: Path
    entries: tuple[DirectoryEntry, ...]


@dataclass(frozen=True)
class DirectoryResult:
    matches: tuple[Path, ...]
    errors: tuple[str, ...]


@dataclass(frozen=True)
class SearchReport:
    query: str
    config: Config
    index_generated_at: datetime
    beets_matches: tuple[Album, ...]
    invalid_beets_matches: tuple[InvalidAlbum, ...]
    beets_error: str | None
    library_result: DirectoryResult
    archive_result: DirectoryResult | None


def normalized(value: str) -> str:
    return unicodedata.normalize("NFC", value).casefold()


def default_config_path() -> Path:
    config_home = os.environ.get("XDG_CONFIG_HOME")
    base = Path(config_home).expanduser() if config_home else Path.home() / ".config"
    return base / "music-find" / "config.toml"


def default_index_path() -> Path:
    cache_home = os.environ.get("XDG_CACHE_HOME")
    base = Path(cache_home).expanduser() if cache_home else Path.home() / ".cache"
    return base / "music-find" / "directories.json"


def load_config(path: Path | None = None) -> Config:
    config_path = path or default_config_path()
    try:
        with config_path.open("rb") as handle:
            values = tomllib.load(handle)
    except FileNotFoundError as error:
        raise MusicFindError(
            f"Configuration not found: {config_path}\n"
            "Create it with library_root and archive_root, then retry."
        ) from error
    except (OSError, tomllib.TOMLDecodeError) as error:
        raise MusicFindError(f"Cannot read configuration {config_path}: {error}") from error

    roots: dict[str, Path] = {}
    for key in ("library_root", "archive_root"):
        value = values.get(key)
        if not isinstance(value, str) or not value.strip():
            raise MusicFindError(
                f"Configuration {config_path} must define non-empty {key}."
            )
        root = Path(value).expanduser()
        if not root.is_absolute():
            raise MusicFindError(f"Configuration value {key} must be an absolute path.")
        roots[key] = root.resolve()

    library_root = roots["library_root"]
    archive_root = roots["archive_root"]
    try:
        library_root.relative_to(archive_root)
    except ValueError as error:
        raise MusicFindError(
            f"Configured library_root must be beneath archive_root ({archive_root})."
        ) from error

    return Config(library_root=library_root, archive_root=archive_root)


def validate_nas(config: Config, mount_checker: Callable[[Path], bool]) -> None:
    if not config.archive_root.is_dir() or not mount_checker(config.archive_root):
        raise MusicFindError(
            f"NAS volume is not mounted at {config.archive_root}.\n"
            "Mount the volume and retry."
        )
    if not config.library_root.is_dir():
        raise MusicFindError(
            f"NAS library directory is unavailable at {config.library_root}.\n"
            "Check the mount and configuration, then retry."
        )


def build_directory_index(
    config: Config,
    path: Path | None = None,
    *,
    progress: Callable[[int], None] | None = None,
) -> DirectoryIndex:
    index_path = path or default_index_path()
    entries: list[DirectoryEntry] = []
    traversal_errors: list[str] = []

    def record_error(error: OSError) -> None:
        location = error.filename or config.archive_root
        detail = error.strerror or str(error)
        traversal_errors.append(f"{location}: {detail}")

    for current, directory_names, _ in os.walk(
        config.archive_root,
        topdown=True,
        onerror=record_error,
        followlinks=False,
    ):
        directory_names.sort(key=normalized)
        current_path = Path(current)
        for name in directory_names:
            directory_path = current_path / name
            if directory_path == config.library_root:
                continue
            relative_path = directory_path.relative_to(config.archive_root)
            source = (
                "library"
                if directory_path.is_relative_to(config.library_root)
                else "archive"
            )
            entries.append(DirectoryEntry(source, str(relative_path)))
            if progress and len(entries) % 1000 == 0:
                progress(len(entries))

    if traversal_errors:
        examples = "\n".join(f"  {error}" for error in traversal_errors[:5])
        remainder = len(traversal_errors) - min(5, len(traversal_errors))
        suffix = f"\n  ... and {remainder} more" if remainder else ""
        raise MusicFindError(
            f"Cannot build a complete NAS index; "
            f"{len(traversal_errors)} directories could not be read:\n"
            f"{examples}{suffix}"
        )

    index = DirectoryIndex(
        generated_at=datetime.now(timezone.utc),
        library_root=config.library_root,
        archive_root=config.archive_root,
        entries=tuple(entries),
    )
    _write_directory_index(index, index_path)
    return index


def _write_directory_index(index: DirectoryIndex, path: Path) -> None:
    document = {
        "version": INDEX_VERSION,
        "generated_at": index.generated_at.isoformat(),
        "library_root": str(index.library_root),
        "archive_root": str(index.archive_root),
        "entries": [
            {"source": entry.source, "path": entry.relative_path}
            for entry in index.entries
        ],
    }
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            dir=path.parent,
            prefix=f".{path.name}.",
            delete=False,
        ) as handle:
            temporary_path = Path(handle.name)
            json.dump(document, handle, ensure_ascii=False, separators=(",", ":"))
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary_path, path)
    except OSError as error:
        raise MusicFindError(f"Cannot write NAS index {path}: {error}") from error
    finally:
        if temporary_path and temporary_path.exists():
            temporary_path.unlink()


def load_directory_index(
    config: Config, path: Path | None = None
) -> DirectoryIndex:
    index_path = path or default_index_path()
    retry = "Run music-find --reindex and retry."
    try:
        with index_path.open(encoding="utf-8") as handle:
            document = json.load(handle)
    except FileNotFoundError as error:
        raise MusicFindError(f"NAS directory index not found: {index_path}\n{retry}") from error
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise MusicFindError(
            f"Cannot read NAS directory index {index_path}: {error}\n{retry}"
        ) from error

    try:
        version = document["version"]
        generated_at = datetime.fromisoformat(document["generated_at"])
        library_root = Path(document["library_root"])
        archive_root = Path(document["archive_root"])
        raw_entries = document["entries"]
    except (KeyError, TypeError, ValueError) as error:
        raise MusicFindError(f"NAS directory index is malformed.\n{retry}") from error

    if version != INDEX_VERSION:
        raise MusicFindError(
            f"NAS directory index version {version!r} is unsupported.\n{retry}"
        )
    if generated_at.tzinfo is None:
        raise MusicFindError(f"NAS directory index timestamp is malformed.\n{retry}")
    if library_root != config.library_root or archive_root != config.archive_root:
        raise MusicFindError(
            "NAS directory index was built for different configured paths.\n"
            f"{retry}"
        )
    if not isinstance(raw_entries, list):
        raise MusicFindError(f"NAS directory index entries are malformed.\n{retry}")

    entries: list[DirectoryEntry] = []
    for raw_entry in raw_entries:
        if not isinstance(raw_entry, dict):
            raise MusicFindError(f"NAS directory index entry is malformed.\n{retry}")
        source = raw_entry.get("source")
        relative_path = raw_entry.get("path")
        if source not in {"library", "archive"} or not isinstance(
            relative_path, str
        ):
            raise MusicFindError(f"NAS directory index entry is malformed.\n{retry}")
        parsed_path = Path(relative_path)
        if parsed_path.is_absolute() or ".." in parsed_path.parts:
            raise MusicFindError(f"NAS directory index contains an unsafe path.\n{retry}")
        entries.append(DirectoryEntry(source, relative_path))

    return DirectoryIndex(
        generated_at=generated_at,
        library_root=library_root,
        archive_root=archive_root,
        entries=tuple(entries),
    )


def search_directory_index(
    index: DirectoryIndex, query: str, source: str
) -> DirectoryResult:
    needle = normalized(query)
    matches: list[Path] = []
    errors: list[str] = []

    for entry in index.entries:
        if entry.source != source:
            continue
        relative_path = Path(entry.relative_path)
        if needle not in normalized(relative_path.name):
            continue
        path = index.archive_root / relative_path
        if path.is_dir():
            matches.append(path)
        else:
            errors.append(f"Indexed directory is missing: {path}; run music-find --reindex")

    matches.sort(
        key=lambda match: normalized(str(match.relative_to(index.archive_root)))
    )
    return DirectoryResult(tuple(matches), tuple(errors))


def parse_beets_output(output: str) -> tuple[Album, ...]:
    prefix, *records = output.split(RECORD_SEPARATOR)
    if prefix.strip():
        raise MusicFindError("Unexpected output from beet before the first album record.")

    albums: list[Album] = []
    for record in records:
        if record.endswith("\n"):
            record = record[:-1]
        if not record:
            continue
        fields = record.split(FIELD_SEPARATOR)
        if len(fields) != 3:
            raise MusicFindError(
                "Cannot parse beet output; an album field contains a control separator."
            )
        artist, title, path = fields
        albums.append(Album(artist=artist, title=title, path=Path(path)))
    return tuple(albums)


def search_beets(query: str, executable: str = "beet") -> tuple[Album, ...]:
    try:
        result = subprocess.run(
            [executable, "list", "--album", "--format", BEET_FORMAT],
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
    except FileNotFoundError as error:
        raise MusicFindError(f"Cannot run beet: executable not found: {executable}") from error
    except subprocess.TimeoutExpired as error:
        raise MusicFindError("beet did not finish within 60 seconds.") from error
    except OSError as error:
        raise MusicFindError(f"Cannot run beet: {error}") from error

    if result.returncode != 0:
        detail = result.stderr.strip() or f"exit status {result.returncode}"
        raise MusicFindError(f"beet search failed: {detail}")

    needle = normalized(query)
    matches = [
        album
        for album in parse_beets_output(result.stdout)
        if needle in normalized(album.artist) or needle in normalized(album.title)
    ]
    return tuple(
        sorted(
            matches,
            key=lambda album: (
                normalized(album.artist),
                normalized(album.title),
                normalized(str(album.path)),
            ),
        )
    )


def validate_album_paths(
    albums: Sequence[Album], library_root: Path
) -> tuple[tuple[Album, ...], tuple[InvalidAlbum, ...]]:
    valid: list[Album] = []
    invalid: list[InvalidAlbum] = []
    library_root = library_root.resolve()

    for album in albums:
        if not album.path.is_dir():
            invalid.append(InvalidAlbum(album, "album directory is missing"))
            continue
        resolved_path = album.path.resolve()
        try:
            resolved_path.relative_to(library_root)
        except ValueError:
            invalid.append(
                InvalidAlbum(album, f"album path is outside {library_root}")
            )
            continue
        valid.append(Album(album.artist, album.title, resolved_path))

    return tuple(valid), tuple(invalid)


def perform_search(
    query: str,
    config: Config,
    *,
    beet_executable: str = "beet",
    index_path: Path | None = None,
    mount_checker: Callable[[Path], bool] = os.path.ismount,
) -> SearchReport:
    validate_nas(config, mount_checker)
    directory_index = load_directory_index(config, index_path)

    beets_error: str | None = None
    beets_matches: tuple[Album, ...] = ()
    invalid_beets_matches: tuple[InvalidAlbum, ...] = ()

    with ThreadPoolExecutor(max_workers=2) as executor:
        beets_future = executor.submit(search_beets, query, beet_executable)
        library_future = executor.submit(
            search_directory_index, directory_index, query, "library"
        )

        try:
            raw_beets_matches = beets_future.result()
            beets_matches, invalid_beets_matches = validate_album_paths(
                raw_beets_matches, config.library_root
            )
        except MusicFindError as error:
            beets_error = str(error)
        except Exception as error:
            beets_error = f"Unexpected beet search failure: {error}"

        try:
            library_result = library_future.result()
        except Exception as error:
            library_result = DirectoryResult((), (f"Unexpected index failure: {error}",))

    organized_match = bool(beets_matches or library_result.matches)
    archive_result = None
    if not organized_match:
        archive_result = search_directory_index(
            directory_index, query, "archive"
        )

    return SearchReport(
        query=query,
        config=config,
        index_generated_at=directory_index.generated_at,
        beets_matches=beets_matches,
        invalid_beets_matches=invalid_beets_matches,
        beets_error=beets_error,
        library_result=library_result,
        archive_result=archive_result,
    )


def _album_count(count: int) -> str:
    return f"{count} album" if count == 1 else f"{count} albums"


def _directory_count(count: int) -> str:
    return f"{count} directory" if count == 1 else f"{count} directories"


def _format_album(album: Album, prefix: str = "") -> list[str]:
    artist = album.artist or "(unknown artist)"
    title = album.title or "(untitled album)"
    return [f"  {prefix}{artist} — {title}", f"    {album.path}"]


def _format_index_age(generated_at: datetime) -> str:
    age_seconds = max(
        0, int((datetime.now(timezone.utc) - generated_at.astimezone(timezone.utc)).total_seconds())
    )
    if age_seconds < 60:
        age = f"{age_seconds} seconds"
    elif age_seconds < 3600:
        age = f"{age_seconds // 60} minutes"
    elif age_seconds < 86400:
        age = f"{age_seconds // 3600} hours"
    else:
        age = f"{age_seconds // 86400} days"
    local_time = generated_at.astimezone().strftime("%Y-%m-%d %H:%M:%S %Z")
    return f"Index: {local_time} ({age} old)"


def render_report(report: SearchReport) -> str:
    lines = [
        f"Query: {report.query}",
        _format_index_age(report.index_generated_at),
        "",
    ]

    if report.beets_error:
        lines.extend(["BEETS — ERROR", f"  {report.beets_error}"])
    elif report.invalid_beets_matches:
        lines.append(
            "BEETS — INVALID "
            f"({_album_count(len(report.beets_matches))} valid, "
            f"{len(report.invalid_beets_matches)} invalid)"
        )
        for album in report.beets_matches:
            lines.extend(_format_album(album))
        for invalid in report.invalid_beets_matches:
            lines.extend(_format_album(invalid.album, "INVALID: "))
            lines.append(f"    {invalid.reason}")
    elif report.beets_matches:
        lines.append(f"BEETS — FOUND ({_album_count(len(report.beets_matches))})")
        for album in report.beets_matches:
            lines.extend(_format_album(album))
    else:
        lines.append("BEETS — NOT FOUND")

    lines.append("")
    lines.extend(
        _render_directory_section(
            "NAS LIBRARY", report.library_result, report.config.library_root
        )
    )
    lines.append("")

    if report.archive_result is None:
        lines.extend(
            ["NAS ARCHIVE — SKIPPED", "  Organized-library result found."]
        )
    else:
        lines.extend(
            _render_directory_section(
                "NAS ARCHIVE", report.archive_result, report.config.archive_root
            )
        )

    if not report_has_matches(report) and not report_has_errors(report):
        lines.extend(["", "No matching album or artist directory found."])

    return "\n".join(lines)


def _render_directory_section(
    name: str, result: DirectoryResult, root: Path
) -> list[str]:
    if result.errors:
        lines = [
            f"{name} — ERROR "
            f"({_directory_count(len(result.matches))} found, "
            f"{len(result.errors)} index errors)"
        ]
    elif result.matches:
        lines = [f"{name} — FOUND ({_directory_count(len(result.matches))})"]
    else:
        lines = [f"{name} — NOT FOUND"]

    for match in result.matches:
        try:
            display_path = match.relative_to(root)
        except ValueError:
            display_path = match
        lines.append(f"  {display_path}")
    for error in result.errors:
        lines.append(f"  ERROR: {error}")
    return lines


def report_has_matches(report: SearchReport) -> bool:
    return bool(
        report.beets_matches
        or report.library_result.matches
        or (report.archive_result and report.archive_result.matches)
    )


def report_has_errors(report: SearchReport) -> bool:
    return bool(
        report.beets_error
        or report.invalid_beets_matches
        or report.library_result.errors
        or (report.archive_result and report.archive_result.errors)
    )


def report_exit_code(report: SearchReport) -> int:
    if report_has_errors(report):
        return 2
    return 0 if report_has_matches(report) else 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="music-find",
        description="Find an album or artist in beets and NAS directories.",
        allow_abbrev=False,
    )
    parser.add_argument(
        "query",
        nargs="?",
        help="literal, case-insensitive artist or album substring",
    )
    parser.add_argument(
        "--reindex",
        action="store_true",
        help="rebuild the local NAS directory index",
    )
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    arguments = parser.parse_args(argv)
    if arguments.reindex and arguments.query is not None:
        parser.error("query cannot be combined with --reindex")
    if not arguments.reindex and arguments.query is None:
        parser.error("a query or --reindex is required")
    if arguments.query is not None and not arguments.query.strip():
        parser.error("query must not be empty")

    try:
        config = load_config()
        validate_nas(config, os.path.ismount)
        if arguments.reindex:
            started = time.monotonic()
            print(f"Building NAS directory index from {config.archive_root}...")
            index = build_directory_index(
                config,
                progress=lambda count: print(
                    f"  Indexed {count:,} directories...", flush=True
                ),
            )
            elapsed = time.monotonic() - started
            print(
                f"Indexed {len(index.entries):,} directories in {elapsed:.1f} seconds."
            )
            print(f"Index written to {default_index_path()}.")
            return 0
        report = perform_search(arguments.query, config)
    except MusicFindError as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 2

    print(render_report(report))
    return report_exit_code(report)


if __name__ == "__main__":
    raise SystemExit(main())

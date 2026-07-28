from __future__ import annotations

from collections import Counter, defaultdict
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from types import MappingProxyType

from beets import config, ui, util
from beets.library import Library

from .models import (
    Classification,
    LibraryItem,
    LibrarySnapshot,
    Presence,
    ReleaseMetadata,
    normalize_text,
)


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

    @classmethod
    def load(cls) -> BeetsSettings:
        loader = getattr(ui, "load_config", None)
        if loader is None:
            config.read()
        else:
            loader()

        return cls(
            library_path=Path(config["library"].as_filename()),
            library_root=Path(config["directory"].as_filename()),
            musicbrainz_host=config["musicbrainz"]["host"].get(str),
            plugins=tuple(config["plugins"].as_str_seq()),
            duplicate_action=config["import"]["duplicate_action"].get(str),
            duplicates_delete=config["duplicates"]["delete"].get(bool),
            duplicates_remove=config["duplicates"]["remove"].get(bool),
            duplicates_move=config["duplicates"]["move"].get(str),
        )


class BeetsLibrary:
    def __init__(self, settings: BeetsSettings):
        self._library = Library(str(settings.library_path))
        self._exact_index: Mapping[str, tuple[LibraryItem, ...]] = (
            MappingProxyType({})
        )
        self._title_index: Mapping[
            tuple[str, str], tuple[LibraryItem, ...]
        ] = MappingProxyType({})
        self.refresh()

    def refresh(self) -> None:
        exact: defaultdict[str, list[LibraryItem]] = defaultdict(list)
        titles: defaultdict[tuple[str, str], list[LibraryItem]] = defaultdict(
            list
        )

        for raw_item in self._library.items():
            catalog_item = _catalog_item(raw_item)
            exact[catalog_item.release_id].append(catalog_item)
            title_key = (
                normalize_text(catalog_item.albumartist),
                normalize_text(catalog_item.album),
            )
            titles[title_key].append(catalog_item)

        self._exact_index = MappingProxyType(
            {key: tuple(items) for key, items in exact.items()}
        )
        self._title_index = MappingProxyType(
            {key: tuple(items) for key, items in titles.items()}
        )

    def snapshot(self, release: ReleaseMetadata) -> LibrarySnapshot:
        exact_items = self._exact_index.get(release.release_id, ())
        title_key = (
            normalize_text(release.albumartist),
            normalize_text(release.album),
        )
        plausible_items = tuple(
            item
            for item in self._title_index.get(title_key, ())
            if item.release_id != release.release_id
        )
        return LibrarySnapshot(exact_items, plausible_items)


def _catalog_item(item) -> LibraryItem:
    return LibraryItem(
        item_id=int(item.id),
        release_id=_text(item.mb_albumid),
        recording_id=_text(item.mb_trackid),
        albumartist=_text(item.albumartist),
        album=_text(item.album),
        medium=_integer(item.disc),
        track=_integer(item.track),
        tracktotal=_integer(item.tracktotal),
        path=Path(util.syspath(item.path)),
        format=_text(item.format),
        bitrate=_integer(item.bitrate),
    )


def _text(value: object) -> str:
    return "" if value is None else str(value)


def _integer(value: object) -> int:
    return 0 if value is None else int(value)


def classify_presence(
    release: ReleaseMetadata,
    snapshot: LibrarySnapshot,
    library_root: Path,
) -> Classification:
    reasons: set[str] = set()

    if snapshot.exact_items:
        _check_recordings(release, snapshot.exact_items, reasons)
        _check_medium_and_tracks(release, snapshot.exact_items, reasons)
        _check_paths(snapshot.exact_items, library_root, reasons)

    if snapshot.plausible_items:
        reasons.add(
            "plausible artist/title candidate has a different or missing "
            "release ID"
        )

    if reasons:
        return Classification(
            Presence.UNCERTAIN, tuple(sorted(reasons)), snapshot
        )
    if snapshot.exact_items:
        return Classification(Presence.PRESENT, (), snapshot)
    return Classification(Presence.ABSENT, (), snapshot)


def _check_recordings(
    release: ReleaseMetadata,
    items: tuple[LibraryItem, ...],
    reasons: set[str],
) -> None:
    expected = Counter(release.recording_ids)
    actual = Counter(item.recording_id for item in items)

    missing = expected - actual
    if missing:
        reasons.add(f"missing recordings: {_format_counts(missing)}")

    duplicates = Counter(
        {
            recording_id: actual_count - expected[recording_id]
            for recording_id, actual_count in actual.items()
            if recording_id in expected
            and actual_count > expected[recording_id]
        }
    )
    if duplicates:
        reasons.add(f"duplicate recordings: {_format_counts(duplicates)}")

    unexpected = Counter(
        {
            recording_id: count
            for recording_id, count in actual.items()
            if recording_id not in expected
        }
    )
    if unexpected:
        reasons.add(f"unexpected recordings: {_format_counts(unexpected)}")


def _check_medium_and_tracks(
    release: ReleaseMetadata,
    items: tuple[LibraryItem, ...],
    reasons: set[str],
) -> None:
    media = {item.medium for item in items}
    if media != {release.medium}:
        reasons.add(
            f"medium mismatch: expected {release.medium}; "
            f"found {_format_integers(media)}"
        )

    expected_total = len(release.tracks)
    track_totals = {item.tracktotal for item in items}
    if len(track_totals) > 1:
        reasons.add(
            f"inconsistent track totals: {_format_integers(track_totals)}"
        )
    elif track_totals != {expected_total}:
        reasons.add(
            f"track total mismatch: expected {expected_total}; "
            f"found {_format_integers(track_totals)}"
        )

    expected_tracks = Counter(
        (track.recording_id, track.position) for track in release.tracks
    )
    actual_tracks = Counter((item.recording_id, item.track) for item in items)
    if actual_tracks != expected_tracks:
        reasons.add("recording/track positions do not match expected medium")


def _check_paths(
    items: tuple[LibraryItem, ...],
    library_root: Path,
    reasons: set[str],
) -> None:
    try:
        resolved_root = library_root.resolve(strict=True)
    except (OSError, RuntimeError):
        resolved_root = None
        reasons.add(f"missing library root: {library_root}")
    else:
        if not resolved_root.is_dir():
            reasons.add(f"library root is not a directory: {library_root}")

    for item in items:
        try:
            resolved_path = item.path.resolve(strict=True)
        except (OSError, RuntimeError):
            reasons.add(f"missing path: {item.path}")
            continue

        if not resolved_path.is_file():
            reasons.add(f"path is not a regular file: {item.path}")

        if resolved_root is not None:
            try:
                resolved_path.relative_to(resolved_root)
            except ValueError:
                reasons.add(f"path outside library root: {item.path}")


def _format_counts(counts: Counter[str]) -> str:
    return ", ".join(
        value if count == 1 else f"{value} x{count}"
        for value, count in sorted(counts.items())
    )


def _format_integers(values: set[int]) -> str:
    return ", ".join(str(value) for value in sorted(values)) or "none"

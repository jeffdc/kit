from __future__ import annotations

from collections import Counter, defaultdict
from pathlib import Path
import stat

from cd2beets.models import (
    LibraryItem,
    LibrarySnapshot,
    OwnedMp3,
    ReleaseMetadata,
    VerificationReport,
)

_MINIMUM_BITRATE = 319_000
_MAXIMUM_BITRATE = 321_000


def verify_import(
    release: ReleaseMetadata,
    pre_snapshot: LibrarySnapshot,
    post_snapshot: LibrarySnapshot,
    owned: tuple[OwnedMp3, ...],
    staging_root: Path,
    library_root: Path,
) -> VerificationReport:
    """Independently verify a completed import without changing files or beets."""
    errors: list[str] = []
    destination_paths: list[Path] = []

    for source in sorted(
        owned, key=lambda candidate: (candidate.track_position, str(candidate.path))
    ):
        _verify_owned_source(source, errors)

    post_by_recording: defaultdict[str, list[LibraryItem]] = defaultdict(list)
    for item in post_snapshot.exact_items + post_snapshot.plausible_items:
        post_by_recording[item.recording_id].append(item)
    selected_items = tuple(
        sorted(
            (
                item
                for item in post_snapshot.exact_items
                if item.release_id == release.release_id
                and item.medium == release.medium
            ),
            key=lambda item: (
                item.track,
                item.recording_id,
                item.item_id,
                str(item.path),
            ),
        )
    )
    _verify_album_group(selected_items, errors)
    selected_by_occurrence: defaultdict[
        tuple[str, int], list[LibraryItem]
    ] = defaultdict(list)
    for item in selected_items:
        selected_by_occurrence[(item.recording_id, item.track)].append(item)

    expected_counts = Counter(track.recording_id for track in release.tracks)
    selected_counts = Counter(item.recording_id for item in selected_items)
    ordered_recordings = tuple(
        dict.fromkeys(
            track.recording_id
            for track in sorted(
                release.tracks, key=lambda candidate: candidate.position
            )
        )
    )
    for recording_id in ordered_recordings:
        actual_count = selected_counts[recording_id]
        expected_count = expected_counts[recording_id]
        if actual_count != expected_count:
            expectation = (
                "exactly 1"
                if expected_count == 1
                else str(expected_count)
            )
            errors.append(
                f"expected recording {recording_id} has {actual_count} "
                f"post-import items (expected {expectation})"
            )

    expected_occurrences = {
        (track.recording_id, track.position) for track in release.tracks
    }
    pre_items = pre_snapshot.exact_items + pre_snapshot.plausible_items
    pre_item_ids = {item.item_id for item in pre_items}
    pre_ids_by_occurrence: defaultdict[
        tuple[int, int, str], set[int]
    ] = defaultdict(set)
    for item in pre_items:
        occurrence = (item.medium, item.track, item.recording_id)
        pre_ids_by_occurrence[occurrence].add(item.item_id)

    resolved_staging = _resolve_root(staging_root, "staging", errors)
    resolved_library = _resolve_root(library_root, "library", errors)

    for item in selected_items:
        occurrence = (item.recording_id, item.track)
        if occurrence not in expected_occurrences:
            errors.append(
                "unexpected selected post item recording "
                f"{item.recording_id} at track {item.track}"
            )
        _verify_destination(
            item,
            resolved_staging,
            resolved_library,
            errors,
        )

    for track in sorted(release.tracks, key=lambda candidate: candidate.position):
        recording_items = post_by_recording.get(track.recording_id, ())
        occurrence_items = tuple(
            sorted(
                (
                    item
                    for item in recording_items
                    if item.track == track.position
                ),
                key=lambda item: (
                    item.release_id,
                    item.medium,
                    item.item_id,
                    str(item.path),
                ),
            )
        )
        matches = tuple(
            selected_by_occurrence.get(
                (track.recording_id, track.position), ()
            )
        )
        count_is_valid = (
            selected_counts[track.recording_id]
            == expected_counts[track.recording_id]
        )
        if len(matches) != 1:
            if count_is_valid:
                errors.append(
                    f"expected track {track.position} recording "
                    f"{track.recording_id} has {len(matches)} post-import "
                    "items (expected exactly 1)"
                )
            if not matches:
                for item in occurrence_items:
                    _verify_selected_release(item, release, errors)
            for item in matches:
                _verify_new_item_id(
                    release.medium,
                    track.position,
                    track.recording_id,
                    item,
                    pre_ids_by_occurrence,
                    pre_item_ids,
                    errors,
                )
            continue

        item = matches[0]
        destination_paths.append(item.path)
        _verify_new_item_id(
            release.medium,
            track.position,
            track.recording_id,
            item,
            pre_ids_by_occurrence,
            pre_item_ids,
            errors,
        )

    return VerificationReport(tuple(errors), tuple(destination_paths))


def _verify_owned_source(source: OwnedMp3, errors: list[str]) -> None:
    try:
        details = source.path.lstat()
    except FileNotFoundError:
        errors.append(f"owned source is missing: {source.path}")
        return
    except OSError as error:
        errors.append(
            f"owned source cannot be inspected: {source.path}: {error.strerror}"
        )
        return

    if not stat.S_ISREG(details.st_mode):
        errors.append(f"owned source is not a regular file: {source.path}")
    if details.st_dev != source.device or details.st_ino != source.inode:
        errors.append(f"owned source identity changed: {source.path}")


def _resolve_root(
    root: Path, label: str, errors: list[str]
) -> Path | None:
    try:
        return root.resolve(strict=False)
    except (OSError, RuntimeError) as error:
        errors.append(f"{label} root cannot be resolved: {root}: {error}")
        return None


def _verify_selected_release(
    item: LibraryItem, release: ReleaseMetadata, errors: list[str]
) -> None:
    if item.release_id != release.release_id:
        errors.append(
            f"post item for recording {item.recording_id} has unexpected "
            f"release ID {item.release_id} (expected {release.release_id})"
        )
    if item.medium != release.medium:
        errors.append(
            f"post item for recording {item.recording_id} has unexpected "
            f"medium {item.medium} (expected {release.medium})"
        )


def _verify_new_item_id(
    medium: int,
    track_position: int,
    recording_id: str,
    item: LibraryItem,
    pre_ids_by_occurrence: dict[tuple[int, int, str], set[int]],
    pre_item_ids: set[int],
    errors: list[str],
) -> None:
    occurrence = (medium, track_position, recording_id)
    if occurrence not in pre_ids_by_occurrence:
        if item.item_id in pre_item_ids:
            errors.append(
                f"expected track {track_position} recording {recording_id} "
                "was absent before import but post item ID "
                f"{item.item_id} is not new"
            )
        return

    if (
        item.item_id in pre_item_ids
        and item.item_id not in pre_ids_by_occurrence[occurrence]
    ):
        errors.append(
            f"post item ID {item.item_id} for track {track_position} "
            f"recording {recording_id} belonged to a different pre-import "
            "occurrence"
        )


def _verify_destination(
    item: LibraryItem,
    resolved_staging: Path | None,
    resolved_library: Path | None,
    errors: list[str],
) -> None:
    try:
        resolved_destination = item.path.resolve(strict=False)
    except (OSError, RuntimeError) as error:
        errors.append(f"destination cannot be resolved: {item.path}: {error}")
        resolved_destination = None

    if resolved_destination is not None:
        if resolved_library is None or not _strictly_beneath(
            resolved_destination, resolved_library
        ):
            errors.append(
                f"destination is not strictly beneath library root: {item.path}"
            )
        if resolved_staging is not None and _beneath_or_equal(
            resolved_destination, resolved_staging
        ):
            errors.append(
                f"destination remains under staging root: {item.path}"
            )

    try:
        details = item.path.lstat()
    except FileNotFoundError:
        errors.append(f"destination is missing: {item.path}")
    except OSError as error:
        errors.append(
            f"destination cannot be inspected: {item.path}: {error.strerror}"
        )
    else:
        if stat.S_ISLNK(details.st_mode):
            errors.append(f"destination is a symlink: {item.path}")
        elif not stat.S_ISREG(details.st_mode):
            errors.append(f"destination is not a regular file: {item.path}")
        elif details.st_size == 0:
            errors.append(f"destination is empty: {item.path}")

    if item.format != "MP3":
        errors.append(
            f"destination format is not MP3 for recording "
            f"{item.recording_id}: {item.format}"
        )
    if not _MINIMUM_BITRATE <= item.bitrate <= _MAXIMUM_BITRATE:
        errors.append(
            "destination bitrate is outside 319000-321000 bps for recording "
            f"{item.recording_id}: {item.bitrate}"
        )


def _strictly_beneath(path: Path, root: Path) -> bool:
    return path != root and path.is_relative_to(root)


def _beneath_or_equal(path: Path, root: Path) -> bool:
    return path == root or path.is_relative_to(root)


def _verify_album_group(
    selected_items: tuple[LibraryItem, ...], errors: list[str]
) -> None:
    album_ids = {item.album_id for item in selected_items}
    if len(album_ids) == 1 and None not in album_ids:
        return

    found = ", ".join(
        "none" if album_id is None else str(album_id)
        for album_id in sorted(
            album_ids, key=lambda value: (value is not None, value or 0)
        )
    )
    errors.append(
        "selected post items must share one non-null album ID; "
        f"found {found or 'none'}"
    )

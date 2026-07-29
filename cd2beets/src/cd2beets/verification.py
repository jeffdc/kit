from __future__ import annotations

from collections import defaultdict
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
    for item in post_snapshot.exact_items:
        post_by_recording[item.recording_id].append(item)

    pre_items = pre_snapshot.exact_items + pre_snapshot.plausible_items
    pre_item_ids = {item.item_id for item in pre_items}
    pre_recordings = {item.recording_id for item in pre_items}

    resolved_staging = _resolve_root(staging_root, "staging", errors)
    resolved_library = _resolve_root(library_root, "library", errors)

    for track in sorted(release.tracks, key=lambda candidate: candidate.position):
        recording_items = post_by_recording.get(track.recording_id, ())
        occurrence_items = tuple(
            item for item in recording_items if item.track == track.position
        )
        matches = tuple(
            item
            for item in occurrence_items
            if item.release_id == release.release_id
            and item.medium == release.medium
        )
        if len(matches) != 1:
            errors.append(
                f"expected recording {track.recording_id} has {len(matches)} "
                "post-import items (expected exactly 1)"
            )
            if not matches:
                for item in occurrence_items:
                    _verify_selected_release(item, release, errors)
            for item in sorted(
                matches,
                key=lambda candidate: (
                    candidate.item_id,
                    str(candidate.path),
                ),
            ):
                _verify_destination(
                    item,
                    resolved_staging,
                    resolved_library,
                    errors,
                )
                _verify_new_item_id(
                    track.recording_id,
                    item,
                    pre_recordings,
                    pre_item_ids,
                    errors,
                )
            continue

        item = matches[0]
        destination_paths.append(item.path)
        _verify_destination(
            item,
            resolved_staging,
            resolved_library,
            errors,
        )
        _verify_new_item_id(
            track.recording_id,
            item,
            pre_recordings,
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
    recording_id: str,
    item: LibraryItem,
    pre_recordings: set[str],
    pre_item_ids: set[int],
    errors: list[str],
) -> None:
    if recording_id not in pre_recordings and item.item_id in pre_item_ids:
        errors.append(
            f"expected recording {recording_id} was absent before import but "
            f"post item ID {item.item_id} is not new"
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

from __future__ import annotations

from dataclasses import dataclass, replace
from pathlib import Path

import pytest

from cd2beets.models import (
    LibraryItem,
    LibrarySnapshot,
    OwnedMp3,
    ReleaseMetadata,
    TrackMetadata,
)
from cd2beets.verification import verify_import


@dataclass(frozen=True)
class Scenario:
    release: ReleaseMetadata
    pre_snapshot: LibrarySnapshot
    post_snapshot: LibrarySnapshot
    owned: tuple[OwnedMp3, ...]
    staging_root: Path
    library_root: Path


def make_scenario(tmp_path: Path) -> Scenario:
    staging_root = tmp_path / "staging"
    library_root = tmp_path / "library"
    staging_root.mkdir()
    library_root.mkdir()

    release = ReleaseMetadata(
        release_id="release-1",
        albumartist="Artist",
        album="Album",
        date="2001",
        country="US",
        disambiguation="",
        medium=1,
        tracks=(
            TrackMetadata(1, "recording-1", "One", 180_000),
            TrackMetadata(2, "recording-2", "Two", 190_000),
        ),
    )

    owned = tuple(make_owned(staging_root, position) for position in (1, 2))
    destinations = tuple(
        make_destination(library_root, position) for position in (1, 2)
    )
    items = (
        make_item(destinations[0], item_id=101, position=1),
        make_item(destinations[1], item_id=102, position=2),
    )
    return Scenario(
        release=release,
        pre_snapshot=LibrarySnapshot((), ()),
        post_snapshot=LibrarySnapshot(items, ()),
        owned=owned,
        staging_root=staging_root,
        library_root=library_root,
    )


def make_owned(staging_root: Path, position: int) -> OwnedMp3:
    path = staging_root / f"{position:02}.mp3"
    path.write_bytes(f"source-{position}".encode())
    details = path.lstat()
    return OwnedMp3(path, details.st_dev, details.st_ino, position)


def make_destination(library_root: Path, position: int) -> Path:
    path = library_root / f"{position:02}.mp3"
    path.write_bytes(f"destination-{position}".encode())
    return path


def make_item(
    path: Path,
    *,
    item_id: int,
    position: int,
    release_id: str = "release-1",
    recording_id: str | None = None,
    medium: int = 1,
    format: str = "MP3",
    bitrate: int = 320_000,
) -> LibraryItem:
    return LibraryItem(
        item_id=item_id,
        album_id=10,
        release_id=release_id,
        recording_id=recording_id or f"recording-{position}",
        albumartist="Artist",
        album="Album",
        medium=medium,
        track=position,
        tracktotal=2,
        path=path,
        format=format,
        bitrate=bitrate,
    )


def verify(scenario: Scenario):
    return verify_import(
        scenario.release,
        scenario.pre_snapshot,
        scenario.post_snapshot,
        scenario.owned,
        scenario.staging_root,
        scenario.library_root,
    )


def with_post_items(
    scenario: Scenario, items: tuple[LibraryItem, ...]
) -> Scenario:
    return replace(scenario, post_snapshot=LibrarySnapshot(items, ()))


def file_state(path: Path) -> tuple[Path, int, int, int, int, int, bytes]:
    details = path.lstat()
    return (
        path,
        details.st_dev,
        details.st_ino,
        details.st_mode,
        details.st_size,
        details.st_mtime_ns,
        path.read_bytes(),
    )


def test_complete_copy_to_valid_nas_mp3s_succeeds_without_mutation(tmp_path):
    scenario = make_scenario(tmp_path)
    tracked_paths = tuple(item.path for item in scenario.post_snapshot.exact_items) + tuple(
        source.path for source in scenario.owned
    )
    before = tuple(file_state(path) for path in tracked_paths)

    report = verify(scenario)

    after = tuple(file_state(path) for path in tracked_paths)
    assert report.ok is True
    assert report.errors == ()
    assert report.destination_paths == (
        scenario.library_root / "01.mp3",
        scenario.library_root / "02.mp3",
    )
    assert after == before


@pytest.mark.parametrize(
    ("album_id", "found"),
    [(None, "none, 10"), (11, "10, 11")],
)
def test_selected_post_items_share_one_non_null_album_id(
    tmp_path, album_id, found
):
    scenario = make_scenario(tmp_path)
    first = replace(
        scenario.post_snapshot.exact_items[0], album_id=album_id
    )
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "selected post items must share one non-null album ID; "
        f"found {found}"
    ) in report.errors


def test_destination_paths_are_ordered_by_expected_track_position(tmp_path):
    scenario = make_scenario(tmp_path)
    reversed_release = replace(
        scenario.release, tracks=tuple(reversed(scenario.release.tracks))
    )
    reversed_post = LibrarySnapshot(
        tuple(reversed(scenario.post_snapshot.exact_items)), ()
    )

    report = verify(
        replace(
            scenario,
            release=reversed_release,
            post_snapshot=reversed_post,
        )
    )

    assert report.ok is True
    assert report.destination_paths == (
        scenario.library_root / "01.mp3",
        scenario.library_root / "02.mp3",
    )


def test_existing_partial_track_may_retain_its_item_id(tmp_path):
    scenario = make_scenario(tmp_path)
    existing = replace(scenario.post_snapshot.exact_items[0])
    scenario = replace(
        scenario,
        pre_snapshot=LibrarySnapshot((existing,), ()),
    )

    report = verify(scenario)

    assert report.ok is True


def test_plausible_preexisting_recording_may_retain_its_item_id(tmp_path):
    scenario = make_scenario(tmp_path)
    plausible = replace(
        scenario.post_snapshot.exact_items[0], release_id="other-release"
    )
    scenario = replace(
        scenario,
        pre_snapshot=LibrarySnapshot((), (plausible,)),
    )

    report = verify(scenario)

    assert report.ok is True


def test_same_recording_on_another_medium_does_not_duplicate_selected_track(
    tmp_path,
):
    scenario = make_scenario(tmp_path)
    other_path = make_destination(scenario.library_root, 3)
    other_medium = replace(
        scenario.post_snapshot.exact_items[0],
        item_id=103,
        medium=2,
        path=other_path,
    )
    scenario = with_post_items(
        scenario,
        scenario.post_snapshot.exact_items + (other_medium,),
    )

    report = verify(scenario)

    assert report.ok is True
    assert report.destination_paths == (
        scenario.library_root / "01.mp3",
        scenario.library_root / "02.mp3",
    )


def test_repeated_recording_id_matches_distinct_expected_track_positions(
    tmp_path,
):
    scenario = make_scenario(tmp_path)
    repeated_release = replace(
        scenario.release,
        tracks=(
            scenario.release.tracks[0],
            replace(
                scenario.release.tracks[1], recording_id="recording-1"
            ),
        ),
    )
    second = replace(
        scenario.post_snapshot.exact_items[1],
        recording_id="recording-1",
    )
    scenario = replace(
        with_post_items(
            scenario, (scenario.post_snapshot.exact_items[0], second)
        ),
        release=repeated_release,
    )

    report = verify(scenario)

    assert report.ok is True
    assert report.destination_paths == (
        scenario.library_root / "01.mp3",
        scenario.library_root / "02.mp3",
    )


def test_missing_owned_source_proves_import_moved_instead_of_copied(tmp_path):
    scenario = make_scenario(tmp_path)
    source = scenario.owned[0].path
    source.rename(source.with_suffix(".moved"))

    report = verify(scenario)

    assert report.ok is False
    assert f"owned source is missing: {source}" in report.errors


@pytest.mark.parametrize("changed_identity", ["device", "inode"])
def test_owned_source_must_remain_the_same_device_and_inode(
    tmp_path, changed_identity
):
    scenario = make_scenario(tmp_path)
    source = scenario.owned[0].path
    if changed_identity == "device":
        changed = replace(
            scenario.owned[0], device=scenario.owned[0].device + 1
        )
        scenario = replace(scenario, owned=(changed,) + scenario.owned[1:])
    else:
        source.rename(source.with_suffix(".original"))
        source.write_bytes(b"replacement")

    report = verify(scenario)

    assert report.ok is False
    assert f"owned source identity changed: {source}" in report.errors


def test_owned_source_must_still_be_a_regular_file(tmp_path):
    scenario = make_scenario(tmp_path)
    source = scenario.owned[0].path
    source.rename(source.with_suffix(".original"))
    source.mkdir()

    report = verify(scenario)

    assert report.ok is False
    assert f"owned source is not a regular file: {source}" in report.errors


def test_owned_source_symlink_is_checked_with_lstat(tmp_path):
    scenario = make_scenario(tmp_path)
    source = scenario.owned[0].path
    original = source.with_suffix(".original")
    source.rename(original)
    source.symlink_to(original)

    report = verify(scenario)

    assert report.ok is False
    assert f"owned source is not a regular file: {source}" in report.errors
    assert f"owned source identity changed: {source}" in report.errors


@pytest.mark.parametrize("count", [0, 2])
def test_each_expected_recording_must_have_exactly_one_post_item(
    tmp_path, count
):
    scenario = make_scenario(tmp_path)
    second = scenario.post_snapshot.exact_items[1]
    matching = tuple(
        replace(
            scenario.post_snapshot.exact_items[0],
            item_id=200 + index,
            path=make_destination(scenario.library_root, 10 + index),
        )
        for index in range(count)
    )
    scenario = with_post_items(scenario, matching + (second,))

    report = verify(scenario)

    assert report.ok is False
    assert (
        f"expected recording recording-1 has {count} post-import items "
        "(expected exactly 1)"
    ) in report.errors


def test_duplicate_items_still_accumulate_independent_destination_errors(
    tmp_path,
):
    scenario = make_scenario(tmp_path)
    outside = tmp_path / "duplicate.mp3"
    outside.write_bytes(b"duplicate")
    duplicate = replace(
        scenario.post_snapshot.exact_items[0],
        item_id=201,
        path=outside,
        format="FLAC",
        bitrate=1,
    )
    scenario = with_post_items(
        scenario,
        (scenario.post_snapshot.exact_items[0], duplicate)
        + scenario.post_snapshot.exact_items[1:],
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "expected recording recording-1 has 2 post-import items "
        "(expected exactly 1)"
    ) in report.errors
    assert (
        f"destination is not strictly beneath library root: {outside}"
        in report.errors
    )
    assert (
        "destination format is not MP3 for recording recording-1: FLAC"
        in report.errors
    )
    assert (
        "destination bitrate is outside 319000-321000 bps for recording "
        "recording-1: 1"
    ) in report.errors


def test_recording_count_includes_duplicates_at_extra_positions(tmp_path):
    scenario = make_scenario(tmp_path)
    extra_path = make_destination(scenario.library_root, 9)
    extra = replace(
        scenario.post_snapshot.exact_items[0],
        item_id=109,
        track=9,
        path=extra_path,
    )
    scenario = with_post_items(
        scenario, scenario.post_snapshot.exact_items + (extra,)
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "expected recording recording-1 has 2 post-import items "
        "(expected exactly 1)"
    ) in report.errors
    assert (
        "unexpected selected post item recording recording-1 at track 9"
        in report.errors
    )


@pytest.mark.parametrize(
    ("change", "expected"),
    [
        (
            {"track": 9},
            "unexpected selected post item recording recording-1 at track 9",
        ),
        (
            {"recording_id": "unexpected", "track": 3},
            "unexpected selected post item recording unexpected at track 3",
        ),
    ],
)
def test_unmatched_selected_recording_or_track_position_fails(
    tmp_path, change, expected
):
    scenario = make_scenario(tmp_path)
    first = replace(scenario.post_snapshot.exact_items[0], **change)
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert expected in report.errors


def test_destination_must_not_remain_under_staging(tmp_path):
    scenario = make_scenario(tmp_path)
    first = replace(
        scenario.post_snapshot.exact_items[0], path=scenario.owned[0].path
    )
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        f"destination remains under staging root: {scenario.owned[0].path}"
        in report.errors
    )


def test_resolved_destination_must_not_enter_staging_via_parent_symlink(
    tmp_path,
):
    scenario = make_scenario(tmp_path)
    staging_alias = tmp_path / "staging-alias"
    staging_alias.symlink_to(scenario.staging_root, target_is_directory=True)
    apparent_destination = staging_alias / scenario.owned[0].path.name
    first = replace(
        scenario.post_snapshot.exact_items[0], path=apparent_destination
    )
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        f"destination remains under staging root: {apparent_destination}"
        in report.errors
    )


def test_destination_must_be_strictly_beneath_library_root(tmp_path):
    scenario = make_scenario(tmp_path)
    outside = tmp_path / "outside.mp3"
    outside.write_bytes(b"outside")
    first = replace(scenario.post_snapshot.exact_items[0], path=outside)
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        f"destination is not strictly beneath library root: {outside}"
        in report.errors
    )


def test_resolved_destination_cannot_escape_library_via_parent_symlink(
    tmp_path,
):
    scenario = make_scenario(tmp_path)
    outside = tmp_path / "outside"
    outside.mkdir()
    escape = scenario.library_root / "escape"
    escape.symlink_to(outside, target_is_directory=True)
    apparent_destination = escape / "01.mp3"
    apparent_destination.write_bytes(b"escaped")
    first = replace(
        scenario.post_snapshot.exact_items[0], path=apparent_destination
    )
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "destination is not strictly beneath library root: "
        f"{apparent_destination}"
    ) in report.errors


@pytest.mark.parametrize("kind", ["missing", "symlink", "empty", "directory"])
def test_destination_must_be_present_non_symlink_non_empty_regular_file(
    tmp_path, kind
):
    scenario = make_scenario(tmp_path)
    path = scenario.library_root / f"invalid-{kind}.mp3"
    if kind == "symlink":
        path.symlink_to(scenario.post_snapshot.exact_items[0].path)
        expected = f"destination is a symlink: {path}"
    elif kind == "empty":
        path.touch()
        expected = f"destination is empty: {path}"
    elif kind == "directory":
        path.mkdir()
        expected = f"destination is not a regular file: {path}"
    else:
        expected = f"destination is missing: {path}"

    first = replace(scenario.post_snapshot.exact_items[0], path=path)
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert expected in report.errors


@pytest.mark.parametrize("format", ["FLAC", "mp3"])
def test_destination_format_must_be_exactly_mp3(tmp_path, format):
    scenario = make_scenario(tmp_path)
    first = replace(scenario.post_snapshot.exact_items[0], format=format)
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        f"destination format is not MP3 for recording recording-1: {format}"
        in report.errors
    )


@pytest.mark.parametrize("bitrate", [318_999, 321_001])
def test_destination_bitrate_must_be_within_explicit_tolerance(
    tmp_path, bitrate
):
    scenario = make_scenario(tmp_path)
    first = replace(scenario.post_snapshot.exact_items[0], bitrate=bitrate)
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "destination bitrate is outside 319000-321000 bps for recording "
        f"recording-1: {bitrate}"
    ) in report.errors


@pytest.mark.parametrize("bitrate", [319_000, 321_000])
def test_destination_bitrate_boundaries_are_inclusive(tmp_path, bitrate):
    scenario = make_scenario(tmp_path)
    first = replace(
        scenario.post_snapshot.exact_items[0], bitrate=bitrate
    )
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is True


def test_pre_absent_recording_must_have_a_new_post_item_id(tmp_path):
    scenario = make_scenario(tmp_path)
    preexisting = scenario.post_snapshot.exact_items[0]
    reused = replace(scenario.post_snapshot.exact_items[1], item_id=preexisting.item_id)
    scenario = replace(
        with_post_items(scenario, (preexisting, reused)),
        pre_snapshot=LibrarySnapshot((preexisting,), ()),
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "expected track 2 recording recording-2 was absent before import "
        "but post item ID 101 is not new"
    ) in report.errors


@pytest.mark.parametrize("change", [{"medium": 2}, {"track": 9}])
def test_preexisting_recording_at_other_occurrence_does_not_excuse_new_id(
    tmp_path, change
):
    scenario = make_scenario(tmp_path)
    wrong_occurrence = replace(
        scenario.post_snapshot.exact_items[0], **change
    )
    scenario = replace(
        scenario,
        pre_snapshot=LibrarySnapshot((wrong_occurrence,), ()),
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "expected track 1 recording recording-1 was absent before import "
        "but post item ID 101 is not new"
    ) in report.errors


def test_repeated_recording_id_newness_is_proved_per_track_occurrence(
    tmp_path,
):
    scenario = make_scenario(tmp_path)
    repeated_release = replace(
        scenario.release,
        tracks=(
            scenario.release.tracks[0],
            replace(
                scenario.release.tracks[1], recording_id="recording-1"
            ),
        ),
    )
    preexisting = scenario.post_snapshot.exact_items[0]
    reused = replace(
        scenario.post_snapshot.exact_items[1],
        item_id=preexisting.item_id,
        recording_id="recording-1",
    )
    scenario = replace(
        with_post_items(scenario, (preexisting, reused)),
        release=repeated_release,
        pre_snapshot=LibrarySnapshot((preexisting,), ()),
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "expected track 2 recording recording-1 was absent before import "
        "but post item ID 101 is not new"
    ) in report.errors


@pytest.mark.parametrize(
    ("change", "expected"),
    [
        (
            {"release_id": "other-release"},
            "post item for recording recording-1 has unexpected release ID "
            "other-release (expected release-1)",
        ),
        (
            {"medium": 2},
            "post item for recording recording-1 has unexpected medium 2 "
            "(expected 1)",
        ),
    ],
)
def test_post_item_must_match_selected_release_and_medium(
    tmp_path, change, expected
):
    scenario = make_scenario(tmp_path)
    first = replace(scenario.post_snapshot.exact_items[0], **change)
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.ok is False
    assert expected in report.errors


def test_wrong_release_in_plausible_post_partition_is_diagnosed(tmp_path):
    scenario = make_scenario(tmp_path)
    wrong_release = replace(
        scenario.post_snapshot.exact_items[0],
        release_id="other-release",
    )
    scenario = replace(
        scenario,
        post_snapshot=LibrarySnapshot(
            (scenario.post_snapshot.exact_items[1],),
            (wrong_release,),
        ),
    )

    report = verify(scenario)

    assert report.ok is False
    assert (
        "post item for recording recording-1 has unexpected release ID "
        "other-release (expected release-1)"
    ) in report.errors


def test_verification_accumulates_failures_in_deterministic_order(tmp_path):
    scenario = make_scenario(tmp_path)
    missing_source = scenario.owned[0].path
    missing_source.rename(missing_source.with_suffix(".moved"))
    first = replace(
        scenario.post_snapshot.exact_items[0], format="FLAC", bitrate=1
    )
    scenario = with_post_items(
        scenario, (first, scenario.post_snapshot.exact_items[1])
    )

    report = verify(scenario)

    assert report.errors == (
        f"owned source is missing: {missing_source}",
        "destination format is not MP3 for recording recording-1: FLAC",
        "destination bitrate is outside 319000-321000 bps for recording "
        "recording-1: 1",
    )

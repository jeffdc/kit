from __future__ import annotations

from dataclasses import FrozenInstanceError
from pathlib import Path
from types import SimpleNamespace

import pytest

from cd2beets.library import (
    BeetsLibrary,
    BeetsSettings,
    classify_presence,
)
from cd2beets.models import (
    LibraryItem,
    LibrarySnapshot,
    Presence,
    ReleaseMetadata,
    TrackMetadata,
)


def release() -> ReleaseMetadata:
    return ReleaseMetadata(
        release_id="release-1",
        albumartist="Beyoncé",
        album="The Album!",
        date="2001",
        country="US",
        disambiguation="",
        medium=1,
        tracks=(
            TrackMetadata(1, "recording-1", "One", 180_000),
            TrackMetadata(2, "recording-2", "Two", 190_000),
        ),
    )


def item(
    path: Path,
    *,
    item_id: int = 1,
    release_id: str = "release-1",
    recording_id: str = "recording-1",
    albumartist: str = "Beyonce",
    album: str = "The Album",
    medium: int = 1,
    track: int = 1,
    tracktotal: int = 2,
    album_id: int | None = 10,
) -> LibraryItem:
    return LibraryItem(
        item_id=item_id,
        album_id=album_id,
        release_id=release_id,
        recording_id=recording_id,
        albumartist=albumartist,
        album=album,
        medium=medium,
        track=track,
        tracktotal=tracktotal,
        path=path,
        format="MP3",
        bitrate=320_000,
    )


def complete_items(root: Path) -> tuple[LibraryItem, LibraryItem]:
    first = root / "01.mp3"
    second = root / "02.mp3"
    first.write_bytes(b"one")
    second.write_bytes(b"two")
    return (
        item(first),
        item(second, item_id=2, recording_id="recording-2", track=2),
    )


def classify(
    metadata: ReleaseMetadata,
    root: Path,
    *,
    exact: tuple[LibraryItem, ...] = (),
    plausible: tuple[LibraryItem, ...] = (),
):
    snapshot = LibrarySnapshot(exact, plausible)
    return classify_presence(metadata, snapshot, root)


def test_complete_exact_release_is_present(tmp_path):
    result = classify(release(), tmp_path, exact=complete_items(tmp_path))

    assert result.presence is Presence.PRESENT
    assert result.reasons == ()
    assert result.snapshot.exact_items == complete_items_from_paths(tmp_path)


def complete_items_from_paths(root: Path) -> tuple[LibraryItem, LibraryItem]:
    return (
        item(root / "01.mp3"),
        item(
            root / "02.mp3",
            item_id=2,
            recording_id="recording-2",
            track=2,
        ),
    )


def test_no_exact_or_normalized_title_candidate_is_absent(tmp_path):
    result = classify(release(), tmp_path)

    assert result.presence is Presence.ABSENT
    assert result.reasons == ()


def test_exact_release_missing_recording_is_uncertain(tmp_path):
    exact = complete_items(tmp_path)[:1]

    result = classify(release(), tmp_path, exact=exact)

    assert result.presence is Presence.UNCERTAIN
    assert "missing recordings: recording-2" in result.reasons


def test_exact_release_duplicate_recording_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    duplicate_path = tmp_path / "01-duplicate.mp3"
    duplicate_path.write_bytes(b"duplicate")
    duplicate = item(duplicate_path, item_id=3)

    result = classify(release(), tmp_path, exact=(first, duplicate, second))

    assert result.presence is Presence.UNCERTAIN
    assert "duplicate recordings: recording-1" in result.reasons


def test_exact_release_missing_file_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    first.path.unlink()

    result = classify(release(), tmp_path, exact=(first, second))

    assert result.presence is Presence.UNCERTAIN
    assert f"missing path: {first.path}" in result.reasons


def test_exact_release_file_outside_root_is_uncertain(tmp_path):
    root = tmp_path / "library"
    root.mkdir()
    first, second = complete_items(root)
    outside = tmp_path / "outside.mp3"
    outside.write_bytes(b"outside")
    first = item(outside)

    result = classify(release(), root, exact=(first, second))

    assert result.presence is Presence.UNCERTAIN
    assert f"path outside library root: {outside}" in result.reasons


@pytest.mark.parametrize("candidate_release_id", ["", "release-2"])
def test_normalized_artist_title_candidate_is_uncertain(
    tmp_path, candidate_release_id
):
    path = tmp_path / "candidate.mp3"
    path.write_bytes(b"candidate")
    candidate = item(
        path,
        release_id=candidate_release_id,
        albumartist="BEYONCE",
        album="the album",
    )

    result = classify(release(), tmp_path, plausible=(candidate,))

    assert result.presence is Presence.UNCERTAIN
    assert result.reasons == (
        "plausible artist/title candidate has a different or missing release ID",
    )


def test_exact_release_on_different_medium_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    exact = (
        item(first.path, medium=2),
        item(
            second.path,
            item_id=2,
            recording_id="recording-2",
            medium=2,
            track=2,
        ),
    )

    result = classify(release(), tmp_path, exact=exact)

    assert result.presence is Presence.UNCERTAIN
    assert "exact release has no items for medium 1" in result.reasons


def test_complete_selected_medium_ignores_other_release_media(tmp_path):
    selected = complete_items(tmp_path)
    other_path = tmp_path / "disc-2-01.mp3"
    other_path.write_bytes(b"other medium")
    other_medium = item(
        other_path,
        item_id=3,
        recording_id="recording-on-disc-2",
        medium=2,
        tracktotal=1,
    )

    result = classify(
        release(), tmp_path, exact=(*selected, other_medium)
    )

    assert result.presence is Presence.PRESENT
    assert result.reasons == ()


def test_selected_medium_split_across_beets_albums_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    split = (
        first,
        item(
            second.path,
            item_id=2,
            album_id=20,
            recording_id="recording-2",
            track=2,
        ),
    )

    result = classify(release(), tmp_path, exact=split)

    assert result.presence is Presence.UNCERTAIN
    assert "selected medium split across Beets albums: 10, 20" in result.reasons


def test_selected_medium_missing_beets_album_identity_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    unidentified = (
        item(first.path, album_id=None),
        item(
            second.path,
            item_id=2,
            album_id=None,
            recording_id="recording-2",
            track=2,
        ),
    )

    result = classify(release(), tmp_path, exact=unidentified)

    assert result.presence is Presence.UNCERTAIN
    assert "selected medium is missing Beets album identity" in result.reasons


def test_exact_release_with_inconsistent_track_totals_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    exact = (
        first,
        item(
            second.path,
            item_id=2,
            recording_id="recording-2",
            track=2,
            tracktotal=3,
        ),
    )

    result = classify(release(), tmp_path, exact=exact)

    assert result.presence is Presence.UNCERTAIN
    assert "inconsistent track totals: 2, 3" in result.reasons


def test_exact_release_with_wrong_uniform_track_total_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    exact = (
        item(first.path, tracktotal=3),
        item(
            second.path,
            item_id=2,
            recording_id="recording-2",
            track=2,
            tracktotal=3,
        ),
    )

    result = classify(release(), tmp_path, exact=exact)

    assert result.presence is Presence.UNCERTAIN
    assert "track total mismatch: expected 2; found 3" in result.reasons


def test_exact_release_with_recordings_on_wrong_tracks_is_uncertain(tmp_path):
    first, second = complete_items(tmp_path)
    exact = (
        item(first.path, track=2),
        item(
            second.path,
            item_id=2,
            recording_id="recording-2",
            track=1,
        ),
    )

    result = classify(release(), tmp_path, exact=exact)

    assert result.presence is Presence.UNCERTAIN
    assert "recording/track positions do not match expected medium" in result.reasons


def test_plausible_candidate_keeps_valid_exact_release_uncertain(tmp_path):
    exact = complete_items(tmp_path)
    candidate_path = tmp_path / "candidate.mp3"
    candidate_path.write_bytes(b"candidate")
    candidate = item(candidate_path, release_id="release-2")

    result = classify(release(), tmp_path, exact=exact, plausible=(candidate,))

    assert result.presence is Presence.UNCERTAIN
    assert (
        "plausible artist/title candidate has a different or missing release ID"
        in result.reasons
    )


class ConfigValue:
    def __init__(self, value, expected_method: str | None = None):
        self.value = value
        self.expected_method = expected_method
        self.children: dict[str, ConfigValue] = {}

    def __getitem__(self, key: str) -> ConfigValue:
        return self.children[key]

    def as_filename(self):
        assert self.expected_method == "as_filename"
        return self.value

    def as_str_seq(self):
        assert self.expected_method == "as_str_seq"
        return self.value

    def get(self, value_type):
        assert self.expected_method == f"get({value_type.__name__})"
        return self.value

    def add(self, values):
        for key, value in values.items():
            self.children.setdefault(key, ConfigValue(value))


def config_tree(tmp_path: Path) -> ConfigValue:
    root = ConfigValue(None)
    root.children = {
        "library": ConfigValue(
            str(tmp_path / "library.db"), "as_filename"
        ),
        "directory": ConfigValue(str(tmp_path / "music"), "as_filename"),
        "plugins": ConfigValue(["musicbrainz", "duplicates"], "as_str_seq"),
        "musicbrainz": ConfigValue(None),
        "import": ConfigValue(None),
        "duplicates": ConfigValue(None),
    }
    root.children["musicbrainz"].children = {
        "host": ConfigValue("musicbrainz.internal:5000", "get(str)")
    }
    root.children["import"].children = {
        "duplicate_action": ConfigValue("skip", "get(str)")
    }
    root.children["duplicates"].children = {
        "delete": ConfigValue(False, "get(bool)"),
        "remove": ConfigValue(True, "get(bool)"),
        "move": ConfigValue("/quarantine", "get(str)"),
    }
    return root


def test_settings_loads_active_beets_configuration(monkeypatch, tmp_path):
    from cd2beets import library as library_module

    calls = []
    monkeypatch.setattr(
        library_module.ui,
        "load_config",
        lambda: calls.append("load"),
        raising=False,
    )
    monkeypatch.setattr(library_module, "config", config_tree(tmp_path))

    settings = BeetsSettings.load()

    assert calls == ["load"]
    assert settings == BeetsSettings(
        library_path=tmp_path / "library.db",
        library_root=tmp_path / "music",
        musicbrainz_host="musicbrainz.internal:5000",
        plugins=("musicbrainz", "duplicates"),
        duplicate_action="skip",
        duplicates_delete=False,
        duplicates_remove=True,
        duplicates_move="/quarantine",
    )
    with pytest.raises(FrozenInstanceError):
        settings.library_path = tmp_path / "other.db"


def test_settings_loads_real_beets_plugin_defaults(monkeypatch, tmp_path):
    from beets import IncludeLazyConfig

    from cd2beets import library as library_module

    config_dir = tmp_path / "config"
    config_dir.mkdir()
    (config_dir / "config.yaml").write_text(
        "\n".join(
            (
                f"library: {tmp_path / 'active.db'}",
                f"directory: {tmp_path / 'music'}",
                "plugins: [musicbrainz, duplicates]",
                "import:",
                "    duplicate_action: ask",
            )
        )
    )
    monkeypatch.setenv("BEETSDIR", str(config_dir))
    active_config = IncludeLazyConfig("beets", "beets")
    monkeypatch.setattr(library_module, "config", active_config)
    monkeypatch.delattr(library_module.ui, "load_config", raising=False)

    loaded = BeetsSettings.load()

    assert loaded.library_path == tmp_path / "active.db"
    assert loaded.library_root == tmp_path / "music"
    assert loaded.musicbrainz_host == "musicbrainz.org"
    assert loaded.plugins == ("musicbrainz", "duplicates")
    assert loaded.duplicate_action == "ask"
    assert loaded.duplicates_delete is False
    assert loaded.duplicates_remove is False
    assert loaded.duplicates_move == ""


class RawItem:
    def __init__(
        self,
        item_id: int,
        path: bytes,
        *,
        release_id: str,
        recording_id: str,
        albumartist: str,
        album: str,
        album_id: int | None = 10,
    ):
        self.id = item_id
        self.album_id = album_id
        self.mb_albumid = release_id
        self.mb_trackid = recording_id
        self.albumartist = albumartist
        self.album = album
        self.disc = 1
        self.track = item_id
        self.tracktotal = 2
        self.path = path
        self.format = "MP3"
        self.bitrate = 320_000

    def store(self):
        raise AssertionError("refresh must never mutate an item")

    def remove(self, *args, **kwargs):
        raise AssertionError("refresh must never mutate an item")

    def move(self, *args, **kwargs):
        raise AssertionError("refresh must never mutate an item")


class ReadOnlyCatalog:
    def __init__(self, raw_items):
        self.raw_items = raw_items
        self.reads = 0

    def items(self):
        self.reads += 1
        return tuple(self.raw_items)

    def add(self, *args, **kwargs):
        raise AssertionError("catalog must never add")


def settings(tmp_path: Path) -> BeetsSettings:
    return BeetsSettings(
        library_path=tmp_path / "library.db",
        library_root=tmp_path,
        musicbrainz_host="musicbrainz.org",
        plugins=("musicbrainz",),
        duplicate_action="ask",
        duplicates_delete=False,
        duplicates_remove=False,
        duplicates_move="",
    )


def test_catalog_opens_read_only_snapshot_and_refreshes_only_when_requested(
    monkeypatch, tmp_path
):
    from cd2beets import library as library_module

    first_path = tmp_path / "01.mp3"
    second_path = tmp_path / "02.mp3"
    first_path.write_bytes(b"one")
    second_path.write_bytes(b"two")
    raw_items = [
        RawItem(
            1,
            bytes(first_path),
            release_id="release-1",
            recording_id="recording-1",
            albumartist="Beyoncé",
            album="The Album!",
        )
    ]
    catalog = ReadOnlyCatalog(raw_items)
    opened_paths = []

    def open_library(path):
        opened_paths.append(path)
        return catalog

    syspath_calls = []

    def syspath(path):
        syspath_calls.append(path)
        return path.decode()

    monkeypatch.setattr(library_module, "Library", open_library)
    monkeypatch.setattr(library_module.util, "syspath", syspath)

    adapter = BeetsLibrary(settings(tmp_path))
    initial = adapter.snapshot(release())
    raw_items.append(
        RawItem(
            2,
            bytes(second_path),
            release_id="release-1",
            recording_id="recording-2",
            albumartist="Beyonce",
            album="The Album",
        )
    )
    cached = adapter.snapshot(release())

    assert opened_paths == [str(tmp_path / "library.db")]
    assert catalog.reads == 1
    assert initial == cached
    assert initial.exact_items == (
        item(first_path, albumartist="Beyoncé", album="The Album!"),
    )
    assert syspath_calls == [bytes(first_path)]

    adapter.refresh()
    refreshed = adapter.snapshot(release())

    assert catalog.reads == 2
    assert tuple(value.item_id for value in refreshed.exact_items) == (1, 2)
    assert syspath_calls == [bytes(first_path), bytes(first_path), bytes(second_path)]


def test_catalog_indexes_normalized_plausible_candidates(monkeypatch, tmp_path):
    from cd2beets import library as library_module

    path = tmp_path / "candidate.mp3"
    path.write_bytes(b"candidate")
    catalog = ReadOnlyCatalog(
        [
            RawItem(
                1,
                bytes(path),
                release_id="release-2",
                recording_id="recording-1",
                albumartist="BEYONCE",
                album="the album",
            )
        ]
    )
    monkeypatch.setattr(library_module, "Library", lambda _: catalog)
    monkeypatch.setattr(library_module.util, "syspath", lambda value: value.decode())

    snapshot = BeetsLibrary(settings(tmp_path)).snapshot(release())

    assert snapshot.exact_items == ()
    assert len(snapshot.plausible_items) == 1
    assert snapshot.plausible_items[0].release_id == "release-2"

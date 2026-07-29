import stat
from pathlib import Path
from types import SimpleNamespace

import pytest

from cd2beets.drive import DriveInfo
from cd2beets.library import BeetsSettings
from cd2beets.preflight import Preflight


DRIVE = DriveInfo(1, "hp", "CDDVDW TS-L633R", "0200", "USB", "Unsupported")


class FakeVersionProbe:
    def __init__(self, installed="2.8.0", error=None):
        self.installed = installed
        self.error = error
        self.calls = []

    def __call__(self, distribution):
        self.calls.append(distribution)
        if self.error is not None:
            raise self.error
        return self.installed


class FakeDrive:
    def __init__(self, drives=(DRIVE,), error=None):
        self.drives = drives
        self.error = error
        self.calls = []

    def list_drives(self):
        self.calls.append("list")
        if self.error is not None:
            raise self.error
        return self.drives


class FakeMetadata:
    def __init__(self, error=None):
        self.error = error
        self.calls = []

    def ping(self):
        self.calls.append("ping")
        if self.error is not None:
            raise self.error


def settings(tmp_path, **overrides):
    values = {
        "library_path": tmp_path / "library.db",
        "library_root": tmp_path / "nas" / "library",
        "musicbrainz_host": "musicbrainz.org",
        "plugins": ("musicbrainz", "duplicates", "missing"),
        "duplicate_action": "skip",
        "duplicates_delete": False,
        "duplicates_remove": False,
        "duplicates_move": "",
    }
    values.update(overrides)
    return BeetsSettings(**values)


def make_preflight(tmp_path, *, configured=None, existing=None, writable=None, readable=True,
                   devices=None, modes=None, which=None, version_probe=None, drive=None,
                   metadata=None):
    configured = configured or settings(tmp_path)
    staging = tmp_path / "staging"
    library_path = configured.library_path.resolve()
    library_root = configured.library_root.resolve()
    root = Path("/").resolve()
    existing = {library_path, library_root, root} if existing is None else set(existing)
    writable = {library_root} if writable is None else set(writable)
    devices = {library_path: 20, library_root: 20, root: 10} if devices is None else devices
    modes = {
        library_path: stat.S_IFREG,
        library_root: stat.S_IFDIR,
        root: stat.S_IFDIR,
    } if modes is None else modes

    def fake_stat(path):
        resolved = Path(path).resolve()
        if resolved not in existing:
            raise FileNotFoundError(resolved)
        return SimpleNamespace(
            st_dev=devices.get(resolved, 20),
            st_mode=modes.get(resolved, stat.S_IFREG),
        )

    def fake_access(path, mode):
        resolved = Path(path).resolve()
        if mode == 4:
            return readable and resolved == library_path
        if mode == 2:
            return resolved in writable
        return False

    return Preflight(
        configured,
        staging,
        drive or FakeDrive(),
        metadata or FakeMetadata(),
        which=which or (lambda name: f"/usr/bin/{name}"),
        version_probe=version_probe or FakeVersionProbe(),
        stat_path=fake_stat,
        access=fake_access,
    )


def error_text(report):
    return "\n".join(report.errors)


@pytest.mark.parametrize("binary", ["drutil", "cdda2wav", "lame", "beet", "pgrep"])
def test_missing_prerequisite_is_named(tmp_path, binary):
    preflight = make_preflight(
        tmp_path,
        which=lambda name: None if name == binary else f"/usr/bin/{name}",
    )

    report = preflight.run()

    assert f"missing prerequisite: {binary}" in report.errors


@pytest.mark.parametrize("installed", ["2.7.0", "2.8.1", "2.8.0.dev1", "unexpected"])
def test_installed_beets_distribution_must_be_exactly_2_8_0(
    tmp_path, installed
):
    report = make_preflight(
        tmp_path, version_probe=FakeVersionProbe(installed)
    ).run()

    assert any("beets version" in error and "2.8.0" in error for error in report.errors)


def test_beets_version_probe_is_read_only_and_does_not_start_beet_cli(tmp_path):
    version_probe = FakeVersionProbe()

    report = make_preflight(tmp_path, version_probe=version_probe).run()

    assert report.errors == ()
    assert version_probe.calls == ["beets"]


def test_unreadable_library_database_is_named(tmp_path):
    report = make_preflight(tmp_path, readable=False).run()

    assert "library database is unreadable" in error_text(report)


@pytest.mark.parametrize(
    ("failure", "message"),
    [
        ("absent", "NAS library root is absent"),
        ("unwritable", "NAS library root is unwritable"),
        ("root_device", "NAS library root is not a separate mounted filesystem"),
    ],
)
def test_nas_root_failures_are_named(tmp_path, failure, message):
    configured = settings(tmp_path)
    library_path = configured.library_path.resolve()
    library_root = configured.library_root.resolve()
    root = Path("/").resolve()
    existing = {library_path, library_root, root}
    writable = {library_root}
    devices = {library_path: 20, library_root: 20, root: 10}
    if failure == "absent":
        existing.remove(library_root)
    elif failure == "unwritable":
        writable.clear()
    else:
        devices[library_root] = devices[root]

    report = make_preflight(
        tmp_path,
        configured=configured,
        existing=existing,
        writable=writable,
        devices=devices,
    ).run()

    assert message in error_text(report)

def test_nas_root_must_be_a_directory(tmp_path):
    configured = settings(tmp_path)
    library_path = configured.library_path.resolve()
    library_root = configured.library_root.resolve()
    root = Path("/").resolve()
    modes = {
        library_path: stat.S_IFREG,
        library_root: stat.S_IFREG,
        root: stat.S_IFDIR,
    }

    report = make_preflight(
        tmp_path,
        configured=configured,
        modes=modes,
    ).run()

    assert "NAS library root is not a directory" in error_text(report)


def test_staging_must_not_be_nested_under_library_root(tmp_path):
    configured = settings(tmp_path)
    preflight = make_preflight(tmp_path, configured=configured)
    preflight.staging_dir = configured.library_root / "incoming"

    report = preflight.run()

    assert "staging directory is inside the library root" in error_text(report)


@pytest.mark.parametrize("drives", [(), (DRIVE, DRIVE)])
def test_preflight_requires_exactly_one_optical_drive(tmp_path, drives):
    report = make_preflight(tmp_path, drive=FakeDrive(drives=drives)).run()

    assert "expected exactly one optical drive" in error_text(report)


def test_drive_listing_failure_is_named(tmp_path):
    report = make_preflight(
        tmp_path, drive=FakeDrive(error=RuntimeError("drutil list failed"))
    ).run()

    assert "optical drive check failed: drutil list failed" in report.errors


def test_unknown_active_plugins_are_rejected(tmp_path):
    configured = settings(tmp_path, plugins=("musicbrainz", "lastgenre"))

    report = make_preflight(tmp_path, configured=configured).run()

    assert "unknown active beets plugin: lastgenre" in report.errors


def test_duplicate_action_must_be_skip(tmp_path):
    configured = settings(tmp_path, duplicate_action="merge")

    report = make_preflight(tmp_path, configured=configured).run()

    assert "beets import duplicate_action must be 'skip'" in error_text(report)


@pytest.mark.parametrize(
    ("overrides", "message"),
    [
        ({"duplicates_delete": True}, "duplicates delete must be disabled"),
        ({"duplicates_remove": True}, "duplicates remove must be disabled"),
        ({"duplicates_move": "/tmp/duplicates"}, "duplicates move must be disabled"),
    ],
)
def test_destructive_duplicates_configuration_is_rejected(tmp_path, overrides, message):
    configured = settings(tmp_path, **overrides)

    report = make_preflight(tmp_path, configured=configured).run()

    assert message in error_text(report)


def test_musicbrainz_ping_failure_is_named(tmp_path):
    report = make_preflight(
        tmp_path, metadata=FakeMetadata(RuntimeError("connection refused"))
    ).run()

    assert "MusicBrainz ping failed: connection refused" in report.errors


def test_failures_are_aggregated_in_one_report(tmp_path):
    configured = settings(
        tmp_path,
        plugins=("dangerous",),
        duplicate_action="merge",
        duplicates_delete=True,
    )
    report = make_preflight(
        tmp_path,
        configured=configured,
        readable=False,
        writable=set(),
        drive=FakeDrive(drives=()),
        metadata=FakeMetadata(RuntimeError("offline")),
        which=lambda name: None,
    ).run()

    assert len(report.errors) >= 10
    assert "missing prerequisite: drutil" in report.errors
    assert "MusicBrainz ping failed: offline" in report.errors


def test_success_returns_resolved_paths_and_selected_drive_without_mutation(tmp_path):
    version_probe = FakeVersionProbe()
    drive = FakeDrive()
    metadata = FakeMetadata()
    preflight = make_preflight(
        tmp_path,
        version_probe=version_probe,
        drive=drive,
        metadata=metadata,
    )

    report = preflight.run()

    assert report.errors == ()
    assert report.library_path == preflight.settings.library_path.resolve()
    assert report.library_root == preflight.settings.library_root.resolve()
    assert report.staging_dir == preflight.staging_dir.resolve()
    assert report.drive == DRIVE
    assert version_probe.calls == ["beets"]
    assert drive.calls == ["list"]
    assert metadata.calls == ["ping"]
    assert not preflight.staging_dir.exists()

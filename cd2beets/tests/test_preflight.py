from pathlib import Path
from types import SimpleNamespace

import pytest

from cd2beets.drive import DriveInfo
from cd2beets.library import BeetsSettings
from cd2beets.preflight import Preflight
from cd2beets.process import CommandResult


DRIVE = DriveInfo(1, "hp", "CDDVDW TS-L633R", "0200", "USB", "Unsupported")


class FakeRunner:
    def __init__(self, result=CommandResult(0, "beets version 2.8.0\n", "")):
        self.result = result
        self.calls = []

    def run(self, args):
        self.calls.append(list(args))
        return self.result


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
                   devices=None, which=None, runner=None, drive=None, metadata=None):
    configured = configured or settings(tmp_path)
    staging = tmp_path / "staging"
    library_path = configured.library_path.resolve()
    library_root = configured.library_root.resolve()
    root = Path("/").resolve()
    existing = {library_path, library_root, root} if existing is None else set(existing)
    writable = {library_root} if writable is None else set(writable)
    devices = {library_path: 20, library_root: 20, root: 10} if devices is None else devices

    def fake_stat(path):
        resolved = Path(path).resolve()
        if resolved not in existing:
            raise FileNotFoundError(resolved)
        return SimpleNamespace(st_dev=devices.get(resolved, 20))

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
        runner or FakeRunner(),
        drive or FakeDrive(),
        metadata or FakeMetadata(),
        which=which or (lambda name: f"/usr/bin/{name}"),
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


@pytest.mark.parametrize(
    "output",
    [
        "beets version 2.7.0\n",
        "beets version 2.8.1\n",
        "beets version 2.8.0.dev1\n",
        "unexpected output\n",
    ],
)
def test_beets_version_must_be_exactly_2_8_0(tmp_path, output):
    report = make_preflight(
        tmp_path, runner=FakeRunner(CommandResult(0, output, ""))
    ).run()

    assert any("beets version" in error and "2.8.0" in error for error in report.errors)


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
    runner = FakeRunner()
    drive = FakeDrive()
    metadata = FakeMetadata()
    preflight = make_preflight(
        tmp_path, runner=runner, drive=drive, metadata=metadata
    )

    report = preflight.run()

    assert report.errors == ()
    assert report.library_path == preflight.settings.library_path.resolve()
    assert report.library_root == preflight.settings.library_root.resolve()
    assert report.staging_dir == preflight.staging_dir.resolve()
    assert report.drive == DRIVE
    assert runner.calls == [["beet", "version"]]
    assert drive.calls == ["list"]
    assert metadata.calls == ["ping"]
    assert not preflight.staging_dir.exists()

from __future__ import annotations

import json
import os
import stat
from pathlib import Path
from types import SimpleNamespace

import pytest

from cd2beets.journal import Journal
from cd2beets.models import OwnedMp3, VerificationReport
from cd2beets.staging import CleanupRefused, PartialCleanupError, StagingRun


VERIFIED = VerificationReport(errors=(), destination_paths=())
UNVERIFIED = VerificationReport(errors=("copy missing",), destination_paths=())


def make_run(tmp_path: Path, run_id: str = "run-abc") -> tuple[StagingRun, Path]:
    root = tmp_path / "staging"
    journal_path = root / "lifecycle.jsonl"
    run = StagingRun(root, Journal(journal_path), run_id=run_id)
    return run, journal_path


def reserve(run: StagingRun, position: int = 1) -> OwnedMp3:
    owned, output = run.reserve(position)
    output.write(f"track-{position}".encode())
    output.close()
    return owned


def cleanup_events(journal_path: Path) -> list[dict]:
    return [
        record
        for record in map(json.loads, journal_path.read_text().splitlines())
        if record["event"].startswith("track_cleanup_")
    ]


def test_staging_root_and_run_directory_are_private(tmp_path):
    run, _ = make_run(tmp_path)

    assert stat.S_IMODE(run.staging_root.stat().st_mode) == 0o700
    assert stat.S_IMODE(run.run_dir.stat().st_mode) == 0o700


def test_staging_run_uses_uuid_hex_by_default(tmp_path, monkeypatch):
    monkeypatch.setattr(
        "cd2beets.staging.uuid.uuid4",
        lambda: SimpleNamespace(hex="generated-run-id"),
    )
    root = tmp_path / "staging"

    run = StagingRun(root, Journal(root / "lifecycle.jsonl"))

    assert run.run_id == "generated-run-id"
    assert run.run_dir == root / "generated-run-id"


def test_reserve_exclusively_creates_no_follow_private_mp3_and_records_fstat(
    tmp_path, monkeypatch
):
    run, journal_path = make_run(tmp_path)
    calls: list[tuple[Path, int, int]] = []
    original_open = os.open

    def open_spy(path, flags, mode=0o777, *, dir_fd=None):
        if Path(path).parent == run.run_dir:
            calls.append((Path(path), flags, mode))
        return original_open(path, flags, mode, dir_fd=dir_fd)

    monkeypatch.setattr("cd2beets.staging.os.open", open_spy)

    owned, output = run.reserve(3)
    descriptor_stat = os.fstat(output.fileno())
    output.close()

    assert owned.path == run.run_dir / "run-abc-track03.mp3"
    assert owned.track_position == 3
    assert (owned.device, owned.inode) == (
        descriptor_stat.st_dev,
        descriptor_stat.st_ino,
    )
    assert stat.S_IMODE(owned.path.stat().st_mode) == 0o600
    assert len(calls) == 1
    _, flags, mode = calls[0]
    required = os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW
    assert flags & required == required
    assert mode == 0o600
    events = [
        json.loads(line)
        for line in journal_path.read_text(encoding="utf-8").splitlines()
    ]
    assert [event["event"] for event in events] == [
        "track_reserved",
        "track_created",
    ]
    assert events[-1]["path"] == str(owned.path)
    assert events[-1]["device"] == owned.device
    assert events[-1]["inode"] == owned.inode


def test_second_reservation_fails_without_overwriting_first(tmp_path):
    run, _ = make_run(tmp_path)
    first = reserve(run)
    original = first.path.read_bytes()

    with pytest.raises(FileExistsError):
        run.reserve(1)

    assert first.path.read_bytes() == original
    assert first.path.stat().st_ino == first.inode


def assert_cleanup_refused_without_unlink(
    run: StagingRun,
    tracks: tuple[OwnedMp3, ...],
    report: VerificationReport,
    monkeypatch,
    match: str,
):
    calls: list[Path] = []

    def unlink_spy(path, *args, **kwargs):
        calls.append(path)
        raise AssertionError("rejected cleanup reached unlink")

    monkeypatch.setattr(Path, "unlink", unlink_spy)

    with pytest.raises(CleanupRefused, match=match):
        run.cleanup(tracks, report)

    assert calls == []


def test_cleanup_rejects_unverified_report_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    track = reserve(run)

    assert_cleanup_refused_without_unlink(
        run, (track,), UNVERIFIED, monkeypatch, "verification"
    )
    assert track.path.exists()


def test_cleanup_rejects_path_outside_staging_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    outside = tmp_path / "outside.mp3"
    outside.write_bytes(b"outside")
    details = outside.lstat()
    candidate = OwnedMp3(outside, details.st_dev, details.st_ino, 1)

    assert_cleanup_refused_without_unlink(
        run, (candidate,), VERIFIED, monkeypatch, "outside"
    )
    assert outside.exists()


def test_cleanup_rejects_non_mp3_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    path = run.run_dir / "not-a-track.wav"
    path.write_bytes(b"wav")
    details = path.lstat()
    candidate = OwnedMp3(path, details.st_dev, details.st_ino, 1)

    assert_cleanup_refused_without_unlink(
        run, (candidate,), VERIFIED, monkeypatch, r"\.mp3"
    )
    assert path.exists()


def test_cleanup_rejects_symlink_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    track = reserve(run)
    run.cleanup((track,), VERIFIED)
    target = run.run_dir / "target"
    target.write_bytes(b"target")
    track.path.symlink_to(target)

    assert_cleanup_refused_without_unlink(
        run, (track,), VERIFIED, monkeypatch, "symlink|regular"
    )
    assert track.path.is_symlink()


def test_cleanup_rejects_non_regular_file_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    track = reserve(run)
    run.cleanup((track,), VERIFIED)
    track.path.mkdir()

    assert_cleanup_refused_without_unlink(
        run, (track,), VERIFIED, monkeypatch, "regular"
    )
    assert track.path.is_dir()


def test_cleanup_rejects_mismatched_inode_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    first = reserve(run, 1)
    second = reserve(run, 2)
    run.cleanup((first,), VERIFIED)
    os.link(second.path, first.path)
    assert first.path.stat().st_ino != first.inode

    assert_cleanup_refused_without_unlink(
        run, (first,), VERIFIED, monkeypatch, "inode"
    )
    assert first.path.exists()
    assert second.path.exists()


def test_cleanup_rejects_mismatched_device_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    track = reserve(run)
    actual = track.path.lstat()
    original_lstat = Path.lstat

    def lstat_spy(path):
        if path == track.path:
            return SimpleNamespace(
                st_mode=actual.st_mode,
                st_dev=actual.st_dev + 1,
                st_ino=actual.st_ino,
            )
        return original_lstat(path)

    monkeypatch.setattr(Path, "lstat", lstat_spy)
    assert_cleanup_refused_without_unlink(
        run, (track,), VERIFIED, monkeypatch, "device"
    )


def test_cleanup_rejects_unowned_path_before_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    path = run.run_dir / "unowned.mp3"
    path.write_bytes(b"not reserved")
    details = path.lstat()
    candidate = OwnedMp3(path, details.st_dev, details.st_ino, 1)

    assert_cleanup_refused_without_unlink(
        run, (candidate,), VERIFIED, monkeypatch, "owned"
    )
    assert path.exists()


def test_one_invalid_candidate_prevents_every_unlink(tmp_path, monkeypatch):
    run, _ = make_run(tmp_path)
    valid = reserve(run)
    bad_path = run.run_dir / "unowned.mp3"
    bad_path.write_bytes(b"bad")
    bad_stat = bad_path.lstat()
    invalid = OwnedMp3(bad_path, bad_stat.st_dev, bad_stat.st_ino, 2)

    assert_cleanup_refused_without_unlink(
        run, (valid, invalid), VERIFIED, monkeypatch, "owned"
    )
    assert valid.path.exists()
    assert bad_path.exists()


def test_validate_cleanup_returns_all_candidates_in_order(tmp_path):
    run, _ = make_run(tmp_path)
    first = reserve(run, 1)
    second = reserve(run, 2)

    assert run.validate_cleanup((second, first), VERIFIED) == (second, first)
    assert first.path.exists()
    assert second.path.exists()


def test_successful_cleanup_removes_only_owned_mp3s_and_keeps_artifacts(tmp_path):
    run, journal_path = make_run(tmp_path)
    first = reserve(run, 1)
    second = reserve(run, 2)
    unrelated_mp3 = run.run_dir / "unrelated.mp3"
    unrelated_mp3.write_bytes(b"keep")
    log = run.run_dir / "encoder.log"
    log.write_text("keep")

    result = run.cleanup((first, second), VERIFIED)

    assert result == (first, second)
    assert not first.path.exists()
    assert not second.path.exists()
    assert unrelated_mp3.read_bytes() == b"keep"
    assert log.read_text() == "keep"
    assert run.run_dir.is_dir()
    assert journal_path.is_file()
    assert [event["event"] for event in cleanup_events(journal_path)] == [
        "track_cleanup_started",
        "track_cleanup_succeeded",
        "track_cleanup_started",
        "track_cleanup_succeeded",
    ]


def test_later_unlink_failure_reports_partial_cleanup_and_preserves_remaining(
    tmp_path, monkeypatch
):
    run, journal_path = make_run(tmp_path)
    first = reserve(run, 1)
    second = reserve(run, 2)
    third = reserve(run, 3)
    original_unlink = Path.unlink

    def fail_second(path, *args, **kwargs):
        if path == second.path:
            raise PermissionError("unlink denied")
        return original_unlink(path, *args, **kwargs)

    monkeypatch.setattr(Path, "unlink", fail_second)

    with pytest.raises(PartialCleanupError) as raised:
        run.cleanup((first, second, third), VERIFIED)

    error = raised.value
    assert error.removed == (first,)
    assert error.remaining == (second, third)
    assert isinstance(error.cause, PermissionError)
    assert not first.path.exists()
    assert second.path.exists()
    assert third.path.exists()
    assert run.run_dir.is_dir()
    assert journal_path.is_file()
    events = cleanup_events(journal_path)
    assert [event["event"] for event in events] == [
        "track_cleanup_started",
        "track_cleanup_succeeded",
        "track_cleanup_started",
        "track_cleanup_failed",
    ]
    assert events[-1]["path"] == str(second.path)
    assert "unlink denied" in events[-1]["error"]

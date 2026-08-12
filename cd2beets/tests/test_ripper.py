from pathlib import Path

import pytest

from cd2beets.models import DiscToc, OwnedMp3, TrackMetadata
from cd2beets.process import PipelineResult
from cd2beets.ripper import RipError, Ripper


class RecordingJournal:
    def __init__(self):
        self.records = []

    def append(self, event, **fields):
        self.records.append((event, fields))


class RaisingJournal(RecordingJournal):
    def __init__(self, failing_event: str):
        super().__init__()
        self.failing_event = failing_event
        self.error = OSError(f"cannot journal {failing_event}")

    def append(self, event, **fields):
        if event == self.failing_event:
            raise self.error
        super().append(event, **fields)


class FakeStaging:
    def __init__(self, tmp_path: Path):
        self.run_id = "run-1"
        self.path = tmp_path / "run-1-track03.mp3"
        self.owned = []
        self.output = None

    def reserve(self, track_position: int):
        self.path.touch(exist_ok=False)
        owned = OwnedMp3(
            path=self.path,
            device=self.path.stat().st_dev,
            inode=self.path.stat().st_ino,
            track_position=track_position,
        )
        self.owned.append(owned)
        self.output = self.path.open("wb")
        return owned, self.output


class FakeRunner:
    def __init__(self, result: PipelineResult):
        self.result = result
        self.calls = []

    def pipeline(self, source_args, sink_args, destination):
        self.calls.append((list(source_args), list(sink_args), destination))
        destination.write(b"partial-or-complete-mp3")
        return self.result


def make_toc() -> DiscToc:
    return DiscToc(
        disc_id="disc-id",
        device="1",
        first_track=1,
        last_track=10,
        sectors=12345,
    )


def make_track() -> TrackMetadata:
    return TrackMetadata(
        position=3,
        recording_id="recording-id",
        title="Track Three",
        length_ms=180_000,
    )


def test_rip_track_streams_verified_cd_paranoia_into_lame(tmp_path: Path):
    result = PipelineResult(0, 0, "reader note", "encoder note")
    runner = FakeRunner(result)
    journal = RecordingJournal()
    staging = FakeStaging(tmp_path)

    owned = Ripper(runner, journal).rip_track(make_toc(), make_track(), staging)

    assert owned is staging.owned[0]
    source_args, sink_args, destination = runner.calls[0]
    assert source_args == [
        "cd-paranoia",
        "-q",
        "-e",
        "-X",
        "-p",
        "3",
        "-",
    ]
    assert sink_args == ["lame", "--silent", "-r", "-b", "320", "-", "-"]
    assert destination is staging.output
    assert destination.closed
    assert staging.path.read_bytes() == b"partial-or-complete-mp3"
    assert [event for event, _ in journal.records] == [
        "track_rip_started",
        "track_rip_succeeded",
    ]
    assert journal.records[-1][1] == {
        "run_id": "run-1",
        "track_position": 3,
        "path": str(staging.path),
        "source_returncode": 0,
        "sink_returncode": 0,
        "source_stderr": "reader note",
        "sink_stderr": "encoder note",
    }


def test_cd_paranoia_skip_marker_fails_even_when_both_processes_exit_zero(
    tmp_path: Path,
):
    result = PipelineResult(
        0,
        0,
        "##: 6 [skip] @ 12345\n",
        "",
    )
    runner = FakeRunner(result)
    journal = RecordingJournal()
    staging = FakeStaging(tmp_path)

    with pytest.raises(RipError, match="uncorrected skip"):
        Ripper(runner, journal).rip_track(make_toc(), make_track(), staging)

    assert staging.path.exists()
    assert [event for event, _ in journal.records] == [
        "track_rip_started",
        "track_rip_failed",
    ]


@pytest.mark.parametrize(
    "result",
    [
        PipelineResult(1, 0, "reader failed", "encoder note"),
        PipelineResult(0, 2, "reader note", "encoder failed"),
    ],
)
def test_rip_failure_preserves_owned_output_and_reports_both_processes(
    tmp_path: Path, result: PipelineResult
):
    runner = FakeRunner(result)
    journal = RecordingJournal()
    staging = FakeStaging(tmp_path)

    with pytest.raises(RipError) as raised:
        Ripper(runner, journal).rip_track(make_toc(), make_track(), staging)

    message = str(raised.value)
    assert f"source exit status {result.source_returncode}" in message
    assert f"sink exit status {result.sink_returncode}" in message
    assert result.source_stderr in message
    assert result.sink_stderr in message
    assert raised.value.result is result
    assert staging.output is not None and staging.output.closed
    assert staging.path.read_bytes() == b"partial-or-complete-mp3"
    assert len(staging.owned) == 1
    assert [event for event, _ in journal.records] == [
        "track_rip_started",
        "track_rip_failed",
    ]


def test_rip_closes_and_preserves_destination_when_pipeline_cannot_start(tmp_path: Path):
    class RaisingRunner:
        def pipeline(self, source_args, sink_args, destination):
            destination.write(b"reserved")
            raise OSError("extractor unavailable")

    journal = RecordingJournal()
    staging = FakeStaging(tmp_path)

    with pytest.raises(OSError, match="extractor unavailable"):
        Ripper(RaisingRunner(), journal).rip_track(
            make_toc(), make_track(), staging
        )

    assert staging.output is not None and staging.output.closed
    assert staging.path.read_bytes() == b"reserved"
    assert len(staging.owned) == 1
    assert [event for event, _ in journal.records] == [
        "track_rip_started",
        "track_rip_failed",
    ]
    assert journal.records[-1][1]["error"] == "extractor unavailable"


def test_rip_closes_and_preserves_destination_when_started_journal_fails(
    tmp_path: Path,
):
    runner = FakeRunner(PipelineResult(0, 0, "", ""))
    journal = RaisingJournal("track_rip_started")
    staging = FakeStaging(tmp_path)

    with pytest.raises(OSError, match="cannot journal track_rip_started"):
        Ripper(runner, journal).rip_track(make_toc(), make_track(), staging)

    assert runner.calls == []
    assert staging.output is not None and staging.output.closed
    assert staging.path.exists()
    assert len(staging.owned) == 1


def test_rip_error_is_not_masked_when_failure_journal_raises(tmp_path: Path):
    failed = PipelineResult(1, 2, "reader failed", "encoder failed")
    runner = FakeRunner(failed)
    journal = RaisingJournal("track_rip_failed")
    staging = FakeStaging(tmp_path)

    with pytest.raises(RipError) as raised:
        Ripper(runner, journal).rip_track(make_toc(), make_track(), staging)

    assert raised.value.result is failed
    assert raised.value.__cause__ is journal.error
    assert "reader failed" in str(raised.value)
    assert "encoder failed" in str(raised.value)
    assert staging.output is not None and staging.output.closed
    assert staging.path.exists()
    assert len(staging.owned) == 1

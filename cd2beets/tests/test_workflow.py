from __future__ import annotations

from errno import EBUSY
from pathlib import Path
from typing import Any

import pytest

from cd2beets.models import (
    Classification,
    DiscToc,
    FailureDecision,
    LibraryItem,
    LibrarySnapshot,
    MediaState,
    OperatorDecision,
    OwnedMp3,
    Presence,
    ReleaseMetadata,
    TrackMetadata,
    VerificationReport,
)
from cd2beets.process import CommandResult
from cd2beets.staging import CleanupRefused, PartialCleanupError
from cd2beets.workflow import (
    DiscOutcome,
    DiscWorkflow,
    MusicProcessDetector,
)


class FakeJournal:
    def __init__(self, log: list[str]) -> None:
        self.log = log
        self.events: list[tuple[str, dict[str, Any]]] = []
        self.failures: dict[str, list[Exception]] = {}

    def append(self, event: str, **fields: Any) -> None:
        self.log.append(f"journal:{event}")
        failures = self.failures.get(event, [])
        if failures:
            raise failures.pop(0)
        self.events.append((event, fields))


class FakeDrive:
    def __init__(
        self,
        log: list[str],
        statuses: list[MediaState | BaseException] | None = None,
        *,
        eject_error: Exception | None = None,
    ) -> None:
        self.log = log
        self.statuses = list(statuses or [])
        self.eject_error = eject_error

    def status(self) -> MediaState:
        if not self.statuses:
            raise AssertionError("unexpected drive.status call")
        result = self.statuses.pop(0)
        if isinstance(result, BaseException):
            raise result
        self.log.append(f"drive.status:{result.value}")
        return result

    def eject(self) -> None:
        self.log.append("drive.eject")
        if self.eject_error is not None:
            raise self.eject_error


class FakeReader:
    def __init__(
        self,
        log: list[str],
        toc: DiscToc,
        *,
        read_errors: list[BaseException] | None = None,
    ) -> None:
        self.log = log
        self.toc = toc
        self.read_errors = list(read_errors or [])

    def default_device(self) -> str:
        self.log.append("reader.default_device")
        return self.toc.device

    def read(self, device: str) -> DiscToc:
        self.log.append(f"reader.read:{device}")
        if self.read_errors:
            raise self.read_errors.pop(0)
        return self.toc


class FakeMetadata:
    def __init__(
        self,
        log: list[str],
        candidates: tuple[ReleaseMetadata, ...],
        *,
        error: Exception | None = None,
    ) -> None:
        self.log = log
        self.candidates = candidates
        self.error = error

    def lookup(self, toc: DiscToc) -> tuple[ReleaseMetadata, ...]:
        self.log.append(f"metadata.lookup:{toc.disc_id}")
        if self.error is not None:
            raise self.error
        return self.candidates


class FakeLibrary:
    def __init__(self, log: list[str]) -> None:
        self.log = log
        self.snapshots = 0
        self.snapshot_error: Exception | None = None

    def snapshot(self, release: ReleaseMetadata) -> LibrarySnapshot:
        self.snapshots += 1
        self.log.append(f"library.snapshot:{self.snapshots}")
        if self.snapshot_error is not None:
            raise self.snapshot_error
        return LibrarySnapshot((), ())

    def refresh(self) -> None:
        self.log.append("library.refresh")


class FakeRipper:
    def __init__(
        self,
        log: list[str],
        *,
        fail_position: int | None = None,
        interrupt_position: int | None = None,
    ) -> None:
        self.log = log
        self.fail_position = fail_position
        self.interrupt_position = interrupt_position
        self.generated: list[OwnedMp3] = []

    def rip_track(
        self,
        toc: DiscToc,
        release: ReleaseMetadata,
        track: TrackMetadata,
        staging: "FakeStaging",
    ) -> OwnedMp3:
        self.log.append(f"ripper.rip:{track.position}")
        if track.position == self.interrupt_position:
            raise KeyboardInterrupt
        if track.position == self.fail_position:
            raise RuntimeError(f"rip failed for track {track.position}")
        owned = OwnedMp3(
            staging.run_dir / f"track-{track.position}.mp3",
            10,
            100 + track.position,
            track.position,
        )
        self.generated.append(owned)
        return owned


class FakeImporter:
    def __init__(
        self, log: list[str], outcomes: list[Exception | None] | None = None
    ) -> None:
        self.log = log
        self.outcomes = list(outcomes or [None])
        self.calls = 0

    def import_release(self, release_id: str, run_dir: Path) -> object:
        self.calls += 1
        self.log.append(f"importer.import:{self.calls}:{release_id}:{run_dir.name}")
        if not self.outcomes:
            raise AssertionError("unexpected import retry")
        outcome = self.outcomes.pop(0)
        if outcome is not None:
            raise outcome
        return object()


class FakeVerifier:
    def __init__(
        self,
        log: list[str],
        outcomes: list[VerificationReport | Exception] | None = None,
    ) -> None:
        self.log = log
        self.outcomes = list(
            outcomes
            or [VerificationReport((), (Path("/library/track-1.mp3"),))]
        )
        self.calls = 0

    def __call__(
        self,
        release: ReleaseMetadata,
        pre_snapshot: LibrarySnapshot,
        post_snapshot: LibrarySnapshot,
        owned: tuple[OwnedMp3, ...],
        staging_root: Path,
        library_root: Path,
    ) -> VerificationReport:
        self.calls += 1
        self.log.append(f"verifier.verify:{self.calls}")
        if not self.outcomes:
            raise AssertionError("unexpected verification retry")
        outcome = self.outcomes.pop(0)
        if isinstance(outcome, Exception):
            raise outcome
        return outcome


class FakeStaging:
    def __init__(
        self,
        log: list[str],
        *,
        cleanup_error: Exception | None = None,
    ) -> None:
        self.log = log
        self.staging_root = Path("/staging")
        self.run_id = "run-1"
        self.run_dir = self.staging_root / self.run_id
        self.cleanup_error = cleanup_error
        self.cleanup_calls = 0
        self.cleaned: tuple[OwnedMp3, ...] = ()

    def cleanup(
        self, tracks: tuple[OwnedMp3, ...], report: VerificationReport
    ) -> tuple[OwnedMp3, ...]:
        self.cleanup_calls += 1
        self.log.append("staging.cleanup")
        assert report.ok
        if self.cleanup_error is not None:
            raise self.cleanup_error
        self.cleaned = tuple(tracks)
        return self.cleaned


class FakeUI:
    def __init__(
        self,
        log: list[str],
        *,
        uncertain: OperatorDecision = OperatorDecision.SKIP_EJECT,
        failures: list[FailureDecision] | None = None,
    ) -> None:
        self.log = log
        self.uncertain = uncertain
        self.failures = list(failures or [])
        self.failure_prompts: list[tuple[str, tuple[FailureDecision, ...]]] = []
        self.preservation_messages = 0
        self.preservation_states: list[str] = []
        self.busy_messages = 0
        self.reported_errors: list[str] = []

    def select_release(
        self, candidates: tuple[ReleaseMetadata, ...]
    ) -> ReleaseMetadata:
        self.log.append("ui.select_release")
        if not candidates:
            raise ValueError("no complete release candidates")
        return candidates[0]

    def decide_uncertain(self, reasons: tuple[str, ...]) -> OperatorDecision:
        self.log.append("ui.decide_uncertain")
        return self.uncertain

    def decide_failure(
        self, error: BaseException | str, allowed: tuple[FailureDecision, ...]
    ) -> FailureDecision:
        self.log.append("ui.decide_failure")
        self.failure_prompts.append((str(error), allowed))
        if not self.failures:
            raise AssertionError("missing fake failure decision")
        decision = self.failures.pop(0)
        assert decision in allowed
        return decision

    def report_error(self, error: BaseException | str) -> None:
        self.reported_errors.append(str(error))
        self.log.append("ui.report_error")


    def report_unusable_media(self) -> None:
        self.log.append("ui.unusable_media")

    def report_preservation(self, state: str) -> None:
        self.preservation_messages += 1
        self.preservation_states.append(state)
        self.log.append("ui.preservation")

    def report_busy_music(self) -> None:
        self.busy_messages += 1
        self.log.append("ui.busy_music")


class FakeMusicDetector:
    def __init__(self, log: list[str], running: bool = False) -> None:
        self.log = log
        self.running = running
        self.calls = 0

    def is_running(self) -> bool:
        self.calls += 1
        self.log.append("music.detect")
        return self.running


class Scenario:
    def __init__(
        self,
        *,
        presence: Presence = Presence.ABSENT,
        reasons: tuple[str, ...] = (),
        uncertain: OperatorDecision = OperatorDecision.SKIP_EJECT,
        failures: list[FailureDecision] | None = None,
        metadata_error: Exception | None = None,
        read_errors: list[BaseException] | None = None,
        fail_rip: int | None = None,
        interrupt_rip: int | None = None,
        import_outcomes: list[Exception | None] | None = None,
        verification_outcomes: list[VerificationReport | Exception] | None = None,
        cleanup_error: Exception | None = None,
        statuses: list[MediaState | BaseException] | None = None,
        music_running: bool = False,
        interrupt_sleep: bool = False,
    ) -> None:
        self.log: list[str] = []
        self.release = ReleaseMetadata(
            release_id="11111111-1111-1111-1111-111111111111",
            albumartist="Artist",
            album="Album",
            date="2000",
            country="US",
            disambiguation="original",
            medium=1,
            tracks=(
                TrackMetadata(1, "recording-1", "One", 1000),
                TrackMetadata(2, "recording-2", "Two", 2000),
            ),
        )
        self.toc = DiscToc("disc-1", "/dev/rdisk9", 1, 2, 10000)
        self.journal = FakeJournal(self.log)
        self.drive = FakeDrive(self.log, statuses)
        self.reader = FakeReader(self.log, self.toc, read_errors=read_errors)
        self.metadata = FakeMetadata(
            self.log, (self.release,), error=metadata_error
        )
        self.library = FakeLibrary(self.log)
        self.ripper = FakeRipper(
            self.log,
            fail_position=fail_rip,
            interrupt_position=interrupt_rip,
        )
        self.importer = FakeImporter(self.log, import_outcomes)
        self.verifier = FakeVerifier(self.log, verification_outcomes)
        self.staging = FakeStaging(self.log, cleanup_error=cleanup_error)
        self.ui = FakeUI(
            self.log, uncertain=uncertain, failures=failures
        )
        self.music = FakeMusicDetector(self.log, music_running)
        self.classification = Classification(
            presence,
            reasons,
            LibrarySnapshot((), ()),
        )

        def classify(
            release: ReleaseMetadata,
            snapshot: LibrarySnapshot,
            library_root: Path,
        ) -> Classification:
            self.log.append("library.classify")
            return self.classification

        def create_staging() -> FakeStaging:
            self.log.append("staging.create")
            return self.staging

        self.sleeps: list[float] = []

        def sleep(seconds: float) -> None:
            self.sleeps.append(seconds)
            self.log.append(f"sleep:{seconds}")
            if interrupt_sleep:
                raise KeyboardInterrupt

        self.workflow = DiscWorkflow(
            drive=self.drive,
            disc_reader=self.reader,
            metadata=self.metadata,
            library=self.library,
            library_root=Path("/library"),
            journal=self.journal,
            ripper=self.ripper,
            importer=self.importer,
            staging_factory=create_staging,
            verifier=self.verifier,
            ui=self.ui,
            music_detector=self.music,
            classifier=classify,
            sleep=sleep,
        )

    @property
    def event_names(self) -> list[str]:
        return [name for name, _ in self.journal.events]


def event_fields(scenario: Scenario, name: str) -> list[dict[str, Any]]:
    return [fields for event, fields in scenario.journal.events if event == name]


def test_present_disc_classifies_then_ejects_without_staging_or_mutation() -> None:
    scenario = Scenario(presence=Presence.PRESENT)

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert scenario.log == [
        "reader.default_device",
        "reader.read:/dev/rdisk9",
        "journal:disc_identified",
        "metadata.lookup:disc-1",
        "journal:release_candidates",
        "ui.select_release",
        "journal:release_selected",
        "library.snapshot:1",
        "library.classify",
        "journal:library_classified",
        "drive.eject",
        "journal:eject_outcome",
    ]


def test_absent_disc_runs_complete_ordered_lifecycle_and_journal() -> None:
    scenario = Scenario(presence=Presence.ABSENT)

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert scenario.log == [
        "reader.default_device",
        "reader.read:/dev/rdisk9",
        "journal:disc_identified",
        "metadata.lookup:disc-1",
        "journal:release_candidates",
        "ui.select_release",
        "journal:release_selected",
        "library.snapshot:1",
        "library.classify",
        "journal:library_classified",
        "staging.create",
        "journal:staging_started",
        "ripper.rip:1",
        "journal:track_generated",
        "ripper.rip:2",
        "journal:track_generated",
        "importer.import:1:11111111-1111-1111-1111-111111111111:run-1",
        "journal:import_outcome",
        "library.refresh",
        "library.snapshot:2",
        "verifier.verify:1",
        "journal:verification_report",
        "staging.cleanup",
        "journal:track_cleanup_result",
        "journal:track_cleanup_result",
        "drive.eject",
        "journal:eject_outcome",
    ]
    assert scenario.event_names == [
        "disc_identified",
        "release_candidates",
        "release_selected",
        "library_classified",
        "staging_started",
        "track_generated",
        "track_generated",
        "import_outcome",
        "verification_report",
        "track_cleanup_result",
        "track_cleanup_result",
        "eject_outcome",
    ]
    assert event_fields(scenario, "disc_identified")[0]["disc_id"] == "disc-1"
    candidates = event_fields(scenario, "release_candidates")[0]["candidates"]
    assert candidates == [
        {
            "release_id": "11111111-1111-1111-1111-111111111111",
            "albumartist": "Artist",
            "album": "Album",
            "date": "2000",
            "country": "US",
            "disambiguation": "original",
            "medium": 1,
            "track_count": 2,
            "tracks": [
                {
                    "position": 1,
                    "recording_id": "recording-1",
                    "title": "One",
                    "length_ms": 1000,
                },
                {
                    "position": 2,
                    "recording_id": "recording-2",
                    "title": "Two",
                    "length_ms": 2000,
                },
            ],
        }
    ]
    selected = event_fields(scenario, "release_selected")[0]
    assert selected["release_id"] == scenario.release.release_id
    classified = event_fields(scenario, "library_classified")[0]
    assert classified["presence"] == "absent"
    assert classified["reasons"] == []
    assert [fields["track_position"] for fields in event_fields(scenario, "track_generated")] == [1, 2]
    assert event_fields(scenario, "import_outcome")[0]["ok"] is True
    report = event_fields(scenario, "verification_report")[0]
    assert report["ok"] is True
    assert report["errors"] == []
    assert report["destination_paths"] == ["/library/track-1.mp3"]
    assert report["post_snapshot"] == {
        "exact_items": [],
        "plausible_items": [],
    }
    assert [fields["result"] for fields in event_fields(scenario, "track_cleanup_result")] == ["removed", "removed"]
    assert event_fields(scenario, "eject_outcome")[0]["ok"] is True


def test_uncertain_default_skip_does_not_rip_or_import_and_ejects() -> None:
    scenario = Scenario(
        presence=Presence.UNCERTAIN,
        reasons=("missing track 2",),
        uncertain=OperatorDecision.SKIP_EJECT,
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert "ui.decide_uncertain" in scenario.log
    assert "staging.create" not in scenario.log
    assert not any(call.startswith("ripper.") for call in scenario.log)
    assert not any(call.startswith("importer.") for call in scenario.log)
    assert scenario.log[-3:] == [
        "journal:operator_decision",
        "drive.eject",
        "journal:eject_outcome",
    ]


def test_uncertain_explicit_proceed_uses_normal_rip_import_path() -> None:
    scenario = Scenario(
        presence=Presence.UNCERTAIN,
        reasons=("different release ID",),
        uncertain=OperatorDecision.PROCEED,
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert scenario.log.index("journal:operator_decision") < scenario.log.index(
        "staging.create"
    )
    assert [call for call in scenario.log if call.startswith("ripper.rip")] == [
        "ripper.rip:1",
        "ripper.rip:2",
    ]
    assert any(call.startswith("importer.import:1") for call in scenario.log)


def test_uncertain_abort_keeps_disc_loaded_and_journals_held_state() -> None:
    scenario = Scenario(
        presence=Presence.UNCERTAIN,
        reasons=("split album",),
        uncertain=OperatorDecision.ABORT_KEEP_LOADED,
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.HELD
    assert "drive.eject" not in scenario.log
    assert "staging.create" not in scenario.log
    assert scenario.event_names[-2:] == ["operator_decision", "workflow_held"]
    assert event_fields(scenario, "workflow_held")[0]["reason"] == "operator_abort"


@pytest.mark.parametrize("failure_stage", ["metadata", "rip", "import", "verification"])
def test_stage_failure_preserves_generated_tracks_and_never_ejects_implicitly(
    failure_stage: str,
) -> None:
    options: dict[str, Any] = {"failures": [FailureDecision.QUIT]}
    if failure_stage == "metadata":
        options["metadata_error"] = RuntimeError("metadata stderr")
    elif failure_stage == "rip":
        options["fail_rip"] = 2
    elif failure_stage == "import":
        options["import_outcomes"] = [RuntimeError("import stderr")]
    else:
        options["verification_outcomes"] = [
            VerificationReport(("destination missing",), ())
        ]
    scenario = Scenario(**options)

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.QUIT
    assert "drive.eject" not in scenario.log
    assert "staging.cleanup" not in scenario.log
    assert scenario.staging.cleaned == ()
    assert "workflow_failed" in scenario.event_names
    assert scenario.event_names[-2:] == ["failure_decision", "workflow_quit"]
    assert scenario.ui.failure_prompts[-1][1][-2:] == (
        FailureDecision.EJECT,
        FailureDecision.QUIT,
    )
    generated_paths = [track.path for track in scenario.ripper.generated]
    assert bool(generated_paths) is (failure_stage != "metadata")
    assert all(path not in scenario.staging.cleaned for path in generated_paths)
    if failure_stage == "rip":
        interrupted = event_fields(scenario, "track_generation_interrupted")[0]
        assert interrupted["run_id"] == "run-1"
        assert interrupted["track_position"] == 2
        assert interrupted["preservation"] == "staging run left intact"


def test_import_retry_repeats_only_import_then_continues() -> None:
    scenario = Scenario(
        import_outcomes=[RuntimeError("temporary import failure"), None],
        failures=[FailureDecision.RETRY_IMPORT],
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert scenario.importer.calls == 2
    assert [call for call in scenario.log if call.startswith("ripper.rip")] == [
        "ripper.rip:1",
        "ripper.rip:2",
    ]
    assert scenario.verifier.calls == 1
    assert scenario.ui.failure_prompts == [
        (
            "temporary import failure",
            (
                FailureDecision.RETRY_IMPORT,
                FailureDecision.EJECT,
                FailureDecision.QUIT,
            ),
        )
    ]
    outcomes = event_fields(scenario, "import_outcome")
    assert [event["ok"] for event in outcomes] == [False, True]


def test_verification_retry_repeats_only_refresh_snapshot_and_verify() -> None:
    scenario = Scenario(
        verification_outcomes=[
            VerificationReport(("copy incomplete",), ()),
            VerificationReport((), (Path("/library/one.mp3"), Path("/library/two.mp3"))),
        ],
        failures=[FailureDecision.RETRY_VERIFY],
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert scenario.importer.calls == 1
    assert scenario.verifier.calls == 2
    assert scenario.log.count("library.refresh") == 2
    assert scenario.library.snapshots == 3
    assert [call for call in scenario.log if call.startswith("ripper.rip")] == [
        "ripper.rip:1",
        "ripper.rip:2",
    ]
    assert scenario.ui.failure_prompts == [
        (
            "copy incomplete",
            (
                FailureDecision.RETRY_VERIFY,
                FailureDecision.EJECT,
                FailureDecision.QUIT,
            ),
        )
    ]
    reports = event_fields(scenario, "verification_report")
    assert [event["ok"] for event in reports] == [False, True]


def test_partial_cleanup_holds_disc_and_journals_removed_and_preserved_tracks() -> None:
    scenario = Scenario()
    track_one = OwnedMp3(Path("/staging/run-1/track-1.mp3"), 10, 101, 1)
    track_two = OwnedMp3(Path("/staging/run-1/track-2.mp3"), 10, 102, 2)
    scenario.staging.cleanup_error = PartialCleanupError(
        (track_one,), (track_two,), OSError("filesystem failure")
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.HELD
    assert "drive.eject" not in scenario.log
    cleanup = event_fields(scenario, "track_cleanup_result")
    assert [(event["track_position"], event["result"]) for event in cleanup] == [
        (1, "removed"),
        (2, "preserved"),
    ]
    assert scenario.event_names[-2:] == ["workflow_failed", "workflow_held"]
    assert event_fields(scenario, "workflow_held")[0]["reason"] == "partial_cleanup"


def test_continuous_mode_waits_for_no_media_before_accepting_next_disc() -> None:
    scenario = Scenario(
        presence=Presence.PRESENT,
        statuses=[
            MediaState.AUDIO,
            MediaState.AUDIO,
            MediaState.NO_MEDIA,
            MediaState.AUDIO,
            MediaState.NO_MEDIA,
            KeyboardInterrupt(),
        ],
    )

    result = scenario.workflow.run(once=False, poll_interval=0.25)

    assert result is None
    read_indices = [
        index
        for index, call in enumerate(scenario.log)
        if call == "reader.default_device"
    ]
    no_media_indices = [
        index
        for index, call in enumerate(scenario.log)
        if call == "drive.status:no_media"
    ]
    assert len(read_indices) == 2
    assert read_indices[0] < no_media_indices[0] < read_indices[1]
    assert scenario.sleeps == [0.25]
    assert scenario.ui.preservation_messages == 1
    assert scenario.ui.preservation_states == ["no_media"]
    interrupted = event_fields(scenario, "workflow_interrupted")[0]
    assert interrupted["media_state"] == "no_media"


def test_once_ignores_empty_polls_then_exits_after_one_processed_disc() -> None:
    scenario = Scenario(
        presence=Presence.PRESENT,
        statuses=[
            MediaState.NO_MEDIA,
            MediaState.UNUSABLE,
            MediaState.AUDIO,
            MediaState.NO_MEDIA,
            MediaState.AUDIO,
        ],
    )

    outcome = scenario.workflow.run(once=True, poll_interval=0.5)

    assert outcome is DiscOutcome.EJECTED
    assert scenario.log.count("reader.default_device") == 1
    assert scenario.sleeps == [0.5, 0.5]
    assert scenario.ui.log.count("ui.unusable_media") == 1
    assert scenario.drive.statuses == [MediaState.AUDIO]


def test_keyboard_interrupt_is_caught_only_by_outer_loop_and_preserves_everything() -> None:
    scenario = Scenario(
        interrupt_rip=2,
        statuses=[MediaState.AUDIO],
    )

    outcome = scenario.workflow.run(once=True, poll_interval=1.0)

    assert outcome is None
    assert scenario.ripper.generated
    assert "staging.cleanup" not in scenario.log
    assert "drive.eject" not in scenario.log
    assert scenario.ui.preservation_messages == 1
    assert scenario.ui.preservation_states == ["processing"]


def test_busy_disc_with_music_running_only_detects_and_gives_manual_guidance() -> None:
    scenario = Scenario(
        read_errors=[OSError(EBUSY, "Device busy")],
        music_running=True,
        failures=[FailureDecision.QUIT],
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.QUIT
    assert scenario.music.calls == 1
    assert scenario.ui.busy_messages == 1
    assert scenario.ui.failure_prompts[0][1] == (
        FailureDecision.EJECT,
        FailureDecision.QUIT,
    )
    assert "drive.eject" not in scenario.log
    assert all("signal" not in call and "terminate" not in call for call in scenario.log)


def test_failure_eject_occurs_only_after_explicit_eject_decision() -> None:
    scenario = Scenario(
        metadata_error=RuntimeError("metadata unavailable"),
        failures=[FailureDecision.EJECT],
    )

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.EJECTED
    assert scenario.log.index("journal:failure_decision") < scenario.log.index(
        "drive.eject"
    )
    assert event_fields(scenario, "failure_decision")[0]["decision"] == "eject"


def test_classification_journal_preserves_album_group_evidence() -> None:
    scenario = Scenario(
        presence=Presence.UNCERTAIN,
        reasons=("inconsistent album grouping",),
        uncertain=OperatorDecision.ABORT_KEEP_LOADED,
    )
    item = LibraryItem(
        item_id=9,
        album_id=77,
        release_id=scenario.release.release_id,
        recording_id="recording-1",
        albumartist="Artist",
        album="Album",
        medium=1,
        track=1,
        tracktotal=2,
        path=Path("/library/Artist/Album/01.mp3"),
        format="MP3",
        bitrate=320_000,
    )
    scenario.classification = Classification(
        Presence.UNCERTAIN,
        ("inconsistent album grouping",),
        LibrarySnapshot((item,), ()),
    )

    assert scenario.workflow.process_disc() is DiscOutcome.HELD

    evidence = event_fields(scenario, "library_classified")[0]["snapshot"]
    assert evidence["exact_items"] == [
        {
            "item_id": 9,
            "album_id": 77,
            "release_id": scenario.release.release_id,
            "recording_id": "recording-1",
            "albumartist": "Artist",
            "album": "Album",
            "medium": 1,
            "track": 1,
            "tracktotal": 2,
            "path": "/library/Artist/Album/01.mp3",
            "format": "MP3",
            "bitrate": 320_000,
        }
    ]


def test_tracks_are_ripped_in_metadata_position_order() -> None:
    scenario = Scenario()
    scenario.release = ReleaseMetadata(
        release_id=scenario.release.release_id,
        albumartist=scenario.release.albumartist,
        album=scenario.release.album,
        date=scenario.release.date,
        country=scenario.release.country,
        disambiguation=scenario.release.disambiguation,
        medium=scenario.release.medium,
        tracks=tuple(reversed(scenario.release.tracks)),
    )
    scenario.metadata.candidates = (scenario.release,)

    assert scenario.workflow.process_disc() is DiscOutcome.EJECTED

    assert [call for call in scenario.log if call.startswith("ripper.rip")] == [
        "ripper.rip:1",
        "ripper.rip:2",
    ]


@pytest.mark.parametrize(("returncode", "expected"), [(0, True), (1, False)])
def test_music_detector_only_runs_exact_pgrep_probe(
    returncode: int, expected: bool
) -> None:
    calls: list[list[str]] = []

    class Runner:
        def run(self, args: list[str]) -> CommandResult:
            calls.append(args)
            return CommandResult(returncode, "", "")

    detector = MusicProcessDetector(Runner())

    assert detector.is_running() is expected
    assert calls == [["pgrep", "-x", "Music"]]


def test_eject_failure_is_not_retried_and_leaves_disc_held() -> None:
    scenario = Scenario(
        presence=Presence.PRESENT,
        failures=[FailureDecision.QUIT],
    )
    scenario.drive.eject_error = RuntimeError("eject failed")

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.QUIT
    assert scenario.log.count("drive.eject") == 1
    assert event_fields(scenario, "eject_outcome")[0]["ok"] is False
    assert scenario.event_names[-1] == "workflow_held"


def test_failure_eject_error_retains_generated_track_evidence() -> None:
    scenario = Scenario(
        import_outcomes=[RuntimeError("import failed")],
        failures=[FailureDecision.EJECT, FailureDecision.QUIT],
    )
    scenario.drive.eject_error = RuntimeError("eject failed")

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.QUIT
    held = event_fields(scenario, "workflow_held")[0]
    assert [track["track_position"] for track in held["preserved_tracks"]] == [1, 2]


@pytest.mark.parametrize(
    ("journal_event", "options", "original_error"),
    [
        (
            "workflow_failed",
            {"metadata_error": RuntimeError("metadata original")},
            "metadata original",
        ),
        (
            "import_outcome",
            {"import_outcomes": [RuntimeError("import original")]},
            "import original",
        ),
        (
            "verification_report",
            {
                "verification_outcomes": [
                    VerificationReport(("verification original",), ())
                ]
            },
            "verification original",
        ),
    ],
)
def test_journal_failure_never_masks_stage_error_or_bypasses_safe_prompt(
    journal_event: str,
    options: dict[str, Any],
    original_error: str,
) -> None:
    scenario = Scenario(failures=[FailureDecision.QUIT], **options)
    scenario.journal.failures[journal_event] = [OSError("journal unavailable")]

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.QUIT
    shown, allowed = scenario.ui.failure_prompts[-1]
    assert original_error in shown
    assert "journal unavailable" in shown
    assert "journal append failed" in shown.casefold()
    assert allowed[-1] is FailureDecision.QUIT
    assert "staging.cleanup" not in scenario.log
    assert "drive.eject" not in scenario.log


@pytest.mark.parametrize("journal_event", ["import_outcome", "verification_report"])
def test_successful_stage_with_failed_lifecycle_append_stops_before_cleanup(
    journal_event: str,
) -> None:
    scenario = Scenario(failures=[FailureDecision.QUIT])
    scenario.journal.failures[journal_event] = [OSError("journal full")]

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.QUIT
    assert "journal full" in scenario.ui.failure_prompts[-1][0]
    assert "staging.cleanup" not in scenario.log
    assert "drive.eject" not in scenario.log


@pytest.mark.parametrize(
    "cleanup_error",
    [
        CleanupRefused("cleanup guard refused"),
        PartialCleanupError((), (), OSError("partial cleanup stopped")),
    ],
)
def test_cleanup_failure_is_reported_verbatim_and_held_even_if_journal_fails(
    cleanup_error: Exception,
) -> None:
    scenario = Scenario(cleanup_error=cleanup_error)
    scenario.journal.failures = {
        "track_cleanup_result": [OSError("cleanup result journal failed")],
        "workflow_failed": [OSError("failure journal failed")],
        "workflow_held": [OSError("held journal failed")],
    }

    outcome = scenario.workflow.process_disc()

    assert outcome is DiscOutcome.HELD
    assert scenario.ui.reported_errors[0] == str(cleanup_error)
    assert all(
        message.startswith("Journal append failed:")
        for message in scenario.ui.reported_errors[1:]
    )
    assert "drive.eject" not in scenario.log


@pytest.mark.parametrize("busy_source", ["metadata", "journal", "library"])
def test_music_guidance_is_not_shown_for_non_disc_access_busy_errors(
    busy_source: str,
) -> None:
    options: dict[str, Any] = {
        "music_running": True,
        "failures": [FailureDecision.QUIT],
    }
    if busy_source == "metadata":
        options["metadata_error"] = RuntimeError("metadata busy")
    scenario = Scenario(**options)
    if busy_source == "journal":
        scenario.journal.failures["disc_identified"] = [
            OSError(EBUSY, "journal busy")
        ]
    elif busy_source == "library":
        scenario.library.snapshot_error = OSError(EBUSY, "library busy")

    assert scenario.workflow.process_disc() is DiscOutcome.QUIT

    assert scenario.music.calls == 0
    assert scenario.ui.busy_messages == 0


def test_interrupt_during_unusable_media_poll_reports_polling_state() -> None:
    scenario = Scenario(
        statuses=[MediaState.UNUSABLE],
        interrupt_sleep=True,
    )

    assert scenario.workflow.run(once=False, poll_interval=0.5) is None

    assert scenario.ui.preservation_states == ["polling"]
    assert event_fields(scenario, "workflow_interrupted")[0]["media_state"] == "polling"

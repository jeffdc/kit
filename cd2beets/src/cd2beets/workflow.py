from __future__ import annotations

import errno
import time
from collections.abc import Callable
from enum import StrEnum
from pathlib import Path
from typing import Any

from cd2beets.library import classify_presence
from cd2beets.models import (
    Classification,
    FailureDecision,
    LibraryItem,
    LibrarySnapshot,
    MediaState,
    OperatorDecision,
    OwnedMp3,
    Presence,
    ReleaseMetadata,
    VerificationReport,
)
from cd2beets.staging import PartialCleanupError


class DiscOutcome(StrEnum):
    EJECTED = "ejected"
    HELD = "held"
    QUIT = "quit"


class MusicProcessDetector:
    """Read-only detection of likely optical-drive contention by Music."""

    def __init__(self, runner: Any) -> None:
        self._runner = runner

    def is_running(self) -> bool:
        return self._runner.run(["pgrep", "-x", "Music"]).returncode == 0


class DiscWorkflow:
    def __init__(
        self,
        *,
        drive: Any,
        disc_reader: Any,
        metadata: Any,
        library: Any,
        library_root: Path,
        journal: Any,
        ripper: Any,
        importer: Any,
        staging_factory: Callable[[], Any],
        verifier: Callable[
            [
                ReleaseMetadata,
                LibrarySnapshot,
                LibrarySnapshot,
                tuple[OwnedMp3, ...],
                Path,
                Path,
            ],
            VerificationReport,
        ],
        ui: Any,
        music_detector: Any | None = None,
        classifier: Callable[
            [ReleaseMetadata, LibrarySnapshot, Path], Classification
        ] = classify_presence,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        self._drive = drive
        self._disc_reader = disc_reader
        self._metadata = metadata
        self._library = library
        self._library_root = Path(library_root)
        self._journal = journal
        self._ripper = ripper
        self._importer = importer
        self._staging_factory = staging_factory
        self._verifier = verifier
        self._ui = ui
        self._music_detector = music_detector
        self._classifier = classifier
        self._sleep = sleep

    def process_disc(self) -> DiscOutcome:
        generated: list[OwnedMp3] = []
        disc_id: str | None = None

        try:
            device = self._disc_reader.default_device()
            toc = self._disc_reader.read(device)
        except Exception as error:
            if self._is_busy_error(error) and self._music_is_running():
                self._ui.report_busy_music()
            decision = self._prompt_failure(
                "disc_access",
                error,
                (),
                None,
                (FailureDecision.EJECT, FailureDecision.QUIT),
            )
            return self._finish_failure(
                "disc_access", decision, (), None
            )

        disc_id = toc.disc_id
        try:
            self._journal.append(
                "disc_identified",
                disc_id=toc.disc_id,
                device=device,
                first_track=toc.first_track,
                last_track=toc.last_track,
                sectors=toc.sectors,
            )
            candidates = self._metadata.lookup(toc)
            self._journal.append(
                "release_candidates",
                disc_id=toc.disc_id,
                candidates=[
                    self._release_fields(candidate) for candidate in candidates
                ],
            )
            release = self._ui.select_release(candidates)
            self._journal.append(
                "release_selected",
                disc_id=toc.disc_id,
                **self._release_fields(release),
            )
            pre_snapshot = self._library.snapshot(release)
            classification = self._classifier(
                release, pre_snapshot, self._library_root
            )
            self._journal.append(
                "library_classified",
                disc_id=toc.disc_id,
                release_id=release.release_id,
                presence=classification.presence.value,
                reasons=list(classification.reasons),
                snapshot=self._snapshot_fields(classification.snapshot),
            )
        except Exception as error:
            decision = self._prompt_failure(
                "metadata",
                error,
                tuple(generated),
                disc_id,
                (FailureDecision.EJECT, FailureDecision.QUIT),
            )
            return self._finish_failure(
                "metadata", decision, tuple(generated), disc_id
            )

        if classification.presence is Presence.PRESENT:
            return self._eject("already_present", disc_id)

        if classification.presence is Presence.UNCERTAIN:
            operator_decision = self._ui.decide_uncertain(
                classification.reasons
            )
            self._journal.append(
                "operator_decision",
                disc_id=disc_id,
                release_id=release.release_id,
                decision=operator_decision.value,
                reasons=list(classification.reasons),
            )
            if operator_decision is OperatorDecision.SKIP_EJECT:
                return self._eject("uncertain_skip", disc_id)
            if operator_decision is OperatorDecision.ABORT_KEEP_LOADED:
                self._journal.append(
                    "workflow_held",
                    disc_id=disc_id,
                    reason="operator_abort",
                    preserved_tracks=[],
                )
                return DiscOutcome.HELD

        try:
            staging = self._staging_factory()
            self._journal.append(
                "staging_started",
                disc_id=disc_id,
                run_id=staging.run_id,
                staging_root=str(staging.staging_root),
                run_dir=str(staging.run_dir),
            )
            for track in sorted(
                release.tracks, key=lambda candidate: candidate.position
            ):
                try:
                    owned = self._ripper.rip_track(toc, release, track, staging)
                except Exception as error:
                    journal_error = self._append_journal(
                        "track_generation_interrupted",
                        disc_id=disc_id,
                        run_id=staging.run_id,
                        run_dir=str(staging.run_dir),
                        track_position=track.position,
                        error=str(error),
                        preservation="staging run left intact",
                    )
                    if journal_error is not None:
                        raise self._supplement_error(
                            error, (journal_error,)
                        ) from error
                    raise
                generated.append(owned)
                self._journal.append(
                    "track_generated",
                    disc_id=disc_id,
                    run_id=staging.run_id,
                    **self._owned_fields(owned),
                )
        except Exception as error:
            decision = self._prompt_failure(
                "rip",
                error,
                tuple(generated),
                disc_id,
                (FailureDecision.EJECT, FailureDecision.QUIT),
            )
            return self._finish_failure(
                "rip", decision, tuple(generated), disc_id
            )

        import_attempt = 0
        while True:
            import_attempt += 1
            try:
                self._importer.import_release(
                    release.release_id, staging.run_dir
                )
            except Exception as error:
                outcome_journal_error = self._append_journal(
                    "import_outcome",
                    disc_id=disc_id,
                    run_id=staging.run_id,
                    release_id=release.release_id,
                    attempt=import_attempt,
                    ok=False,
                    error=str(error),
                )
                decision = self._prompt_failure(
                    "import",
                    error,
                    tuple(generated),
                    disc_id,
                    (
                        FailureDecision.RETRY_IMPORT,
                        FailureDecision.EJECT,
                        FailureDecision.QUIT,
                    ),
                    (
                        ()
                        if outcome_journal_error is None
                        else (outcome_journal_error,)
                    ),
                )
                if decision is FailureDecision.RETRY_IMPORT:
                    continue
                return self._finish_failure(
                    "import", decision, tuple(generated), disc_id
                )
            outcome_journal_error = self._append_journal(
                "import_outcome",
                disc_id=disc_id,
                run_id=staging.run_id,
                release_id=release.release_id,
                attempt=import_attempt,
                ok=True,
            )
            if outcome_journal_error is not None:
                error = RuntimeError(
                    "Import completed but its lifecycle event was not recorded"
                )
                decision = self._prompt_failure(
                    "import",
                    error,
                    tuple(generated),
                    disc_id,
                    (FailureDecision.EJECT, FailureDecision.QUIT),
                    (outcome_journal_error,),
                )
                return self._finish_failure(
                    "import", decision, tuple(generated), disc_id
                )
            break

        verification_attempt = 0
        while True:
            verification_attempt += 1
            post_snapshot: LibrarySnapshot | None = None
            try:
                self._library.refresh()
                post_snapshot = self._library.snapshot(release)
                report = self._verifier(
                    release,
                    pre_snapshot,
                    post_snapshot,
                    tuple(generated),
                    staging.staging_root,
                    self._library_root,
                )
            except Exception as error:
                report_journal_error = self._append_journal(
                    "verification_report",
                    disc_id=disc_id,
                    run_id=staging.run_id,
                    attempt=verification_attempt,
                    ok=False,
                    errors=[str(error)],
                    destination_paths=[],
                    post_snapshot=(
                        None
                        if post_snapshot is None
                        else self._snapshot_fields(post_snapshot)
                    ),
                )
                decision = self._prompt_failure(
                    "verification",
                    error,
                    tuple(generated),
                    disc_id,
                    (
                        FailureDecision.RETRY_VERIFY,
                        FailureDecision.EJECT,
                        FailureDecision.QUIT,
                    ),
                    (
                        ()
                        if report_journal_error is None
                        else (report_journal_error,)
                    ),
                )
                if decision is FailureDecision.RETRY_VERIFY:
                    continue
                return self._finish_failure(
                    "verification", decision, tuple(generated), disc_id
                )

            report_journal_error = self._append_journal(
                "verification_report",
                disc_id=disc_id,
                run_id=staging.run_id,
                attempt=verification_attempt,
                ok=report.ok,
                errors=list(report.errors),
                destination_paths=[
                    str(path) for path in report.destination_paths
                ],
                post_snapshot=self._snapshot_fields(post_snapshot),
            )
            if report_journal_error is not None:
                error = (
                    RuntimeError("\n".join(report.errors))
                    if not report.ok
                    else RuntimeError(
                        "Verification completed but its lifecycle event "
                        "was not recorded"
                    )
                )
                decision = self._prompt_failure(
                    "verification",
                    error,
                    tuple(generated),
                    disc_id,
                    (
                        FailureDecision.RETRY_VERIFY,
                        FailureDecision.EJECT,
                        FailureDecision.QUIT,
                    ),
                    (report_journal_error,),
                )
                if decision is FailureDecision.RETRY_VERIFY:
                    continue
                return self._finish_failure(
                    "verification", decision, tuple(generated), disc_id
                )
            if report.ok:
                break

            error = RuntimeError("\n".join(report.errors))
            decision = self._prompt_failure(
                "verification",
                error,
                tuple(generated),
                disc_id,
                (
                    FailureDecision.RETRY_VERIFY,
                    FailureDecision.EJECT,
                    FailureDecision.QUIT,
                ),
            )
            if decision is FailureDecision.RETRY_VERIFY:
                continue
            return self._finish_failure(
                "verification", decision, tuple(generated), disc_id
            )

        try:
            removed = staging.cleanup(tuple(generated), report)
        except PartialCleanupError as error:
            self._ui.report_error(error)
            for track in error.removed:
                self._append_or_report(
                    "track_cleanup_result",
                    disc_id=disc_id,
                    run_id=staging.run_id,
                    result="removed",
                    **self._owned_fields(track),
                )
            for track in error.remaining:
                self._append_or_report(
                    "track_cleanup_result",
                    disc_id=disc_id,
                    run_id=staging.run_id,
                    result="preserved",
                    **self._owned_fields(track),
                )
            failure_journal_error = self._journal_failure(
                "cleanup", error, tuple(error.remaining), disc_id
            )
            if failure_journal_error is not None:
                self._report_journal_error(failure_journal_error)
            self._append_or_report(
                "workflow_held",
                disc_id=disc_id,
                reason="partial_cleanup",
                preserved_tracks=[
                    self._owned_fields(track) for track in error.remaining
                ],
            )
            return DiscOutcome.HELD
        except Exception as error:
            self._ui.report_error(error)
            for track in generated:
                self._append_or_report(
                    "track_cleanup_result",
                    disc_id=disc_id,
                    run_id=staging.run_id,
                    result="preserved",
                    **self._owned_fields(track),
                )
            failure_journal_error = self._journal_failure(
                "cleanup", error, tuple(generated), disc_id
            )
            if failure_journal_error is not None:
                self._report_journal_error(failure_journal_error)
            self._append_or_report(
                "workflow_held",
                disc_id=disc_id,
                reason="cleanup_failure",
                preserved_tracks=[
                    self._owned_fields(track) for track in generated
                ],
            )
            return DiscOutcome.HELD

        for track in removed:
            self._journal.append(
                "track_cleanup_result",
                disc_id=disc_id,
                run_id=staging.run_id,
                result="removed",
                **self._owned_fields(track),
            )
        return self._eject("completed", disc_id)

    def run(
        self, once: bool, poll_interval: float
    ) -> DiscOutcome | None:
        if poll_interval <= 0:
            raise ValueError("poll interval must be positive")
        phase = "polling"
        last_media_state: MediaState | None = None

        try:
            while True:
                phase = "polling"
                state = self._drive.status()
                last_media_state = state
                if state is MediaState.NO_MEDIA:
                    self._sleep(poll_interval)
                    continue
                if state is MediaState.UNUSABLE:
                    self._ui.report_unusable_media()
                    self._sleep(poll_interval)
                    continue

                phase = "processing"
                outcome = self.process_disc()
                if outcome is DiscOutcome.EJECTED:
                    if once:
                        return outcome
                    phase = "after_eject"
                    self._wait_for_no_media(poll_interval)
                    last_media_state = MediaState.NO_MEDIA
                if outcome is not DiscOutcome.EJECTED:
                    return outcome
        except KeyboardInterrupt:
            if phase == "processing":
                interrupt_state = "processing"
            elif last_media_state is MediaState.NO_MEDIA:
                interrupt_state = "no_media"
            elif phase == "after_eject":
                interrupt_state = "after_eject"
            else:
                interrupt_state = "polling"
            try:
                self._journal.append(
                    "workflow_interrupted",
                    media_state=interrupt_state,
                    preservation="no interrupt-triggered cleanup or eject",
                )
            except Exception:
                pass
            self._ui.report_preservation(interrupt_state)
            return None

    def _wait_for_no_media(self, poll_interval: float) -> None:
        while self._drive.status() is not MediaState.NO_MEDIA:
            self._sleep(poll_interval)

    def _prompt_failure(
        self,
        stage: str,
        error: Exception,
        generated: tuple[OwnedMp3, ...],
        disc_id: str | None,
        allowed: tuple[FailureDecision, ...],
        supplemental_journal_errors: tuple[Exception, ...] = (),
    ) -> FailureDecision:
        journal_errors = list(supplemental_journal_errors)
        failure_journal_error = self._journal_failure(
            stage, error, generated, disc_id
        )
        if failure_journal_error is not None:
            journal_errors.append(failure_journal_error)
        displayed_error = self._supplement_error(error, tuple(journal_errors))
        decision = self._ui.decide_failure(displayed_error, allowed)
        decision_journal_error = self._append_journal(
            "failure_decision",
            disc_id=disc_id,
            stage=stage,
            decision=decision.value,
        )
        if decision_journal_error is not None:
            self._report_journal_error(decision_journal_error)
        return decision

    def _finish_failure(
        self,
        stage: str,
        decision: FailureDecision,
        generated: tuple[OwnedMp3, ...],
        disc_id: str | None,
    ) -> DiscOutcome:
        if decision is FailureDecision.EJECT:
            return self._eject(
                f"{stage}_failure_operator_eject",
                disc_id,
                preserved_tracks=generated,
            )
        if decision in {
            FailureDecision.RETRY_IMPORT,
            FailureDecision.RETRY_VERIFY,
        }:
            raise RuntimeError(f"unhandled retry decision for {stage}: {decision}")
        journal_error = self._append_journal(
            "workflow_quit",
            disc_id=disc_id,
            stage=stage,
            preserved_tracks=[
                self._owned_fields(track) for track in generated
            ],
        )
        if journal_error is not None:
            self._report_journal_error(journal_error)
        return DiscOutcome.QUIT

    def _eject(
        self,
        reason: str,
        disc_id: str | None,
        *,
        preserved_tracks: tuple[OwnedMp3, ...] = (),
    ) -> DiscOutcome:
        try:
            self._drive.eject()
        except Exception as error:
            outcome_journal_error = self._append_journal(
                "eject_outcome",
                disc_id=disc_id,
                reason=reason,
                ok=False,
                error=str(error),
            )
            decision = self._prompt_failure(
                "eject",
                error,
                preserved_tracks,
                disc_id,
                (FailureDecision.QUIT,),
                (
                    ()
                    if outcome_journal_error is None
                    else (outcome_journal_error,)
                ),
            )
            self._append_or_report(
                "workflow_held",
                disc_id=disc_id,
                reason="eject_failure",
                preserved_tracks=[
                    self._owned_fields(track) for track in preserved_tracks
                ],
            )
            if decision is not FailureDecision.QUIT:
                raise RuntimeError(
                    f"unsupported eject failure decision: {decision}"
                )
            return DiscOutcome.QUIT
        outcome_journal_error = self._append_journal(
            "eject_outcome",
            disc_id=disc_id,
            reason=reason,
            ok=True,
        )
        if outcome_journal_error is not None:
            self._report_journal_error(outcome_journal_error)
        return DiscOutcome.EJECTED

    def _journal_failure(
        self,
        stage: str,
        error: Exception,
        generated: tuple[OwnedMp3, ...],
        disc_id: str | None,
    ) -> Exception | None:
        return self._append_journal(
            "workflow_failed",
            disc_id=disc_id,
            stage=stage,
            error=str(error),
            preserved_tracks=[
                self._owned_fields(track) for track in generated
            ],
        )

    def _append_journal(self, event: str, **fields: Any) -> Exception | None:
        try:
            self._journal.append(event, **fields)
        except Exception as error:
            return error
        return None

    def _append_or_report(self, event: str, **fields: Any) -> None:
        error = self._append_journal(event, **fields)
        if error is not None:
            self._report_journal_error(error)

    def _report_journal_error(self, error: Exception) -> None:
        self._ui.report_error(f"Journal append failed: {error}")

    @staticmethod
    def _supplement_error(
        original: Exception,
        journal_errors: tuple[Exception, ...],
    ) -> Exception:
        if not journal_errors:
            return original
        details = [str(original)]
        details.extend(
            f"Journal append failed: {error}" for error in journal_errors
        )
        supplemented = RuntimeError("\n".join(details))
        supplemented.__cause__ = original
        return supplemented

    def _music_is_running(self) -> bool:
        if self._music_detector is None:
            return False
        try:
            return bool(self._music_detector.is_running())
        except Exception:
            return False

    @staticmethod
    def _is_busy_error(error: BaseException) -> bool:
        if isinstance(error, OSError) and error.errno in {
            errno.EBUSY,
            errno.EAGAIN,
        }:
            return True
        return "busy" in str(error).casefold()

    @staticmethod
    def _release_fields(release: ReleaseMetadata) -> dict[str, Any]:
        return {
            "release_id": release.release_id,
            "albumartist": release.albumartist,
            "album": release.album,
            "date": release.date,
            "country": release.country,
            "disambiguation": release.disambiguation,
            "medium": release.medium,
            "track_count": len(release.tracks),
            "tracks": [
                {
                    "position": track.position,
                    "recording_id": track.recording_id,
                    "title": track.title,
                    "length_ms": track.length_ms,
                }
                for track in sorted(
                    release.tracks,
                    key=lambda candidate: candidate.position,
                )
            ],
        }

    @classmethod
    def _snapshot_fields(cls, snapshot: LibrarySnapshot) -> dict[str, Any]:
        return {
            "exact_items": [
                cls._library_item_fields(item) for item in snapshot.exact_items
            ],
            "plausible_items": [
                cls._library_item_fields(item)
                for item in snapshot.plausible_items
            ],
        }

    @staticmethod
    def _library_item_fields(item: LibraryItem) -> dict[str, Any]:
        return {
            "item_id": item.item_id,
            "album_id": item.album_id,
            "release_id": item.release_id,
            "recording_id": item.recording_id,
            "albumartist": item.albumartist,
            "album": item.album,
            "medium": item.medium,
            "track": item.track,
            "tracktotal": item.tracktotal,
            "path": str(item.path),
            "format": item.format,
            "bitrate": item.bitrate,
        }

    @staticmethod
    def _owned_fields(track: OwnedMp3) -> dict[str, Any]:
        return {
            "path": str(track.path),
            "device": track.device,
            "inode": track.inode,
            "track_position": track.track_position,
        }

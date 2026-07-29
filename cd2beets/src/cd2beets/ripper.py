from __future__ import annotations

from cd2beets.journal import Journal
from cd2beets.models import DiscToc, OwnedMp3, TrackMetadata
from cd2beets.process import PipelineResult, ProcessRunner
from cd2beets.staging import StagingRun


class RipError(RuntimeError):
    def __init__(self, result: PipelineResult) -> None:
        details = [
            "track rip failed: "
            f"source exit status {result.source_returncode}; "
            f"sink exit status {result.sink_returncode}"
        ]
        if result.source_stderr:
            details.append(f"source stderr: {result.source_stderr}")
        if result.sink_stderr:
            details.append(f"sink stderr: {result.sink_stderr}")
        super().__init__("\n".join(details))
        self.result = result


class Ripper:
    def __init__(self, runner: ProcessRunner, journal: Journal) -> None:
        self._runner = runner
        self._journal = journal

    def rip_track(
        self,
        toc: DiscToc,
        track: TrackMetadata,
        staging: StagingRun,
    ) -> OwnedMp3:
        owned, destination = staging.reserve(track.position)
        try:
            common_fields = {
                "run_id": staging.run_id,
                "track_position": track.position,
                "path": str(owned.path),
            }
            source_args = [
                "cdda2wav",
                "-D",
                toc.device,
                "-paranoia",
                "-O",
                "wav",
                "-no-infofile",
                "-t",
                str(track.position),
                "-",
            ]
            sink_args = ["lame", "--silent", "-b", "320", "-", "-"]
            self._journal.append("track_rip_started", **common_fields)

            try:
                result = self._runner.pipeline(
                    source_args,
                    sink_args,
                    destination,
                )
            except BaseException as error:
                try:
                    self._journal.append(
                        "track_rip_failed",
                        **common_fields,
                        error=str(error),
                    )
                except BaseException as journal_error:
                    raise error from journal_error
                raise

            result_fields = {
                **common_fields,
                "source_returncode": result.source_returncode,
                "sink_returncode": result.sink_returncode,
                "source_stderr": result.source_stderr,
                "sink_stderr": result.sink_stderr,
            }
            if not result.ok:
                rip_error = RipError(result)
                try:
                    self._journal.append("track_rip_failed", **result_fields)
                except BaseException as journal_error:
                    raise rip_error from journal_error
                raise rip_error
            self._journal.append("track_rip_succeeded", **result_fields)
            return owned
        finally:
            destination.close()

from __future__ import annotations

from pathlib import Path
from uuid import UUID

from cd2beets.journal import Journal
from cd2beets.process import CommandResult, ProcessRunner


class BeetsImportError(RuntimeError):
    def __init__(self, result: CommandResult) -> None:
        details = [f"beet import failed with exit status {result.returncode}"]
        if result.stdout:
            details.append(f"stdout: {result.stdout}")
        if result.stderr:
            details.append(f"stderr: {result.stderr}")
        super().__init__("\n".join(details))
        self.result = result


class BeetsImporter:
    def __init__(self, runner: ProcessRunner, journal: Journal) -> None:
        self._runner = runner
        self._journal = journal

    def import_release(self, release_id: str, run_dir: Path) -> CommandResult:
        run_dir_text = str(run_dir)
        common_fields = {
            "release_id": release_id,
            "run_dir": run_dir_text,
        }
        try:
            parsed_release_id = UUID(release_id)
        except (AttributeError, TypeError, ValueError):
            parsed_release_id = None
        if parsed_release_id is None or str(parsed_release_id) != release_id:
            error = f"invalid MusicBrainz release ID: {release_id}"
            self._journal.append(
                "beets_import_failed",
                **common_fields,
                error=error,
            )
            raise ValueError(error)

        args = [
            "beet",
            "import",
            "--copy",
            "--quiet",
            "--quiet-fallback=skip",
            "--noresume",
            "--noincremental",
            f"--search-id={release_id}",
            run_dir_text,
        ]
        self._journal.append("beets_import_started", **common_fields)
        try:
            result = self._runner.run(args)
        except BaseException as error:
            try:
                self._journal.append(
                    "beets_import_failed",
                    **common_fields,
                    error=str(error),
                )
            except BaseException as journal_error:
                raise error from journal_error
            raise

        result_fields = {
            **common_fields,
            "returncode": result.returncode,
            "stdout": result.stdout,
            "stderr": result.stderr,
        }
        if result.returncode != 0:
            import_error = BeetsImportError(result)
            try:
                self._journal.append("beets_import_failed", **result_fields)
            except BaseException as journal_error:
                raise import_error from journal_error
            raise import_error
        self._journal.append("beets_import_succeeded", **result_fields)
        return result

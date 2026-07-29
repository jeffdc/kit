from pathlib import Path

import pytest

from cd2beets.importer import BeetsImportError, BeetsImporter
from cd2beets.process import CommandResult


RELEASE_ID = "11111111-1111-1111-1111-111111111111"
RUN_DIR = Path("/local/staging/run-1")


class RecordingJournal:
    def __init__(self):
        self.records = []

    def append(self, event, **fields):
        self.records.append((event, fields))


class FakeRunner:
    def __init__(self, result: CommandResult):
        self.result = result
        self.calls = []

    def run(self, args):
        self.calls.append(list(args))
        return self.result


def test_import_release_runs_only_fixed_copy_command():
    expected = CommandResult(0, "imported", "diagnostic")
    runner = FakeRunner(expected)
    journal = RecordingJournal()

    result = BeetsImporter(runner, journal).import_release(RELEASE_ID, RUN_DIR)

    assert result is expected
    assert runner.calls == [
        [
            "beet",
            "import",
            "--copy",
            "--quiet",
            "--quiet-fallback=skip",
            "--noresume",
            "--noincremental",
            "--search-id=11111111-1111-1111-1111-111111111111",
            "/local/staging/run-1",
        ]
    ]
    assert journal.records == [
        (
            "beets_import_started",
            {"release_id": RELEASE_ID, "run_dir": str(RUN_DIR)},
        ),
        (
            "beets_import_succeeded",
            {
                "release_id": RELEASE_ID,
                "run_dir": str(RUN_DIR),
                "returncode": 0,
                "stdout": "imported",
                "stderr": "diagnostic",
            },
        ),
    ]


def test_import_release_accepts_no_extra_beets_arguments():
    importer = BeetsImporter(FakeRunner(CommandResult(0, "", "")), RecordingJournal())

    with pytest.raises(TypeError):
        importer.import_release(RELEASE_ID, RUN_DIR, ["--pretend"])


def test_import_release_rejects_noncanonical_musicbrainz_uuid_without_running():
    runner = FakeRunner(CommandResult(0, "", ""))
    journal = RecordingJournal()

    with pytest.raises(ValueError, match="MusicBrainz release ID"):
        BeetsImporter(runner, journal).import_release("not-a-release-id", RUN_DIR)

    assert runner.calls == []
    assert journal.records == [
        (
            "beets_import_failed",
            {
                "release_id": "not-a-release-id",
                "run_dir": str(RUN_DIR),
                "error": "invalid MusicBrainz release ID: not-a-release-id",
            },
        )
    ]


@pytest.mark.parametrize(
    "release_id",
    [
        "11111111111111111111111111111111",
        "11111111-1111-1111-1111-11111111111",
        "11111111-1111-1111-1111-111111111111 ",
    ],
)
def test_import_release_requires_canonical_uuid_spelling(release_id: str):
    runner = FakeRunner(CommandResult(0, "", ""))

    with pytest.raises(ValueError, match="MusicBrainz release ID"):
        BeetsImporter(runner, RecordingJournal()).import_release(release_id, RUN_DIR)

    assert runner.calls == []


def test_import_failure_preserves_stdout_and_stderr():
    failed = CommandResult(4, "partial stdout", "specific stderr")
    runner = FakeRunner(failed)
    journal = RecordingJournal()

    with pytest.raises(BeetsImportError) as raised:
        BeetsImporter(runner, journal).import_release(RELEASE_ID, RUN_DIR)

    assert raised.value.result is failed
    assert "exit status 4" in str(raised.value)
    assert "partial stdout" in str(raised.value)
    assert "specific stderr" in str(raised.value)
    assert journal.records == [
        (
            "beets_import_started",
            {"release_id": RELEASE_ID, "run_dir": str(RUN_DIR)},
        ),
        (
            "beets_import_failed",
            {
                "release_id": RELEASE_ID,
                "run_dir": str(RUN_DIR),
                "returncode": 4,
                "stdout": "partial stdout",
                "stderr": "specific stderr",
            },
        ),
    ]

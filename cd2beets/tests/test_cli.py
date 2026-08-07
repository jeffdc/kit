from pathlib import Path

import pytest

from cd2beets import cli
from cd2beets.drive import DriveInfo
from cd2beets.library import BeetsSettings
from cd2beets.preflight import PreflightReport
from cd2beets.workflow import DiscOutcome


DRIVE = DriveInfo(1, "hp", "CDDVDW TS-L633R", "0200", "USB", "Unsupported")


def configured_settings(tmp_path):
    return BeetsSettings(
        library_path=tmp_path / "library.db",
        library_root=tmp_path / "library",
        musicbrainz_host="musicbrainz.org",
        plugins=("musicbrainz",),
        duplicate_action="skip",
        duplicates_delete=False,
        duplicates_remove=False,
        duplicates_move="",
    )


def test_parser_defaults():
    args = cli.build_parser().parse_args([])

    assert args.staging_dir == Path("~/Music/cd2beets-staging").expanduser()
    assert args.poll_interval == 2.0
    assert args.once is False


def test_parser_accepts_only_the_three_custom_flags(tmp_path):
    staging = tmp_path / "stage"

    args = cli.build_parser().parse_args(
        ["--staging-dir", str(staging), "--poll-interval", "0.5", "--once"]
    )

    assert args.staging_dir == staging
    assert args.poll_interval == 0.5
    assert args.once is True
    custom_options = {
        option
        for action in cli.build_parser()._actions
        for option in action.option_strings
        if option != "--help" and option != "-h"
    }
    assert custom_options == {"--staging-dir", "--poll-interval", "--once"}


@pytest.mark.parametrize("value", ["0", "-1", "-0.01"])
def test_parser_rejects_non_positive_poll_interval(value):
    with pytest.raises(SystemExit):
        cli.build_parser().parse_args(["--poll-interval", value])


@pytest.mark.parametrize(
    "arguments",
    [
        ["cleanup"],
        ["--cleanup"],
        ["--beet-args", "remove", "artist:foo"],
        ["--beets-command", "import"],
    ],
)
def test_parser_rejects_cleanup_and_arbitrary_beets_passthrough(arguments):
    with pytest.raises(SystemExit):
        cli.build_parser().parse_args(arguments)


class Runtime:
    def __init__(self, tmp_path, monkeypatch, *, errors=(), outcome=DiscOutcome.EJECTED,
                 interrupt=False):
        self.settings = configured_settings(tmp_path)
        self.events = []
        self.workflow_kwargs = None
        self.workflow_calls = []
        self.staging_calls = []
        self.staging_initializations = []
        runtime = self

        monkeypatch.setattr(
            cli.BeetsSettings,
            "load",
            classmethod(lambda cls: runtime.settings),
        )

        class Runner:
            pass

        class Drive:
            def __init__(self, runner):
                self.runner = runner
                self.eject_calls = 0
                runtime.drive = self

            def eject(self):
                self.eject_calls += 1

        class Metadata:
            def __init__(self, host):
                self.host = host
                runtime.metadata = self

        class FakePreflight:
            def __init__(self, settings, staging_dir, drive, metadata):
                runtime.preflight_args = (
                    settings,
                    staging_dir,
                    drive,
                    metadata,
                )

            def run(self):
                runtime.events.append("preflight")
                return PreflightReport(
                    tuple(errors),
                    runtime.settings.library_path.resolve(),
                    runtime.settings.library_root.resolve(),
                    runtime.preflight_args[1].resolve(),
                    DRIVE if not errors else None,
                )

        class FakeJournal:
            def __init__(self, path):
                self.path = path
                runtime.journal = self

        class FakeLibrary:
            def __init__(self, settings):
                self.settings = settings
                runtime.library = self

        class FakeReader:
            def __init__(self):
                runtime.reader = self

        class FakeRipper:
            def __init__(self, runner, journal):
                self.args = (runner, journal)
                runtime.ripper = self

        class FakeImporter:
            def __init__(self, runner, journal):
                self.args = (runner, journal)
                runtime.importer = self

        class FakeUI:
            def __init__(self):
                runtime.ui = self

        class FakeDetector:
            def __init__(self, runner):
                self.runner = runner
                runtime.detector = self

        class FakeStaging:
            @classmethod
            def _initialize_staging_root(cls, staging_dir):
                runtime.staging_initializations.append(staging_dir)

            def __init__(self, staging_dir, journal):
                runtime.staging_calls.append((staging_dir, journal))

        class FakeWorkflow:
            def __init__(self, **kwargs):
                runtime.workflow_kwargs = kwargs

            def run(self, once, poll_interval):
                runtime.workflow_calls.append((once, poll_interval))
                if interrupt:
                    raise KeyboardInterrupt
                return outcome

        monkeypatch.setattr(cli, "ProcessRunner", Runner)
        monkeypatch.setattr(cli, "DrutilDrive", Drive)
        monkeypatch.setattr(cli, "MusicBrainzClient", Metadata)
        monkeypatch.setattr(cli, "Preflight", FakePreflight)
        monkeypatch.setattr(cli, "Journal", FakeJournal)
        monkeypatch.setattr(cli, "BeetsLibrary", FakeLibrary)
        monkeypatch.setattr(cli, "DiscidReader", FakeReader)
        monkeypatch.setattr(cli, "Ripper", FakeRipper)
        monkeypatch.setattr(cli, "BeetsImporter", FakeImporter)
        monkeypatch.setattr(cli, "TerminalUI", FakeUI)
        monkeypatch.setattr(cli, "MusicProcessDetector", FakeDetector)
        monkeypatch.setattr(cli, "StagingRun", FakeStaging)
        monkeypatch.setattr(cli, "DiscWorkflow", FakeWorkflow)


def test_main_prints_show_cd_reminder_before_preflight(tmp_path, monkeypatch, capsys):
    runtime = Runtime(tmp_path, monkeypatch)
    original_run = cli.Preflight.run

    def assert_reminder_then_run(preflight):
        reminder = capsys.readouterr().out
        assert "Show CD" in reminder
        assert "do not select an import option" in reminder
        return original_run(preflight)

    monkeypatch.setattr(cli.Preflight, "run", assert_reminder_then_run)

    assert cli.main(["--once"]) == 0
    assert runtime.events == ["preflight"]


def test_main_prints_exact_aggregated_preflight_errors_and_returns_two(
    tmp_path, monkeypatch, capsys
):
    errors = ("missing prerequisite: lame", "MusicBrainz ping failed: offline")
    runtime = Runtime(tmp_path, monkeypatch, errors=errors)

    result = cli.main(["--once"])

    output = capsys.readouterr().out
    assert result == 2
    assert output.splitlines()[-2:] == [
        "missing prerequisite: lame",
        "MusicBrainz ping failed: offline",
    ]
    assert runtime.workflow_kwargs is None
    assert runtime.staging_calls == []
    assert runtime.staging_initializations == []
    assert runtime.drive.eject_calls == 0


def test_main_composes_reviewed_adapters_after_successful_preflight(
    tmp_path, monkeypatch
):
    runtime = Runtime(tmp_path, monkeypatch)
    staging = tmp_path / "local-stage"

    result = cli.main(
        ["--staging-dir", str(staging), "--poll-interval", "0.25", "--once"]
    )

    assert result == 0
    assert runtime.preflight_args[0] is runtime.settings
    assert runtime.preflight_args[1] == staging.resolve()
    assert runtime.preflight_args[2] is runtime.drive
    assert runtime.preflight_args[3] is runtime.metadata
    assert runtime.metadata.host == "https://musicbrainz.org"
    assert runtime.journal.path == staging.resolve() / "lifecycle.jsonl"
    assert runtime.staging_initializations == [staging.resolve()]
    assert runtime.workflow_kwargs["drive"] is runtime.drive
    assert runtime.workflow_kwargs["disc_reader"] is runtime.reader
    assert runtime.workflow_kwargs["metadata"] is runtime.metadata
    assert runtime.workflow_kwargs["library"] is runtime.library
    assert runtime.workflow_kwargs["library_root"] == runtime.settings.library_root.resolve()
    assert runtime.workflow_kwargs["journal"] is runtime.journal
    assert runtime.workflow_kwargs["ripper"] is runtime.ripper
    assert runtime.workflow_kwargs["importer"] is runtime.importer
    assert runtime.workflow_kwargs["ui"] is runtime.ui
    assert runtime.workflow_kwargs["music_detector"] is runtime.detector
    assert callable(runtime.workflow_kwargs["staging_factory"])
    assert runtime.workflow_calls == [(True, 0.25)]
    assert runtime.staging_calls == []


@pytest.mark.parametrize(
    ("outcome", "expected"),
    [
        (DiscOutcome.EJECTED, 0),
        (DiscOutcome.HELD, 1),
        (DiscOutcome.QUIT, 1),
        (None, 0),
    ],
)
def test_main_maps_workflow_outcomes_to_exit_codes(
    tmp_path, monkeypatch, outcome, expected
):
    Runtime(tmp_path, monkeypatch, outcome=outcome)

    assert cli.main(["--once"]) == expected


def test_main_preserves_interrupt_without_cleanup_or_eject(
    tmp_path, monkeypatch
):
    runtime = Runtime(tmp_path, monkeypatch, interrupt=True)

    assert cli.main(["--once"]) == 0
    assert runtime.staging_calls == []
    assert runtime.drive.eject_calls == 0

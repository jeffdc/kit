from __future__ import annotations

import argparse
import math
from pathlib import Path
from typing import Sequence

from cd2beets.drive import DrutilDrive
from cd2beets.importer import BeetsImporter
from cd2beets.journal import Journal
from cd2beets.library import BeetsLibrary, BeetsSettings
from cd2beets.metadata import DiscidReader, MusicBrainzClient
from cd2beets.preflight import Preflight
from cd2beets.process import ProcessRunner
from cd2beets.ripper import Ripper
from cd2beets.staging import StagingRun
from cd2beets.ui import TerminalUI
from cd2beets.verification import verify_import
from cd2beets.workflow import DiscOutcome, DiscWorkflow, MusicProcessDetector


_DEFAULT_STAGING_DIR = Path("~/Music/cd2beets-staging").expanduser()
_IGNORE_CD_REMINDER = (
    "Before continuing, set Music > Settings > General > "
    "When a CD is inserted to Ignore CD."
)


def _positive_float(value: str) -> float:
    try:
        parsed = float(value)
    except ValueError as error:
        raise argparse.ArgumentTypeError("must be a positive number") from error
    if not math.isfinite(parsed) or parsed <= 0:
        raise argparse.ArgumentTypeError("must be a positive number")
    return parsed


def _expanded_path(value: str) -> Path:
    return Path(value).expanduser()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="cd2beets",
        description="Safely rip audio CDs into the configured beets library.",
        allow_abbrev=False,
    )
    parser.add_argument(
        "--staging-dir",
        type=_expanded_path,
        default=_DEFAULT_STAGING_DIR,
        metavar="PATH",
        help="private local staging root (default: %(default)s)",
    )
    parser.add_argument(
        "--poll-interval",
        type=_positive_float,
        default=2.0,
        metavar="SECONDS",
        help="seconds between drive status checks (default: %(default)s)",
    )
    parser.add_argument(
        "--once",
        action="store_true",
        help="process at most one detected disc",
    )
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    print(_IGNORE_CD_REMINDER, flush=True)

    try:
        settings = BeetsSettings.load()
        staging_dir = args.staging_dir.expanduser().resolve()
        runner = ProcessRunner()
        drive = DrutilDrive(runner)
        metadata = MusicBrainzClient(settings.musicbrainz_host)
        journal = Journal(staging_dir / "lifecycle.jsonl")
        report = Preflight(
            settings,
            staging_dir,
            runner,
            drive,
            metadata,
        ).run()
    except KeyboardInterrupt:
        return 0
    except Exception as error:
        print(f"preflight failed: {error}", flush=True)
        return 2

    if report.errors:
        for error in report.errors:
            print(error, flush=True)
        return 2

    try:
        StagingRun._initialize_staging_root(report.staging_dir)
        library = BeetsLibrary(settings)
        disc_reader = DiscidReader()
        ripper = Ripper(runner, journal)
        importer = BeetsImporter(runner, journal)
        ui = TerminalUI()
        music_detector = MusicProcessDetector(runner)
        workflow = DiscWorkflow(
            drive=drive,
            disc_reader=disc_reader,
            metadata=metadata,
            library=library,
            library_root=report.library_root,
            journal=journal,
            ripper=ripper,
            importer=importer,
            staging_factory=lambda: StagingRun(report.staging_dir, journal),
            verifier=verify_import,
            ui=ui,
            music_detector=music_detector,
        )
        outcome = workflow.run(
            once=args.once,
            poll_interval=args.poll_interval,
        )
    except KeyboardInterrupt:
        return 0
    except Exception as error:
        print(f"workflow failed: {error}", flush=True)
        return 1

    if outcome is None or outcome is DiscOutcome.EJECTED:
        return 0
    return 1

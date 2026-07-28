from pathlib import Path

import pytest

from cd2beets.drive import (
    DriveError,
    DriveInfo,
    DrutilDrive,
    parse_drive_list,
    parse_media_state,
)
from cd2beets.models import MediaState
from cd2beets.process import CommandResult


FIXTURES = Path(__file__).parent / "fixtures"


class FakeRunner:
    def __init__(self, *results: CommandResult):
        self.results = list(results)
        self.calls: list[list[str]] = []

    def run(self, args):
        self.calls.append(list(args))
        return self.results.pop(0)


def fixture_text(name: str) -> str:
    return (FIXTURES / name).read_text()


def test_parse_drive_list_uses_numbered_fixed_column_rows():
    list_text = fixture_text("drutil_list_one_drive.txt")

    assert parse_drive_list(list_text) == (
        DriveInfo(1, "hp", "CDDVDW TS-L633R", "0200", "USB", "Unsupported"),
    )


def test_parse_media_state_recognizes_no_media():
    no_media_text = fixture_text("drutil_status_no_media.txt")

    assert parse_media_state(no_media_text) is MediaState.NO_MEDIA


def test_parse_media_state_recognizes_audio_cd():
    audio_text = fixture_text("drutil_status_audio.txt")

    assert parse_media_state(audio_text) is MediaState.AUDIO


def test_parse_media_state_treats_other_media_as_unusable():
    assert parse_media_state("           Type: DVD-ROM\n") is MediaState.UNUSABLE


def test_list_drives_uses_exact_drutil_vector():
    runner = FakeRunner(
        CommandResult(0, fixture_text("drutil_list_one_drive.txt"), "")
    )

    drives = DrutilDrive(runner).list_drives()

    assert drives == (
        DriveInfo(1, "hp", "CDDVDW TS-L633R", "0200", "USB", "Unsupported"),
    )
    assert runner.calls == [["drutil", "list"]]


@pytest.mark.parametrize(
    ("stdout", "description"),
    [
        (
            "   Vendor   Product           Rev   Bus       SupportLevel\n",
            "zero",
        ),
        (
            "   Vendor   Product           Rev   Bus       SupportLevel\n"
            "1  hp       CDDVDW TS-L633R   0200  USB       Unsupported\n"
            "2  acme     Optical Drive     0001  USB       Supported\n",
            "multiple",
        ),
    ],
)
def test_list_drives_rejects_zero_or_multiple_selected_drives(stdout, description):
    runner = FakeRunner(CommandResult(0, stdout, ""))

    with pytest.raises(DriveError, match=description):
        DrutilDrive(runner).list_drives()

    assert runner.calls == [["drutil", "list"]]


def test_status_uses_exact_vector_without_ejecting():
    runner = FakeRunner(
        CommandResult(0, fixture_text("drutil_status_no_media.txt"), "")
    )

    state = DrutilDrive(runner).status()

    assert state is MediaState.NO_MEDIA
    assert runner.calls == [["drutil", "status"]]


def test_status_error_preserves_command_output():
    runner = FakeRunner(CommandResult(1, "partial status", "device unavailable"))

    with pytest.raises(DriveError) as raised:
        DrutilDrive(runner).status()

    assert "partial status" in str(raised.value)
    assert "device unavailable" in str(raised.value)
    assert runner.calls == [["drutil", "status"]]


def test_eject_uses_exact_drutil_vector():
    runner = FakeRunner(CommandResult(0, "ejected", ""))

    DrutilDrive(runner).eject()

    assert runner.calls == [["drutil", "eject"]]


def test_nonzero_eject_preserves_stderr():
    runner = FakeRunner(CommandResult(1, "", "disc is busy"))

    with pytest.raises(DriveError, match="disc is busy"):
        DrutilDrive(runner).eject()

    assert runner.calls == [["drutil", "eject"]]

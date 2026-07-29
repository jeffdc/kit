from __future__ import annotations

from io import StringIO

import pytest

from cd2beets.models import (
    FailureDecision,
    OperatorDecision,
    ReleaseMetadata,
    TrackMetadata,
)
from cd2beets.ui import TerminalUI


def make_release(
    release_id: str = "release-1",
    *,
    artist: str = "The Artist",
    album: str = "The Album",
    date: str = "2001-02-03",
    country: str = "GB",
    disambiguation: str = "deluxe edition",
    medium: int = 2,
    track_count: int = 2,
) -> ReleaseMetadata:
    return ReleaseMetadata(
        release_id=release_id,
        albumartist=artist,
        album=album,
        date=date,
        country=country,
        disambiguation=disambiguation,
        medium=medium,
        tracks=tuple(
            TrackMetadata(position, f"recording-{position}", f"Track {position}", None)
            for position in range(1, track_count + 1)
        ),
    )


def make_ui(responses: str = "") -> tuple[TerminalUI, StringIO]:
    output = StringIO()
    return TerminalUI(input_stream=StringIO(responses), output_stream=output), output


def test_one_complete_candidate_is_displayed_and_selected_without_prompt() -> None:
    ui, output = make_ui()
    release = make_release()

    selected = ui.select_release((release,))

    assert selected is release
    text = output.getvalue()
    assert "release-1" in text
    assert "The Artist" in text
    assert "The Album" in text
    assert "2001-02-03" in text
    assert "GB" in text
    assert "deluxe edition" in text
    assert "Medium: 2" in text
    assert "Track count: 2" in text
    assert "Select release" not in text


def test_multiple_candidates_display_every_field_and_require_numeric_selection() -> None:
    first = make_release(
        "release-1",
        artist="First Artist",
        album="First Album",
        date="1999",
        country="US",
        disambiguation="first pressing",
        medium=1,
        track_count=1,
    )
    second = make_release(
        "release-2",
        artist="Second Artist",
        album="Second Album",
        date="2005-06",
        country="CA",
        disambiguation="remaster",
        medium=3,
        track_count=3,
    )
    ui, output = make_ui("2\n")

    selected = ui.select_release((first, second))

    assert selected is second
    text = output.getvalue()
    for expected in (
        "release-1",
        "First Artist",
        "First Album",
        "1999",
        "US",
        "first pressing",
        "Medium: 1",
        "Track count: 1",
        "release-2",
        "Second Artist",
        "Second Album",
        "2005-06",
        "CA",
        "remaster",
        "Medium: 3",
        "Track count: 3",
        "Select release [1-2]",
    ):
        assert expected in text


def test_invalid_release_selection_is_reprompted_without_selecting() -> None:
    releases = (make_release("release-1"), make_release("release-2"))
    ui, output = make_ui("0\nwords\n3\n1\n")

    assert ui.select_release(releases) is releases[0]
    assert output.getvalue().count("Select release [1-2]") == 4
    assert output.getvalue().count("Invalid selection") == 3


def test_select_release_rejects_an_empty_candidate_set() -> None:
    ui, _ = make_ui()

    with pytest.raises(ValueError, match="no complete release candidates"):
        ui.select_release(())


@pytest.mark.parametrize(
    ("response", "expected"),
    [
        ("\n", OperatorDecision.SKIP_EJECT),
        ("s\n", OperatorDecision.SKIP_EJECT),
        ("p\n", OperatorDecision.PROCEED),
        ("a\n", OperatorDecision.ABORT_KEEP_LOADED),
    ],
)
def test_uncertain_presence_prompt_has_conservative_explicit_choices(
    response: str, expected: OperatorDecision
) -> None:
    ui, output = make_ui(response)

    decision = ui.decide_uncertain(("missing expected track",))

    assert decision is expected
    text = output.getvalue()
    assert "missing expected track" in text
    assert "[s]kip/eject (default), [p]roceed, [a]bort/keep loaded" in text


def test_uncertain_presence_eof_keeps_disc_loaded() -> None:
    ui, _ = make_ui()

    assert (
        ui.decide_uncertain(("conflicting candidate",))
        is OperatorDecision.ABORT_KEEP_LOADED
    )


def test_invalid_uncertain_presence_input_is_reprompted_without_action() -> None:
    ui, output = make_ui("yes\ne\np\n")

    assert ui.decide_uncertain(("conflicting candidate",)) is OperatorDecision.PROCEED
    assert output.getvalue().count("[s]kip/eject (default)") == 3
    assert output.getvalue().count("Invalid choice") == 2


@pytest.mark.parametrize(
    ("allowed", "response", "expected", "shown", "hidden"),
    [
        (
            (
                FailureDecision.RETRY_IMPORT,
                FailureDecision.EJECT,
                FailureDecision.QUIT,
            ),
            "i\n",
            FailureDecision.RETRY_IMPORT,
            "[i] retry import",
            "retry verification",
        ),
        (
            (
                FailureDecision.RETRY_VERIFY,
                FailureDecision.EJECT,
                FailureDecision.QUIT,
            ),
            "v\n",
            FailureDecision.RETRY_VERIFY,
            "[v] retry verification",
            "retry import",
        ),
        (
            (FailureDecision.EJECT, FailureDecision.QUIT),
            "e\n",
            FailureDecision.EJECT,
            "[e] eject",
            "retry",
        ),
        (
            (FailureDecision.EJECT, FailureDecision.QUIT),
            "\n",
            FailureDecision.QUIT,
            "[q] quit/keep loaded (default)",
            "retry",
        ),
    ],
)
def test_failure_prompt_exposes_only_stage_valid_decisions(
    allowed: tuple[FailureDecision, ...],
    response: str,
    expected: FailureDecision,
    shown: str,
    hidden: str,
) -> None:
    ui, output = make_ui(response)

    decision = ui.decide_failure("command failed\nstderr: device busy", allowed)

    assert decision is expected
    text = output.getvalue()
    assert "command failed\nstderr: device busy" in text
    assert shown in text
    assert hidden not in text


def test_failure_prompt_eof_quits_and_keeps_disc_loaded() -> None:
    ui, _ = make_ui()

    assert (
        ui.decide_failure(
            "import failed",
            (
                FailureDecision.RETRY_IMPORT,
                FailureDecision.EJECT,
                FailureDecision.QUIT,
            ),
        )
        is FailureDecision.QUIT
    )


def test_failure_prompt_reprompts_invalid_or_stage_invalid_input() -> None:
    ui, output = make_ui("i\nanything\nq\n")

    decision = ui.decide_failure(
        "verification invariant failed",
        (FailureDecision.RETRY_VERIFY, FailureDecision.EJECT, FailureDecision.QUIT),
    )

    assert decision is FailureDecision.QUIT
    assert output.getvalue().count("Failure choices:") == 3
    assert output.getvalue().count("Invalid choice") == 2


def test_failure_prompt_rejects_empty_or_duplicate_decision_sets() -> None:
    ui, _ = make_ui()

    with pytest.raises(ValueError, match="at least one failure decision"):
        ui.decide_failure("failed", ())
    with pytest.raises(ValueError, match="duplicate failure decision"):
        ui.decide_failure(
            "failed", (FailureDecision.EJECT, FailureDecision.EJECT)
        )

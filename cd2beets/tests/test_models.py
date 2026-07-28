from pathlib import Path

from cd2beets.models import (
    ReleaseMetadata,
    TrackMetadata,
    VerificationReport,
    normalize_text,
)


def test_normalize_text_is_case_accent_and_punctuation_insensitive():
    assert normalize_text("Beyoncé — Live!") == "beyonce live"


def test_release_exposes_expected_recording_ids():
    release = ReleaseMetadata(
        release_id="release-1",
        albumartist="Artist",
        album="Album",
        date="2001",
        country="US",
        disambiguation="",
        medium=1,
        tracks=(
            TrackMetadata(1, "track-1", "One", 120_000),
            TrackMetadata(2, "track-2", "Two", 180_000),
        ),
    )
    assert release.recording_ids == ("track-1", "track-2")


def test_verification_report_is_immutable_and_boolean_by_errors():
    report = VerificationReport(errors=("missing track",), destination_paths=(Path("/nas/a.mp3"),))
    assert report.ok is False

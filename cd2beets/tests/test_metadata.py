from copy import deepcopy
from io import BytesIO
import json
from pathlib import Path
from types import SimpleNamespace

import pytest

from cd2beets import metadata
from cd2beets.metadata import DiscidReader, MetadataError, MusicBrainzClient
from cd2beets.models import DiscToc, ReleaseMetadata, TrackMetadata


FIXTURES = Path(__file__).parent / "fixtures"
TOC = DiscToc("disc-1", "/dev/disk7", 1, 2, 123456)


def fixture_payload() -> dict:
    return json.loads((FIXTURES / "musicbrainz_disc.json").read_text())


def test_configured_host_without_scheme_produces_http_base_url():
    client = MusicBrainzClient("10.6.9.160:5000")

    assert client.base_url == "http://10.6.9.160:5000"


def test_public_https_endpoint_uses_tls_port():
    client = MusicBrainzClient("https://musicbrainz.org")

    assert client.base_url == "https://musicbrainz.org"
    assert client._hostname == "musicbrainz.org"
    assert client._port == 443


def test_configured_host_wraps_url_parser_errors():
    with pytest.raises(MetadataError, match="Invalid MusicBrainz host") as raised:
        MusicBrainzClient("[::1")

    assert "Invalid IPv6 URL" in str(raised.value)
    assert isinstance(raised.value.__cause__, ValueError)


def test_configured_host_rejects_port_zero():
    with pytest.raises(MetadataError, match="port must be between 1 and 65535"):
        MusicBrainzClient("musicbrainz.local:0")


def test_parse_lookup_filters_matching_medium_and_normalizes_tracks():
    releases = MusicBrainzClient("10.6.9.160:5000").parse_lookup_response(
        TOC, fixture_payload()
    )

    assert releases == (
        ReleaseMetadata(
            release_id="release-1",
            albumartist="Primary Artist feat. Guest Artist",
            album="The Album",
            date="2001-02-03",
            country="US",
            disambiguation="second pressing",
            medium=2,
            tracks=(
                TrackMetadata(1, "recording-1", "First Track", 111000),
                TrackMetadata(2, "recording-2", "Second Track", 222000),
            ),
        ),
    )
    assert releases[0].recording_ids == ("recording-1", "recording-2")


def test_parse_lookup_sorts_releases_and_deduplicates_release_media_pairs():
    payload = fixture_payload()
    payload["releases"].append(deepcopy(payload["releases"][0]))
    earlier_release = deepcopy(payload["releases"][0])
    earlier_release.update(
        {
            "id": "release-0",
            "title": "Another Album",
            "date": "1999",
            "country": "GB",
            "disambiguation": "",
            "artist-credit": [{"name": "Aardvark Artist"}],
        }
    )
    payload["releases"].append(earlier_release)

    releases = MusicBrainzClient("musicbrainz.local").parse_lookup_response(
        TOC, payload
    )

    assert [(release.release_id, release.medium) for release in releases] == [
        ("release-0", 2),
        ("release-1", 2),
    ]


def test_parse_lookup_rejects_missing_recording_id():
    payload = fixture_payload()
    del payload["releases"][0]["media"][1]["tracks"][0]["recording"]["id"]

    with pytest.raises(MetadataError, match="recording ID"):
        MusicBrainzClient("musicbrainz.local").parse_lookup_response(TOC, payload)


@pytest.mark.parametrize(
    ("mutation", "message"),
    [
        (
            lambda release, medium, track: release.pop("id"),
            "release ID",
        ),
        (
            lambda release, medium, track: release.pop("title"),
            "release title",
        ),
        (
            lambda release, medium, track: release.update({"artist-credit": []}),
            "artist credit",
        ),
        (
            lambda release, medium, track: medium.pop("position"),
            "medium position",
        ),
        (
            lambda release, medium, track: track.pop("position"),
            "track position",
        ),
        (
            lambda release, medium, track: track["recording"].pop("title"),
            "track title",
        ),
        (
            lambda release, medium, track: medium.update({"tracks": []}),
            "empty matching medium",
        ),
    ],
    ids=[
        "release-id",
        "release-title",
        "artist-credit",
        "medium-position",
        "track-position",
        "track-title",
        "empty-medium",
    ],
)
def test_parse_lookup_rejects_incomplete_matching_identity(mutation, message):
    payload = fixture_payload()
    release = payload["releases"][0]
    medium = release["media"][1]
    mutation(release, medium, medium["tracks"][0])

    with pytest.raises(MetadataError, match=message):
        MusicBrainzClient("musicbrainz.local").parse_lookup_response(TOC, payload)


@pytest.mark.parametrize("value", [True, 1.5, "1"])
def test_parse_lookup_rejects_non_integer_positions(value):
    payload = fixture_payload()
    payload["releases"][0]["media"][1]["position"] = value

    with pytest.raises(MetadataError, match="invalid medium position"):
        MusicBrainzClient("musicbrainz.local").parse_lookup_response(TOC, payload)


def test_default_device_uses_libdiscid(monkeypatch):
    monkeypatch.setattr(metadata.discid, "get_default_device", lambda: "/dev/disk7")

    assert DiscidReader().default_device() == "/dev/disk7"


@pytest.mark.parametrize("device", [None, "", "   "])
def test_default_device_rejects_an_empty_device(monkeypatch, device):
    monkeypatch.setattr(metadata.discid, "get_default_device", lambda: device)

    with pytest.raises(MetadataError, match="default optical device"):
        DiscidReader().default_device()


def test_default_device_wraps_libdiscid_errors(monkeypatch):
    def fail():
        raise RuntimeError("libdiscid failed")

    monkeypatch.setattr(metadata.discid, "get_default_device", fail)

    with pytest.raises(MetadataError, match="libdiscid failed"):
        DiscidReader().default_device()


def test_read_converts_libdiscid_disc_and_preserves_selected_device(monkeypatch):
    disc = SimpleNamespace(
        id="disc-1",
        first_track_num=1,
        last_track_num=2,
        sectors=123456,
    )
    calls = []

    def read(device):
        calls.append(device)
        return disc

    monkeypatch.setattr(metadata.discid, "read", read)

    assert DiscidReader().read("/dev/disk7") == TOC
    assert calls == ["/dev/disk7"]


def test_read_wraps_libdiscid_errors(monkeypatch):
    def fail(device):
        raise RuntimeError(f"cannot read {device}")

    monkeypatch.setattr(metadata.discid, "read", fail)

    with pytest.raises(MetadataError, match="cannot read /dev/disk7"):
        DiscidReader().read("/dev/disk7")


def test_ping_opens_configured_host_and_port_for_two_seconds(monkeypatch):
    calls = []

    class Connection:
        closed = False

        def close(self):
            self.closed = True

    connection = Connection()

    def create_connection(address, timeout):
        calls.append((address, timeout))
        return connection

    monkeypatch.setattr(metadata.socket, "create_connection", create_connection)

    MusicBrainzClient("10.6.9.160:5000").ping()

    assert calls == [(('10.6.9.160', 5000), 2.0)]
    assert connection.closed


def test_ping_wraps_connection_errors(monkeypatch):
    def fail(address, timeout):
        raise OSError("connection refused")

    monkeypatch.setattr(metadata.socket, "create_connection", fail)

    with pytest.raises(MetadataError, match="connection refused"):
        MusicBrainzClient("10.6.9.160:5000").ping()


def test_lookup_uses_exact_read_only_request_and_normalizes_response(monkeypatch):
    requests = []

    def urlopen(request):
        requests.append(request)
        payload = fixture_payload()
        payload["releases"][0]["media"][1]["discs"][0]["id"] = "disc / one"
        return BytesIO(json.dumps(payload).encode())

    monkeypatch.setattr(metadata.request, "urlopen", urlopen)

    releases = MusicBrainzClient("10.6.9.160:5000").lookup(
        DiscToc("disc / one", "/dev/disk7", 1, 2, 123456)
    )

    assert requests[0].full_url == (
        "http://10.6.9.160:5000/ws/2/discid/disc%20%2F%20one"
        "?inc=artist-credits+recordings&fmt=json"
    )
    assert requests[0].get_method() == "GET"
    headers = {name.lower(): value for name, value in requests[0].header_items()}
    assert headers == {
        "user-agent": "cd2beets/0.1 (local batch ripper)",
        "accept": "application/json",
    }
    assert [(release.release_id, release.recording_ids) for release in releases] == [
        ("release-1", ("recording-1", "recording-2"))
    ]


def test_lookup_wraps_http_and_json_errors(monkeypatch):
    def urlopen(request):
        return BytesIO(b"not JSON")

    monkeypatch.setattr(metadata.request, "urlopen", urlopen)

    with pytest.raises(MetadataError, match="MusicBrainz lookup"):
        MusicBrainzClient("musicbrainz.local").lookup(TOC)

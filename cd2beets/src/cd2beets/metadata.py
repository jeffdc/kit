from __future__ import annotations

from collections.abc import Mapping
from contextlib import closing
import json
import socket
from typing import Any
from urllib import error, parse, request

import discid

from cd2beets.models import DiscToc, ReleaseMetadata, TrackMetadata


_USER_AGENT = "cd2beets/0.1 (local batch ripper)"


class MetadataError(RuntimeError):
    pass


class DiscidReader:
    def default_device(self) -> str:
        try:
            device = discid.get_default_device()
        except Exception as exc:
            raise MetadataError(f"Unable to discover default optical device: {exc}") from exc
        if not isinstance(device, str) or not device.strip():
            raise MetadataError("libdiscid did not return a default optical device")
        return device

    def read(self, device: str) -> DiscToc:
        try:
            disc = discid.read(device)
            disc_id = disc.id
            if not isinstance(disc_id, str) or not disc_id.strip():
                raise ValueError("missing disc ID")
            return DiscToc(
                disc_id=disc_id,
                device=device,
                first_track=int(disc.first_track_num),
                last_track=int(disc.last_track_num),
                sectors=int(disc.sectors),
            )
        except Exception as exc:
            raise MetadataError(f"Unable to read disc metadata from {device}: {exc}") from exc


class MusicBrainzClient:
    def __init__(self, configured_host: str):
        if not isinstance(configured_host, str) or not configured_host.strip():
            raise MetadataError("MusicBrainz host is empty")

        host = configured_host.strip()
        parsed = parse.urlsplit(host if "://" in host else f"http://{host}")
        try:
            port = parsed.port or 80
        except ValueError as exc:
            raise MetadataError(f"Invalid MusicBrainz host {configured_host!r}: {exc}") from exc

        if (
            parsed.scheme != "http"
            or not parsed.hostname
            or parsed.username is not None
            or parsed.password is not None
            or parsed.path not in ("", "/")
            or parsed.query
            or parsed.fragment
        ):
            raise MetadataError(f"Invalid MusicBrainz host: {configured_host!r}")

        self.base_url = f"http://{parsed.netloc}"
        self._hostname = parsed.hostname
        self._port = port

    def ping(self) -> None:
        try:
            with closing(
                socket.create_connection((self._hostname, self._port), timeout=2.0)
            ):
                pass
        except OSError as exc:
            raise MetadataError(
                f"Unable to connect to MusicBrainz at {self.base_url}: {exc}"
            ) from exc

    def lookup(self, toc: DiscToc) -> tuple[ReleaseMetadata, ...]:
        disc_id = parse.quote(toc.disc_id, safe="")
        url = (
            f"{self.base_url}/ws/2/discid/{disc_id}"
            "?inc=artist-credits+recordings&fmt=json"
        )
        http_request = request.Request(
            url,
            headers={
                "User-Agent": _USER_AGENT,
                "Accept": "application/json",
            },
            method="GET",
        )
        try:
            with request.urlopen(http_request) as response:
                payload = json.load(response)
        except (OSError, error.URLError, json.JSONDecodeError, UnicodeDecodeError) as exc:
            raise MetadataError(f"MusicBrainz lookup failed for {toc.disc_id}: {exc}") from exc

        return self.parse_lookup_response(toc, payload)

    def parse_lookup_response(
        self, toc: DiscToc, payload: Mapping[str, Any]
    ) -> tuple[ReleaseMetadata, ...]:
        if not isinstance(payload, Mapping):
            raise MetadataError("MusicBrainz lookup response is not an object")
        releases = payload.get("releases", [])
        if not isinstance(releases, list):
            raise MetadataError("MusicBrainz lookup response has invalid releases")

        candidates: list[ReleaseMetadata] = []
        for release in releases:
            if not isinstance(release, Mapping):
                raise MetadataError("MusicBrainz lookup response has an invalid release")
            media = release.get("media", [])
            if not isinstance(media, list):
                raise MetadataError("MusicBrainz release has invalid media")
            for medium in media:
                if not isinstance(medium, Mapping):
                    raise MetadataError("MusicBrainz release has an invalid medium")
                if self._medium_matches_disc(medium, toc.disc_id):
                    candidates.append(self._normalize_medium(release, medium))

        candidates.sort(
            key=lambda candidate: (
                candidate.albumartist,
                candidate.album,
                candidate.date,
                candidate.country,
                candidate.release_id,
                candidate.medium,
            )
        )
        unique_candidates: list[ReleaseMetadata] = []
        seen: set[tuple[str, int]] = set()
        for candidate in candidates:
            identity = (candidate.release_id, candidate.medium)
            if identity not in seen:
                seen.add(identity)
                unique_candidates.append(candidate)
        return tuple(unique_candidates)

    @staticmethod
    def _medium_matches_disc(medium: Mapping[str, Any], disc_id: str) -> bool:
        discs = medium.get("discs", [])
        if not isinstance(discs, list):
            return False
        return any(
            isinstance(disc, Mapping) and disc.get("id") == disc_id
            for disc in discs
        )

    def _normalize_medium(
        self, release: Mapping[str, Any], medium: Mapping[str, Any]
    ) -> ReleaseMetadata:
        release_id = self._required_text(release, "id", "release ID")
        album = self._required_text(release, "title", "release title")
        albumartist = self._artist_credit(release.get("artist-credit"))
        medium_position = self._required_int(
            medium, "position", "medium position"
        )

        raw_tracks = medium.get("tracks")
        if not isinstance(raw_tracks, list) or not raw_tracks:
            raise MetadataError(
                f"Release {release_id} has an empty matching medium {medium_position}"
            )
        tracks = tuple(
            sorted(
                (self._normalize_track(track, release_id) for track in raw_tracks),
                key=lambda track: track.position,
            )
        )

        return ReleaseMetadata(
            release_id=release_id,
            albumartist=albumartist,
            album=album,
            date=self._optional_text(release, "date"),
            country=self._optional_text(release, "country"),
            disambiguation=self._optional_text(release, "disambiguation"),
            medium=medium_position,
            tracks=tracks,
        )

    def _normalize_track(
        self, track: Any, release_id: str
    ) -> TrackMetadata:
        if not isinstance(track, Mapping):
            raise MetadataError(f"Release {release_id} has an invalid track")
        position = self._required_int(track, "position", "track position")
        recording = track.get("recording")
        if not isinstance(recording, Mapping):
            raise MetadataError(
                f"Release {release_id} track {position} has no recording ID"
            )
        recording_id = self._required_text(recording, "id", "recording ID")
        title = self._required_text(recording, "title", "track title")

        raw_length = track.get("length")
        if raw_length is None:
            length_ms = None
        else:
            if isinstance(raw_length, bool):
                raise MetadataError(
                    f"Release {release_id} track {position} has invalid length"
                )
            try:
                length_ms = int(raw_length)
            except (TypeError, ValueError) as exc:
                raise MetadataError(
                    f"Release {release_id} track {position} has invalid length"
                ) from exc

        return TrackMetadata(position, recording_id, title, length_ms)

    def _artist_credit(self, raw_credit: Any) -> str:
        if not isinstance(raw_credit, list) or not raw_credit:
            raise MetadataError("MusicBrainz release has no artist credit")
        parts: list[str] = []
        for entry in raw_credit:
            if not isinstance(entry, Mapping):
                raise MetadataError("MusicBrainz release has invalid artist credit")
            name = self._required_text(entry, "name", "artist credit name")
            joinphrase = entry.get("joinphrase", "")
            if not isinstance(joinphrase, str):
                raise MetadataError("MusicBrainz release has invalid artist credit joinphrase")
            parts.append(f"{name}{joinphrase}")
        return "".join(parts)

    @staticmethod
    def _required_text(
        source: Mapping[str, Any], key: str, description: str
    ) -> str:
        value = source.get(key)
        if not isinstance(value, str) or not value.strip():
            raise MetadataError(f"MusicBrainz response is missing {description}")
        return value

    @staticmethod
    def _required_int(
        source: Mapping[str, Any], key: str, description: str
    ) -> int:
        value = source.get(key)
        if value is None or isinstance(value, bool):
            raise MetadataError(f"MusicBrainz response is missing {description}")
        try:
            return int(value)
        except (TypeError, ValueError) as exc:
            raise MetadataError(
                f"MusicBrainz response has invalid {description}"
            ) from exc

    @staticmethod
    def _optional_text(source: Mapping[str, Any], key: str) -> str:
        value = source.get(key)
        if value is None:
            return ""
        if not isinstance(value, str):
            raise MetadataError(f"MusicBrainz response has invalid {key}")
        return value

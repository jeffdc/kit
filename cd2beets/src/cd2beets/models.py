from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
import re
import unicodedata


class MediaState(StrEnum):
    NO_MEDIA = "no_media"
    AUDIO = "audio"
    UNUSABLE = "unusable"


class Presence(StrEnum):
    PRESENT = "present"
    ABSENT = "absent"
    UNCERTAIN = "uncertain"


class OperatorDecision(StrEnum):
    SKIP_EJECT = "skip_eject"
    PROCEED = "proceed"
    ABORT_KEEP_LOADED = "abort_keep_loaded"


class FailureDecision(StrEnum):
    RETRY_IMPORT = "retry_import"
    RETRY_VERIFY = "retry_verify"
    EJECT = "eject"
    QUIT = "quit"


def normalize_text(value: str) -> str:
    decomposed = unicodedata.normalize("NFKD", value.casefold())
    ascii_text = "".join(char for char in decomposed if not unicodedata.combining(char))
    return " ".join(re.findall(r"[a-z0-9]+", ascii_text))


@dataclass(frozen=True)
class DiscToc:
    disc_id: str
    device: str
    first_track: int
    last_track: int
    sectors: int

    @property
    def track_count(self) -> int:
        return self.last_track - self.first_track + 1


@dataclass(frozen=True)
class TrackMetadata:
    position: int
    recording_id: str
    title: str
    length_ms: int | None


@dataclass(frozen=True)
class ReleaseMetadata:
    release_id: str
    albumartist: str
    album: str
    date: str
    country: str
    disambiguation: str
    medium: int
    tracks: tuple[TrackMetadata, ...]

    @property
    def recording_ids(self) -> tuple[str, ...]:
        return tuple(track.recording_id for track in self.tracks)


@dataclass(frozen=True)
class LibraryItem:
    item_id: int
    release_id: str
    recording_id: str
    albumartist: str
    album: str
    medium: int
    track: int
    tracktotal: int
    path: Path
    format: str
    bitrate: int


@dataclass(frozen=True)
class LibrarySnapshot:
    exact_items: tuple[LibraryItem, ...]
    plausible_items: tuple[LibraryItem, ...]


@dataclass(frozen=True)
class Classification:
    presence: Presence
    reasons: tuple[str, ...]
    snapshot: LibrarySnapshot


@dataclass(frozen=True)
class OwnedMp3:
    path: Path
    device: int
    inode: int
    track_position: int


@dataclass(frozen=True)
class VerificationReport:
    errors: tuple[str, ...]
    destination_paths: tuple[Path, ...]

    @property
    def ok(self) -> bool:
        return not self.errors

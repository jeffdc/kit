from __future__ import annotations

from dataclasses import dataclass
import re

from cd2beets.models import MediaState
from cd2beets.process import CommandResult, ProcessRunner


@dataclass(frozen=True)
class DriveInfo:
    index: int
    vendor: str
    product: str
    revision: str
    bus: str
    support_level: str


class DriveError(RuntimeError):
    pass


_DRIVE_ROW = re.compile(r"^\s*(\d+)\s")
_VENDOR_START = 3
_PRODUCT_START = 12
_REVISION_START = 30
_BUS_START = 36
_SUPPORT_LEVEL_START = 46


def parse_drive_list(text: str) -> tuple[DriveInfo, ...]:
    drives = []
    for line in text.splitlines():
        match = _DRIVE_ROW.match(line)
        if match is None:
            continue
        drives.append(
            DriveInfo(
                index=int(match.group(1)),
                vendor=line[_VENDOR_START:_PRODUCT_START].strip(),
                product=line[_PRODUCT_START:_REVISION_START].strip(),
                revision=line[_REVISION_START:_BUS_START].strip(),
                bus=line[_BUS_START:_SUPPORT_LEVEL_START].strip(),
                support_level=line[_SUPPORT_LEVEL_START:].strip(),
            )
        )
    return tuple(drives)


def parse_media_state(text: str) -> MediaState:
    types = {line.strip() for line in text.splitlines()}
    if "Type: No Media Inserted" in types:
        return MediaState.NO_MEDIA
    if "Type: Audio CD" in types:
        return MediaState.AUDIO
    return MediaState.UNUSABLE


class DrutilDrive:
    def __init__(self, runner: ProcessRunner):
        self._runner = runner

    def list_drives(self) -> tuple[DriveInfo, ...]:
        result = self._runner.run(["drutil", "list"])
        self._raise_on_failure("list", result)
        drives = parse_drive_list(result.stdout)
        if not drives:
            raise DriveError("expected one selected optical drive, found zero")
        if len(drives) > 1:
            raise DriveError(
                f"expected one selected optical drive, found multiple ({len(drives)})"
            )
        return drives

    def status(self) -> MediaState:
        result = self._runner.run(["drutil", "status"])
        self._raise_on_failure("status", result)
        return parse_media_state(result.stdout)

    def eject(self) -> None:
        result = self._runner.run(["drutil", "eject"])
        self._raise_on_failure("eject", result)

    @staticmethod
    def _raise_on_failure(action: str, result: CommandResult) -> None:
        if result.returncode == 0:
            return
        details = [f"drutil {action} failed with exit status {result.returncode}"]
        if result.stdout:
            details.append(f"stdout: {result.stdout}")
        if result.stderr:
            details.append(f"stderr: {result.stderr}")
        raise DriveError("\n".join(details))

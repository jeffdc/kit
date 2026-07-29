from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
import os
from pathlib import Path
import re
import shutil
from typing import Any

from cd2beets.drive import DriveInfo
from cd2beets.library import BeetsSettings
from cd2beets.process import ProcessRunner


_REQUIRED_BINARIES = ("drutil", "cdda2wav", "lame", "beet", "pgrep")
_ALLOWED_PLUGINS = frozenset({"musicbrainz", "web", "duplicates", "missing"})
_REQUIRED_BEETS_VERSION = "2.8.0"
_BEETS_VERSION_LINE = re.compile(r"^beets version (\S+)\s*$", re.MULTILINE)


@dataclass(frozen=True)
class PreflightReport:
    errors: tuple[str, ...]
    library_path: Path
    library_root: Path
    staging_dir: Path
    drive: DriveInfo | None

    @property
    def ok(self) -> bool:
        return not self.errors


def _stat_path(path: Path) -> os.stat_result:
    return path.stat()


class Preflight:
    """Read-only startup checks for every prerequisite to disc processing."""

    def __init__(
        self,
        settings: BeetsSettings,
        staging_dir: Path,
        runner: ProcessRunner,
        drive: Any,
        metadata: Any,
        *,
        which: Callable[[str], str | None] = shutil.which,
        stat_path: Callable[[Path], Any] = _stat_path,
        access: Callable[[Path, int], bool] = os.access,
    ) -> None:
        self.settings = settings
        self.staging_dir = Path(staging_dir)
        self._runner = runner
        self._drive = drive
        self._metadata = metadata
        self._which = which
        self._stat_path = stat_path
        self._access = access

    def run(self) -> PreflightReport:
        errors: list[str] = []
        library_path = self.settings.library_path.expanduser().resolve()
        library_root = self.settings.library_root.expanduser().resolve()
        staging_dir = self.staging_dir.expanduser().resolve()

        available = self._check_binaries(errors)
        if "beet" in available:
            self._check_beets_version(errors)

        self._check_library_database(library_path, errors)
        self._check_nas_root(library_root, errors)
        self._check_staging(staging_dir, library_root, errors)
        self._check_configuration(errors)
        selected_drive = self._check_drive(errors)
        self._check_musicbrainz(errors)

        return PreflightReport(
            tuple(errors),
            library_path,
            library_root,
            staging_dir,
            selected_drive,
        )

    def _check_binaries(self, errors: list[str]) -> set[str]:
        available: set[str] = set()
        for binary in _REQUIRED_BINARIES:
            try:
                location = self._which(binary)
            except Exception as error:
                errors.append(f"prerequisite check failed for {binary}: {error}")
                continue
            if location is None:
                errors.append(f"missing prerequisite: {binary}")
            else:
                available.add(binary)
        return available

    def _check_beets_version(self, errors: list[str]) -> None:
        try:
            result = self._runner.run(["beet", "version"])
        except Exception as error:
            errors.append(f"beets version check failed: {error}")
            return
        if result.returncode != 0:
            errors.append(
                f"beets version check failed with exit status {result.returncode}"
            )
            return
        match = _BEETS_VERSION_LINE.search(result.stdout)
        actual = match.group(1) if match is not None else "unrecognized output"
        if actual != _REQUIRED_BEETS_VERSION:
            errors.append(
                "beets version must be exactly "
                f"{_REQUIRED_BEETS_VERSION}; found {actual}"
            )

    def _check_library_database(
        self, library_path: Path, errors: list[str]
    ) -> None:
        try:
            self._stat_path(library_path)
        except OSError as error:
            errors.append(
                f"library database is unreadable: {library_path}: {error}"
            )
            return
        try:
            readable = self._access(library_path, os.R_OK)
        except Exception as error:
            errors.append(
                f"library database readability check failed: {library_path}: {error}"
            )
            return
        if not readable:
            errors.append(f"library database is unreadable: {library_path}")

    def _check_nas_root(
        self, library_root: Path, errors: list[str]
    ) -> None:
        try:
            library_details = self._stat_path(library_root)
        except OSError as error:
            errors.append(f"NAS library root is absent: {library_root}: {error}")
            return

        try:
            writable = self._access(library_root, os.W_OK)
        except Exception as error:
            errors.append(
                f"NAS library root writability check failed: {library_root}: {error}"
            )
        else:
            if not writable:
                errors.append(f"NAS library root is unwritable: {library_root}")

        root = Path("/").resolve()
        try:
            root_details = self._stat_path(root)
        except OSError as error:
            errors.append(f"root filesystem device check failed: {error}")
            return
        if library_details.st_dev == root_details.st_dev:
            errors.append(
                "NAS library root is not a separate mounted filesystem: "
                f"{library_root}"
            )

    @staticmethod
    def _check_staging(
        staging_dir: Path, library_root: Path, errors: list[str]
    ) -> None:
        if staging_dir == library_root or staging_dir.is_relative_to(library_root):
            errors.append(
                "staging directory is inside the library root: "
                f"{staging_dir}"
            )

    def _check_configuration(self, errors: list[str]) -> None:
        for plugin in sorted(set(self.settings.plugins) - _ALLOWED_PLUGINS):
            errors.append(f"unknown active beets plugin: {plugin}")

        if self.settings.duplicate_action != "skip":
            errors.append(
                "beets import duplicate_action must be 'skip'; found "
                f"{self.settings.duplicate_action!r}"
            )
        if self.settings.duplicates_delete:
            errors.append("duplicates delete must be disabled")
        if self.settings.duplicates_remove:
            errors.append("duplicates remove must be disabled")
        if self.settings.duplicates_move:
            errors.append("duplicates move must be disabled")

    def _check_drive(self, errors: list[str]) -> DriveInfo | None:
        try:
            drives = tuple(self._drive.list_drives())
        except Exception as error:
            errors.append(f"optical drive check failed: {error}")
            return None
        if len(drives) != 1:
            errors.append(
                f"expected exactly one optical drive; found {len(drives)}"
            )
            return None
        return drives[0]

    def _check_musicbrainz(self, errors: list[str]) -> None:
        try:
            self._metadata.ping()
        except Exception as error:
            errors.append(f"MusicBrainz ping failed: {error}")

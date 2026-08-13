from __future__ import annotations

import os
import stat
import uuid
from pathlib import Path
from typing import BinaryIO, Iterable

from cd2beets.journal import Journal
from cd2beets.models import OwnedMp3, VerificationReport


class CleanupRefused(RuntimeError):
    """Raised before cleanup when any deletion guard is not satisfied."""


class StagingRootError(RuntimeError):
    """Raised when the staging root is not the private directory recorded."""


class PartialCleanupError(RuntimeError):
    """Raised when cleanup stops after one or more filesystem side effects."""

    def __init__(
        self,
        removed: tuple[OwnedMp3, ...],
        remaining: tuple[OwnedMp3, ...],
        cause: OSError,
    ) -> None:
        super().__init__(f"cleanup stopped after {len(removed)} removal(s): {cause}")
        self.removed = removed
        self.remaining = remaining
        self.cause = cause


class StagingRun:
    def __init__(
        self,
        staging_root: Path,
        journal: Journal,
        *,
        run_id: str | None = None,
    ) -> None:
        self.staging_root = Path(staging_root).expanduser().absolute()
        self.journal = journal
        self.run_id = run_id or uuid.uuid4().hex
        if Path(self.run_id).name != self.run_id or self.run_id in {".", ".."}:
            raise ValueError("run ID must be a single path component")

        self._staging_identity = self._initialize_staging_root(
            self.staging_root
        )
        self.run_dir = self.staging_root / self.run_id
        self.run_dir.mkdir(mode=0o700)
        self.run_dir.chmod(0o700)
        self._revalidate_staging_root()
        self._owned: dict[Path, OwnedMp3] = {}

    @classmethod
    def _initialize_staging_root(cls, path: Path) -> tuple[int, int]:
        try:
            path.mkdir(mode=0o700)
        except FileExistsError:
            pass
        else:
            path.chmod(0o700)
        details = cls._validate_staging_root(path)
        return details.st_dev, details.st_ino

    @staticmethod
    def _validate_staging_root(path: Path):
        try:
            details = path.lstat()
        except OSError as error:
            raise StagingRootError(
                f"cannot lstat staging root: {path}: {error}"
            ) from error
        if stat.S_ISLNK(details.st_mode):
            raise StagingRootError(f"staging root is a symlink: {path}")
        if not stat.S_ISDIR(details.st_mode):
            raise StagingRootError(f"staging root is not a directory: {path}")
        if details.st_uid != os.geteuid():
            raise StagingRootError(
                f"staging root has the wrong owner: {path}"
            )
        if stat.S_IMODE(details.st_mode) != 0o700:
            raise StagingRootError(f"staging root mode is not 0700: {path}")
        if not os.access(path, os.W_OK):
            raise StagingRootError(f"staging root is not writable: {path}")
        return details

    def _revalidate_staging_root(self) -> None:
        details = self._validate_staging_root(self.staging_root)
        if (details.st_dev, details.st_ino) != self._staging_identity:
            raise StagingRootError(
                f"staging root identity changed: {self.staging_root}"
            )

    def reserve(self, track_position: int) -> tuple[OwnedMp3, BinaryIO]:
        self._revalidate_staging_root()
        path = self.run_dir / f"{self.run_id}-track{track_position:02d}.mp3"
        self.journal.append(
            "track_reserved",
            run_id=self.run_id,
            track_position=track_position,
            path=str(path),
        )
        flags = os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW
        descriptor = os.open(path, flags, 0o600)
        try:
            os.fchmod(descriptor, 0o600)
            details = os.fstat(descriptor)
            owned = OwnedMp3(
                path=path,
                device=details.st_dev,
                inode=details.st_ino,
                track_position=track_position,
            )
            self._owned[path] = owned
            self.journal.append(
                "track_created",
                run_id=self.run_id,
                track_position=track_position,
                path=str(path),
                device=owned.device,
                inode=owned.inode,
            )
            output = os.fdopen(descriptor, "wb", closefd=True)
        except BaseException:
            os.close(descriptor)
            raise
        return owned, output

    def validate_cleanup(
        self,
        tracks: Iterable[OwnedMp3],
        report: VerificationReport,
    ) -> tuple[OwnedMp3, ...]:
        if not report.ok:
            raise CleanupRefused("cleanup requires a successful verification report")
        self._revalidate_staging_root()

        candidates = tuple(tracks)
        normalized_root = self.staging_root.resolve(strict=True)
        normalized_run_dir = self.run_dir.resolve(strict=True)

        for candidate in candidates:
            path = candidate.path
            normalized_path = Path(os.path.abspath(os.path.normpath(path)))
            try:
                normalized_path.relative_to(normalized_root)
            except ValueError as error:
                raise CleanupRefused(f"path is outside staging root: {path}") from error
            if normalized_path.parent != normalized_run_dir:
                raise CleanupRefused(f"path is outside this staging run: {path}")
            if path.suffix != ".mp3":
                raise CleanupRefused(f"cleanup path must have .mp3 suffix: {path}")

            try:
                details = path.lstat()
            except OSError as error:
                raise CleanupRefused(f"cannot lstat cleanup path: {path}: {error}") from error
            if stat.S_ISLNK(details.st_mode):
                raise CleanupRefused(f"cleanup path is a symlink: {path}")
            if not stat.S_ISREG(details.st_mode):
                raise CleanupRefused(f"cleanup path is not a regular file: {path}")

            recorded = self._owned.get(normalized_path)
            if recorded is None:
                raise CleanupRefused(f"cleanup path is not owned by this run: {path}")
            if candidate != recorded:
                raise CleanupRefused(f"cleanup ownership record does not match: {path}")
            if details.st_dev != recorded.device:
                raise CleanupRefused(f"cleanup device does not match: {path}")
            if details.st_ino != recorded.inode:
                raise CleanupRefused(f"cleanup inode does not match: {path}")

        return candidates

    def cleanup(
        self,
        tracks: Iterable[OwnedMp3],
        report: VerificationReport,
    ) -> tuple[OwnedMp3, ...]:
        candidates = self.validate_cleanup(tracks, report)
        removed: list[OwnedMp3] = []

        for index, candidate in enumerate(candidates):
            path = candidate.path
            try:
                self.journal.append(
                    "track_cleanup_started",
                    run_id=self.run_id,
                    track_position=candidate.track_position,
                    path=str(path),
                )
            except OSError as cause:
                removed_tracks = tuple(removed)
                remaining = candidates[index:]
                self._journal_cleanup_failed(
                    candidate, removed_tracks, remaining, cause
                )
                raise PartialCleanupError(
                    removed_tracks, remaining, cause
                ) from cause
            try:
                path.unlink()
            except OSError as cause:
                removed_tracks = tuple(removed)
                remaining = candidates[index:]
                self._journal_cleanup_failed(
                    candidate, removed_tracks, remaining, cause
                )
                raise PartialCleanupError(
                    removed_tracks, remaining, cause
                ) from cause
            removed.append(candidate)
            try:
                self.journal.append(
                    "track_cleanup_succeeded",
                    run_id=self.run_id,
                    track_position=candidate.track_position,
                    path=str(path),
                )
            except OSError as cause:
                removed_tracks = tuple(removed)
                remaining = candidates[index + 1 :]
                self._journal_cleanup_failed(
                    candidate, removed_tracks, remaining, cause
                )
                raise PartialCleanupError(
                    removed_tracks, remaining, cause
                ) from cause

        return tuple(removed)


    @staticmethod
    def _track_identity(track: OwnedMp3) -> dict[str, str | int]:
        return {
            "path": str(track.path),
            "device": track.device,
            "inode": track.inode,
            "track_position": track.track_position,
        }

    def _journal_cleanup_failed(
        self,
        candidate: OwnedMp3,
        removed: tuple[OwnedMp3, ...],
        remaining: tuple[OwnedMp3, ...],
        cause: OSError,
    ) -> None:
        try:
            self.journal.append(
                "track_cleanup_failed",
                run_id=self.run_id,
                track_position=candidate.track_position,
                path=str(candidate.path),
                error=str(cause),
                removed=[self._track_identity(track) for track in removed],
                remaining=[
                    self._track_identity(track) for track in remaining
                ],
            )
        except OSError:
            pass

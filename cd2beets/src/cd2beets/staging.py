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


class PartialCleanupError(RuntimeError):
    """Raised after an unlink failure leaves part of a validated set present."""

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

        self._ensure_private_directory(self.staging_root)
        self.run_dir = self.staging_root / self.run_id
        self._ensure_private_directory(self.run_dir)
        self._owned: dict[Path, OwnedMp3] = {}

    @staticmethod
    def _ensure_private_directory(path: Path) -> None:
        try:
            path.mkdir(mode=0o700)
        except FileExistsError:
            if not path.is_dir():
                raise
        else:
            path.chmod(0o700)

    def reserve(self, track_position: int) -> tuple[OwnedMp3, BinaryIO]:
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
            self.journal.append(
                "track_cleanup_started",
                run_id=self.run_id,
                track_position=candidate.track_position,
                path=str(path),
            )
            try:
                path.unlink()
            except OSError as cause:
                remaining = candidates[index:]
                self.journal.append(
                    "track_cleanup_failed",
                    run_id=self.run_id,
                    track_position=candidate.track_position,
                    path=str(path),
                    error=str(cause),
                )
                raise PartialCleanupError(tuple(removed), remaining, cause) from cause
            removed.append(candidate)
            self.journal.append(
                "track_cleanup_succeeded",
                run_id=self.run_id,
                track_position=candidate.track_position,
                path=str(path),
            )

        return tuple(removed)

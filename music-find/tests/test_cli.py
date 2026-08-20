from __future__ import annotations

import contextlib
import io
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from music_find.cli import (
    FIELD_SEPARATOR,
    RECORD_SEPARATOR,
    Album,
    Config,
    MusicFindError,
    build_directory_index,
    load_config,
    load_directory_index,
    main,
    perform_search,
    render_report,
    report_exit_code,
    search_beets,
    search_directory_index,
    validate_album_paths,
    validate_nas,
)


class MusicFindTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary_directory.name)

    def tearDown(self) -> None:
        self.temporary_directory.cleanup()

    def write_fake_beet(
        self, records: list[tuple[str, str, Path]], *, exit_code: int = 0
    ) -> Path:
        executable = self.root / "beet"
        serialized = "".join(
            f"{RECORD_SEPARATOR}{artist}{FIELD_SEPARATOR}{title}"
            f"{FIELD_SEPARATOR}{path}\n"
            for artist, title, path in records
        )
        executable.write_text(
            "#!/usr/bin/env python3\n"
            "import sys\n"
            f"sys.stdout.write({serialized!r})\n"
            f"raise SystemExit({exit_code})\n"
        )
        executable.chmod(executable.stat().st_mode | stat.S_IXUSR)
        return executable

    def make_config(self) -> Config:
        archive = self.root / "music"
        library = archive / "library"
        library.mkdir(parents=True)
        return Config(library_root=library, archive_root=archive)

    def build_test_index(self, config: Config) -> Path:
        index_path = self.root / "directories.json"
        build_directory_index(config, index_path)
        return index_path

    def test_load_config_reads_absolute_library_and_archive_roots(self) -> None:
        config = self.make_config()
        config_path = self.root / "config.toml"
        config_path.write_text(
            f'library_root = "{config.library_root}"\n'
            f'archive_root = "{config.archive_root}"\n'
        )

        loaded = load_config(config_path)

        self.assertEqual(config.library_root.resolve(), loaded.library_root)
        self.assertEqual(config.archive_root.resolve(), loaded.archive_root)

    def test_reindex_classifies_library_and_archive_directories(self) -> None:
        config = self.make_config()
        library_match = config.library_root / "Beyoncé [LIVE]*"
        archive_match = config.archive_root / "unsorted" / "Beyoncé [LIVE]*"
        library_match.mkdir(parents=True)
        archive_match.mkdir(parents=True)
        index_path = self.build_test_index(config)

        index = load_directory_index(config, index_path)
        library_result = search_directory_index(index, "BEYONCÉ [live]*", "library")
        archive_result = search_directory_index(index, "BEYONCÉ [live]*", "archive")

        self.assertEqual((library_match,), library_result.matches)
        self.assertEqual((archive_match,), archive_result.matches)

    def test_reindex_writes_a_versioned_json_index_atomically(self) -> None:
        config = self.make_config()
        (config.library_root / "Artist" / "Album").mkdir(parents=True)
        index_path = self.build_test_index(config)

        document = json.loads(index_path.read_text())

        self.assertEqual(1, document["version"])
        self.assertIn("generated_at", document)
        self.assertEqual([], list(index_path.parent.glob(f".{index_path.name}.*")))

    def test_index_rejects_different_configured_roots(self) -> None:
        config = self.make_config()
        index_path = self.build_test_index(config)
        other_archive = self.root / "other"
        other_library = other_archive / "library"
        other_library.mkdir(parents=True)
        other_config = Config(other_library, other_archive)

        with self.assertRaisesRegex(MusicFindError, "different configured paths"):
            load_directory_index(other_config, index_path)

    def test_missing_index_requests_reindex(self) -> None:
        config = self.make_config()

        with self.assertRaisesRegex(MusicFindError, "music-find --reindex"):
            load_directory_index(config, self.root / "missing.json")

    def test_missing_indexed_directory_is_an_integrity_error(self) -> None:
        config = self.make_config()
        indexed_path = config.library_root / "Vanishing Album"
        indexed_path.mkdir()
        index_path = self.build_test_index(config)
        index = load_directory_index(config, index_path)
        indexed_path.rmdir()

        result = search_directory_index(index, "vanishing", "library")

        self.assertEqual((), result.matches)
        self.assertIn("run music-find --reindex", result.errors[0])

    def test_beets_search_treats_query_metacharacters_as_literal_text(self) -> None:
        matching_path = self.root / "library" / ".gif from god" / "Album [1]*"
        other_path = self.root / "library" / "Other" / "Plain"
        executable = self.write_fake_beet(
            [
                (".gif from god", "Album [1]*", matching_path),
                ("Other", "Plain", other_path),
            ]
        )

        matches = search_beets("[1]*", str(executable))

        self.assertEqual((Album(".gif from god", "Album [1]*", matching_path),), matches)

    def test_beets_paths_must_exist_beneath_the_library(self) -> None:
        config = self.make_config()
        valid_path = config.library_root / "Artist" / "Album"
        valid_path.mkdir(parents=True)
        missing_path = config.library_root / "Missing"
        outside_path = self.root / "outside"
        outside_path.mkdir()
        albums = (
            Album("Artist", "Album", valid_path),
            Album("Artist", "Missing", missing_path),
            Album("Artist", "Outside", outside_path),
        )

        valid, invalid = validate_album_paths(albums, config.library_root)

        self.assertEqual(
            (Album("Artist", "Album", valid_path.resolve()),),
            valid,
        )
        self.assertEqual(2, len(invalid))
        self.assertIn("missing", invalid[0].reason)
        self.assertIn("outside", invalid[1].reason)

    def test_nas_preflight_requests_retry_when_archive_is_not_mounted(self) -> None:
        config = self.make_config()

        with self.assertRaisesRegex(MusicFindError, "Mount the volume and retry"):
            validate_nas(config, lambda _: False)

    def test_archive_index_runs_automatically_after_organized_miss(self) -> None:
        config = self.make_config()
        archive_match = config.archive_root / "unfiled" / "Lost Album"
        archive_match.mkdir(parents=True)
        index_path = self.build_test_index(config)
        executable = self.write_fake_beet([])

        report = perform_search(
            "lost album",
            config,
            beet_executable=str(executable),
            index_path=index_path,
            mount_checker=lambda _: True,
        )

        self.assertEqual((), report.library_result.matches)
        self.assertIsNotNone(report.archive_result)
        self.assertEqual((archive_match,), report.archive_result.matches)
        self.assertEqual(0, report_exit_code(report))

    def test_archive_index_is_skipped_after_organized_match(self) -> None:
        config = self.make_config()
        (config.library_root / "Found Artist").mkdir()
        index_path = self.build_test_index(config)
        executable = self.write_fake_beet([])

        report = perform_search(
            "found",
            config,
            beet_executable=str(executable),
            index_path=index_path,
            mount_checker=lambda _: True,
        )

        self.assertIsNone(report.archive_result)
        self.assertEqual(0, report_exit_code(report))

    def test_main_fails_before_invoking_beet_when_nas_is_unmounted(self) -> None:
        config = self.make_config()
        config_home = self.root / "config-home"
        config_dir = config_home / "music-find"
        config_dir.mkdir(parents=True)
        (config_dir / "config.toml").write_text(
            f'library_root = "{config.library_root}"\n'
            f'archive_root = "{config.archive_root}"\n'
        )
        marker = self.root / "beet-was-called"
        executable = self.root / "beet"
        executable.write_text(
            "#!/usr/bin/env python3\n"
            "from pathlib import Path\n"
            f"Path({str(marker)!r}).touch()\n"
        )
        executable.chmod(executable.stat().st_mode | stat.S_IXUSR)
        stderr = io.StringIO()

        with (
            patch.dict(
                os.environ,
                {
                    "XDG_CONFIG_HOME": str(config_home),
                    "PATH": f"{self.root}:{os.environ['PATH']}",
                },
            ),
            patch("music_find.cli.os.path.ismount", return_value=False),
            contextlib.redirect_stderr(stderr),
        ):
            exit_code = main(["Kind of Blue"])

        self.assertEqual(2, exit_code)
        self.assertIn("Mount the volume and retry", stderr.getvalue())
        self.assertFalse(marker.exists())

    def test_reindex_command_builds_configured_cache_index(self) -> None:
        config = self.make_config()
        (config.library_root / "Artist" / "Album").mkdir(parents=True)
        config_home = self.root / "config-home"
        cache_home = self.root / "cache-home"
        config_dir = config_home / "music-find"
        config_dir.mkdir(parents=True)
        (config_dir / "config.toml").write_text(
            f'library_root = "{config.library_root}"\n'
            f'archive_root = "{config.archive_root}"\n'
        )
        stdout = io.StringIO()

        with (
            patch.dict(
                os.environ,
                {
                    "XDG_CONFIG_HOME": str(config_home),
                    "XDG_CACHE_HOME": str(cache_home),
                },
            ),
            patch("music_find.cli.os.path.ismount", return_value=True),
            contextlib.redirect_stdout(stdout),
        ):
            exit_code = main(["--reindex"])

        self.assertEqual(0, exit_code)
        self.assertTrue((cache_home / "music-find" / "directories.json").is_file())
        self.assertIn("Indexed 2 directories", stdout.getvalue())

    def test_complete_miss_returns_one_and_separates_source_sections(self) -> None:
        config = self.make_config()
        index_path = self.build_test_index(config)
        executable = self.write_fake_beet([])

        report = perform_search(
            "not present",
            config,
            beet_executable=str(executable),
            index_path=index_path,
            mount_checker=lambda _: True,
        )

        output = render_report(report)
        self.assertEqual(1, report_exit_code(report))
        self.assertIn("Index:", output)
        self.assertIn("BEETS — NOT FOUND", output)
        self.assertIn("NAS LIBRARY — NOT FOUND", output)
        self.assertIn("NAS ARCHIVE — NOT FOUND", output)
        self.assertIn("No matching album or artist directory found", output)


if __name__ == "__main__":
    unittest.main()

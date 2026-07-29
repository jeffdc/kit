from __future__ import annotations

import json
import os
from datetime import datetime
from pathlib import Path

import pytest

from cd2beets.journal import Journal


def test_append_adds_compact_fsynced_json_lines_without_rewriting(
    tmp_path, monkeypatch
):
    path = tmp_path / "lifecycle.jsonl"
    journal = Journal(path)
    open_modes: list[str] = []
    fsynced: list[int] = []
    original_open = Path.open
    original_fsync = os.fsync

    def open_spy(self, mode="r", *args, **kwargs):
        if self == path:
            open_modes.append(mode)
        return original_open(self, mode, *args, **kwargs)

    def fsync_spy(fd):
        fsynced.append(fd)
        original_fsync(fd)

    monkeypatch.setattr(Path, "open", open_spy)
    monkeypatch.setattr("cd2beets.journal.os.fsync", fsync_spy)

    journal.append("run_started", run_id="run-1")
    assert open_modes == ["a"]
    first_append = path.read_bytes()
    journal.append("track_reserved", position=1)
    assert open_modes[-1] == "a"

    assert path.read_bytes().startswith(first_append)
    lines = path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 2
    assert open_modes.count("a") == 2
    assert "w" not in open_modes
    assert len(fsynced) == 2
    assert all(": " not in line and ", " not in line for line in lines)

    records = [json.loads(line) for line in lines]
    assert records[0]["event"] == "run_started"
    assert records[0]["run_id"] == "run-1"
    assert records[1]["event"] == "track_reserved"
    assert records[1]["position"] == 1
    for record in records:
        timestamp = datetime.fromisoformat(record["timestamp"])
        assert timestamp.utcoffset() is not None
        assert timestamp.utcoffset().total_seconds() == 0


def test_append_rejects_unserializable_fields_before_opening(tmp_path):
    path = tmp_path / "lifecycle.jsonl"

    with pytest.raises(TypeError):
        Journal(path).append("bad_event", value=object())

    assert not path.exists()


def test_append_validates_supplied_fields_even_when_name_is_reserved(tmp_path):
    path = tmp_path / "lifecycle.jsonl"

    with pytest.raises(TypeError):
        Journal(path).append("bad_event", timestamp=object())

    assert not path.exists()

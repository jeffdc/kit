from pathlib import Path

import pytest

import cd2beets.process as process_module
from cd2beets.process import ProcessRunner


def test_run_captures_output_without_a_shell():
    result = ProcessRunner().run(["/usr/bin/printf", "%s", "hello"])
    assert result.returncode == 0
    assert result.stdout == "hello"
    assert result.stderr == ""


def test_pipeline_streams_source_through_sink(tmp_path: Path):
    destination = tmp_path / "pipeline.mp3"
    with destination.open("xb") as output:
        result = ProcessRunner().pipeline(
            ["/usr/bin/printf", "abc"],
            ["/usr/bin/tr", "a-z", "A-Z"],
            output,
        )
    assert result.source_returncode == 0
    assert result.sink_returncode == 0
    assert destination.read_bytes() == b"ABC"


def test_pipeline_reaps_source_when_sink_fails_to_start(monkeypatch, tmp_path: Path):
    started = []
    real_popen = process_module.Popen

    def recording_popen(args, **kwargs):
        process = real_popen(args, **kwargs)
        started.append(process)
        return process

    monkeypatch.setattr(process_module, "Popen", recording_popen)

    destination = tmp_path / "failed-pipeline.mp3"
    with destination.open("xb") as output:
        with pytest.raises(FileNotFoundError):
            ProcessRunner().pipeline(
                ["/usr/bin/yes"],
                ["/definitely/not/a/command"],
                output,
            )
    source = started[0]
    try:
        assert source.poll() is not None
        assert source.stdout is not None and source.stdout.closed
        assert source.stderr is not None and source.stderr.closed
    finally:
        if source.poll() is None:
            source.terminate()
            source.wait()
        if source.stdout is not None:
            source.stdout.close()
        if source.stderr is not None:
            source.stderr.close()

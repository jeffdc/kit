from io import BytesIO
from pathlib import Path
from subprocess import TimeoutExpired

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
                ["/bin/sleep", "30"],
                ["/definitely/not/a/command"],
                output,
            )
    source = started[0]
    try:
        assert source.returncode is not None
        assert source.stdout is not None and source.stdout.closed
        assert source.stderr is not None and source.stderr.closed
    finally:
        if source.returncode is None:
            source.terminate()
            source.wait()
        if source.stdout is not None:
            source.stdout.close()
        if source.stderr is not None:
            source.stderr.close()


def test_pipeline_kills_source_that_does_not_terminate(monkeypatch):
    class StubbornSource:
        def __init__(self):
            self.stdout = BytesIO()
            self.stderr = BytesIO()
            self.returncode = None
            self.terminated = False
            self.killed = False
            self.wait_timeouts = []

        def poll(self):
            return self.returncode

        def terminate(self):
            self.terminated = True

        def kill(self):
            self.killed = True

        def wait(self, timeout=None):
            self.wait_timeouts.append(timeout)
            if not self.killed:
                if timeout is None:
                    raise AssertionError("terminate wait must be bounded")
                raise TimeoutExpired(["quiet-source"], timeout)
            self.returncode = -9
            return self.returncode

    source = StubbornSource()
    calls = 0

    def failing_sink_popen(args, **kwargs):
        nonlocal calls
        calls += 1
        if calls == 1:
            return source
        raise FileNotFoundError("sink executable missing")

    monkeypatch.setattr(process_module, "Popen", failing_sink_popen)

    with pytest.raises(FileNotFoundError, match="sink executable missing"):
        ProcessRunner().pipeline(
            ["quiet-source"],
            ["missing-sink"],
            BytesIO(),
        )

    assert source.terminated
    assert source.killed
    assert source.wait_timeouts[0] is not None
    assert source.wait_timeouts[-1] is None
    assert source.returncode == -9

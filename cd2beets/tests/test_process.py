from pathlib import Path

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

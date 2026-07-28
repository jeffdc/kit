from __future__ import annotations

from dataclasses import dataclass
from subprocess import PIPE, Popen, TimeoutExpired, run
from threading import Thread
from typing import BinaryIO, Sequence


@dataclass(frozen=True)
class CommandResult:
    returncode: int
    stdout: str
    stderr: str


@dataclass(frozen=True)
class PipelineResult:
    source_returncode: int
    sink_returncode: int
    source_stderr: str
    sink_stderr: str

    @property
    def ok(self) -> bool:
        return self.source_returncode == 0 and self.sink_returncode == 0


class ProcessRunner:
    def run(self, args: Sequence[str], timeout: float | None = None) -> CommandResult:
        completed = run(
            list(args),
            capture_output=True,
            text=True,
            timeout=timeout,
            shell=False,
            check=False,
        )
        return CommandResult(completed.returncode, completed.stdout, completed.stderr)

    def pipeline(
        self,
        source_args: Sequence[str],
        sink_args: Sequence[str],
        destination: BinaryIO,
    ) -> PipelineResult:
        source = Popen(list(source_args), stdout=PIPE, stderr=PIPE, shell=False)
        sink = None
        try:
            if source.stdout is None or source.stderr is None:
                raise RuntimeError("source process pipes unavailable")
            sink = Popen(
                list(sink_args),
                stdin=source.stdout,
                stdout=destination,
                stderr=PIPE,
                shell=False,
            )
            source.stdout.close()
            if sink.stderr is None:
                raise RuntimeError("sink process stderr unavailable")
            captured: dict[str, bytes] = {}

            def drain(name: str, pipe: BinaryIO) -> None:
                captured[name] = pipe.read()

            source_thread = Thread(
                target=drain, args=("source", source.stderr), daemon=True
            )
            sink_thread = Thread(target=drain, args=("sink", sink.stderr), daemon=True)
            source_thread.start()
            sink_thread.start()
            sink_returncode = sink.wait()
            source_returncode = source.wait()
            source_thread.join()
            sink_thread.join()
            return PipelineResult(
                source_returncode,
                sink_returncode,
                captured["source"].decode(errors="replace"),
                captured["sink"].decode(errors="replace"),
            )
        except BaseException:
            if source.stdout is not None:
                try:
                    source.stdout.close()
                except Exception:
                    pass
            for process in (sink, source):
                if process is None:
                    continue
                if process.poll() is None:
                    try:
                        process.terminate()
                    except ProcessLookupError:
                        pass
                try:
                    process.wait(timeout=1.0)
                except TimeoutExpired:
                    try:
                        process.kill()
                    except ProcessLookupError:
                        pass
                    try:
                        process.wait()
                    except Exception:
                        pass
                except Exception:
                    pass
            for pipe in (
                source.stderr,
                sink.stderr if sink is not None else None,
            ):
                if pipe is not None:
                    try:
                        pipe.close()
                    except Exception:
                        pass
            raise

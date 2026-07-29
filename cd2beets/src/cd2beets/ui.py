from __future__ import annotations

import sys
from typing import TextIO

from cd2beets.models import (
    FailureDecision,
    OperatorDecision,
    ReleaseMetadata,
)


_FAILURE_LABELS = {
    FailureDecision.RETRY_IMPORT: ("i", "retry import"),
    FailureDecision.RETRY_VERIFY: ("v", "retry verification"),
    FailureDecision.EJECT: ("e", "eject"),
    FailureDecision.QUIT: ("q", "quit/keep loaded (default)"),
}


class TerminalUI:
    """All interactive terminal decisions, with conservative defaults."""

    def __init__(
        self,
        *,
        input_stream: TextIO | None = None,
        output_stream: TextIO | None = None,
    ) -> None:
        self._input = input_stream if input_stream is not None else sys.stdin
        self._output = output_stream if output_stream is not None else sys.stdout

    def select_release(
        self, candidates: tuple[ReleaseMetadata, ...]
    ) -> ReleaseMetadata:
        if not candidates:
            raise ValueError("no complete release candidates")

        for index, candidate in enumerate(candidates, start=1):
            self._print_release(index, candidate)

        if len(candidates) == 1:
            return candidates[0]

        while True:
            response = self._readline(
                f"Select release [1-{len(candidates)}]: "
            ).strip()
            try:
                selected = int(response)
            except ValueError:
                selected = 0
            if 1 <= selected <= len(candidates):
                return candidates[selected - 1]
            self._write("Invalid selection.\n")

    def decide_uncertain(
        self, reasons: tuple[str, ...]
    ) -> OperatorDecision:
        self._write("Library presence is uncertain.\n")
        for reason in reasons:
            self._write(f"- {reason}\n")

        prompt = (
            "Choose [s]kip/eject (default), [p]roceed, "
            "[a]bort/keep loaded: "
        )
        choices = {
            "": OperatorDecision.SKIP_EJECT,
            "s": OperatorDecision.SKIP_EJECT,
            "p": OperatorDecision.PROCEED,
            "a": OperatorDecision.ABORT_KEEP_LOADED,
        }
        while True:
            try:
                response = self._readline(prompt).strip().casefold()
            except EOFError:
                return OperatorDecision.ABORT_KEEP_LOADED
            decision = choices.get(response)
            if decision is not None:
                return decision
            self._write("Invalid choice.\n")

    def decide_failure(
        self,
        error: BaseException | str,
        allowed: tuple[FailureDecision, ...],
    ) -> FailureDecision:
        if not allowed:
            raise ValueError("at least one failure decision is required")
        if len(set(allowed)) != len(allowed):
            raise ValueError("duplicate failure decision")

        invalid = tuple(
            decision
            for decision in allowed
            if not isinstance(decision, FailureDecision)
        )
        if invalid:
            raise ValueError(f"unsupported failure decision: {invalid[0]}")

        message = str(error)
        if message:
            self._write(message)
            if not message.endswith("\n"):
                self._write("\n")

        key_to_decision = {
            _FAILURE_LABELS[decision][0]: decision for decision in allowed
        }
        prompt = "Failure choices: " + ", ".join(
            f"[{_FAILURE_LABELS[decision][0]}] {_FAILURE_LABELS[decision][1]}"
            for decision in allowed
        )
        prompt += ": "

        while True:
            try:
                response = self._readline(prompt).strip().casefold()
            except EOFError:
                if FailureDecision.QUIT in allowed:
                    return FailureDecision.QUIT
                raise
            if response == "" and FailureDecision.QUIT in allowed:
                return FailureDecision.QUIT
            decision = key_to_decision.get(response)
            if decision is not None:
                return decision
            self._write("Invalid choice.\n")

    def report_error(self, error: BaseException | str) -> None:
        message = str(error)
        self._write(message)
        if not message.endswith("\n"):
            self._write("\n")


    def report_unusable_media(self) -> None:
        self._write("Optical media is present but is not an audio CD.\n")

    def report_preservation(self, state: str) -> None:
        if state == "processing":
            message = (
                "Interrupted during disc processing. No further cleanup or "
                "eject will be started; media and generated files remain "
                "as they are."
            )
        elif state == "no_media":
            message = (
                "Interrupted while no disc workflow was active; no loaded "
                "disc or generated files are being claimed."
            )
        elif state == "after_eject":
            message = (
                "Interrupted after eject. No further disc or file action "
                "will be started."
            )
        else:
            message = (
                "Interrupted while polling optical media state is unknown. "
                "No cleanup or eject was initiated by the interrupt."
            )
        self._write(f"{message}\n")

    def report_busy_music(self) -> None:
        self._write(
            "The Music app is running and may be holding the optical drive. "
            "Close Music manually before trying again; cd2beets will not "
            "quit or signal Music.\n"
        )

    def _print_release(self, index: int, release: ReleaseMetadata) -> None:
        self._write(f"Release candidate {index}:\n")
        self._write(f"  Release ID: {release.release_id}\n")
        self._write(f"  Artist: {release.albumartist}\n")
        self._write(f"  Album: {release.album}\n")
        self._write(f"  Date: {release.date}\n")
        self._write(f"  Country: {release.country}\n")
        self._write(f"  Disambiguation: {release.disambiguation}\n")
        self._write(f"  Medium: {release.medium}\n")
        self._write(f"  Track count: {len(release.tracks)}\n")

    def _readline(self, prompt: str) -> str:
        self._write(prompt)
        response = self._input.readline()
        if response == "":
            raise EOFError("terminal input closed")
        return response

    def _write(self, text: str) -> None:
        self._output.write(text)
        self._output.flush()

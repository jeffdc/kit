from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


class Journal:
    """A durable append-only JSON Lines lifecycle journal."""

    def __init__(self, path: Path) -> None:
        self.path = Path(path)

    def append(self, event: str, **fields: Any) -> None:
        if "timestamp" in fields:
            json.dumps(fields["timestamp"])
        record = {
            **fields,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "event": event,
        }
        line = json.dumps(record, separators=(",", ":"))

        with self.path.open("a", encoding="utf-8", newline="\n") as output:
            output.write(line)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())

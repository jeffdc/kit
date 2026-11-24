#!/usr/bin/env bash
# Install the OMP guard hook. OMP skips symlinks in its hooks folder, so the
# hook is copied, not linked: run this again after changing hooks/guards.ts.
set -euo pipefail

SRC="$(cd "$(dirname "$0")" && pwd)/hooks/guards.ts"
AGENT_DIR="$HOME/.omp/agent"
DST="$AGENT_DIR/hooks/pre/guards.ts"
CONFIG="$AGENT_DIR/guards.json"

mkdir -p "$(dirname "$DST")"

if [ -L "$DST" ]; then
  echo "WARNING  $DST is a symlink; OMP skips symlinks, so the guard is not loaded. Replace it with a copy."
  exit 1
elif [ ! -e "$DST" ]; then
  cp "$SRC" "$DST"
  echo "installed guards.ts"
elif cmp -s "$SRC" "$DST"; then
  echo "ok       guards.ts (up to date)"
elif [ "${1:-}" = "--force" ]; then
  cp "$SRC" "$DST"
  echo "updated  guards.ts"
else
  echo "WARNING  $DST differs from hooks/guards.ts; run with --force to replace it"
  exit 1
fi

if [ ! -f "$CONFIG" ]; then
  echo "note     no $CONFIG, so no extra directories are protected"
fi

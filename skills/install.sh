#!/usr/bin/env bash
set -euo pipefail

SKILLS_SRC="$(cd "$(dirname "$0")" && pwd)"
SKILLS_DST="$HOME/.claude/skills"

SKILLS=(
  brainstorming
  dispatching-parallel-agents
  executing-plans
  finishing-a-development-branch
  receiving-code-review
  requesting-code-review
  user-profile
  systematic-debugging
  test-driven-development
  using-git-worktrees
  using-writs
  verification-before-completion
  writing-plans
  writing-skills
)

mkdir -p "$SKILLS_DST"

for name in "${SKILLS[@]}"; do
  src="$SKILLS_SRC/$name"
  dst="$SKILLS_DST/$name"

  if [ -L "$dst" ]; then
    target="$(readlink "$dst")"
    if [ "$target" = "$src" ]; then
      echo "ok       $name (already linked)"
    else
      echo "WARNING  $name -> $target (symlink exists but points elsewhere)"
    fi
  elif [ -e "$dst" ]; then
    echo "WARNING  $name already exists at $dst and is not a symlink; skipping"
  else
    ln -s "$src" "$dst"
    echo "linked   $name"
  fi
done

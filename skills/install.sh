#!/usr/bin/env bash
set -euo pipefail

SKILLS_SRC="$(cd "$(dirname "$0")" && pwd)"
SKILLS_DST="$HOME/.claude/skills"

SKILLS=(
  brainstorming
  dispatching-parallel-agents
  executing-plans
  finishing-a-development-branch
  production-investigation
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

# --- SessionStart hook for using-writs ---
HOOK_CMD="$SKILLS_SRC/hooks/session-start.sh"
SETTINGS="$HOME/.claude/settings.json"

if [ ! -f "$SETTINGS" ]; then
  echo "WARNING  $SETTINGS not found; skipping hook install"
  exit 0
fi

# Check if hook already installed
if jq -e '.hooks.SessionStart[]? | .hooks[]? | select(.command == "'"$HOOK_CMD"'")' "$SETTINGS" >/dev/null 2>&1; then
  echo "ok       session-start hook (already installed)"
else
  # Append our hook entry to the SessionStart array
  jq --arg cmd "$HOOK_CMD" '
    .hooks.SessionStart += [{
      "matcher": "",
      "hooks": [{"type": "command", "command": $cmd, "async": false}]
    }]
  ' "$SETTINGS" > "${SETTINGS}.tmp" && mv "${SETTINGS}.tmp" "$SETTINGS"
  echo "hooked   session-start (using-writs injected at startup)"
fi

# --- Read permissions for skill files and user profile ---
ALLOW_RULES=(
  "Read(~/.claude/skills/**)"
  "Read(~/.claude/user-profile.md)"
)

for rule in "${ALLOW_RULES[@]}"; do
  if jq -e --arg r "$rule" '.permissions.allow | index($r)' "$SETTINGS" >/dev/null 2>&1; then
    echo "ok       permission: $rule"
  else
    jq --arg r "$rule" '.permissions.allow += [$r]' "$SETTINGS" > "${SETTINGS}.tmp" && mv "${SETTINGS}.tmp" "$SETTINGS"
    echo "allowed  $rule"
  fi
done

#!/bin/bash
# Watchmen status line for Claude Code
# Reads JSON context from stdin and outputs status line text

# Read stdin (Claude Code context - not used but required)
input=$(cat)

# Query watchmen for current timer status
if TIMER_STATUS=$(watchmen status --json 2>/dev/null); then
  # Check if we got an empty object (no active timer)
  if [ "$TIMER_STATUS" = "{}" ]; then
    echo ""
    exit 0
  fi

  # Parse JSON fields
  RUNNING=$(echo "$TIMER_STATUS" | jq -r '.running // false')
  PAUSED=$(echo "$TIMER_STATUS" | jq -r '.paused // false')
  PROJECT=$(echo "$TIMER_STATUS" | jq -r '.project_name // "Unknown"')
  ELAPSED=$(echo "$TIMER_STATUS" | jq -r '.elapsed_seconds // 0')

  # Format elapsed time
  HOURS=$((ELAPSED / 3600))
  MINUTES=$(((ELAPSED % 3600) / 60))
  SECONDS=$((ELAPSED % 60))

  # Build status line with appropriate icon and color
  if [ "$RUNNING" = "true" ]; then
    # Running: green with timer icon
    printf "\033[32m⏱ %s\033[0m %dh %dm %ds" "$PROJECT" "$HOURS" "$MINUTES" "$SECONDS"
  elif [ "$PAUSED" = "true" ]; then
    # Paused: yellow with pause icon
    printf "\033[33m⏸ %s\033[0m %dh %dm" "$PROJECT" "$HOURS" "$MINUTES"
  else
    # Unknown state
    echo ""
  fi
else
  # watchmen command failed or not found - silent failure
  echo ""
fi

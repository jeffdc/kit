#!/bin/bash
# Install Watchmen Claude Code integration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="$HOME/.claude"
SKILLS_DIR="$CLAUDE_DIR/skills"

echo "Installing Watchmen Claude Code integration..."
echo

# Create directories if they don't exist
mkdir -p "$CLAUDE_DIR"
mkdir -p "$SKILLS_DIR"

# Install status line script
echo "→ Installing status line script..."
cp "$SCRIPT_DIR/statusline.sh" "$CLAUDE_DIR/watchmen-statusline.sh"
chmod +x "$CLAUDE_DIR/watchmen-statusline.sh"
echo "  ✓ Installed to $CLAUDE_DIR/watchmen-statusline.sh"

# Install skills
echo
echo "→ Installing timer control skills..."
for skill in timer-start timer-pause timer-resume timer-stop wrap; do
  mkdir -p "$SKILLS_DIR/$skill"
  cp "$SCRIPT_DIR/skills/$skill/SKILL.md" "$SKILLS_DIR/$skill/"
  echo "  ✓ Installed /$skill"
done

# Check if settings.json exists and update it
echo
echo "→ Configuring settings..."
SETTINGS_FILE="$CLAUDE_DIR/settings.json"
STATUSLINE_PATH="$CLAUDE_DIR/watchmen-statusline.sh"

if [ -f "$SETTINGS_FILE" ]; then
  # Check if statusLine already exists
  if grep -q '"statusLine"' "$SETTINGS_FILE"; then
    echo "  ⚠ statusLine already configured in settings.json"
    echo "  Please manually update if needed:"
    echo "    \"statusLine\": {"
    echo "      \"type\": \"command\","
    echo "      \"command\": \"$STATUSLINE_PATH\","
    echo "      \"padding\": 0"
    echo "    }"
  else
    echo "  ℹ To enable the status line, add this to $SETTINGS_FILE:"
    echo
    echo "    \"statusLine\": {"
    echo "      \"type\": \"command\","
    echo "      \"command\": \"$STATUSLINE_PATH\","
    echo "      \"padding\": 0"
    echo "    }"
    echo
    echo "  Add it after the \"model\" setting in the JSON file."
  fi
else
  echo "  ℹ No settings.json found. Create $SETTINGS_FILE with:"
  echo
  echo "    {"
  echo "      \"statusLine\": {"
  echo "        \"type\": \"command\","
  echo "        \"command\": \"$STATUSLINE_PATH\","
  echo "        \"padding\": 0"
  echo "      }"
  echo "    }"
fi

echo
echo "✓ Installation complete!"
echo
echo "Next steps:"
echo "  1. Update $SETTINGS_FILE (see above)"
echo "  2. Restart Claude Code"
echo "  3. Try /timer-start, /timer-pause, /wrap, etc."
echo
echo "Requirements:"
echo "  - watchmen must be in your PATH"
echo "  - jq must be installed (for JSON parsing)"
echo

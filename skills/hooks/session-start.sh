#!/usr/bin/env bash
# SessionStart hook: inject using-writs skill into session context
# so Claude checks for applicable skills before every response.

set -euo pipefail

SKILL_FILE="${HOME}/.claude/skills/using-writs/SKILL.md"

if [ ! -f "$SKILL_FILE" ]; then
  exit 0
fi

# Strip YAML frontmatter (--- block) before injecting
content=$(awk '
  NR==1 && /^---$/ { skip=1; next }
  skip==1 && /^---$/ { skip=0; next }
  skip==0 { print }
' "$SKILL_FILE")

# Escape for JSON embedding
escape_for_json() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    s="${s//$'\r'/\\r}"
    s="${s//$'\t'/\\t}"
    printf '%s' "$s"
}

escaped=$(escape_for_json "$content")
context="<EXTREMELY_IMPORTANT>\n**Below is your 'using-writs' skill — your guide to finding and using skills. For all other skills, use the Skill tool:**\n\n${escaped}\n</EXTREMELY_IMPORTANT>"

cat <<EOF
{
  "additional_context": "${context}",
  "hookSpecificOutput": {
    "hookEventName": "SessionStart",
    "additionalContext": "${context}"
  }
}
EOF

exit 0

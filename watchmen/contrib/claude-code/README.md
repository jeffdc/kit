# Claude Code Integration

This directory contains integration files for using Watchmen with Claude Code.

## Installation

### 1. Status Line (shows timer in status bar)

Copy the status line script to your global Claude Code directory:

```bash
cp contrib/claude-code/statusline.sh ~/.claude/watchmen-statusline.sh
chmod +x ~/.claude/watchmen-statusline.sh
```

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "/Users/YOUR_USERNAME/.claude/watchmen-statusline.sh",
    "padding": 0
  }
}
```

**Note:** Replace `YOUR_USERNAME` with your actual username, or use the full path from `echo ~/.claude/watchmen-statusline.sh`

### 2. Timer Control Skills

Copy the skills to your global Claude Code skills directory:

```bash
cp -r contrib/claude-code/skills/* ~/.claude/skills/
```

After installation, restart Claude Code.

## Available Skills

- `/timer-start <project>` - Start tracking a project
- `/timer-pause` - Pause current timer
- `/timer-resume` - Resume paused timer
- `/timer-stop` - Stop and save entry
- `/wrap [context]` - Wrap up session (reviews commits, generates summary, stops timer)

## Status Line Display

When a timer is running, you'll see in the status bar:

- **Running**: `⏱ project-name 2h 15m 30s` (green)
- **Paused**: `⏸ project-name 2h 15m` (yellow)
- **No timer**: (blank)

The status line updates automatically and shows real-time elapsed time.

## Requirements

- `watchmen` must be installed and in your PATH
- `jq` must be installed for JSON parsing in the status line script
- Watchmen version must support `--json` flag on `status` command

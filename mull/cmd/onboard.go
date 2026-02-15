package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const mullHookCommand = "mull prime --context"

var hookInstall bool
var hookUninstall bool

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Print setup instructions for Claude Code integration",
	Long:  `Shows how to integrate mull with Claude Code using either hooks or the skill.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(`# Mull — Claude Code Integration

There are two ways to integrate mull with Claude Code:

## Option 1: Hooks (recommended)

Workflow context is injected automatically at session start. Run:

    mull onboard hooks --install

## Option 2: Skill

A Claude Code skill that is invoked on demand. Run:

    mull onboard skill
`)
		return nil
	},
}

var onboardHooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage Claude Code hook integration",
	Long: `Install or uninstall mull hooks in ~/.claude/settings.json.

Without flags, prints the hook configuration for manual setup.
Use --install to add hooks automatically, --uninstall to remove them.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if hookInstall {
			return installHooks()
		}
		if hookUninstall {
			return uninstallHooks()
		}

		fmt.Print(`# Mull — Hook Integration

Add the following to your ~/.claude/settings.json under the "hooks" key.
The hook runs on session start and before context compaction, injecting
mull's workflow context automatically. It exits silently in non-mull projects.

    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "mull prime --context"
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "mull prime --context"
          }
        ]
      }
    ]

Or run "mull onboard hooks --install" to add them automatically.
`)
		return nil
	},
}

var onboardSkillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print skill setup instructions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(`# Mull — Skill Integration

Symlink the mull skill into your Claude Code skills directory:

    ln -s "$(pwd)/skills/SKILL.md" ~/.claude/skills/mull.md

Run this from the mull source directory (where skills/SKILL.md lives).
The skill will be available as /mull in Claude Code sessions.
`)
		return nil
	},
}

func init() {
	onboardHooksCmd.Flags().BoolVar(&hookInstall, "install", false, "Install hooks into ~/.claude/settings.json")
	onboardHooksCmd.Flags().BoolVar(&hookUninstall, "uninstall", false, "Remove hooks from ~/.claude/settings.json")
	onboardHooksCmd.MarkFlagsMutuallyExclusive("install", "uninstall")
	onboardCmd.AddCommand(onboardHooksCmd)
	onboardCmd.AddCommand(onboardSkillCmd)
	rootCmd.AddCommand(onboardCmd)
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// mullHookEntry returns the hook entry object for mull.
func mullHookEntry() map[string]any {
	return map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": mullHookCommand,
			},
		},
	}
}

// hasMullHook checks if a hooks array already contains a mull hook.
func hasMullHook(entries []any) bool {
	for _, entry := range entries {
		e, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := e["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == mullHookCommand {
				return true
			}
		}
	}
	return false
}

// removeMullHook removes mull hook entries from a hooks array.
func removeMullHook(entries []any) []any {
	var result []any
	for _, entry := range entries {
		e, ok := entry.(map[string]any)
		if !ok {
			result = append(result, entry)
			continue
		}
		hooks, ok := e["hooks"].([]any)
		if !ok {
			result = append(result, entry)
			continue
		}
		isMull := false
		for _, h := range hooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == mullHookCommand {
				isMull = true
				break
			}
		}
		if !isMull {
			result = append(result, entry)
		}
	}
	return result
}

func installHooks() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	added := false
	for _, key := range []string{"SessionStart", "PreCompact"} {
		entries, _ := hooks[key].([]any)
		if hasMullHook(entries) {
			continue
		}
		hooks[key] = append(entries, mullHookEntry())
		added = true
	}

	if !added {
		fmt.Println("Mull hooks already installed.")
		return nil
	}

	settings["hooks"] = hooks
	if err := writeSettings(path, settings); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	fmt.Println("Installed mull hooks into " + path)
	return nil
}

func uninstallHooks() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		fmt.Println("No hooks found.")
		return nil
	}

	removed := false
	for _, key := range []string{"SessionStart", "PreCompact"} {
		entries, _ := hooks[key].([]any)
		filtered := removeMullHook(entries)
		if len(filtered) != len(entries) {
			removed = true
		}
		hooks[key] = filtered
	}

	if !removed {
		fmt.Println("No mull hooks found to remove.")
		return nil
	}

	settings["hooks"] = hooks
	if err := writeSettings(path, settings); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	fmt.Println("Removed mull hooks from " + path)
	return nil
}

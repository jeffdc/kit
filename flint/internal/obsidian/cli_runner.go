package obsidian

import (
	"os/exec"
	"strings"
)

// CLIRunner calls the obsidian binary.
type CLIRunner struct{}

func (r *CLIRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("obsidian", args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

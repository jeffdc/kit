// Package launchagent runs mull serve at login on macOS, as a launchd
// LaunchAgent, so the page pinned in a browser tab always answers. It writes
// one property list in ~/Library/LaunchAgents and loads it into the user's
// GUI domain with launchctl; nothing else on the machine changes.
package launchagent

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// Label is the agent's launchd label, and its property list's name.
const Label = "mull.serve"

// Agent is mull serve as a LaunchAgent for one user.
type Agent struct {
	Home string            // the user's home directory
	UID  int               // the user's ID: the agent loads into gui/<UID>
	Exe  string            // the mull binary to run
	Port int               // mull serve's port
	Env  map[string]string // the environment it runs with (PATH, so it finds git; XDG_CONFIG_HOME, so it finds the registry)
	// Launchctl runs launchctl with args (exec in production, a fake in
	// tests).
	Launchctl func(args ...string) ([]byte, error)
}

// PlistPath is where the agent's property list lives.
func (a Agent) PlistPath() string {
	return filepath.Join(a.Home, "Library", "LaunchAgents", Label+".plist")
}

// LogPath is where mull serve's output goes while it runs as an agent.
func (a Agent) LogPath() string {
	return filepath.Join(a.Home, "Library", "Logs", "mull-serve.log")
}

// Args is the command the agent runs.
func (a Agent) Args() []string {
	return []string{a.Exe, "serve", "--port", strconv.Itoa(a.Port)}
}

func (a Agent) domain() string  { return "gui/" + strconv.Itoa(a.UID) }
func (a Agent) service() string { return a.domain() + "/" + Label }

// Plist is the agent's property list: run at load (so at login), restarted
// if it stops, with at least 30 seconds between starts so a port that's
// taken doesn't spin.
func (a Agent) Plist() []byte {
	var b bytes.Buffer
	str := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s))
		return "<string>" + e.String() + "</string>"
	}
	b.WriteString(xml.Header)
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\">\n<dict>\n")
	b.WriteString("\t<key>Label</key>\n\t" + str(Label) + "\n")
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range a.Args() {
		b.WriteString("\t\t" + str(arg) + "\n")
	}
	b.WriteString("\t</array>\n")
	if len(a.Env) > 0 {
		keys := make([]string, 0, len(a.Env))
		for k := range a.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		for _, k := range keys {
			b.WriteString("\t\t<key>" + k + "</key>\n\t\t" + str(a.Env[k]) + "\n")
		}
		b.WriteString("\t</dict>\n")
	}
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>KeepAlive</key>\n\t<true/>\n")
	b.WriteString("\t<key>ThrottleInterval</key>\n\t<integer>30</integer>\n")
	// Interactive: the human is waiting on every page. Background (and, per
	// launchd.plist(5), leaving it unset) throttles CPU and I/O, git children
	// included, which made pages 3-4x slower.
	b.WriteString("\t<key>ProcessType</key>\n\t" + str("Interactive") + "\n")
	b.WriteString("\t<key>StandardOutPath</key>\n\t" + str(a.LogPath()) + "\n")
	b.WriteString("\t<key>StandardErrorPath</key>\n\t" + str(a.LogPath()) + "\n")
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}

// Error is launchctl failing.
type Error struct {
	Args   []string
	Output string
	Err    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("launchctl %v: %v: %s", e.Args, e.Err, bytes.TrimSpace([]byte(e.Output)))
}

func (e *Error) Unwrap() error { return e.Err }

// Install writes the property list and (re)loads the agent: an agent
// already loaded is unloaded first, so installing again after upgrading mull
// runs the new binary.
func (a Agent) Install() error {
	if err := os.MkdirAll(filepath.Dir(a.PlistPath()), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.LogPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(a.PlistPath(), a.Plist(), 0o644); err != nil {
		return err
	}
	_, _ = a.Launchctl("bootout", a.service()) // not loaded is fine
	// launchd can refuse a bootstrap that follows a bootout too closely
	// (an I/O error); a few tries a moment apart get past it.
	args := []string{"bootstrap", a.domain(), a.PlistPath()}
	for try := 1; ; try++ {
		out, err := a.Launchctl(args...)
		if err == nil {
			return nil
		}
		if try == 4 {
			return &Error{Args: args, Output: string(out), Err: err}
		}
		pause(time.Duration(try) * 250 * time.Millisecond)
	}
}

// pause waits between tries (tests skip it).
var pause = time.Sleep

// Uninstall unloads the agent and removes its property list. It reports
// whether there was a property list to remove; unloading an agent that
// isn't loaded is fine.
func (a Agent) Uninstall() (bool, error) {
	_, _ = a.Launchctl("bootout", a.service())
	err := os.Remove(a.PlistPath())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

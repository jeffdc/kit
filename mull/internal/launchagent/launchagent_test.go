package launchagent

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fake records launchctl calls; fail makes the first n bootstraps fail.
type fake struct {
	calls [][]string
	fail  int
}

func (f *fake) run(args ...string) ([]byte, error) {
	f.calls = append(f.calls, args)
	if args[0] == "bootstrap" && f.fail > 0 {
		f.fail--
		return []byte("Bootstrap failed: 5: Input/output error"), errors.New("exit status 5")
	}
	if args[0] == "bootout" {
		return []byte("Boot-out failed: 3: No such process"), errors.New("exit status 3") // not loaded
	}
	return nil, nil
}

func agent(t *testing.T, f *fake) Agent {
	t.Helper()
	pause = func(time.Duration) {}
	t.Cleanup(func() { pause = time.Sleep })
	return Agent{Home: t.TempDir(), UID: 501, Exe: "/Users/me/go/bin/mull", Port: 6855,
		Env: map[string]string{"PATH": "/opt/homebrew/bin:/usr/bin:/bin", "XDG_CONFIG_HOME": "/Users/me/.config & co"}, Launchctl: f.run}
}

func TestInstall(t *testing.T) {
	f := &fake{fail: 2}
	a := agent(t, f)
	if err := a.Install(); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"bootout", "gui/501/mull.serve"},
		{"bootstrap", "gui/501", a.PlistPath()},
		{"bootstrap", "gui/501", a.PlistPath()},
		{"bootstrap", "gui/501", a.PlistPath()},
	}
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("launchctl calls = %v, want %v (unload, then bootstrap until it takes)", f.calls, want)
	}
	plist, err := os.ReadFile(a.PlistPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"<string>mull.serve</string>", "<string>/Users/me/go/bin/mull</string>", "<string>serve</string>", "<string>6855</string>",
		"<key>PATH</key>", "/Users/me/.config &amp; co", "<key>RunAtLoad</key>\n\t<true/>", "<key>KeepAlive</key>\n\t<true/>",
		"<key>ProcessType</key>\n\t<string>Interactive</string>", a.LogPath()} {
		if !strings.Contains(string(plist), s) {
			t.Errorf("plist lacks %q:\n%s", s, plist)
		}
	}
	if _, err := os.Stat(filepath.Dir(a.LogPath())); err != nil {
		t.Errorf("log directory not made: %v", err)
	}
	// The property list must be one launchd reads.
	if plutil, err := exec.LookPath("plutil"); err == nil {
		if out, err := exec.Command(plutil, "-lint", a.PlistPath()).CombinedOutput(); err != nil {
			t.Errorf("plutil -lint: %v\n%s", err, out)
		}
	}
}

func TestInstallGivesUp(t *testing.T) {
	f := &fake{fail: 10}
	a := agent(t, f)
	err := a.Install()
	var le *Error
	if !errors.As(err, &le) || !strings.Contains(err.Error(), "Input/output error") {
		t.Fatalf("Install = %v, want a launchctl error carrying its output", err)
	}
	if n := len(f.calls); n != 5 {
		t.Errorf("%d launchctl calls, want a bootout and 4 bootstraps", n)
	}
}

func TestUninstall(t *testing.T) {
	f := &fake{}
	a := agent(t, f)
	if removed, err := a.Uninstall(); err != nil || removed {
		t.Errorf("Uninstall with nothing installed = %v, %v; want false, nil", removed, err)
	}
	if err := a.Install(); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if removed, err := a.Uninstall(); err != nil || !removed {
		t.Errorf("Uninstall = %v, %v; want true, nil", removed, err)
	}
	if want := [][]string{{"bootout", "gui/501/mull.serve"}}; !reflect.DeepEqual(f.calls, want) {
		t.Errorf("launchctl calls = %v, want %v", f.calls, want)
	}
	if _, err := os.Stat(a.PlistPath()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("plist still there: %v", err)
	}
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"mull/internal/core"
	"mull/internal/git"
	"mull/internal/launchagent"
	"mull/internal/registry"
	"mull/internal/ui"
)

func unregisterCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "unregister [<path>]",
		Short: "Take a repository (default: this one) off this machine's registry, so mull serve stops showing it",
		Long: `Take a repository off this machine's registry, so mull serve stops showing it.

The repository is the one containing path (default: the current directory),
by its main worktree, as every command registers it; a path registered as it
is (one since deleted, say) is removed as given. mull serve drops it on its
next read of the registry. It prints {path, registry, removed}: removed is
false when the path wasn't registered. Running any mull command in the
repository again registers it again, unless it lives in the system's
temporary directory, which is never registered.`,
		Args: upTo(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.unregistered = true
			path := a.dir
			if len(args) == 1 {
				path = args[0]
				if !filepath.IsAbs(path) {
					path = filepath.Join(a.dir, path)
				}
			}
			path = filepath.Clean(path)
			paths := []string{path}
			if r, err := filepath.EvalSymlinks(path); err == nil && r != path {
				paths = append(paths, r)
			}
			repos, err := registry.Repos()
			if err != nil {
				return err
			}
			if !listed(repos, paths) {
				if r, err := git.Open(path); err == nil {
					path = r.MainWorktree()
					paths = []string{path}
				}
			}
			removed, err := registry.Remove(paths...)
			if err != nil {
				return err
			}
			file, err := registry.File()
			if err != nil {
				return err
			}
			return a.json(map[string]any{"path": path, "registry": file, "removed": removed})
		},
	}
}

// listed reports whether any of paths is in repos.
func listed(repos, paths []string) bool {
	for _, r := range repos {
		for _, p := range paths {
			if r == p {
				return true
			}
		}
	}
	return false
}

func serveCmd(a *app) *cobra.Command {
	var port int
	var install, uninstall bool
	c := &cobra.Command{
		Use:   "serve",
		Short: "The human's view: a local web UI of every repository on mull, on 127.0.0.1",
		Long: `The human's view: a local web UI of every repository on mull.

Listens on 127.0.0.1 only (default port ` + strconv.Itoa(ui.DefaultPort) + `) and runs until interrupted.
It prints {url, registry, repos} once it is listening. Repositories are found
with no configuration: every mull command registers its repository in the
registry file (except one in the system's temporary directory), and mull
unregister takes one off. The page updates live: mull serve watches each repository's
mull branch, and the registry, and tells the page what changed. mull serve
never writes mull data; the only thing it writes is when you last looked at
each repository, beside the registry.

On macOS, --install runs it at login instead, as a LaunchAgent
(~/Library/LaunchAgents/mull.serve.plist, logging to
~/Library/Logs/mull-serve.log), restarted if it stops; --uninstall removes
it. It runs this mull binary with --port and with your PATH and
XDG_CONFIG_HOME as they are now. After upgrading mull, run --install again
(make install does it for you when the agent is installed); open pages
reload themselves onto the new build when they reconnect.`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case install && uninstall:
				return &core.Error{Code: core.CodeUsage, Msg: "pass --install or --uninstall, not both"}
			case install || uninstall:
				return serveAgent(a, port, install)
			}
			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				if errors.Is(err, syscall.EADDRINUSE) {
					return &core.Error{Code: core.CodePortInUse, Msg: fmt.Sprintf("%s is in use (is mull serve already running?); pass --port to use another", addr)}
				}
				return err
			}
			defer ln.Close()
			file, err := registry.File()
			if err != nil {
				return err
			}
			repos, err := registry.Repos()
			if err != nil {
				return err
			}
			bound := ln.Addr().(*net.TCPAddr).Port
			err = a.json(map[string]any{"url": fmt.Sprintf("http://localhost:%d/", bound), "registry": file, "repos": repos})
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			srv := &http.Server{Handler: ui.Handler(registry.Repos), ReadHeaderTimeout: 10 * time.Second}
			done := make(chan error, 1)
			go func() { done <- srv.Serve(ln) }()
			select {
			case err := <-done:
				return err
			case <-ctx.Done():
			}
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return srv.Shutdown(shutdown)
		},
	}
	c.Flags().IntVar(&port, "port", ui.DefaultPort, "port to listen on, on 127.0.0.1")
	c.Flags().BoolVar(&install, "install", false, "run mull serve at login as a macOS LaunchAgent (again after upgrading mull)")
	c.Flags().BoolVar(&uninstall, "uninstall", false, "stop and remove the LaunchAgent")
	return c
}

// serveAgent installs or removes mull serve's LaunchAgent.
func serveAgent(a *app, port int, install bool) error {
	if runtime.GOOS != "darwin" {
		return &core.Error{Code: core.CodeNotAllowed, Msg: "--install and --uninstall set up a macOS LaunchAgent; elsewhere, run mull serve under your own service manager"}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	agent := launchagent.Agent{Home: home, UID: os.Getuid(), Exe: exe, Port: port, Env: map[string]string{},
		Launchctl: func(args ...string) ([]byte, error) { return exec.Command("launchctl", args...).CombinedOutput() }}
	if !install {
		removed, err := agent.Uninstall()
		if err != nil {
			return err
		}
		return a.json(map[string]any{"label": launchagent.Label, "plist": agent.PlistPath(), "removed": removed})
	}
	if strings.Contains(exe, string(filepath.Separator)+"go-build") {
		return &core.Error{Code: core.CodeNotAllowed, Msg: "this mull is a temporary go run build: install mull (make install) and run --install from that"}
	}
	for _, k := range []string{"PATH", "XDG_CONFIG_HOME"} {
		if v := os.Getenv(k); v != "" {
			agent.Env[k] = v
		}
	}
	warnings := []string{}
	// A mull serve already on the port keeps the agent's from starting.
	if ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err != nil {
		warnings = append(warnings, fmt.Sprintf("port %d is in use (a mull serve you started?); stop it and the agent's takes over within 30 seconds", port))
	} else {
		ln.Close()
	}
	if err := agent.Install(); err != nil {
		return &core.Error{Code: core.CodeLaunchd, Msg: err.Error()}
	}
	return a.json(map[string]any{"label": launchagent.Label, "plist": agent.PlistPath(), "log": agent.LogPath(),
		"url": fmt.Sprintf("http://localhost:%d/", port), "program": agent.Args(), "warnings": warnings})
}

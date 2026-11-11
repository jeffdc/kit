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

	"mull/internal/launchagent"
	"mull/internal/mull"
	"mull/internal/registry"
	"mull/internal/serve"
)

func serveCmd(a *app) *cobra.Command {
	var port int
	var install, uninstall bool
	c := &cobra.Command{
		Use:   "serve",
		Short: "The human's view: a local web UI of every repository on mull, on 127.0.0.1",
		Long: `The human's view: a local web UI of every repository on mull.

Listens on 127.0.0.1 only (default port ` + strconv.Itoa(serve.DefaultPort) + `) and runs until interrupted.
It prints {url, registry, repos} once it is listening. Repositories are found
with no configuration: every mull command registers its repository in the
registry file. The page updates live: mull serve watches each repository's
mull branch, and the registry, and tells the page what changed. mull serve
never writes mull data; the only thing it writes is when you last looked at
each repository, beside the registry.

On macOS, --install runs it at login instead, as a LaunchAgent
(~/Library/LaunchAgents/mull.serve.plist, logging to
~/Library/Logs/mull-serve.log), restarted if it stops; run --install again
after upgrading mull, and --uninstall to remove it. It runs this mull binary
with --port and with your PATH and XDG_CONFIG_HOME as they are now.`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case install && uninstall:
				return &mull.Error{Code: mull.CodeUsage, Msg: "pass --install or --uninstall, not both"}
			case install || uninstall:
				return serveAgent(a, port, install)
			}
			addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				if errors.Is(err, syscall.EADDRINUSE) {
					return &mull.Error{Code: mull.CodePortInUse, Msg: fmt.Sprintf("%s is in use (is mull serve already running?); pass --port to use another", addr)}
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
			watcher, err := serve.Watch(registry.Repos, file)
			if err != nil {
				return err
			}
			defer watcher.Close()
			bound := ln.Addr().(*net.TCPAddr).Port
			err = a.json(map[string]any{"url": fmt.Sprintf("http://localhost:%d/", bound), "registry": file, "repos": repos})
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			srv := &http.Server{Handler: serve.Handler(registry.Repos, serve.WithEvents(watcher)), ReadHeaderTimeout: 10 * time.Second}
			srv.RegisterOnShutdown(watcher.Close) // ends the event streams, which Shutdown would otherwise wait out
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
	c.Flags().IntVar(&port, "port", serve.DefaultPort, "port to listen on, on 127.0.0.1")
	c.Flags().BoolVar(&install, "install", false, "run mull serve at login as a macOS LaunchAgent (again after upgrading mull)")
	c.Flags().BoolVar(&uninstall, "uninstall", false, "stop and remove the LaunchAgent")
	return c
}

// serveAgent installs or removes mull serve's LaunchAgent.
func serveAgent(a *app, port int, install bool) error {
	if runtime.GOOS != "darwin" {
		return &mull.Error{Code: mull.CodeNotAllowed, Msg: "--install and --uninstall set up a macOS LaunchAgent; elsewhere, run mull serve under your own service manager"}
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
		return &mull.Error{Code: mull.CodeNotAllowed, Msg: "this mull is a temporary go run build: install mull (make install) and run --install from that"}
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
		return &mull.Error{Code: mull.CodeLaunchd, Msg: err.Error()}
	}
	return a.json(map[string]any{"label": launchagent.Label, "plist": agent.PlistPath(), "log": agent.LogPath(),
		"url": fmt.Sprintf("http://localhost:%d/", port), "program": agent.Args(), "warnings": warnings})
}

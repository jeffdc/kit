package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"mull/internal/mull"
	"mull/internal/registry"
	"mull/internal/serve"
)

func serveCmd(a *app) *cobra.Command {
	var port int
	c := &cobra.Command{
		Use:   "serve",
		Short: "The human's view: a local web UI of every repository on mull, on 127.0.0.1",
		Long: `The human's view: a local web UI of every repository on mull.

Listens on 127.0.0.1 only (default port ` + strconv.Itoa(serve.DefaultPort) + `) and runs until interrupted.
It prints {url, registry, repos} once it is listening. Repositories are found
with no configuration: every mull command registers its repository in the
registry file. mull serve only reads; it never writes mull data.`,
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			bound := ln.Addr().(*net.TCPAddr).Port
			err = a.json(map[string]any{"url": fmt.Sprintf("http://localhost:%d/", bound), "registry": file, "repos": repos})
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			srv := &http.Server{Handler: serve.Handler(registry.Repos), ReadHeaderTimeout: 10 * time.Second}
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
	return c
}

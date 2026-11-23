// Package cmd is mull's command line. Every command prints JSON to stdout
// (a few render for humans behind --text). Errors go to stderr as
// {"code": "...", "error": "..."} with exit status 1. The shapes are
// documented in mull guide json.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mull/internal/core"
	"mull/internal/guide"
	"mull/internal/human"
	"mull/internal/registry"
)

// Version is set at build time.
var Version = "dev"

type app struct {
	stdin  io.Reader
	stdout io.Writer
	dir    string
	// unregistered is set by mull unregister, so Execute doesn't add the
	// repository straight back.
	unregistered bool
}

func (a *app) open() (*core.Mull, error) { return core.Open(a.dir) }

func (a *app) json(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// text returns s, or stdin's content when s is "-".
func (a *app) text(s string) (string, error) {
	if s != "-" {
		return s, nil
	}
	b, err := io.ReadAll(a.stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\n"), nil
}

// classify turns any error into a coded mull error, recognising cobra's own
// command-line errors as usage errors.
func classify(err error) *core.Error {
	var me *core.Error
	if errors.As(err, &me) {
		return me
	}
	msg := err.Error()
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand", "flag needs an argument", "invalid argument", "required flag"} {
		if strings.HasPrefix(msg, p) {
			return &core.Error{Code: core.CodeUsage, Msg: msg}
		}
	}
	return core.AsError(err)
}

// Execute runs mull with args in dir and returns the process exit code.
func Execute(args []string, dir string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{stdin: stdin, stdout: stdout, dir: dir}
	root := newRoot(a)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.Execute()
	// Every command registers its repository for mull serve (after running,
	// so init counts), except unregister. It is best effort and never fails
	// the command.
	if !a.unregistered {
		_ = registry.Register(dir)
	}
	if err != nil {
		e := *classify(err)
		if name, herr := human.Name(); herr == nil && name != "" {
			// mull's own words in this machine's name for the human (mull name).
			e.Msg = guide.Personal(e.Msg, name)
			e.Lines = append([]core.LineError(nil), e.Lines...)
			for i := range e.Lines {
				e.Lines[i].Error = guide.Personal(e.Lines[i].Error, name)
			}
		}
		b, _ := json.Marshal(&e)
		fmt.Fprintln(stderr, string(b))
		return 1
	}
	return 0
}

// Main is the binary's entry point.
func Main() {
	dir, err := os.Getwd()
	if err != nil {
		b, _ := json.Marshal(&core.Error{Code: core.CodeInternal, Msg: err.Error()})
		fmt.Fprintln(os.Stderr, string(b))
		os.Exit(1)
	}
	os.Exit(Execute(os.Args[1:], dir, os.Stdin, os.Stdout, os.Stderr))
}

func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   "mull",
		Short: "The record of a program: plans, phases, items, and the inbox of what needs you",
		Long: `The record of a program: plans, phases, items, and the inbox of what needs you.

Every command prints JSON; errors are {"code","error"} on stderr with exit 1.
Agents: mull guide agents. The human's view: mull status, or mull serve.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return usage(err) })
	root.AddCommand(initCmd(a), versionCmd(a), nameCmd(a), syncCmd(a), configCmd(a), batchCmd(a), guideCmd(a),
		planCmd(a), phaseCmd(a), serveCmd(a), unregisterCmd(a))
	for _, group := range [][]*cobra.Command{addCmds(a), itemCmds(a), workCmds(a), inboxCmds(a), viewCmds(a)} {
		root.AddCommand(group...)
	}

	return root
}

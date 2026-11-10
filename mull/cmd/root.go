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

	"mull/internal/mull"
)

// Version is set at build time.
var Version = "dev"

type app struct {
	stdin  io.Reader
	stdout io.Writer
	dir    string
}

func (a *app) open() (*mull.Mull, error) { return mull.Open(a.dir) }

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
func classify(err error) *mull.Error {
	var me *mull.Error
	if errors.As(err, &me) {
		return me
	}
	msg := err.Error()
	for _, p := range []string{"unknown command", "unknown flag", "unknown shorthand", "flag needs an argument", "invalid argument", "required flag"} {
		if strings.HasPrefix(msg, p) {
			return &mull.Error{Code: mull.CodeUsage, Msg: msg}
		}
	}
	return mull.AsError(err)
}

// Execute runs mull with args in dir and returns the process exit code.
func Execute(args []string, dir string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{stdin: stdin, stdout: stdout, dir: dir}
	root := newRoot(a)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		b, _ := json.Marshal(classify(err))
		fmt.Fprintln(stderr, string(b))
		return 1
	}
	return 0
}

// Main is the binary's entry point.
func Main() {
	dir, err := os.Getwd()
	if err != nil {
		b, _ := json.Marshal(&mull.Error{Code: mull.CodeInternal, Msg: err.Error()})
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
Shapes and error codes: mull guide json. Many writes at once: mull batch.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return usage(err) })
	root.AddCommand(
		initCmd(a), versionCmd(a), syncCmd(a), viewCmd(a), configCmd(a), batchCmd(a),
		checkCmd(a), doneCmd(a), reportCmd(a), contextCmd(a), guideCmd(a),
		planCmd(a), phaseCmd(a),
		addCmd(a), showCmd(a), listCmd(a), setCmd(a), dropCmd(a),
		needCmd(a, true), needCmd(a, false), readyCmd(a),
		startCmd(a), releaseCmd(a),
		captureCmd(a, "decided"), captureCmd(a, "ask"),
		captureItemCmd(a, "bug"), captureItemCmd(a, "idea"), commitCmd(a),
		retiredCmd("loose", `capture it as an item with mull bug "..." or mull idea "..." (--source says where it came from)`),
		retiredCmd("ideas", "captured ideas are items (mull list --kind idea); older inbox ideas: mull inbox --kind idea"),
		inboxCmd(a), acceptCmd(a), decideCmd(a), doCmd(a), parkCmd(a),
		decisionsCmd(a),
	)
	return root
}

// Package cmd is mull's command line. Data commands print JSON to stdout;
// errors go to stderr as {"error": "..."} with exit status 1.
package cmd

import (
	"encoding/json"
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

// splitIDs accepts "a,b c" style lists.
func splitIDs(vals []string) []string {
	var out []string
	for _, v := range vals {
		for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' }) {
			out = append(out, f)
		}
	}
	return out
}

// Execute runs mull with args in dir and returns the process exit code.
func Execute(args []string, dir string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{stdin: stdin, stdout: stdout, dir: dir}
	root := newRoot(a)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(stderr, string(b))
		return 1
	}
	return 0
}

// Main is the binary's entry point.
func Main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", err.Error())
		os.Exit(1)
	}
	os.Exit(Execute(os.Args[1:], dir, os.Stdin, os.Stdout, os.Stderr))
}

func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "mull",
		Short:         "The record of a program: plans, epics, items, and the inbox of what needs you",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		initCmd(a),
		planCmd(a), epicCmd(a),
		addCmd(a), showCmd(a), listCmd(a), setCmd(a), dropCmd(a),
		needCmd(a, true), needCmd(a, false), readyCmd(a),
		startCmd(a), releaseCmd(a),
		captureCmd(a, "decided"), captureCmd(a, "ask"), captureCmd(a, "loose"),
		inboxCmd(a), acceptCmd(a), decideCmd(a), doCmd(a), parkCmd(a),
		decisionsCmd(a),
	)
	return root
}

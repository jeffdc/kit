// Package guide holds the plain-text procedures mull prints for agents.
// They are harness-neutral: any agent that can run a shell command can follow them.
package guide

import (
	_ "embed"
	"fmt"
	"sort"
)

//go:embed agents.md
var agents string

//go:embed supervisor.md
var supervisor string

//go:embed intake.md
var intake string

var guides = map[string]string{
	"agents":     agents,
	"supervisor": supervisor,
	"intake":     intake,
}

// Get returns the named guide.
func Get(name string) (string, error) {
	g, ok := guides[name]
	if !ok {
		return "", fmt.Errorf("no guide %q (guides: %v)", name, Names())
	}
	return g, nil
}

// Names lists the guides.
func Names() []string {
	var out []string
	for k := range guides {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

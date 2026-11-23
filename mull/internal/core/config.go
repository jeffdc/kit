package core

import (
	"slices"
	"strconv"
	"strings"
)

// Config changes. The settings that judge agents' work (checks, risk, the
// map, setup, the parallel cap) are the human's: an agent passes
// onRequest when they've asked.

func (m *Mull) configTx(what string, human bool, onRequest bool, fn func(c *Config) error) (Config, error) {
	var out Config
	err := m.update("config "+what, func(s *state) error {
		if human && !m.asked(onRequest) {
			return errf(CodeNotAllowed, "only the human changes %s: pass --on-request when they've asked", what)
		}
		c := s.config
		if err := fn(&c); err != nil {
			return err
		}
		s.config, out = c, c
		return s.putConfig()
	})
	return out, err
}

// ConfigSet sets integration, release, remote, worktrees or parallel.
func (m *Mull) ConfigSet(key, value string, onRequest bool) (Config, error) {
	return m.configTx(key, key == "parallel", onRequest, func(c *Config) error {
		value = strings.TrimSpace(value)
		switch key {
		case "integration":
			c.Integration = value
		case "release":
			c.Release = value
		case "remote":
			c.Remote = value
		case "worktrees":
			c.Worktrees = value
		case "parallel":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				return errf(CodeInvalid, "parallel is a number from 1")
			}
			c.Parallel = n
		default:
			return errf(CodeInvalid, "settable keys are integration, release, remote, worktrees and parallel")
		}
		return nil
	})
}

// ConfigChecks replaces the hard checks.
func (m *Mull) ConfigChecks(commands []string, onRequest bool) (Config, error) {
	return m.configTx("the hard checks", true, onRequest, func(c *Config) error {
		c.Checks = trimAll(commands)
		return nil
	})
}

// ConfigSetup replaces the commands start runs in a new worktree.
func (m *Mull) ConfigSetup(commands []string, onRequest bool) (Config, error) {
	return m.configTx("the setup commands", true, onRequest, func(c *Config) error {
		c.Setup = trimAll(commands)
		return nil
	})
}

// ConfigMap sets the commands that print the code's connections.
func (m *Mull) ConfigMap(edges, allowed string, onRequest bool) (Config, error) {
	return m.configTx("the map", true, onRequest, func(c *Config) error {
		c.Map = MapConfig{Edges: strings.TrimSpace(edges), Allowed: strings.TrimSpace(allowed)}
		return nil
	})
}

// RiskInput changes how risk is scored: globs added and removed, and the
// limits (0 keeps one; Sample -1 samples nothing).
type RiskInput struct {
	Add, Remove                []string
	MaxLines, MaxFiles, Sample int
	OnRequest                  bool
}

// ConfigRisk changes how done scores risk.
func (m *Mull) ConfigRisk(in RiskInput) (Config, error) {
	return m.configTx("risk", true, in.OnRequest, func(c *Config) error {
		rc := c.Risk.Effective()
		for _, g := range trimAll(in.Add) {
			if _, err := globRegexp(g); err != nil {
				return errf(CodeInvalid, "bad glob %q", g)
			}
			if !slices.Contains(rc.High, g) {
				rc.High = append(rc.High, g)
			}
		}
		for _, g := range trimAll(in.Remove) {
			rc.High = slices.DeleteFunc(rc.High, func(h string) bool { return h == g })
		}
		if in.MaxLines < 0 || in.MaxFiles < 0 || in.Sample > 100 {
			return errf(CodeInvalid, "lines and files are positive; sample is a percent")
		}
		if in.MaxLines > 0 {
			rc.MaxLines = in.MaxLines
		}
		if in.MaxFiles > 0 {
			rc.MaxFiles = in.MaxFiles
		}
		switch {
		case in.Sample > 0:
			rc.Sample = in.Sample
		case in.Sample < 0:
			rc.Sample = -1
		}
		if rc.High == nil {
			rc.High = []string{}
		}
		c.Risk = rc
		return nil
	})
}

func trimAll(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

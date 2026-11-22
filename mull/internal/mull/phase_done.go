package mull

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"mull/internal/model"
)

// ---- Done on a phase: its integration review ----

// PhaseDoneResult reports a done attempt on a phase.
type PhaseDoneResult struct {
	Report     model.Report `json:"report"`
	ReportPath string       `json:"report_path"`
	Phase      PhaseView    `json:"phase"`
	Review     string       `json:"review_entry,omitempty"`
	Warnings   []string     `json:"warnings,omitempty"`
	Reminder   string       `json:"reminder,omitempty"`
}

// Finish is done for an item or a phase, whichever id names.
func (m *Mull) Finish(id string, in DoneInput) (any, error) {
	isPhase := false
	if err := m.view(func(s *state) error { isPhase = s.phases[id] != nil; return nil }); err != nil {
		return nil, err
	}
	if isPhase {
		return m.PhaseDone(id, in)
	}
	return m.Done(id, in)
}

// doneText checks done's summary and design notes, and returns the design
// notes trimmed.
func doneText(in DoneInput) (string, error) {
	if strings.TrimSpace(in.Summary) == "" {
		return "", errf(CodeInvalid, "done needs a summary: a few sentences on what changed and how it fits")
	}
	design := strings.TrimSpace(in.Design)
	if design == "" || strings.EqualFold(strings.TrimRight(design, "."), "none") {
		return "", errf(CodeDesignRequired, "done needs design notes with --design: how it was built and why (the approach, the shape of the code, what was rejected)")
	}
	return design, nil
}

// phaseDoneItems returns the done items of a phase, in work order, or an
// error saying why the phase can't be filed for its integration review: it
// doesn't need one, it has open work or nothing done, an accepted review
// already covers it, or one is open (or parked) already.
func (s *state) phaseDoneItems(e *model.Phase) ([]*model.Item, error) {
	if !e.IntegrationReview {
		return nil, errf(CodeState, "phase %s doesn't need an integration review; only the human turns it on (mull phase set %s integration-review on --on-request)", e.ID, e.ID)
	}
	items := s.phaseItems(e.ID)
	s.workOrder(items)
	var done, open []*model.Item
	for _, i := range items {
		switch i.State {
		case model.StateDone:
			done = append(done, i)
		case model.StateCommitted, model.StateActive, model.StateInReview:
			open = append(open, i)
		}
	}
	if len(open) > 0 {
		var names []string
		for _, i := range open {
			names = append(names, fmt.Sprintf("%s (%s)", i.ID, i.State))
		}
		return nil, errf(CodeState, "phase %s has open work, so it isn't ready for its integration review: %s", e.ID, strings.Join(names, ", "))
	}
	if len(done) == 0 {
		return nil, errf(CodeState, "phase %s has no done items to review", e.ID)
	}
	if r := s.openIntegrationReview(e.ID); r != nil {
		return nil, errf(CodeState, "phase %s's integration review %s is open; it's settled first (mull accept %s, or mull do %s --reason \"...\")", e.ID, r.ID, r.ID, r.ID)
	}
	if !s.awaitsIntegrationReview(e.ID) {
		return nil, errf(CodeState, "phase %s is complete: an accepted integration review already covers its done items", e.ID)
	}
	return done, nil
}

// openIntegrationReview returns the phase's unsettled (or parked)
// integration review, if any.
func (s *state) openIntegrationReview(phaseID string) *model.Entry {
	for _, id := range sortedKeys(s.entries) {
		e := s.entries[id]
		if e.Kind == model.KindReview && e.Phase == phaseID && (e.Settled == nil || e.Settled.Action == model.ActPark) {
			return e
		}
	}
	return nil
}

// PhaseDone files a phase's integration review. Run in a clean worktree at
// the tip of the phase's plan's target, once every item in the phase is done
// (or dropped), it runs (or reuses) the hard checks there, writes a report
// with the phase's combined diff and its done items, and files a review
// entry on the phase. Accepting that review completes the phase. If the
// checks fail, the failed report is saved and nothing is filed.
func (m *Mull) PhaseDone(id string, in DoneInput) (PhaseDoneResult, error) {
	var out PhaseDoneResult
	design, err := doneText(in)
	if err != nil {
		return out, err
	}
	var cfg model.Config
	var commands, bases, paths []string
	var target, preview string
	err = m.view(func(s *state) error {
		cfg = s.config
		e, err := s.phase(id)
		if err != nil {
			return err
		}
		done, err := s.phaseDoneItems(e)
		if err != nil {
			return err
		}
		p := s.plans[e.Plan]
		commands = hardChecks(cfg, p)
		target, _ = s.targetOf(e.Plan)
		if p != nil {
			preview = p.Preview
		}
		bases, paths = s.phaseDiffScope(done)
		return nil
	})
	if err != nil {
		return out, err
	}
	if err := m.onTargetTip(target); err != nil {
		return out, err
	}

	var checks model.CheckRun
	if len(commands) == 0 {
		if in.Override == "" {
			return out, errf(CodeNoChecks, "no hard checks configured, so nothing can prove the phase's combined work; "+noChecksHint)
		}
		commit, tree, err := m.cleanTree()
		if err != nil {
			return out, err
		}
		checks = model.CheckRun{Tree: tree, Commit: commit, At: m.now(), By: m.Worktree()}
	} else {
		outcome, err := m.runChecks(commands)
		if err != nil {
			return out, err
		}
		checks = outcome.CheckRun
	}

	changes, base, err := m.phaseChanges(cfg, bases, paths, checks.Commit)
	if err != nil {
		return out, err
	}
	if len(in.Tour) == 0 {
		for _, lc := range changes {
			if lc.Layer == "ui" {
				return out, errf(CodeTourRequired, "the phase changed UI files (%d); give a short UI tour with --tour: what to click and what to see", len(lc.Files))
			}
		}
	}

	warnings := tourWarnings(in.Tour, preview)
	finished := checks.Passed || in.Override != ""
	err = m.update("done "+id, func(s *state) error {
		e, err := s.phase(id)
		if err != nil {
			return err
		}
		done, err := s.phaseDoneItems(e)
		if err != nil {
			return err
		}
		n := len(s.tx.List(fmt.Sprintf("reports/%s-", id))) + 1
		rep := model.Report{
			Phase: id, N: n, Passed: checks.Passed, Override: in.Override, Commit: checks.Commit, Base: base,
			At: m.now(), By: m.Worktree(), Summary: strings.TrimSpace(in.Summary), Tour: in.Tour, Checks: checks,
			Design: design, Changes: changes, Warnings: warnings,
		}
		var ids []string
		for _, i := range done {
			rep.Items = append(rep.Items, model.ReportItem{ID: i.ID, Title: i.Title})
			ids = append(ids, i.ID)
		}
		if p := s.plans[e.Plan]; p != nil {
			rep.Shared = sharedChanges(changes, p.Shared)
		}
		data, err := model.Encode(rep, renderReport(rep, e.Title))
		if err != nil {
			return err
		}
		out.ReportPath = model.ReportPath(id, n)
		s.tx.Put(out.ReportPath, data)
		out.Report = rep
		if finished {
			r, err := m.newEntry(s, model.KindReview, "Integration review: "+e.Title, "", e.Plan, out.ReportPath, "")
			if err != nil {
				return err
			}
			r.Phase, r.Items = id, ids
			if err := s.putEntry(r); err != nil {
				return err
			}
			out.Review = r.ID
		}
		out.Phase = s.phaseView(e)
		return nil
	})
	if err != nil {
		return out, err
	}
	out.Warnings = warnings
	if !finished {
		return out, &ErrChecksFailed{Report: out.ReportPath, Phase: true}
	}
	out.Reminder = m.personal("Show the human the report and the UI tour. Accepting the review completes the phase; " +
		"sending it back (mull do " + out.Review + " --reason \"...\") adds a fix-up item to the phase, which reopens it.")
	return out, nil
}

// onTargetTip checks that HEAD is the tip of target: a phase's integration
// review is of the combined work there.
func (m *Mull) onTargetTip(target string) error {
	tip, err := m.repo.RefCommit("refs/heads/" + target)
	if err != nil {
		return err
	}
	if tip == "" {
		return errf(CodeState, "the target branch %s doesn't exist", target)
	}
	head, err := m.repo.Output(nil, nil, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != tip {
		return errf(CodeState, "run done on a phase in a worktree at the tip of its target, %s (%s); HEAD is %s", target, short(tip), short(head))
	}
	return nil
}

// phaseDiffScope is what a phase's combined diff covers: the bases its done
// items' work started from (each claim's base, else its latest report's) and
// the files their latest done reports changed.
func (s *state) phaseDiffScope(done []*model.Item) (bases, paths []string) {
	for _, i := range done {
		r := s.latestReport(i.ID)
		base := ""
		if i.Claim != nil {
			base = i.Claim.Base
		}
		if base == "" && r != nil {
			base = r.Base
		}
		if base != "" && !slices.Contains(bases, base) {
			bases = append(bases, base)
		}
		if r == nil {
			continue
		}
		for _, lc := range r.Changes {
			for _, f := range lc.Files {
				if !slices.Contains(paths, f.Path) {
					paths = append(paths, f.Path)
				}
			}
		}
	}
	sort.Strings(paths)
	return bases, paths
}

// phaseChanges is a phase's combined diff: the given files, from the merge
// base of the items' bases (those still in the repository) to commit.
func (m *Mull) phaseChanges(cfg model.Config, bases, paths []string, commit string) ([]model.LayerChanges, string, error) {
	var known []string
	for _, b := range bases {
		if m.hasCommit(b) {
			known = append(known, b)
		}
	}
	if len(known) == 0 || len(paths) == 0 {
		return nil, "", nil
	}
	base := known[0]
	if len(known) > 1 {
		mb, err := m.repo.Output(nil, nil, append([]string{"merge-base", "--octopus"}, known...)...)
		if err != nil || mb == "" {
			return nil, "", nil // no common base: report no changes rather than fail
		}
		base = mb
	}
	changes, err := m.diffChanges(cfg, base, commit, paths)
	return changes, base, err
}

// fixUpTitle is a fix-up item's title from the reason its integration
// review was sent back: the reason's first line, cut to 80 characters at a
// word.
func fixUpTitle(reason string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(reason), "\n")
	line = strings.TrimSpace(line)
	const max = 80
	if utf8.RuneCountInString(line) <= max {
		return line
	}
	cut := string([]rune(line)[:max-1])
	if k := strings.LastIndex(cut, " "); k > max/2 {
		cut = cut[:k]
	}
	return strings.TrimRight(cut, " ,;:.") + "…"
}

// sendBackPhase settles integration review r as sent back: it adds a
// committed fix-up item to the phase, from the review, with the reason in
// its notes. The phase is open again until the fix-up is done and a new
// integration review of it is accepted.
func (m *Mull) sendBackPhase(s *state, r *model.Entry, in DoInput) (DoResult, error) {
	if in.As != "" && in.As != "item" {
		return DoResult{}, errf(CodeInvalid, "an integration review sends its phase back as a fix-up item; it can't become a phase")
	}
	e, err := s.phase(r.Phase)
	if err != nil {
		return DoResult{}, err
	}
	if (in.Phase != "" && in.Phase != e.ID) || (in.Plan != "" && in.Plan != e.Plan) {
		return DoResult{}, errf(CodeInvalid, "review %s is phase %s's integration review: its fix-up goes in that phase, so drop --plan and --phase", r.ID, e.ID)
	}
	if strings.TrimSpace(in.Reason) == "" {
		return DoResult{}, errf(CodeInvalid, "say what to fix (--reason): it becomes the fix-up item in the phase")
	}
	if r.Settled != nil && r.Settled.Action != model.ActPark {
		return DoResult{}, errf(CodeSettled, "inbox entry %s is already settled (%s)", r.ID, r.Settled.Action)
	}
	notes := fmt.Sprintf("Fix-up from the integration review of phase %s (%s), inbox %s:\n\n%s", e.ID, e.Title, r.ID, strings.TrimSpace(in.Reason))
	i, err := m.addItem(s, NewItem{Phase: e.ID, Title: fixUpTitle(in.Reason), Notes: notes, From: r.ID})
	if err != nil {
		return DoResult{}, err
	}
	if err := m.settle(s, r, model.ActDo, in.Reason, i.ID, in.SettleOpts); err != nil {
		return DoResult{}, err
	}
	v := s.itemView(i)
	return DoResult{Entry: r, Item: &v}, nil
}

// acceptPhaseReview settles an integration review as accepted. The worktree
// that filed it may not accept it unless the human asked.
func (m *Mull) acceptPhaseReview(s *state, r *model.Entry, o SettleOpts) error {
	if r.By == m.Worktree() && !m.asked(o.OnRequest) {
		return errf(CodeNotAllowed, "integration review %s was filed from this worktree; it can't be accepted from here unless the human asks (use --on-request)", r.ID)
	}
	return m.settle(s, r, model.ActAccept, "", "", o)
}

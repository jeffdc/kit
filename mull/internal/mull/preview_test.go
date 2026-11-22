package mull

import (
	"strings"
	"testing"

	"mull/internal/model"
)

func TestPlanPreviewSet(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "site", "The site", "")

	r := must[SetResult](t)(m.Set("site", "preview", "  http://localhost:4100  "))
	if r.Plan.Preview != "http://localhost:4100" {
		t.Fatalf("preview = %q, want it trimmed", r.Plan.Preview)
	}
	for _, bad := range []string{"localhost:4100", "/trees", "ftp://host", "http://", "not a url", "https:///x"} {
		_, err := m.Set("site", "preview", bad)
		wantCode(t, err, CodeInvalid)
	}
	if p := must[PlanView](t)(m.PlanShow("site")); p.Preview != "http://localhost:4100" {
		t.Errorf("a refused preview changed it: %q", p.Preview)
	}

	// The batch op is plan set, as for target.
	must[[]BatchResult](t)(m.Batch(strings.NewReader(`{"op": "plan set", "slug": "site", "field": "preview", "value": "https://site.example.com/base"}`), false))
	if p := must[PlanView](t)(m.PlanShow("site")); p.Preview != "https://site.example.com/base" {
		t.Errorf("plan set: preview %q", p.Preview)
	}

	// A phase has no preview.
	phase := must[*model.Phase](t)(m.PhaseAdd("site", "Pages", "", nil))
	_, err := m.Set(phase.ID, "preview", "http://localhost:4100")
	wantCode(t, err, CodeInvalid)

	// "" clears it.
	must[SetResult](t)(m.Set("site", "preview", ""))
	if p := must[PlanView](t)(m.PlanShow("site")); p.Preview != "" {
		t.Errorf("cleared preview = %q", p.Preview)
	}
}

func TestContextHandsOverTargetAndPreview(t *testing.T) {
	m, _ := setup(t)
	agreedPlan(t, m, "site", "The site", "")
	agreedPlan(t, m, "tools", "Tools", "")
	must[SetResult](t)(m.Set("site", "target", "site-shell"))
	must[SetResult](t)(m.Set("site", "preview", "http://localhost:4100"))
	site := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "site", Title: "A page"}))
	tools := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "tools", Title: "A tool"}))
	loose := must[ItemView](t)(m.ItemAdd(NewItem{Plan: "none", Title: "Loose"}))

	c := must[Context](t)(m.Context(site.ID))
	if tg := c.Item.Target; tg.Branch != "site-shell" || tg.From != TargetFromPlan || tg.Found {
		t.Errorf("item target = %+v, want site-shell from the plan, not found yet", tg)
	}
	if c.Item.Preview != "http://localhost:4100" {
		t.Errorf("item preview = %q", c.Item.Preview)
	}
	for _, p := range c.Plans {
		want := map[string]string{"site": "http://localhost:4100", "tools": ""}[p.Slug]
		if p.Preview != want {
			t.Errorf("plan %s preview = %q, want %q", p.Slug, p.Preview, want)
		}
	}

	// A plan without one: the target falls back to integration, no preview.
	for _, id := range []string{tools.ID, loose.ID} {
		c = must[Context](t)(m.Context(id))
		if tg := c.Item.Target; tg.Branch != "main" || tg.From != TargetFromIntegration || !tg.Found {
			t.Errorf("%s target = %+v, want main from integration", id, tg)
		}
		if c.Item.Preview != "" {
			t.Errorf("%s preview = %q, want none", id, c.Item.Preview)
		}
	}

	// The view shows the preview under the target.
	v := must[View](t)(m.Board("site"))
	if v.Plans[0].Preview != "http://localhost:4100" || !strings.Contains(v.Text(m.Now()), "  preview http://localhost:4100\n") {
		t.Errorf("view preview %q, text:\n%s", v.Plans[0].Preview, v.Text(m.Now()))
	}
}

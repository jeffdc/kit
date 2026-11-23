package core

import (
	"strings"
	"testing"

	"mull/internal/gittest"
)

func TestMapAndNewConnections(t *testing.T) {
	m, dir := riskSetup(t)
	gittest.Commit(t, dir, "edges.txt", "web -> core\ncore,db\n# a comment\n", "Edges")
	gittest.Commit(t, dir, "allowed.txt", "web -> core\n", "Declared")
	_, err := m.ConfigMap("cat edges.txt", "cat allowed.txt", false)
	wantCode(t, err, CodeNotAllowed)
	must[Config](t)(m.ConfigMap("cat edges.txt", "cat allowed.txt", true))

	mp := must[MapOut](t)(m.Map(false, false))
	if !mp.Configured || strings.Join(mp.Edges, ";") != "core -> db;web -> core" || strings.Join(mp.Undeclared, ";") != "core -> db" {
		t.Fatalf("map = %+v", mp)
	}
	_, err = m.Map(true, false)
	wantCode(t, err, CodeNotAllowed)
	must[MapOut](t)(m.Map(true, true))

	// A change adding a connection is high risk, and accepting it records it.
	it := startWork(t, m, "Web reads the db")
	gittest.Commit(t, dir, "edges.txt", "web -> core\ncore -> db\nweb -> db\n", "Shortcut", "Mull: "+it.ID)
	res := must[DoneOut](t)(m.Done(it.ID, DoneInput{Summary: "Shortcut.", Rules: "none"}))
	if res.Landed || res.Report.Risk != RiskHigh || strings.Join(res.Report.NewEdges, ";") != "web -> db" {
		t.Fatalf("done = %+v", res.Report)
	}
	if v := must[Surface](t)(m.Surface("")); len(v.NewEdges) != 1 {
		t.Fatalf("surface new edges = %v", v.NewEdges)
	}
	must[AcceptResult](t)(m.Accept([]string{it.ID}, true))
	if mp = must[MapOut](t)(m.Map(false, false)); len(mp.New) != 0 {
		t.Fatalf("after accept, new = %v", mp.New)
	}
}

func TestAudit(t *testing.T) {
	m, _ := setup(t)
	_, err := m.RecordAudit("  ")
	wantCode(t, err, CodeInvalid)
	a := must[AuditOut](t)(m.RecordAudit("Components are slipping: three dialogs.\n\n## Grades\n- web: slipping"))
	if a.At == "" || !strings.HasPrefix(a.Text, "Audited: ") {
		t.Fatalf("audit = %+v", a)
	}
	v := must[Surface](t)(m.Surface(""))
	if v.Audit == nil || !strings.HasPrefix(v.Audit.Summary, "Components are slipping") || v.Audit.Stale {
		t.Fatalf("surface audit = %+v", v.Audit)
	}
}

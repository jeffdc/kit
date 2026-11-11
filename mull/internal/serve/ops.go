package serve

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"mull/internal/mull"
	"mull/internal/seen"
)

// uiOps are the ops the UI offers. Each step of editing from the UI adds the
// ops it needs; anything else is refused.
var uiOps = map[string]bool{
	"accept": true, // approve decisions
	"decide": true, // answer a question, or decide a decision differently
	"do":     true, // send reviewed work back
	"drop":   true, // drop a Needs you entry
	"park":   true, // set one aside
	// close a plan, which archives it
	"plan close": true,
}

// OpsReport is what POST …/ops returns: each op's result, as mull batch
// prints it, the mull branch commit written ("" if nothing changed), and
// the records it changed, which the human has now acted on and so aren't
// clouded. AckError says why they couldn't be recorded as acted on: the
// write stands, and they stay clouded.
type OpsReport struct {
	Results      []mull.BatchResult `json:"results"`
	Commit       string             `json:"commit"`
	Acknowledged []mull.RecordRef   `json:"acknowledged"`
	AckError     *mull.Error        `json:"ack_error,omitempty"`
}

// maxOpsBody bounds a write from the page (decisions and reasons are short).
const maxOpsBody = 1 << 20

// registerOps adds the one endpoint a page writes mull data through. The
// write is the human's own: recorded by their name, not a worktree's, and
// never as an agent's on request.
func registerOps(mux *http.ServeMux, repos Repos) {
	mux.HandleFunc("POST /api/projects/{id}/ops", func(w http.ResponseWriter, r *http.Request) {
		if status, e := writeAllowed(r); e != nil {
			writeError(w, status, e)
			return
		}
		var body struct {
			Commit string            `json:"commit"` // the revision the page showed
			Ops    []json.RawMessage `json:"ops"`    // batch lines
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, maxOpsBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, &mull.Error{Code: mull.CodeInvalid, Msg: `want a JSON body {"commit": "<mull branch commit shown>", "ops": [{"op": "<name>", ...fields}]}: ` + err.Error()})
			return
		}
		if e := offered(body.Ops); e != nil {
			writeMullError(w, e)
			return
		}
		path, err := resolve(repos, r.PathValue("id"))
		if err != nil {
			writeMullError(w, err)
			return
		}
		m, err := mull.Open(path)
		if err != nil {
			writeMullError(w, err)
			return
		}
		if m, err = m.AsHuman(); err != nil {
			writeMullError(w, err)
			return
		}
		results, commit, err := m.Apply(body.Ops, body.Commit)
		if err != nil {
			writeMullError(w, err)
			return
		}
		rep := OpsReport{Results: results, Commit: commit, Acknowledged: []mull.RecordRef{}}
		if commit != "" {
			if rep.Acknowledged, err = acknowledge(m, path, commit); err != nil {
				rep.Acknowledged, rep.AckError = []mull.RecordRef{}, mull.AsError(err)
			}
		}
		writeJSON(w, http.StatusOK, rep)
	})
}

// acknowledge records every record commit changed as acted on by the
// human, so what they just did isn't clouded as news, and returns them.
func acknowledge(m *mull.Mull, path, commit string) ([]mull.RecordRef, error) {
	refs, err := m.Wrote(commit)
	if err != nil {
		return nil, err
	}
	acks := make([]seen.Ack, 0, len(refs))
	for _, r := range refs {
		acks = append(acks, seen.Ack{Type: r.Type, ID: r.ID, Commit: commit})
	}
	return refs, seen.Acknowledge(path, acks)
}

// offered refuses ops the UI doesn't offer, and on-request: a write from
// the page is the human's own, not an agent's on their say. Malformed ops
// are left to mull, which reports them per line.
func offered(ops []json.RawMessage) error {
	for _, raw := range ops {
		var head struct {
			Op        string          `json:"op"`
			OnRequest json.RawMessage `json:"on-request"`
		}
		if json.Unmarshal(raw, &head) != nil {
			continue
		}
		if head.Op != "" && !uiOps[head.Op] {
			return &mull.Error{Code: mull.CodeNotAllowed, Msg: "mull serve doesn't offer op " + head.Op + " (it offers: " + strings.Join(offeredOps(), ", ") + ")"}
		}
		if head.OnRequest != nil {
			return &mull.Error{Code: mull.CodeInvalid, Msg: "a write from mull serve is the human's own: leave out on-request, which is for an agent acting on the human's say"}
		}
	}
	return nil
}

func offeredOps() []string {
	out := make([]string, 0, len(uiOps))
	for op := range uiOps {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

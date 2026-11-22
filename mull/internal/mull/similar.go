package mull

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"mull/internal/model"
)

// Similar is an open item whose title overlaps a new capture's: a hint that
// the capture may be a near-duplicate, never a block.
type Similar struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// CaptureView is what bug and idea print: the captured item, and the open
// items whose titles overlap its own.
type CaptureView struct {
	ItemView
	Similar []Similar `json:"similar"`
}

// The most similar items a capture lists, and how much two titles must
// overlap (weighted Dice, 0 to 1) to count: similarThreshold, or for titles
// sharing at least longShared words, longThreshold. Long titles about the
// same thing share many words but also carry many of their own, which a
// single threshold either misses or matches short titles on two words.
const (
	maxSimilar       = 5
	similarThreshold = 0.5
	longThreshold    = 0.35
	longShared       = 4
)

// captureView is the capture's ItemView with the items similar to it.
func (s *state) captureView(i *model.Item) CaptureView {
	return CaptureView{ItemView: s.itemView(i), Similar: s.similar(i)}
}

// similar lists the captured and committed items, other than i, whose titles
// overlap i's a lot in their words, most similar first.
//
// Words are weighted by rarity among those titles (and i's), so words most
// titles share, like a project's "mull serve:" prefix, count for little.
// Two titles are similar when they share two words (or one, when either
// title has only one) and their weighted Dice overlap is at least
// similarThreshold, or longThreshold when they share longShared words.
func (s *state) similar(i *model.Item) []Similar {
	out := []Similar{}
	mine := titleWords(i.Title)
	if len(mine) == 0 {
		return out
	}
	type cand struct {
		item  *model.Item
		words map[string]bool
	}
	var cands []cand
	df := map[string]int{}
	for w := range mine {
		df[w]++
	}
	for _, id := range sortedKeys(s.items) {
		o := s.items[id]
		if o.ID == i.ID || (o.State != model.StateCaptured && o.State != model.StateCommitted) {
			continue
		}
		ws := titleWords(o.Title)
		if len(ws) == 0 {
			continue
		}
		cands = append(cands, cand{o, ws})
		for w := range ws {
			df[w]++
		}
	}
	n := float64(len(cands) + 1)
	weight := func(w string) float64 { return math.Log(1 + n/float64(df[w])) }
	sum := func(ws map[string]bool) float64 {
		t := 0.0
		for w := range ws {
			t += weight(w)
		}
		return t
	}
	mySum := sum(mine)
	type hit struct {
		item  *model.Item
		score float64
	}
	var hits []hit
	for _, c := range cands {
		shared, both := 0, 0.0
		for w := range mine {
			if c.words[w] {
				shared++
				both += weight(w)
			}
		}
		if shared < min(2, len(mine), len(c.words)) {
			continue
		}
		score := 2 * both / (mySum + sum(c.words))
		if score >= similarThreshold || (shared >= longShared && score >= longThreshold) {
			hits = append(hits, hit{c.item, score})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	for _, h := range hits[:min(len(hits), maxSimilar)] {
		out = append(out, Similar{ID: h.item.ID, Title: h.item.Title})
	}
	return out
}

// titleWords is a title's words for comparing: lower case, split on anything
// but letters and digits, with common short words and single letters left
// out and a plural s trimmed.
func titleWords(title string) map[string]bool {
	ws := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if stopWords[w] || (len(w) == 1 && !unicode.IsDigit(rune(w[0]))) {
			continue // a, the, and the s of a possessive
		}
		w = singular(w)
		ws[w] = true
	}
	return ws
}

// singular trims a plural ending: crashes and classes to crash and class,
// tests to test. Words of three letters or fewer, and -ss, are left alone.
func singular(w string) string {
	if len(w) <= 3 || !strings.HasSuffix(w, "s") || strings.HasSuffix(w, "ss") {
		return w
	}
	for _, end := range []string{"sses", "shes", "ches", "xes", "zes"} {
		if strings.HasSuffix(w, end) {
			return w[:len(w)-2]
		}
	}
	return w[:len(w)-1]
}

var stopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an the and or but nor of to in on at by for from with without into onto
		as is are was were be been being it its this that these those there here
		not no do does did can cannot will should would could may might must
		when if then than so up out over under after before about all any each every some
		i we you he she they me us him her them my our your their
		what which who whom whose how why where just only also very too more most such via per`) {
		stopWords[w] = true
	}
}

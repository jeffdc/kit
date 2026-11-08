package storage

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"forage/internal/model"
)

func TestMigrationSeedsReadsFromDateRead(t *testing.T) {
	dir := legacyDB(t) // a1b2: status read, no date
	db, err := sql.Open("sqlite", filepath.Join(dir, "forage.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO books (id, title, author, status, date_added, date_read) VALUES
		('c3d4', 'Babel', 'R. F. Kuang', 'read', '2024-01-01', '2024-03-05'),
		('e5f6', 'Piranesi', 'Susanna Clarke', 'wishlist', '2024-01-01', '')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	cases := map[string][]string{"a1b2": {""}, "c3d4": {"2024-03-05"}, "e5f6": nil}
	for id, want := range cases {
		reads, err := s.ReadsForBook(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(reads) != len(want) {
			t.Fatalf("book %s: %d reads, want %d", id, len(reads), len(want))
		}
		for i := range want {
			if reads[i].DateFinished != want[i] || !ValidID(reads[i].ID) || len(reads[i].ID) != 12 {
				t.Errorf("book %s read %d = %+v, want date %q", id, i, reads[i], want[i])
			}
		}
	}

	b, _ := s.GetBook("c3d4")
	if b.DateRead != "2024-03-05" {
		t.Errorf("derived DateRead = %q", b.DateRead)
	}
	c, err := s.ChangesSince(0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Rev != 1 || c.Epoch == 0 || len(c.Books) != 3 || len(c.Reads) != 2 {
		t.Errorf("after migration: rev=%d epoch=%d books=%d reads=%d", c.Rev, c.Epoch, len(c.Books), len(c.Reads))
	}
}

func TestEveryWriteBumpsRevAndShowsInFeed(t *testing.T) {
	s := testStore(t)
	start, _ := s.ChangesSince(0)

	b, _ := s.CreateBook("Dune", "Frank Herbert", nil)
	other, _ := s.CreateBook("Emma", "Jane Austen", nil)
	afterCreate, _ := s.ChangesSince(start.Rev)
	if afterCreate.Full || len(afterCreate.Books) != 2 || afterCreate.Rev != start.Rev+2 {
		t.Fatalf("after creates: %+v", afterCreate)
	}

	s.UpdateBook(b.ID, map[string]string{"rating": "4"})
	r, err := s.CreateRead(model.Read{ID: "aaaabbbbcccc", BookID: b.ID, DateFinished: "2026-11-01"})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.ChangesSince(afterCreate.Rev)
	if len(c.Books) != 1 || c.Books[0].ID != b.ID || c.Books[0].Rating != 4 || len(c.Reads) != 1 {
		t.Fatalf("incremental feed: %+v", c)
	}
	if c.Books[0].DateRead != "2026-11-01" {
		t.Errorf("DateRead in feed = %q", c.Books[0].DateRead)
	}

	// Deleting a book tombstones it and its reads.
	if err := s.DeleteBook(b.ID); err != nil {
		t.Fatal(err)
	}
	d, _ := s.ChangesSince(c.Rev)
	if len(d.Books) != 1 || !d.Books[0].Deleted || len(d.Reads) != 1 || d.Reads[0].ID != r.ID || !d.Reads[0].Deleted {
		t.Fatalf("delete feed: %+v", d)
	}
	if _, err := s.GetBook(b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetBook after delete: %v", err)
	}
	if err := s.DeleteBook(b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}

	// A full snapshot leaves tombstones out.
	full, _ := s.ChangesSince(0)
	if !full.Full || len(full.Books) != 1 || full.Books[0].ID != other.ID || len(full.Reads) != 0 {
		t.Errorf("full snapshot: %+v", full)
	}
	// A client ahead of the server (e.g. after a restore) gets a full snapshot.
	if ahead, _ := s.ChangesSince(full.Rev + 100); !ahead.Full {
		t.Error("since > rev should return a full snapshot")
	}
	// Nothing changed: an empty, non-full feed.
	if same, _ := s.ChangesSince(full.Rev); same.Full || len(same.Books)+len(same.Reads) != 0 {
		t.Errorf("no-change feed: %+v", same)
	}
}

func TestUpdateBookIsAtomic(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Dune", "Frank Herbert", nil)
	before, _ := s.ChangesSince(0)

	for _, fields := range []map[string]string{
		{"title": "Changed", "bogus": "x"},
		{"title": "Changed", "rating": "nine"},
		{"title": "Changed", "status": "finished"},
		{"title": "  "},
		{},
	} {
		if _, err := s.UpdateBook(b.ID, fields); !errors.Is(err, ErrInvalid) {
			t.Errorf("UpdateBook(%v): err = %v, want ErrInvalid", fields, err)
		}
	}
	got, _ := s.GetBook(b.ID)
	after, _ := s.ChangesSince(0)
	if got.Title != "Dune" || after.Rev != before.Rev {
		t.Errorf("failed updates changed something: title=%q rev %d -> %d", got.Title, before.Rev, after.Rev)
	}
}

func TestReadValidation(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Dune", "Frank Herbert", nil)

	if _, err := s.CreateRead(model.Read{ID: "aaaabbbbcccc", BookID: "ffffffffffff"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("read for missing book: %v", err)
	}
	if _, err := s.CreateRead(model.Read{ID: "aaaabbbbcccc", BookID: b.ID, DateFinished: "2026-13-01"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad date: %v", err)
	}
	if _, err := s.CreateRead(model.Read{ID: "aaaabbbbcccc", BookID: b.ID}); err != nil {
		t.Fatalf("undated read: %v", err)
	}
	if _, err := s.CreateRead(model.Read{ID: "aaaabbbbcccc", BookID: b.ID}); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate read id: %v", err)
	}
	if _, err := s.UpdateRead("aaaabbbbcccc", map[string]string{"date_finished": "2026-11-08", "date_started": "2026-10-01"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRead("aaaabbbbcccc", map[string]string{"rating": "3"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown read field: %v", err)
	}
	reads, _ := s.ReadsForBook(b.ID)
	if len(reads) != 1 || reads[0].DateFinished != "2026-11-08" || reads[0].DateStarted != "2026-10-01" {
		t.Errorf("reads = %+v", reads)
	}
	if err := s.DeleteRead("aaaabbbbcccc"); err != nil {
		t.Fatal(err)
	}
	if reads, _ := s.ReadsForBook(b.ID); len(reads) != 0 {
		t.Errorf("deleted read still listed: %+v", reads)
	}
}

func TestSetLatestReadDate(t *testing.T) {
	s := testStore(t)
	b, _ := s.CreateBook("Dune", "Frank Herbert", nil)

	if err := s.SetLatestReadDate(b.ID, "2020-01-01"); err != nil {
		t.Fatal(err)
	}
	s.CreateRead(model.Read{ID: "aaaabbbbcccc", BookID: b.ID, DateFinished: "2024-06-01"})
	if err := s.SetLatestReadDate(b.ID, "2024-06-02"); err != nil {
		t.Fatal(err)
	}
	reads, _ := s.ReadsForBook(b.ID)
	if len(reads) != 2 || reads[0].DateFinished != "2020-01-01" || reads[1].DateFinished != "2024-06-02" {
		t.Errorf("reads = %+v", reads)
	}
}

func TestApplyOpIsIdempotent(t *testing.T) {
	s := testStore(t)
	op := model.Op{OpID: "op-1", Type: "book.create", Book: &model.Book{ID: "a1b2c3d4e5f6", Title: "Dune", Author: "Frank Herbert"}}

	first, err := s.ApplyOp(op)
	if err != nil || first.Status != "ok" || first.Rev == 0 {
		t.Fatalf("first apply: %+v %v", first, err)
	}
	before, _ := s.ChangesSince(0)
	again, err := s.ApplyOp(op)
	if err != nil || again != first {
		t.Fatalf("replay: %+v %v, want %+v", again, err, first)
	}
	after, _ := s.ChangesSince(0)
	if after.Rev != before.Rev {
		t.Errorf("replay bumped rev %d -> %d", before.Rev, after.Rev)
	}

	b, _ := s.GetBook("a1b2c3d4e5f6")
	if b.Status != "wishlist" || b.SortAuthor != "Herbert, Frank" || b.DateAdded == "" {
		t.Errorf("defaults not applied: %+v", b)
	}
}

func TestApplyOpRejections(t *testing.T) {
	s := testStore(t)
	s.ApplyOp(model.Op{OpID: "seed", Type: "book.create", Book: &model.Book{ID: "a1b2c3d4e5f6", Title: "Dune", Author: "Frank Herbert"}})
	before, _ := s.ChangesSince(0)

	for _, op := range []model.Op{
		{OpID: "", Type: "book.delete", ID: "a1b2c3d4e5f6"},
		{OpID: "x1", Type: "book.create", Book: &model.Book{ID: "a1b2c3d4e5f6", Title: "Other", Author: "Someone"}},
		{OpID: "x2", Type: "book.create"},
		{OpID: "x3", Type: "book.update", ID: "ffffffffffff", Fields: map[string]any{"rating": 3.0}},
		{OpID: "x4", Type: "book.update", ID: "a1b2c3d4e5f6", Fields: map[string]any{"status": "gone"}},
		{OpID: "x5", Type: "read.create", Read: &model.Read{ID: "bbbbbbbbbbbb", BookID: "ffffffffffff"}},
		{OpID: "x6", Type: "read.delete", ID: "bbbbbbbbbbbb"},
		{OpID: "x7", Type: "book.explode"},
	} {
		res, err := s.ApplyOp(op)
		if err != nil {
			t.Errorf("%s: unexpected error %v", op.OpID, err)
		}
		if res.Status != "rejected" || res.Error == "" || res.OpID != op.OpID {
			t.Errorf("%s: %+v, want rejected", op.OpID, res)
		}
	}
	after, _ := s.ChangesSince(0)
	if after.Rev != before.Rev {
		t.Errorf("rejected ops bumped rev %d -> %d", before.Rev, after.Rev)
	}
}

func TestApplyOpReadLifecycle(t *testing.T) {
	s := testStore(t)
	steps := []model.Op{
		{OpID: "1", Type: "book.create", Book: &model.Book{ID: "a1b2c3d4e5f6", Title: "Dune", Author: "Frank Herbert", Tags: []string{" sci-fi ", ""}}},
		{OpID: "2", Type: "book.update", ID: "a1b2c3d4e5f6", Fields: map[string]any{"status": "read", "rating": 5.0, "tags": []any{"sci-fi", "classic"}}},
		{OpID: "3", Type: "read.create", Read: &model.Read{ID: "bbbbbbbbbbbb", BookID: "a1b2c3d4e5f6", DateFinished: "2026-11-08"}},
		{OpID: "4", Type: "read.update", ID: "bbbbbbbbbbbb", Fields: map[string]any{"date_finished": "2026-11-07"}},
	}
	for _, op := range steps {
		if res, err := s.ApplyOp(op); err != nil || res.Status != "ok" {
			t.Fatalf("op %s: %+v %v", op.OpID, res, err)
		}
	}
	b, _ := s.GetBook("a1b2c3d4e5f6")
	if b.Status != "read" || b.Rating != 5 || b.DateRead != "2026-11-07" || len(b.Tags) != 2 || b.Tags[1] != "classic" {
		t.Errorf("book = %+v", b)
	}
	if res, _ := s.ApplyOp(model.Op{OpID: "5", Type: "read.delete", ID: "bbbbbbbbbbbb"}); res.Status != "ok" {
		t.Errorf("read.delete: %+v", res)
	}
	if res, _ := s.ApplyOp(model.Op{OpID: "6", Type: "book.delete", ID: "a1b2c3d4e5f6"}); res.Status != "ok" {
		t.Errorf("book.delete: %+v", res)
	}
}

func TestFieldString(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""}, {"x", "x"}, {4.0, "4"}, {1965.0, "1965"}, {2.5, "2.5"}, {true, "true"},
		{[]any{"a", "b"}, "a,b"}, {[]any{}, ""},
	}
	for _, c := range cases {
		if got := FieldString(c.in); got != c.want {
			t.Errorf("FieldString(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

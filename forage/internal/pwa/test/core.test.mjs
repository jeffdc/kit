// Tests for the PWA's pure logic. Run: node --test internal/pwa/test/
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  newId, newOpId, localToday, formatDate, authorSortKey, overlay, sinceFor, planChanges,
  settleResults, fold, search, wishlistBooks, readingBooks, historyGroups, initial,
  orderSellers, sellerURL, shelfCounts, undatedReads, plausibleDate,
} from "../assets/js/core.js";

const book = (id, title, author, extra = {}) => ({
  id, title, author, status: "wishlist", sort_author: authorSortKey(author), date_added: "2026-01-01", ...extra,
});
const read = (id, book_id, date_finished, extra = {}) => ({ id, book_id, date_finished, updated_at: "2026-01-01T00:00:00Z", ...extra });
const server = (books, reads = []) => ({
  books: new Map(books.map((b) => [b.id, b])),
  reads: new Map(reads.map((r) => [r.id, r])),
});

test("ids and dates", () => {
  assert.match(newId(), /^[0-9a-f]{12}$/);
  assert.notEqual(newOpId(), newOpId());
  assert.equal(localToday(new Date(2026, 10, 8, 23, 30)), "2026-11-08");
  assert.equal(formatDate("2025-11-09"), "Nov 9, 2025");
  assert.equal(formatDate("2025-11-09", false), "Nov 9");
  assert.equal(formatDate(""), "");
  assert.equal(authorSortKey("Ursula K. Le Guin"), "Guin, Ursula K. Le");
  assert.equal(authorSortKey("qntm"), "qntm");
});

test("overlay applies pending ops without touching server state", () => {
  const s = server([book("a1", "Dune", "Frank Herbert")], [read("r1", "a1", "2019-05-01")]);
  const ops = [
    { type: "book.update", id: "a1", fields: { rating: 5, author: "F. Herbert" } },
    { type: "book.create", book: { id: "b2", title: "Emma", author: "Jane Austen" } },
    { type: "read.create", read: { id: "r2", book_id: "b2", date_finished: "2026-11-08" } },
    { type: "read.create", read: { id: "r3", book_id: "missing", date_finished: "2026-11-08" } },
  ];
  const v = overlay(s, ops);
  assert.equal(v.books.get("a1").rating, 5);
  assert.equal(v.books.get("a1").sort_author, "Herbert, F.");
  assert.equal(v.books.get("a1").pending, true);
  assert.equal(v.books.get("a1").date_read, "2019-05-01");
  assert.equal(v.books.get("b2").status, "wishlist");
  assert.equal(v.books.get("b2").date_read, "2026-11-08");
  assert.equal(v.reads.has("r3"), false, "a read for an unknown book is ignored");
  assert.equal(v.readsByBook.has("missing"), false);
  assert.equal(s.books.get("a1").rating, undefined, "server state untouched");
  assert.equal(s.books.has("b2"), false);
});

test("overlay: deleting a book drops its reads; reads sort oldest first", () => {
  const s = server(
    [book("a1", "Dune", "Frank Herbert"), book("b2", "Emma", "Jane Austen")],
    [read("r1", "a1", "2020-01-01"), read("r2", "b2", "2024-02-02"), read("r3", "b2", ""), read("r4", "b2", "2021-03-03")],
  );
  const v = overlay(s, [{ type: "book.delete", id: "a1" }, { type: "read.update", id: "r4", fields: { date_finished: "2025-01-01" } }]);
  assert.equal(v.books.has("a1"), false);
  assert.equal(v.reads.has("r1"), false);
  assert.deepEqual(v.readsByBook.get("b2").map((r) => r.date_finished), ["", "2024-02-02", "2025-01-01"]);
  assert.equal(v.books.get("b2").date_read, "2025-01-01");
});

test("an unchanged author keeps a manual sort key", () => {
  const s = server([book("a1", "Babel", "R. F. Kuang", { sort_author: "Kuang, R.F." })]);
  const v = overlay(s, [{ type: "book.update", id: "a1", fields: { author: "R. F. Kuang", title: "Babel" } }]);
  assert.equal(v.books.get("a1").sort_author, "Kuang, R.F.");
});

test("sync planning", () => {
  assert.equal(sinceFor(null), 0);
  assert.equal(sinceFor({ rev: 10 }), 0, "no epoch yet means full");
  assert.equal(sinceFor({ rev: 10, epoch: 7 }), 10);

  assert.deepEqual(planChanges({ rev: 10, epoch: 7 }, { full: false, epoch: 8, rev: 3, books: [], reads: [] }), { refetchFull: true });

  const p = planChanges({ rev: 10, epoch: 7 }, {
    full: false, epoch: 7, rev: 12,
    books: [{ id: "a1", title: "x" }, { id: "b2", deleted: true }],
    reads: [{ id: "r1" }, { id: "r2", deleted: true }],
  });
  assert.equal(p.replaceAll, false);
  assert.deepEqual(p.putBooks.map((b) => b.id), ["a1"]);
  assert.deepEqual(p.deleteBooks, ["b2"]);
  assert.deepEqual(p.putReads.map((r) => r.id), ["r1"]);
  assert.deepEqual(p.deleteReads, ["r2"]);
  assert.equal(p.rev, 12);

  assert.equal(planChanges(null, { full: true, epoch: 9, rev: 1, books: [], reads: [] }).replaceAll, true);
});

test("settling op results", () => {
  const sent = [{ op_id: "1" }, { op_id: "2" }, { op_id: "3" }];
  const { done, rejected } = settleResults(sent, [
    { op_id: "1", status: "ok" },
    { op_id: "2", status: "rejected", error: "invalid: rating 9" },
  ]);
  assert.deepEqual(done, ["1", "2"]);
  assert.equal(rejected.length, 1);
  assert.equal(rejected[0].error, "invalid: rating 9");
});

test("search", () => {
  const books = [
    book("1", "The Left Hand of Darkness", "Ursula K. Le Guin", { status: "read" }),
    book("2", "Darkness at Noon", "Arthur Koestler"),
    book("3", "Cien años de soledad", "Gabriel García Márquez"),
    book("4", "Dune", "Frank Herbert", { isbn: "9780441172719" }),
    book("5", "Piranesi", "Susanna Clarke"),
    book("6", "Thank You for Arguing", "Jay Heinrichs"),
  ];
  assert.deepEqual(search(books, "guin").map((b) => b.id), ["1"], "words match from their start, not inside other words");
  assert.deepEqual(search(books, "dark").map((b) => b.id), ["2", "1"], "title prefix first, then word match");
  assert.deepEqual(search(books, "left hand").map((b) => b.id), ["1"]);
  assert.deepEqual(search(books, "marquez").map((b) => b.id), ["3"], "accents folded");
  assert.deepEqual(search(books, "anos soledad").map((b) => b.id), ["3"]);
  assert.deepEqual(search(books, "le guin").map((b) => b.id), ["1"]);
  assert.deepEqual(search(books, "978-0-441-17271-9").map((b) => b.id), ["4"]);
  assert.deepEqual(search(books, "  "), []);
  assert.deepEqual(search(books, "zzz"), []);
  assert.equal(fold("Tom & Jerry!"), "tom and jerry");
});

test("tab lists", () => {
  const s = server(
    [
      book("1", "Pastoral", "André Alexis"),
      book("2", "Ring", "André Alexis", { status: "to_read", owned: true }),
      book("3", "The Genius of Birds", "Jennifer Ackerman"),
      book("4", "Dune", "Frank Herbert", { status: "reading" }),
      book("5", "Emma", "Jane Austen", { status: "paused" }),
      book("6", "Babel", "R. F. Kuang", { status: "read", rating: 5 }),
      book("7", "Nope", "Some One", { status: "dropped" }),
    ],
    [read("r1", "6", "2025-03-01"), read("r2", "6", "2022-01-01"), read("r3", "6", ""), read("r4", "4", "2019-01-01")],
  );
  const v = overlay(s, []);
  assert.deepEqual(wishlistBooks(v).map((b) => b.id), ["3", "1"], "by surname; books to read are not on the wishlist");
  assert.deepEqual(wishlistBooks(v, "title").map((b) => b.id), ["3", "1"], "genius, pastoral");
  assert.equal(initial(v.books.get("3"), "title"), "G", "leading article ignored");
  const r = readingBooks(v);
  assert.deepEqual([r.reading.map((b) => b.id), r.paused.map((b) => b.id), r.toRead.map((b) => b.id)], [["4"], ["5"], ["2"]]);
  assert.deepEqual(shelfCounts(v), { wishlist: 2, reading: 3, history: 1, undated: 1 }, "Reading counts paused and to-read books");
  assert.deepEqual(undatedReads(v).map((e) => e.read.id), ["r3"]);
  assert.ok(plausibleDate("2025-11-09") && plausibleDate(""));
  assert.ok(!plausibleDate("0202-11-09") && !plausibleDate("2025-11"), "a year still being typed isn't saved");

  // Undated reads list the most recently added book first.
  s.books.set("8", book("8", "Older", "A. Writer", { status: "read", date_added: "2020-01-01" }));
  s.books.set("9", book("9", "Newer", "B. Writer", { status: "read", date_added: "2026-06-01" }));
  s.reads.set("r8", read("r8", "8", "")); s.reads.set("r9", read("r9", "9", ""));
  const groups = historyGroups(overlay(s, []));
  assert.deepEqual(groups.map((g) => g.label), ["2025", "2022", "2019", "Undated"]);
  assert.equal(groups[0].entries[0].book.id, "6");
  // Book 6 (Babel) was added 2026-01-01 by the helper: between the two.
  assert.deepEqual(groups[3].entries.map((e) => e.book.id), ["9", "6", "8"], "undated: newest added first");
  const flat = historyGroups(overlay(s, []), "author");
  assert.equal(flat.length, 1);
  assert.deepEqual(flat[0].entries.map((e) => e.book.id), ["4", "6", "8", "9"], "one entry per book, by surname");
  assert.equal(flat[0].entries[1].read.date_finished, "2025-03-01", "latest read shown");
});

test("sellers: used first, Amazon last; ISBN where the URL wants it", () => {
  const sellers = [
    { name: "Amazon", url: "https://a.example/s?k={query}" },
    { name: "Biblio", url: "https://b.example/search.php?keyisbn={query}" },
    { name: "Better World Books", url: "https://bwb.example/?q={query}" },
  ];
  assert.deepEqual(orderSellers(sellers).map((s) => s.name), ["Better World Books", "Biblio", "Amazon"]);
  const b = book("1", "Piranesi", "Susanna Clarke", { isbn: "9781635577808" });
  assert.equal(sellerURL(sellers[0], b), "https://a.example/s?k=Piranesi%20Susanna%20Clarke");
  assert.equal(sellerURL(sellers[1], b), "https://b.example/search.php?keyisbn=9781635577808");
});

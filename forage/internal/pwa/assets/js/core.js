// Pure logic shared by the app: ids and dates, the pending-change overlay,
// sync planning, search, sorting and grouping. No DOM, no IndexedDB, no
// network, so it runs (and is tested) under Node as well as in the browser.

export const STATUSES = ["wishlist", "owned", "reading", "paused", "read", "dropped"];

export const STATUS_LABEL = {
  wishlist: "Wishlist", owned: "Owned", reading: "Reading",
  paused: "Paused", read: "Read", dropped: "Dropped",
};

// --- IDs and dates ---

export function randomHex(bytes) {
  const b = new Uint8Array(bytes);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

/** A new 12-hex-character book or read ID. */
export const newId = () => randomHex(6);

/** A unique op ID; sortable by creation time, random tail. */
export const newOpId = () => Date.now().toString(36) + "-" + randomHex(5);

/** Today's date in the device's time zone, as YYYY-MM-DD. */
export function localToday(d = new Date()) {
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** "2025-11-09" -> "Nov 9, 2025"; withYear=false -> "Nov 9". */
export function formatDate(iso, withYear = true) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso || "");
  if (!m) return "";
  const s = `${MONTHS[+m[2] - 1]} ${+m[3]}`;
  return withYear ? `${s}, ${m[1]}` : s;
}

/** "Last, First" from "First Last", matching the server's sort key. */
export function authorSortKey(author) {
  const parts = (author || "").trim().split(/\s+/).filter(Boolean);
  if (parts.length <= 1) return parts.join("");
  return parts[parts.length - 1] + ", " + parts.slice(0, -1).join(" ");
}

// --- The view: server truth plus pending ops ---

/**
 * Builds what the app shows: the server's books and reads with the
 * still-unsent ops applied on top. Records touched by a pending op carry
 * `pending: true`. Inputs are never mutated.
 *
 * @param {{books: Map, reads: Map}} server
 * @param {Array} ops queued ops, oldest first
 */
export function overlay(server, ops) {
  const books = new Map(server.books);
  const reads = new Map(server.reads);
  for (const op of ops) applyOp(books, reads, op);

  const readsByBook = new Map();
  for (const r of reads.values()) {
    if (!books.has(r.book_id)) continue;
    let list = readsByBook.get(r.book_id);
    if (!list) readsByBook.set(r.book_id, (list = []));
    list.push(r);
  }
  for (const list of readsByBook.values()) list.sort(compareReads);

  // date_read is derived from the latest read, as on the server.
  for (const [id, b] of books) {
    const list = readsByBook.get(id);
    const latest = list && list.length ? list[list.length - 1].date_finished : "";
    if ((b.date_read || "") !== latest) books.set(id, { ...b, date_read: latest });
  }
  return { books, reads, readsByBook };
}

/** Oldest first; undated reads sort before dated ones. */
export function compareReads(a, b) {
  return (a.date_finished || "").localeCompare(b.date_finished || "") ||
    (a.updated_at || "").localeCompare(b.updated_at || "");
}

function applyOp(books, reads, op) {
  switch (op.type) {
    case "book.create": {
      const b = op.book;
      books.set(b.id, {
        status: "wishlist", tags: [], rating: 0,
        ...b,
        sort_author: b.sort_author || authorSortKey(b.author),
        pending: true,
      });
      break;
    }
    case "book.update": {
      const b = books.get(op.id);
      if (!b) break;
      const next = { ...b, ...op.fields, pending: true };
      if ("author" in op.fields && !("sort_author" in op.fields) && op.fields.author !== b.author) {
        next.sort_author = authorSortKey(op.fields.author);
      }
      books.set(op.id, next);
      break;
    }
    case "book.delete":
      books.delete(op.id);
      for (const [id, r] of reads) if (r.book_id === op.id) reads.delete(id);
      break;
    case "read.create":
      if (books.has(op.read.book_id)) reads.set(op.read.id, { date_started: "", ...op.read, pending: true });
      break;
    case "read.update": {
      const r = reads.get(op.id);
      if (r) reads.set(op.id, { ...r, ...op.fields, pending: true });
      break;
    }
    case "read.delete":
      reads.delete(op.id);
      break;
  }
}

// --- Sync planning ---

/** The `since` to ask the server for, given what we stored last time. */
export function sinceFor(meta) {
  return meta && meta.epoch && meta.rev > 0 ? meta.rev : 0;
}

/**
 * Turns a /api/sync response into storage instructions. If the server's
 * epoch differs from ours (the database was restored or replaced), an
 * incremental response can't be trusted and a full sync is needed.
 */
export function planChanges(meta, resp) {
  if (!resp.full && meta && meta.epoch && resp.epoch !== meta.epoch) {
    return { refetchFull: true };
  }
  const plan = {
    replaceAll: !!resp.full,
    putBooks: [], deleteBooks: [], putReads: [], deleteReads: [],
    rev: resp.rev, epoch: resp.epoch,
  };
  for (const b of resp.books || []) (b.deleted ? plan.deleteBooks : plan.putBooks).push(b.deleted ? b.id : b);
  for (const r of resp.reads || []) (r.deleted ? plan.deleteReads : plan.putReads).push(r.deleted ? r.id : r);
  return plan;
}

/**
 * Sorts the server's per-op results into ops that are finished (ok or
 * rejected; both leave the queue) and the rejections to show the user.
 * An op with no result stays queued.
 */
export function settleResults(sent, results) {
  const byId = new Map((results || []).map((r) => [r.op_id, r]));
  const done = [];
  const rejected = [];
  for (const op of sent) {
    const r = byId.get(op.op_id);
    if (!r) continue;
    done.push(op.op_id);
    if (r.status !== "ok") rejected.push({ op, error: r.error || "rejected" });
  }
  return { done, rejected };
}

// --- Search ---

/** Lowercase, no accents or punctuation, single spaces. */
export function fold(s) {
  return (s || "")
    .normalize("NFD").replace(/\p{M}/gu, "")
    .toLowerCase()
    .replace(/&/g, " and ")
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .trim();
}

const ARTICLE = /^(the|a|an) /;

/** Title key for sorting: folded, leading article dropped. */
export function titleKey(title) {
  return fold(title).replace(ARTICLE, "");
}

/**
 * Finds books matching every word of the query, in title, author or ISBN.
 * Best first: title starts with the query, then a title word starts with
 * the first query word, then author matches, then anything else.
 */
export function search(books, query) {
  const q = fold(query);
  if (!q) return [];
  const words = q.split(" ");
  const qDigits = query.replace(/[^0-9xX]/g, "");
  const hits = [];
  for (const b of books) {
    const title = fold(b.title);
    const author = fold(b.author);
    const hay = title + " " + author + " " + fold(b.sort_author);
    const isbnHit = qDigits.length >= 10 && (b.isbn || "").replace(/[^0-9xX]/g, "") === qDigits;
    if (!isbnHit && !words.every((w) => hay.includes(w))) continue;
    let rank = 3;
    if (isbnHit || title.startsWith(q) || titleKey(b.title).startsWith(q)) rank = 0;
    else if ((" " + title).includes(" " + words[0])) rank = 1;
    else if (words.every((w) => author.includes(w))) rank = 2;
    hits.push({ b, rank });
  }
  hits.sort((x, y) => x.rank - y.rank || byTitle(x.b, y.b));
  return hits.map((h) => h.b);
}

// --- Sorting and grouping ---

export const byAuthor = (a, b) =>
  fold(a.sort_author || a.author).localeCompare(fold(b.sort_author || b.author)) || byTitle(a, b);
export const byTitle = (a, b) => titleKey(a.title).localeCompare(titleKey(b.title));
export const byAdded = (a, b) => (b.date_added || "").localeCompare(a.date_added || "") || byTitle(a, b);
export const byRating = (a, b) => (b.rating || 0) - (a.rating || 0) || byAuthor(a, b);
export const byYear = (a, b) => (a.first_published || 9999) - (b.first_published || 9999) || byAuthor(a, b);

export const SORTS = {
  author: { label: "Author", fn: byAuthor },
  title: { label: "Title", fn: byTitle },
  added: { label: "Date added", fn: byAdded },
  rating: { label: "Rating", fn: byRating },
  year: { label: "Year published", fn: byYear },
};

/** First letter of the sort key, for list section headers. */
export function initial(book, sort) {
  const key = sort === "title" ? titleKey(book.title) : fold(book.sort_author || book.author);
  const c = key.charAt(0).toUpperCase();
  return /[A-Z]/.test(c) ? c : "#";
}

/** Wishlist tab: wishlist and owned books. */
export function wishlistBooks(view, sort = "author") {
  const list = [...view.books.values()].filter((b) => b.status === "wishlist" || b.status === "owned");
  return list.sort((SORTS[sort] || SORTS.author).fn);
}

/** Reading tab: reading, then paused. */
export function readingBooks(view, sort = "title") {
  const fn = (SORTS[sort] || SORTS.title).fn;
  const all = [...view.books.values()];
  return {
    reading: all.filter((b) => b.status === "reading").sort(fn),
    paused: all.filter((b) => b.status === "paused").sort(fn),
  };
}

/**
 * History tab. Sorted by date: one entry per read (re-reads appear in each
 * year), grouped by year newest first, undated reads last. Sorted any other
 * way: one entry per book, in a single group.
 */
export function historyGroups(view, sort = "date") {
  if (sort !== "date") {
    const fn = (SORTS[sort] || SORTS.author).fn;
    const books = [...view.books.values()].filter((b) => view.readsByBook.has(b.id)).sort(fn);
    return [{ key: "all", label: "", count: books.length, entries: books.map((b) => ({ book: b, read: lastRead(view, b.id) })) }];
  }
  const groups = new Map();
  for (const r of view.reads.values()) {
    const book = view.books.get(r.book_id);
    if (!book) continue;
    const key = r.date_finished ? r.date_finished.slice(0, 4) : "undated";
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push({ book, read: r });
  }
  const out = [];
  for (const [key, entries] of groups) {
    entries.sort((x, y) => (y.read.date_finished || "").localeCompare(x.read.date_finished || "") || byAuthor(x.book, y.book));
    out.push({ key, label: key === "undated" ? "Undated" : key, count: entries.length, entries });
  }
  out.sort((a, b) => (a.key === "undated") - (b.key === "undated") || b.key.localeCompare(a.key));
  return out;
}

export function lastRead(view, bookId) {
  const list = view.readsByBook.get(bookId);
  return list && list.length ? list[list.length - 1] : null;
}

// --- Seller links ---

/** Used-book sellers first, Amazon last, as a price reference. */
const SELLER_ORDER = ["Better World Books", "ThriftBooks", "AbeBooks", "Biblio", "BookFinder", "Bookshop.org", "WorldCat", "Goodreads", "Amazon"];

export function orderSellers(sellers) {
  const rank = (s) => {
    const i = SELLER_ORDER.indexOf(s.name);
    return i < 0 ? SELLER_ORDER.length - 1 : i;
  };
  return [...sellers].sort((a, b) => rank(a) - rank(b));
}

export function sellerURL(seller, book) {
  let q = `${book.title || ""} ${book.author || ""}`.trim();
  if (book.isbn && /isbn/i.test(seller.url)) q = book.isbn;
  return seller.url.includes("{query}") ? seller.url.replace("{query}", encodeURIComponent(q)) : seller.url;
}

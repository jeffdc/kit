// App state: the server's books and reads (as last synced), the queue of
// changes not yet sent, and the view the UI renders (the two combined).
// Every change the user makes becomes an op: saved to IndexedDB first, then
// applied to the view at once, then sent to the server by sync.js.

import * as idb from "./idb.js";
import { overlay, newId, newOpId, localToday } from "./core.js";

const state = {
  server: { books: new Map(), reads: new Map() },
  ops: [],
  view: { books: new Map(), reads: new Map(), readsByBook: new Map() },
  meta: { rev: 0, epoch: null },
  sellers: [],
  sync: { status: "idle", detail: "", lastSync: null, rejected: [] },
};

const listeners = new Set();
let scheduled = false;

export const get = () => state;
export const subscribe = (fn) => (listeners.add(fn), () => listeners.delete(fn));

function recompute() {
  state.view = overlay(state.server, state.ops);
}

/**
 * Notify listeners once, after the current task, however many changes
 * happened. (Not requestAnimationFrame: it doesn't fire while the page isn't
 * painting, which would leave the screen stale when it comes back.)
 */
export function emit() {
  if (scheduled) return;
  scheduled = true;
  setTimeout(() => {
    scheduled = false;
    for (const fn of listeners) fn(state);
  }, 0);
}

export async function load() {
  const [books, reads, ops, sellers, rev, epoch, rejected] = await Promise.all([
    idb.getAll("books"), idb.getAll("reads"), idb.getAll("ops"), idb.getAll("booksellers"),
    idb.getMeta("rev"), idb.getMeta("epoch"), idb.getMeta("rejected"),
  ]);
  state.sync.rejected = rejected || [];
  state.server.books = new Map(books.map((b) => [b.id, b]));
  state.server.reads = new Map(reads.map((r) => [r.id, r]));
  state.ops = ops.sort((a, b) => a.seq - b.seq);
  state.sellers = sellers;
  state.meta = { rev: rev || 0, epoch: epoch || null };
  recompute();
}

// --- Server data ---

/** Stores a planned sync response (see core.planChanges). */
export async function applyServer(plan) {
  await idb.write(["books", "reads", "meta"], (s) => {
    if (plan.replaceAll) {
      s.books.clear();
      s.reads.clear();
    }
    for (const b of plan.putBooks) s.books.put(b);
    for (const id of plan.deleteBooks) s.books.delete(id);
    for (const r of plan.putReads) s.reads.put(r);
    for (const id of plan.deleteReads) s.reads.delete(id);
    s.meta.put({ key: "rev", value: plan.rev });
    s.meta.put({ key: "epoch", value: plan.epoch });
  });
  if (plan.replaceAll) {
    state.server.books = new Map();
    state.server.reads = new Map();
  }
  for (const b of plan.putBooks) state.server.books.set(b.id, b);
  for (const id of plan.deleteBooks) state.server.books.delete(id);
  for (const r of plan.putReads) state.server.reads.set(r.id, r);
  for (const id of plan.deleteReads) state.server.reads.delete(id);
  state.meta = { rev: plan.rev, epoch: plan.epoch };
  recompute();
  emit();
}

export async function setSellers(sellers) {
  await idb.write(["booksellers"], (s) => {
    s.booksellers.clear();
    for (const x of sellers) s.booksellers.put(x);
  });
  state.sellers = sellers;
  emit();
}

/** Removes ops the server has finished with (applied or rejected). */
export async function removeOps(opIds) {
  if (!opIds.length) return;
  const ids = new Set(opIds);
  const gone = state.ops.filter((o) => ids.has(o.op_id));
  await idb.write(["ops"], (s) => {
    for (const o of gone) s.ops.delete(o.seq);
  });
  state.ops = state.ops.filter((o) => !ids.has(o.op_id));
  recompute();
  emit();
}

export function setSync(patch) {
  Object.assign(state.sync, patch);
  // Refused changes are kept until dismissed, across reloads.
  if ("rejected" in patch) idb.setMeta("rejected", patch.rejected).catch(() => {});
  emit();
}

// --- Local changes ---

let onQueued = () => {};
/** sync.js registers here to send queued ops promptly. */
export const setOnQueued = (fn) => (onQueued = fn);

async function enqueue(...ops) {
  const db = await idb.open();
  const t = db.transaction("ops", "readwrite");
  const store = t.objectStore("ops");
  const stamped = ops.map((op) => ({ ...op, op_id: newOpId(), queued_at: new Date().toISOString() }));
  const keys = await Promise.all(stamped.map((op) => idb.done(store.add(op))));
  await idb.complete(t);
  stamped.forEach((op, i) => (op.seq = keys[i]));
  state.ops.push(...stamped);
  recompute();
  emit();
  onQueued();
}

/** Only the fields that differ from the current book. */
function changed(book, fields) {
  const out = {};
  for (const [k, v] of Object.entries(fields)) {
    const cur = book[k];
    const same = Array.isArray(v) ? JSON.stringify(v) === JSON.stringify(cur || []) : (cur ?? "") === (v ?? "");
    if (!same) out[k] = v;
  }
  return out;
}

export async function createBook(fields) {
  const book = { id: newId(), status: "wishlist", date_added: localToday(), ...fields };
  const ops = [{ type: "book.create", book }];
  if (book.status === "read") {
    ops.push({ type: "read.create", read: { id: newId(), book_id: book.id, date_finished: fields.date_finished || "" } });
    delete book.date_finished;
  }
  await enqueue(...ops);
  return book.id;
}

export async function updateBook(id, fields) {
  const book = state.view.books.get(id);
  if (!book) return;
  const diff = changed(book, fields);
  if (Object.keys(diff).length) await enqueue({ type: "book.update", id, fields: diff });
}

export const setStatus = (id, status) => updateBook(id, { status });

/** Logs a finish: a new read, and the book marked read with its rating. */
export async function finishBook(id, dateFinished, rating) {
  const book = state.view.books.get(id);
  if (!book) return;
  const fields = changed(book, { status: "read", rating });
  const ops = [{ type: "read.create", read: { id: newId(), book_id: id, date_finished: dateFinished } }];
  if (Object.keys(fields).length) ops.push({ type: "book.update", id, fields });
  await enqueue(...ops);
}

export const addRead = (bookId, dateFinished) =>
  enqueue({ type: "read.create", read: { id: newId(), book_id: bookId, date_finished: dateFinished } });

export async function updateRead(id, fields) {
  const r = state.view.reads.get(id);
  if (!r) return;
  const diff = changed(r, fields);
  if (Object.keys(diff).length) await enqueue({ type: "read.update", id, fields: diff });
}

export const deleteRead = (id) => enqueue({ type: "read.delete", id });

const FIELD_DEFAULTS = { rating: 0, owned: false, tags: [], first_published: 0, page_count: 0 };

/**
 * Runs a change and returns a function that undoes it, by queueing the
 * inverse of each op it queued (newest first). Ops are usually sent within
 * a second, so inverting is simpler than trying to pull them back out.
 * A deleted read comes back with a new ID (the old one is a tombstone).
 */
export async function undoable(change) {
  const before = { books: new Map(state.view.books), reads: new Map(state.view.reads) };
  const lastSeq = state.ops.reduce((m, o) => Math.max(m, o.seq), 0);
  const known = new Set(state.ops.map((o) => o.op_id));
  await change();
  const added = state.ops.filter((o) => !known.has(o.op_id) && o.seq > lastSeq);
  const pick = (rec, keys) => Object.fromEntries(keys.map((k) => [k, rec[k] ?? FIELD_DEFAULTS[k] ?? ""]));
  return async () => {
    const inverse = [];
    for (const op of [...added].reverse()) {
      if (op.type === "book.update") {
        const b = before.books.get(op.id);
        if (b) inverse.push({ type: "book.update", id: op.id, fields: pick(b, Object.keys(op.fields)) });
      } else if (op.type === "book.create") {
        inverse.push({ type: "book.delete", id: op.book.id });
      } else if (op.type === "read.create") {
        inverse.push({ type: "read.delete", id: op.read.id });
      } else if (op.type === "read.update") {
        const r = before.reads.get(op.id);
        if (r) inverse.push({ type: "read.update", id: op.id, fields: pick(r, Object.keys(op.fields)) });
      } else if (op.type === "read.delete") {
        const r = before.reads.get(op.id);
        if (r) inverse.push({ type: "read.create", read: { id: newId(), book_id: r.book_id, date_finished: r.date_finished, date_started: r.date_started || "" } });
      }
    }
    if (inverse.length) await enqueue(...inverse);
  };
}
export const deleteBook = (id) => enqueue({ type: "book.delete", id });

/** Pending op count per book, for the "not synced" marks. */
export function pendingBookIds() {
  const ids = new Set();
  for (const op of state.ops) {
    if (op.type.startsWith("book.")) ids.add(op.id || op.book.id);
    else if (op.read) ids.add(op.read.book_id);
    else {
      const r = state.view.reads.get(op.id) || state.server.reads.get(op.id);
      if (r) ids.add(r.book_id);
    }
  }
  return ids;
}

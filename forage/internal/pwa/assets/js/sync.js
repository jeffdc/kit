// Talks to the server: sends queued ops, then pulls what changed.
// One sync runs at a time; a request during a run schedules one more.

import * as store from "./store.js";
import * as idb from "./idb.js";
import { planChanges, settleResults, sinceFor } from "./core.js";

const KEY = "forage_api_key"; // same key the original app used, so the phone stays signed in
const BATCH = 200;
const TIMEOUT_MS = 20000;

export function getKey() {
  try { return localStorage.getItem(KEY) || ""; } catch { return ""; }
}
export function setKey(k) {
  try { localStorage.setItem(KEY, k.trim()); } catch { /* private mode */ }
}

class HTTPError extends Error {
  constructor(status, message) { super(message); this.status = status; }
}

export async function api(path, { method = "GET", body } = {}) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS);
  try {
    const res = await fetch(path, {
      method,
      signal: ctrl.signal,
      headers: { Authorization: "Bearer " + getKey(), ...(body ? { "Content-Type": "application/json" } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new HTTPError(res.status, data.error || `HTTP ${res.status}`);
    return data;
  } finally {
    clearTimeout(timer);
  }
}

let running = null;
let again = false;
let timer = null;

/** Asks for a sync soon (coalescing bursts of edits). */
export function requestSync(delay = 400) {
  clearTimeout(timer);
  timer = setTimeout(syncNow, delay);
}

export function syncNow() {
  if (running) {
    again = true;
    return running;
  }
  running = run().finally(() => {
    running = null;
    if (again) {
      again = false;
      syncNow();
    }
  });
  return running;
}

async function run() {
  if (!getKey()) return store.setSync({ status: "nokey", detail: "Add your API key in Settings" });
  if (!navigator.onLine) return store.setSync({ status: "offline", detail: "Changes will sync when you're back online" });

  store.setSync({ status: "syncing", detail: "" });
  try {
    await flushLegacyQueue();
    await push();
    await pull();
    store.setSync({ status: "synced", detail: "", lastSync: new Date().toISOString() });
  } catch (err) {
    if (err.status === 401) store.setSync({ status: "auth", detail: "The API key was refused. Check it in Settings." });
    else store.setSync({ status: "error", detail: navigator.onLine ? err.message : "Offline" });
  }
}

/** Sends queued ops in order, in batches; stops at the first failed request. */
async function push() {
  for (;;) {
    const batch = store.get().ops.slice(0, BATCH);
    if (!batch.length) return;
    const ops = batch.map(({ seq, queued_at, ...op }) => op);
    const resp = await api("/api/ops", { method: "POST", body: { ops } });
    const { done, rejected } = settleResults(batch, resp.results);
    if (rejected.length) {
      store.setSync({ rejected: [...store.get().sync.rejected, ...rejected] });
    }
    await store.removeOps(done);
    if (done.length < batch.length) return; // the rest had no result; try again next sync
  }
}

async function pull() {
  const { meta } = store.get();
  let resp = await api("/api/sync?since=" + sinceFor(meta));
  let plan = planChanges(meta, resp);
  if (plan.refetchFull) {
    resp = await api("/api/sync?since=0");
    plan = planChanges(null, resp);
  }
  if (plan.replaceAll || plan.putBooks.length || plan.deleteBooks.length || plan.putReads.length || plan.deleteReads.length || plan.rev !== meta.rev) {
    await store.applyServer(plan);
  }
  if (plan.replaceAll || !store.get().sellers.length) {
    await store.setSellers(await api("/api/booksellers"));
  }
}

/**
 * The original app queued changes in a "changes" store and sent them to
 * /api/changes. Anything it left behind is sent the old way, once, before
 * the first sync with the new API.
 */
async function flushLegacyQueue() {
  if (!(await idb.hasStore("changes"))) return;
  const changes = await idb.getAll("changes");
  if (!changes.length) return;
  await api("/api/changes", { method: "POST", body: { version: 1, exported: new Date().toISOString(), changes } });
  await idb.write(["changes"], (s) => s.changes.clear());
  // Force a full resync so the result of those changes comes back.
  await store.applyServer({ replaceAll: false, putBooks: [], deleteBooks: [], putReads: [], deleteReads: [], rev: 0, epoch: null });
}

export function start() {
  store.setOnQueued(() => requestSync());
  window.addEventListener("online", () => syncNow());
  window.addEventListener("offline", () => store.setSync({ status: "offline", detail: "Changes will sync when you're back online" }));
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") syncNow();
  });
  setInterval(() => {
    if (document.visibilityState === "visible") syncNow();
  }, 5 * 60 * 1000);
  return syncNow();
}

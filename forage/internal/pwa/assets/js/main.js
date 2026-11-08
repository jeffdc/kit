// Forage PWA: boots the store and sync, renders, and handles input.

import * as store from "./store.js";
import * as sync from "./sync.js";
import * as V from "./views.js";
import { search, STATUS_LABEL, formatDate } from "./core.js";

const $ = (sel) => document.querySelector(sel);

// --- UI state (what's on screen; data lives in store.js) ---

const SORT_KEY = "forage_sorts";
const ui = {
  tab: "wishlist",
  sorts: { wishlist: "author", reading: "title", history: "date", ...readJSON(SORT_KEY) },
  query: "",
  detailId: null,
  detail: { finishing: false, confirmDelete: false, sweep: null, lastStatus: null },
  sheet: null, // "add" | "manual" | "edit" | "settings"
  add: freshAdd(),
  key: sync.getKey(),
  build: null,
};

function freshAdd(q = "") {
  return { q, status: "wishlist", loading: false, result: null, error: null, manual: null };
}

function readJSON(key) {
  try { return JSON.parse(localStorage.getItem(key) || "{}"); } catch { return {}; }
}
function saveSorts() {
  try { localStorage.setItem(SORT_KEY, JSON.stringify(ui.sorts)); } catch { /* ignore */ }
}

// --- Rendering ---

function renderMain() {
  const st = store.get();
  const pending = store.pendingBookIds();
  let html;
  if (ui.query.trim()) {
    html = V.searchView(search(st.view.books.values(), ui.query), ui.query, pending);
  } else if (ui.tab === "reading") {
    html = V.readingView(st.view, ui.sorts.reading, pending);
  } else if (ui.tab === "history") {
    html = V.historyView(st.view, ui.sorts.history, pending);
  } else {
    html = V.wishlistView(st.view, ui.sorts.wishlist, pending);
  }
  $("#main").innerHTML = html;
  document.body.dataset.tab = ui.query.trim() ? "search" : ui.tab;
  for (const b of document.querySelectorAll(".tab[data-tab]")) {
    const on = !ui.query.trim() && b.dataset.tab === ui.tab;
    b.setAttribute("aria-current", on ? "page" : "false");
  }
}

function renderHeader() {
  const st = store.get();
  const label = V.syncLabel(st.sync, st.ops.length);
  const btn = $("#sync-btn");
  btn.className = `sync s-sync-${st.sync.status}${st.ops.length ? " has-pending" : ""}`;
  btn.querySelector(".sync-text").textContent = label;
  btn.setAttribute("aria-label", `Sync status: ${label}. Open settings`);
  $("#q").placeholder = `Search ${st.view.books.size.toLocaleString()} books`;
  $("#q-clear").hidden = !ui.query;
}

function renderDetail() {
  const panel = $("#detail");
  if (!ui.detailId) return;
  const st = store.get();
  const b = st.view.books.get(ui.detailId);
  if (!b) return closeDetail();
  const reads = st.view.readsByBook.get(b.id) || [];
  const d = ui.detail;
  // Let a running sweep finish: rebuilding the cover would cut it off
  // (a sync landing a moment after the status change is the usual case).
  if (d.sweepUntil && Date.now() < d.sweepUntil) {
    clearTimeout(d.sweepTimer);
    d.sweepTimer = setTimeout(renderDetail, d.sweepUntil - Date.now());
    return;
  }
  d.sweep = d.lastStatus && d.lastStatus !== b.status ? d.lastStatus : null;
  d.sweepUntil = d.sweep ? Date.now() + 640 : 0; // 550ms sweep + 70ms band delay
  d.lastStatus = b.status;
  const scroll = panel.scrollTop;
  panel.innerHTML = V.detailView(b, reads, st.sellers, d);
  panel.scrollTop = scroll;
  panel.dataset.status = b.status;
}

function renderSheet() {
  const panel = $("#sheet");
  const st = store.get();
  switch (ui.sheet) {
    case "add": panel.innerHTML = V.addView(ui.add); break;
    case "manual": panel.innerHTML = V.manualView(ui.add); break;
    case "edit": {
      const b = st.view.books.get(ui.detailId);
      if (!b) return closeSheet();
      panel.innerHTML = V.editView(b);
      break;
    }
    case "settings": panel.innerHTML = V.settingsView(st, ui); break;
  }
}

function renderAll() {
  renderHeader();
  renderMain();
  if (ui.detailId && !editingInside("#detail")) renderDetail();
  if (ui.sheet === "settings" && !editingInside("#sheet")) renderSheet();
}

/** True while the user is typing in a field inside the panel (don't clobber it). */
function editingInside(sel) {
  const a = document.activeElement;
  return a && a.closest(sel) && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName);
}

// --- Panels (book detail and sheets), with history so swipe-back closes them ---

// While any panel is open, one history entry is held, so the back gesture
// closes the top panel. Closing the last panel by button gives it back.
const panelStack = [];
let historyHeld = false;
let ignorePop = false;

function openPanel(name, el) {
  if (!historyHeld) {
    history.pushState({ forage: "panel" }, "");
    historyHeld = true;
  }
  panelStack.push({ name, returnFocus: document.activeElement });
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add("open"));
  document.body.classList.add("panel-open");
}

/** fromPop: closed by the back gesture. handoff: another panel opens right after. */
function closePanel(el, { fromPop = false, handoff = false } = {}) {
  const entry = panelStack.pop();
  el.classList.remove("open");
  setTimeout(() => {
    if (!el.classList.contains("open")) el.hidden = true;
  }, 260);
  if (panelStack.length) {
    if (fromPop) {
      history.pushState({ forage: "panel" }, "");
      historyHeld = true;
    }
  } else {
    document.body.classList.remove("panel-open");
    if (!handoff && historyHeld && !fromPop) {
      ignorePop = true;
      history.back();
    }
    if (!handoff) historyHeld = false;
  }
  if (!handoff && entry && entry.returnFocus && entry.returnFocus.focus) entry.returnFocus.focus({ preventScroll: true });
}

window.addEventListener("popstate", () => {
  if (ignorePop) {
    ignorePop = false;
    return;
  }
  historyHeld = false;
  const top = panelStack[panelStack.length - 1];
  if (!top) return;
  if (top.name === "detail") closeDetail({ fromPop: true });
  else closeSheet({ fromPop: true });
});

function openDetail(id) {
  if (ui.sheet) closeSheet({ handoff: !ui.detailId });
  ui.detailId = id;
  ui.detail = { finishing: false, confirmDelete: false, sweep: null, lastStatus: null };
  const panel = $("#detail");
  renderDetail();
  if (!panelStack.some((p) => p.name === "detail")) openPanel("detail", panel);
  // Reset after showing: a hidden panel ignores scrollTop and keeps the last book's position.
  panel.scrollTop = 0;
  panel.querySelector("[data-act=close]").focus({ preventScroll: true });
}

function closeDetail(opts) {
  if (!ui.detailId) return;
  ui.detailId = null;
  closePanel($("#detail"), opts);
}

function openSheet(name) {
  const panel = $("#sheet");
  const wasOpen = !!ui.sheet;
  ui.sheet = name;
  renderSheet();
  if (!wasOpen) openPanel("sheet", panel);
  panel.scrollTop = 0;
  const first = panel.querySelector("input:not([type=hidden]), textarea") || panel.querySelector("button");
  if (first) first.focus({ preventScroll: true });
}

function closeSheet(opts) {
  if (!ui.sheet) return;
  ui.sheet = null;
  closePanel($("#sheet"), opts);
}

// --- Toast ---

let toastTimer = null;
function toast(msg, action) {
  const el = $("#toast");
  el.innerHTML = `<span>${V.esc(msg)}</span>${action ? `<button class="toast-btn">${V.esc(action.label)}</button>` : ""}`;
  el.classList.add("show");
  if (action) el.querySelector("button").onclick = () => { el.classList.remove("show"); action.run(); };
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove("show"), action ? 6000 : 3000);
}

// --- Actions ---

const STATUS_ACTIONS = {
  start: "reading", resume: "reading", reread: "reading", pause: "paused",
  owned: "owned", wishlist: "wishlist", undrop: "wishlist", drop: "dropped",
};

async function onAction(act, el) {
  const id = ui.detailId;
  if (STATUS_ACTIONS[act] && id) {
    await store.setStatus(id, STATUS_ACTIONS[act]);
    toast(`Moved to ${STATUS_LABEL[STATUS_ACTIONS[act]]}`);
    return;
  }
  switch (act) {
    case "close": return closeDetail();
    case "close-sheet": return closeSheet();
    case "finish":
      ui.detail.finishing = true;
      renderDetail();
      $("#detail input[name=date]")?.focus({ preventScroll: true });
      return;
    case "cancel-finish":
      ui.detail.finishing = false;
      return renderDetail();
    case "rate": {
      const n = +el.dataset.rate;
      const b = store.get().view.books.get(id);
      return store.updateBook(id, { rating: b && b.rating === n ? 0 : n });
    }
    case "add-read":
      await store.addRead(id, "");
      return toast("Added a read. Set its date below.");
    case "delete-read":
      if (el.dataset.armed) return store.deleteRead(el.dataset.read);
      el.dataset.armed = "1";
      el.classList.add("armed");
      el.setAttribute("aria-label", "Tap again to delete this read");
      setTimeout(() => { delete el.dataset.armed; el.classList.remove("armed"); }, 3000);
      return;
    case "edit": return openSheet("edit");
    case "delete":
      if (!ui.detail.confirmDelete) {
        ui.detail.confirmDelete = true;
        renderDetail();
        setTimeout(() => { if (ui.detail.confirmDelete) { ui.detail.confirmDelete = false; renderDetail(); } }, 4000);
        return;
      }
      {
        const b = store.get().view.books.get(id);
        await store.deleteBook(id);
        closeDetail();
        toast(`Deleted “${b ? b.title : "book"}”`);
      }
      return;
    case "add": ui.add = freshAdd(); return openSheet("add");
    case "add-from-search": ui.add = freshAdd(ui.query); openSheet("add"); return resolve();
    case "paste": return pasteAndResolve();
    case "manual": {
      const q = ui.add.q.trim();
      ui.add.manual = /^https?:|\d{9}/.test(q) ? {} : splitTitle(q);
      return openSheet("manual");
    }
    case "add-candidate": return addCandidate(+el.dataset.i);
    case "open-match": return openDetail(el.dataset.book);
    case "sync-now": return sync.syncNow();
    case "settings": return openSheet("settings");
    case "dismiss-rejected": {
      const r = store.get().sync.rejected.slice();
      r.splice(+el.dataset.i, 1);
      store.setSync({ rejected: r });
      return renderSheet();
    }
  }
}

function splitTitle(q) {
  const m = /^(.*\S)\s+by\s+(.+)$/i.exec(q);
  return m ? { title: m[1], author: m[2] } : { title: q };
}

async function pasteAndResolve() {
  try {
    const text = await navigator.clipboard.readText();
    if (!text) return;
    ui.add.q = text.trim();
    $("#add-q").value = ui.add.q;
    resolve();
  } catch {
    toast("Paste isn't available here. Long-press the field to paste.");
  }
}

async function resolve() {
  const q = ui.add.q.trim();
  if (!q) return;
  if (!navigator.onLine) {
    ui.add.error = "You're offline, so lookup isn't available. Enter it by hand; it will sync later.";
    return updateAddResults();
  }
  ui.add.loading = true;
  ui.add.error = null;
  ui.add.result = null;
  renderAddForm();
  try {
    ui.add.result = await sync.api("/api/resolve", { method: "POST", body: { input: q } });
  } catch (err) {
    ui.add.error = err.status === 401 ? "Your API key was refused. Check it in Settings." : `Lookup failed: ${err.message}`;
  }
  ui.add.loading = false;
  renderAddForm();
}

function renderAddForm() {
  if (ui.sheet !== "add") return;
  const btn = $("#sheet .resolve button[type=submit]");
  if (btn) {
    btn.disabled = ui.add.loading;
    btn.textContent = ui.add.loading ? "Looking it up…" : "Find";
  }
  updateAddResults();
}

function updateAddResults() {
  const el = $("#add-results");
  if (el) el.innerHTML = V.addResults(ui.add);
}

async function addCandidate(i) {
  const c = ui.add.result && ui.add.result.candidates[i];
  if (!c) return;
  const status = ui.add.status;
  const id = await store.createBook({
    title: c.title, author: c.author || "Unknown",
    first_published: c.first_published || 0, page_count: c.page_count || 0, isbn: c.isbn || "",
    status, ...(status === "read" ? { date_finished: "" } : {}),
  });
  closeSheet();
  toast(`Added to ${STATUS_LABEL[status]}`, { label: "Open", run: () => openDetail(id) });
}

// --- Forms ---

const int = (v) => parseInt(v, 10) || 0;

async function onSubmit(form) {
  const data = Object.fromEntries(new FormData(form));
  switch (form.dataset.form) {
    case "resolve":
      ui.add.q = data.q;
      return resolve();
    case "finish": {
      const rating = form.querySelectorAll(".star-btn[aria-pressed=true]").length;
      await store.finishBook(ui.detailId, data.date, rating);
      ui.detail.finishing = false;
      renderDetail();
      return toast(`Logged as finished ${formatDate(data.date)}`);
    }
    case "manual": {
      if (!data.title.trim() || !data.author.trim()) return;
      const status = ui.add.status;
      const id = await store.createBook({
        title: data.title.trim(), author: data.author.trim(), isbn: (data.isbn || "").trim(),
        first_published: int(data.first_published), page_count: int(data.page_count), status,
        ...(status === "read" ? { date_finished: data.date_finished || "" } : {}),
      });
      closeSheet();
      return toast(`Added to ${STATUS_LABEL[status]}`, { label: "Open", run: () => openDetail(id) });
    }
    case "edit": {
      if (!data.title.trim() || !data.author.trim()) return;
      await store.updateBook(ui.detailId, {
        title: data.title.trim(), author: data.author.trim(), sort_author: data.sort_author.trim(),
        first_published: int(data.first_published), page_count: int(data.page_count),
        isbn: data.isbn.trim(), date_added: data.date_added, body: data.body.trim(),
        tags: data.tags.split(",").map((t) => t.trim()).filter(Boolean),
      });
      closeSheet();
      return toast("Saved");
    }
    case "key":
      sync.setKey(data.key || "");
      ui.key = data.key || "";
      toast(ui.key ? "Key saved" : "Key removed");
      return sync.syncNow();
  }
}

// --- Events ---

function bind() {
  document.addEventListener("click", (e) => {
    const t = e.target;
    const rateBtn = t.closest(".finish .star-btn");
    if (rateBtn) {
      const n = +rateBtn.dataset.rate;
      const all = rateBtn.parentElement.querySelectorAll(".star-btn");
      const current = rateBtn.parentElement.querySelectorAll(".star-btn[aria-pressed=true]").length;
      const next = current === n ? 0 : n;
      all.forEach((b, i) => {
        b.setAttribute("aria-pressed", i < next);
        b.querySelector(".star").classList.toggle("on", i < next);
        b.querySelector("path").setAttribute("fill", i < next ? "currentColor" : "none");
      });
      return;
    }
    const seg = t.closest("[data-add-status]");
    if (seg) {
      ui.add.status = seg.dataset.addStatus;
      if (ui.sheet === "manual") {
        // capture typed values before re-rendering
        ui.add.manual = Object.fromEntries(new FormData($("#sheet form")));
        renderSheet();
      } else {
        for (const b of document.querySelectorAll("[data-add-status]")) b.setAttribute("aria-pressed", b === seg);
      }
      return;
    }
    const act = t.closest("[data-act]");
    if (act) {
      e.preventDefault();
      onAction(act.dataset.act, act);
      return;
    }
    const bookBtn = t.closest("[data-book]");
    if (bookBtn) return openDetail(bookBtn.dataset.book);
    const tab = t.closest("[data-tab]");
    if (tab) {
      ui.tab = tab.dataset.tab;
      setQuery("");
      window.scrollTo({ top: 0 });
    }
  });

  document.addEventListener("submit", (e) => {
    e.preventDefault();
    onSubmit(e.target);
  });

  document.addEventListener("change", (e) => {
    const t = e.target;
    if (t.matches("select[data-sort]")) {
      ui.sorts[t.dataset.sort] = t.value;
      saveSorts();
      renderMain();
    } else if (t.matches("input[data-read]")) {
      store.updateRead(t.dataset.read, { date_finished: t.value });
    }
  });

  document.addEventListener("input", (e) => {
    if (e.target.id === "add-q") ui.add.q = e.target.value;
  });

  let qTimer;
  $("#q").addEventListener("input", (e) => {
    clearTimeout(qTimer);
    const v = e.target.value;
    qTimer = setTimeout(() => setQuery(v, false), 90);
  });
  $("#q-clear").addEventListener("click", () => {
    setQuery("");
    $("#q").focus();
  });
  $("#search").addEventListener("submit", (e) => {
    e.preventDefault();
    $("#q").blur();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      if (ui.sheet) closeSheet();
      else if (ui.detailId) closeDetail();
      else if (ui.query) setQuery("");
    } else if (e.key === "/" && !/^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement.tagName)) {
      e.preventDefault();
      $("#q").focus();
    }
  });
}

function setQuery(q, updateInput = true) {
  ui.query = q;
  if (updateInput) $("#q").value = q;
  renderHeader();
  renderMain();
}

// --- Boot ---

async function boot() {
  bind();
  await store.load();
  let rejectedSeen = store.get().sync.rejected.length;
  store.subscribe((st) => {
    renderAll();
    if (st.sync.rejected.length > rejectedSeen) {
      toast("The server refused a change", { label: "Details", run: () => openSheet("settings") });
    }
    rejectedSeen = st.sync.rejected.length;
  });
  renderAll();

  const params = new URLSearchParams(location.search);
  const shared = params.get("add") || [params.get("title"), params.get("author")].filter(Boolean).join(" by ");
  if (shared) {
    history.replaceState(null, "", location.pathname);
    ui.add = freshAdd(shared);
    openSheet("add");
    resolve();
  }

  sync.start();
  if (!sync.getKey()) openSheet("settings");

  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("./sw.js").catch(() => {});
    // A new version took over: reload once, when it won't interrupt anything.
    let reloaded = false;
    navigator.serviceWorker.addEventListener("controllerchange", () => {
      const reload = () => { if (!reloaded) { reloaded = true; location.reload(); } };
      if (!ui.sheet && !ui.detailId) reload();
      else document.addEventListener("visibilitychange", () => document.visibilityState === "hidden" && reload(), { once: true });
    });
  }
}

// The build version is shown in Settings.
fetch("/api/health").then((r) => r.json()).then((h) => { ui.build = h.build; }).catch(() => {});

boot().catch((err) => {
  $("#main").innerHTML = `<div class="empty"><p class="empty-title">Forage couldn't start.</p><p>${V.esc(err.message)}</p></div>`;
});

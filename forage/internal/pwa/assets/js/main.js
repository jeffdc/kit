// Forage PWA: boots the store and sync, renders, and handles input.

import * as store from "./store.js";
import * as sync from "./sync.js";
import * as V from "./views.js";
import { search, STATUS_LABEL, formatDate, shelfCounts, plausibleDate } from "./core.js";

const $ = (sel) => document.querySelector(sel);

// From 900px the open book is a pane beside the list, not a panel over it:
// picking a row shows it there, with no history entry and no scrim.
const wideMQ = matchMedia("(min-width: 900px)");
const isWide = () => wideMQ.matches;

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
  return { q, status: "wishlist", owned: false, loading: false, result: null, error: null, manual: null };
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
  } else if (ui.tab === "undated") {
    html = V.undatedView(st.view, pending);
  } else {
    html = V.wishlistView(st.view, ui.sorts.wishlist, pending);
  }
  const main = $("#main");
  const top = main.scrollTop; // the list scrolls inside main on wide screens
  main.innerHTML = html;
  main.scrollTop = top;
  markSelected();
  // After a date is saved in Needs a date, its row is gone; the next one takes its place.
  if (ui.focusDate != null) {
    const next = main.querySelectorAll("input[data-read]")[ui.focusDate];
    ui.focusDate = null;
    if (next) next.focus({ preventScroll: false });
  }
  document.body.dataset.view = ui.query.trim() ? "search" : ui.tab;
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
  const counts = shelfCounts(st.view);
  for (const el of document.querySelectorAll("[data-count]")) el.textContent = counts[el.dataset.count].toLocaleString();
}

/** On wide screens the open book's rows are marked in the list. */
function markSelected() {
  for (const r of document.querySelectorAll("#main .row[aria-current]")) r.removeAttribute("aria-current");
  if (!ui.detailId || !isWide()) return;
  for (const r of document.querySelectorAll(`#main .row[data-book="${CSS.escape(ui.detailId)}"]`)) r.setAttribute("aria-current", "true");
}

function renderDetail() {
  const panel = $("#detail");
  if (!ui.detailId) {
    if (isWide()) panel.innerHTML = V.paneEmpty();
    return;
  }
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
  if (!editingInside("#main")) renderMain();
  if ((ui.detailId || isWide()) && !editingInside("#detail")) renderDetail();
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
  if (ui.sheet) closeSheet(isWide() ? undefined : { handoff: !ui.detailId });
  ui.detailId = id;
  ui.detail = { finishing: false, confirmDelete: false, sweep: null, lastStatus: null };
  const panel = $("#detail");
  renderDetail();
  if (isWide()) {
    // Focus stays in the list, so j/k and the arrows keep moving through it.
    markSelected();
    panel.scrollTop = 0;
    return;
  }
  if (!panelStack.some((p) => p.name === "detail")) openPanel("detail", panel);
  // Reset after showing: a hidden panel ignores scrollTop and keeps the last book's position.
  panel.scrollTop = 0;
  panel.querySelector("[data-act=close]").focus({ preventScroll: true });
}

function closeDetail(opts) {
  if (!ui.detailId) return;
  ui.detailId = null;
  if (isWide()) {
    renderDetail();
    markSelected();
    return;
  }
  closePanel($("#detail"), opts);
}

/** Moving across 900px turns the book panel into the pane, or back. */
function applyLayout() {
  const panel = $("#detail");
  if (isWide()) {
    panel.setAttribute("role", "region");
    panel.removeAttribute("aria-modal");
    const i = panelStack.findIndex((p) => p.name === "detail");
    if (i >= 0) {
      panelStack.splice(i, 1);
      if (!panelStack.length) {
        document.body.classList.remove("panel-open");
        if (historyHeld) {
          ignorePop = true;
          history.back();
          historyHeld = false;
        }
      }
    }
    panel.classList.remove("open");
    renderDetail();
  } else {
    panel.setAttribute("role", "dialog");
    panel.setAttribute("aria-modal", "true");
    ui.detailId = null;
    panel.classList.remove("open");
    panel.hidden = true;
  }
  markSelected();
}

/** Wide-screen keys: move through the list and act on the open book. */
function shortcut(e) {
  const k = e.key;
  if (k === "j" || k === "ArrowDown") moveSelection(1);
  else if (k === "k" || k === "ArrowUp") moveSelection(-1);
  else if (k === "n") onAction("add");
  else if (!ui.detailId) return;
  else if (k === "o") onAction("toggle-owned");
  else if (k === "e") onAction("edit");
  else if (k === "f") {
    const b = store.get().view.books.get(ui.detailId);
    if (!b || (b.status !== "reading" && b.status !== "paused")) return;
    onAction("finish");
  } else if (/^[1-5]$/.test(k)) {
    const b = store.get().view.books.get(ui.detailId);
    if (!b || b.status === "wishlist" || b.status === "to_read") return;
    onAction("rate", { dataset: { rate: k } });
  }
  else return;
  e.preventDefault();
}

function moveSelection(step) {
  const rows = [...document.querySelectorAll("#main .row[data-book]")];
  if (!rows.length) return;
  let i = rows.findIndex((r) => r.dataset.book === ui.detailId);
  i = i < 0 ? (step > 0 ? 0 : rows.length - 1) : Math.min(rows.length - 1, Math.max(0, i + step));
  const r = rows[i];
  openDetail(r.dataset.book);
  r.focus({ preventScroll: true });
  r.scrollIntoView({ block: "nearest" });
}

function openSheet(name) {
  const panel = $("#sheet");
  const wasOpen = !!ui.sheet;
  ui.sheet = name;
  renderSheet();
  if (!wasOpen) openPanel("sheet", panel);
  panel.scrollTop = 0;
  const typeFirst = name === "add" || name === "manual";
  const first = (typeFirst && panel.querySelector("input:not([type=hidden]), textarea")) || panel.querySelector(".sheet-head .icon-btn");
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
  const hide = (ms) => { clearTimeout(toastTimer); toastTimer = setTimeout(() => el.classList.remove("show"), ms); };
  hide(action ? 6000 : 3000);
  // A toast with a button waits while the pointer or focus is on it.
  el.onmouseenter = el.onfocusin = () => clearTimeout(toastTimer);
  el.onmouseleave = el.onfocusout = () => hide(action ? 3000 : 1500);
}

/** Makes a change, then says so with an Undo button. */
async function withUndo(msg, change) {
  const undo = await store.undoable(change);
  toast(msg, { label: "Undo", run: async () => { await undo(); toast("Undone"); } });
}

// --- Actions ---

const STATUS_ACTIONS = {
  start: "reading", resume: "reading", reread: "reading", pause: "paused",
  wishlist: "wishlist", undrop: "wishlist", drop: "dropped",
};

async function onAction(act, el) {
  const id = ui.detailId;
  if (STATUS_ACTIONS[act] && id) {
    return withUndo(`Moved to ${STATUS_LABEL[STATUS_ACTIONS[act]]}`, () => store.setStatus(id, STATUS_ACTIONS[act]));
  }
  switch (act) {
    case "close": return closeDetail();
    case "undated":
      if (ui.sheet) closeSheet();
      if (!isWide()) closeDetail();
      ui.tab = "undated";
      setQuery("");
      window.scrollTo({ top: 0 });
      $("#main").scrollTop = 0;
      return;
    case "set-date": {
      const input = [...document.querySelectorAll("#detail input[data-read]")].find((i) => !i.value);
      if (!input) return;
      input.scrollIntoView({ block: "center" });
      input.focus({ preventScroll: true });
      try { input.showPicker(); } catch { /* not supported, or needs a direct tap */ }
      return;
    }
    case "got-it":
      return withUndo(`Moved to ${STATUS_LABEL.to_read}, marked as owned`, () => store.updateBook(id, { status: "to_read", owned: true }));
    case "toggle-owned": {
      const b = store.get().view.books.get(id);
      if (!b) return;
      return withUndo(b.owned ? "Marked as not owned" : "Marked as owned", () => store.updateBook(id, { owned: !b.owned }));
    }
    case "toggle-add-owned":
      // Toggled in place: re-rendering the manual form would drop what's typed.
      ui.add.owned = !ui.add.owned;
      for (const b of document.querySelectorAll("[data-act=toggle-add-owned]")) b.setAttribute("aria-pressed", ui.add.owned);
      return;
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
      if (b && b.rating === n) return withUndo("Rating cleared", () => store.updateBook(id, { rating: 0 }));
      return store.updateBook(id, { rating: n });
    }
    case "add-read":
      return withUndo("Added a read. Set its date below.", () => store.addRead(id, ""));
    case "delete-read": {
      if (el.dataset.armed) return withUndo("Read deleted", () => store.deleteRead(el.dataset.read));
      // Armed: the icon becomes a labelled button for a few seconds.
      const icon = el.innerHTML;
      el.dataset.armed = "1";
      el.classList.add("armed");
      el.textContent = "Delete";
      el.setAttribute("aria-label", "Delete this read");
      setTimeout(() => {
        delete el.dataset.armed;
        el.classList.remove("armed");
        el.innerHTML = icon;
        el.setAttribute("aria-label", "Delete this read");
      }, 4000);
      return;
    }
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
    case "add": ui.add = freshAdd(); ui.add.offline = !navigator.onLine; return openSheet("add");
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
  if (ui.add.result && ui.add.result.candidates.length) $("#add-q")?.blur();
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
    status, owned: ui.add.owned, ...(status === "read" ? { date_finished: "" } : {}),
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
      ui.detail.finishing = false;
      return withUndo(`Logged as finished ${formatDate(data.date)}`, () => store.finishBook(ui.detailId, data.date, rating));
    }
    case "manual": {
      if (!data.title.trim() || !data.author.trim()) return;
      const status = ui.add.status;
      const id = await store.createBook({
        title: data.title.trim(), author: data.author.trim(), isbn: (data.isbn || "").trim(),
        first_published: int(data.first_published), page_count: int(data.page_count), status, owned: ui.add.owned,
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
    // In the drawer layout, a click on the dimmed page outside the book closes it.
    if (ui.detailId && !ui.sheet && !isWide() && !t.closest(".panel, .toast")) {
      e.preventDefault();
      return closeDetail();
    }
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
    const sortCol = t.closest("[data-sort-col]");
    if (sortCol) {
      ui.sorts[sortCol.dataset.sortCol] = sortCol.dataset.key;
      saveSorts();
      return renderMain();
    }
    const bookBtn = t.closest("[data-book]");
    if (bookBtn) return openDetail(bookBtn.dataset.book);
    const tab = t.closest("button.tab[data-tab]"); // only the tab bar (not any ancestor)
    if (tab) {
      ui.tab = tab.dataset.tab;
      setQuery("");
      window.scrollTo({ top: 0 });
      $("#main").scrollTop = 0;
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
      if (!plausibleDate(t.value)) return;
      const inList = t.closest("#main");
      const index = inList ? [...inList.querySelectorAll("input[data-read]")].indexOf(t) : -1;
      store.updateRead(t.dataset.read, { date_finished: t.value }).then(() => {
        if (index < 0) return;
        ui.focusDate = index;
        renderMain();
      });
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
    if (e.key === "Tab" && ui.sheet) trapFocus(e, $("#sheet"));
    if (e.key === "Escape") {
      if (ui.sheet) closeSheet();
      else if (ui.detailId) closeDetail();
      else if (ui.query) setQuery("");
    } else if (/^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement.tagName)) {
      // typing: leave keys alone
    } else if (e.key === "/") {
      e.preventDefault();
      $("#q").focus();
    } else if (isWide() && !ui.sheet && !e.metaKey && !e.ctrlKey && !e.altKey) {
      shortcut(e);
    }
  });
}

function trapFocus(e, panel) {
  const items = [...panel.querySelectorAll("button, input, select, textarea, a[href], summary")].filter((el) => !el.disabled && el.offsetParent);
  if (!items.length) return;
  const first = items[0], last = items[items.length - 1];
  if (!panel.contains(document.activeElement)) { e.preventDefault(); first.focus(); }
  else if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
  else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
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
  applyLayout();
  wideMQ.addEventListener("change", () => { applyLayout(); renderAll(); });
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

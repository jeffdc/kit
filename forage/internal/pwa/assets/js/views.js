// HTML for every screen. Functions take plain data and return strings;
// main.js puts them in the page and wires the events.

import * as ic from "./icons.js";
import {
  STATUS_LABEL, SORTS, formatDate, initial, wishlistBooks, readingBooks, historyGroups,
  orderSellers, sellerURL, localToday,
} from "./core.js";

export const esc = (s) =>
  String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

// A middle dot with even spacing (Gill Sans sets a literal " · " lopsided).
const SEP = '<span class="sep" aria-hidden="true">·</span>';
const joinFacts = (parts) => parts.filter(Boolean).map(esc).join(SEP);

const plural = (n, one, many = one + "s") => `${n.toLocaleString()} ${n === 1 ? one : many}`;

export function stars(n, size = "sm") {
  if (!n) return "";
  let s = "";
  for (let i = 1; i <= 5; i++) s += ic.star(i <= n);
  return `<span class="stars ${size}" role="img" aria-label="${n} of 5 stars">${s}</span>`;
}

const tag = (status) => `<span class="tag s-${status}">${STATUS_LABEL[status]}</span>`;

// --- Rows ---

// Every row keeps the owned slot, so the marks line up in one column.
function row(book, meta, pending) {
  return `<li><button class="row s-${book.status}" data-book="${esc(book.id)}">
    <span class="edge" aria-hidden="true"></span>
    <span class="row-body"><span class="row-title">${esc(book.title)}</span><span class="row-author">${esc(book.author)}</span></span>
    <span class="row-meta">${meta}${pending ? '<span class="unsynced">Not synced</span>' : ""}</span>
    <span class="row-own">${book.owned ? `${ic.owned()}<span class="sr">Owned</span>` : ""}</span>
  </button></li>`;
}

function sortSelect(tab, current, keys) {
  const opts = keys.map((k) => `<option value="${k}"${k === current ? " selected" : ""}>${k === "date" ? "Date read" : SORTS[k].label}</option>`).join("");
  return `<label class="sort"><span class="sr">Sort ${tab} by</span><select data-sort="${tab}">${opts}</select></label>`;
}

function shelfHead(tab, title, count, sortHTML) {
  return `<div class="shelf-head s-${tab}"><h1><span class="swatch" aria-hidden="true"></span>${title}</h1><span class="count">${count}</span>${sortHTML}</div>`;
}

// --- Tabs ---

export function wishlistView(view, sort, pending) {
  const books = wishlistBooks(view, sort);
  const head = shelfHead("wishlist", "Wishlist", plural(books.length, "book"), sortSelect("wishlist", sort, ["author", "title", "added", "year"]));
  if (!books.length) return head + empty("Nothing on the wishlist.", "Add a book with the + button.");
  const lettered = sort === "author" || sort === "title";
  let html = head + '<ul class="list">';
  let letter = "";
  for (const b of books) {
    if (lettered) {
      const l = initial(b, sort);
      if (l !== letter) {
        letter = l;
        html += `<li class="letter" aria-hidden="true">${l}</li>`;
      }
    }
    const meta = b.first_published ? `<span>${b.first_published}</span>` : "";
    html += row(b, meta, pending.has(b.id));
  }
  return html + "</ul>";
}

export function readingView(view, sort, pending) {
  const { reading, paused, toRead } = readingBooks(view, sort);
  let html = shelfHead("reading", "Reading", plural(reading.length, "book"), sortSelect("reading", sort, ["title", "author", "added"]));
  if (!reading.length && !paused.length && !toRead.length) return html + empty("You're not reading anything.", "Open a book and tap Start reading.");
  html += reading.length
    ? '<ul class="list">' + reading.map((b) => row(b, "", pending.has(b.id))).join("") + "</ul>"
    : '<p class="hint">Nothing in progress.</p>';
  const section = (status, title, books) => books.length
    ? `<h2 class="sub-head s-${status}"><span class="swatch" aria-hidden="true"></span>${title} <span class="count">${books.length}</span></h2>` +
      '<ul class="list">' + books.map((b) => row(b, "", pending.has(b.id))).join("") + "</ul>"
    : "";
  return html + section("paused", "Paused", paused) + section("to_read", "To read", toRead);
}

export function historyView(view, sort, pending) {
  const groups = historyGroups(view, sort);
  const total = groups.reduce((n, g) => n + g.count, 0);
  let html = shelfHead("read", "History", plural(total, sort === "date" ? "read" : "book"), sortSelect("history", sort, ["date", "author", "title", "rating"]));
  if (!total) return html + empty("No finished books yet.", "When you finish a book, open it and tap Finished.");
  const max = Math.max(...groups.filter((g) => g.key !== "undated").map((g) => g.count), 1);
  for (const g of groups) {
    html += '<section class="group">';
    if (g.label) {
      const bar = g.key === "undated" ? "" : `<span class="year-bar" style="--w:${(g.count / max) * 100}%" aria-hidden="true"></span>`;
      html += `<h2 class="year"><span class="year-label">${g.label}</span>${bar}<span class="count">${g.count}</span></h2>`;
      if (g.key === "undated") html += `<p class="hint">Finish dates unknown. Open a book to add one.</p>`;
    }
    html += '<ul class="list">';
    for (const { book, read } of g.entries) {
      const d = read && read.date_finished ? `<span class="date">${formatDate(read.date_finished, sort !== "date")}</span>` : "";
      html += row(book, d + stars(book.rating), pending.has(book.id));
    }
    html += "</ul></section>";
  }
  return html;
}

export function searchView(results, query, pending) {
  const head = `<div class="shelf-head"><h1 class="search-title">${plural(results.length, "match", "matches")} in your library</h1></div>`;
  if (!results.length) {
    return head + empty(`Nothing in your library matches “${esc(query)}”.`, "", `<button class="btn" data-act="add-from-search">Look it up to add</button>`);
  }
  return head + '<ul class="list">' + results.slice(0, 200).map((b) => {
    const read = b.date_read ? `<span class="date">${formatDate(b.date_read)}</span>` : "";
    return row(b, tag(b.status) + read + stars(b.rating), pending.has(b.id));
  }).join("") + "</ul>";
}

function empty(title, body, action = "") {
  return `<div class="empty"><p class="empty-title">${title}</p>${body ? `<p>${body}</p>` : ""}${action}</div>`;
}

// --- Book detail: the cover ---

function primaryActions(b) {
  const btn = (act, label, kind = "") => `<button class="band-btn ${kind}" data-act="${act}">${label}</button>`;
  switch (b.status) {
    case "wishlist": return btn("start", "Start reading", "primary") + btn("got-it", "Got it");
    case "to_read": return btn("start", "Start reading", "primary") + btn("wishlist", "Back to wishlist");
    case "reading": return btn("finish", "Finished", "primary") + btn("pause", "Pause");
    case "paused": return btn("resume", "Resume", "primary") + btn("finish", "Finished");
    case "read": return btn("reread", "Read again", "primary");
    case "dropped": return btn("undrop", "Back to wishlist", "primary");
    default: return "";
  }
}

function finishPanel(b, rating) {
  let picker = "";
  for (let i = 1; i <= 5; i++) {
    picker += `<button type="button" class="star-btn" data-rate="${i}" aria-label="${i} star${i > 1 ? "s" : ""}" aria-pressed="${i <= rating}">${ic.star(i <= rating)}</button>`;
  }
  return `<form class="finish" data-form="finish">
    <label class="finish-date"><span>Finished on</span><input type="date" name="date" value="${localToday()}" max="${localToday()}" required></label>
    <div class="finish-stars" role="group" aria-label="Rating">${picker}</div>
    <div class="finish-actions"><button class="band-btn primary" type="submit">Log it</button><button class="band-btn" type="button" data-act="cancel-finish">Cancel</button></div>
  </form>`;
}

function ratingControl(b) {
  let s = "";
  for (let i = 1; i <= 5; i++) {
    s += `<button type="button" class="star-btn" data-act="rate" data-rate="${i}" aria-label="Rate ${i} star${i > 1 ? "s" : ""}" aria-pressed="${i <= (b.rating || 0)}">${ic.star(i <= (b.rating || 0))}</button>`;
  }
  return `<div class="cover-rating" role="group" aria-label="Rating">${s}</div>`;
}

export function detailView(b, reads, sellers, ui) {
  const facts = joinFacts([b.first_published, b.page_count ? `${b.page_count} pages` : ""]);
  const readCount = reads.length;
  const topRight = readCount
    ? joinFacts([plural(readCount, "read"), b.date_read ? "last " + formatDate(b.date_read) : ""])
    : b.status === "wishlist" || b.status === "to_read" ? `Added ${formatDate(b.date_added)}` : "";
  const bottom = ui.finishing ? finishPanel(b, b.rating || 0) : `<div class="band-actions">${primaryActions(b)}</div>`;

  const readRows = reads.slice().reverse().map((r) => `<li class="read-row">
      <label><span class="sr">Date finished</span><input type="date" data-read="${esc(r.id)}" value="${esc(r.date_finished)}" max="${localToday()}"></label>
      ${r.date_finished ? "" : '<span class="undated">Date unknown</span>'}
      <button class="icon-btn" data-act="delete-read" data-read="${esc(r.id)}" aria-label="Delete this read">${ic.trash()}</button>
    </li>`).join("");

  const sellerRows = orderSellers(sellers).map((s) =>
    `<li><a class="seller" href="${esc(sellerURL(s, b))}" target="_blank" rel="noopener">${esc(s.name)}${ic.external()}</a></li>`).join("");

  return `<article class="detail" aria-labelledby="detail-title">
    <div class="detail-bar"><button class="icon-btn" data-act="close" aria-label="Close"><span class="when-phone">${ic.back()}</span><span class="when-drawer">${ic.close()}</span></button>
      <button class="icon-btn" data-act="edit" aria-label="Edit details">${ic.pencil()}</button></div>
    <div class="cover s-${b.status}${ui.sweep ? " sweep" : ""}" style="${ui.sweep ? `--from: var(--band-${ui.sweep})` : ""}">
      <div class="band band-top"><span class="band-label">${STATUS_LABEL[b.status]}</span><span class="band-info">${topRight}</span></div>
      <div class="cover-mid">
        <h2 id="detail-title" class="cover-title">${esc(b.title)}</h2>
        <span class="cover-rule" aria-hidden="true"></span>
        <p class="cover-author">${esc(b.author)}</p>
        ${facts ? `<p class="cover-facts">${facts}</p>` : ""}
        ${ui.finishing ? "" : ratingControl(b)}
        ${ownToggle("toggle-owned", b.owned)}
        ${b.pending ? '<p class="unsynced">Not synced yet</p>' : ""}
      </div>
      <div class="band band-bottom">${bottom}</div>
    </div>

    <section class="detail-sec"><h3>Reads</h3>
      ${readCount ? `<ul class="reads">${readRows}</ul>` : '<p class="muted">Not read yet.</p>'}
      <button class="btn-quiet" data-act="add-read">${ic.plus()}Add an earlier read</button>
    </section>

    ${sellerRows ? `<section class="detail-sec"><h3>Find a copy</h3><ul class="sellers">${sellerRows}</ul></section>` : ""}

    <section class="detail-sec"><h3>Details</h3>
      <dl class="facts">
        <dt>Added</dt><dd>${formatDate(b.date_added) || "—"}</dd>
        ${b.isbn ? `<dt>ISBN</dt><dd class="num">${esc(b.isbn)}</dd>` : ""}
        ${b.tags && b.tags.length ? `<dt>Tags</dt><dd>${esc(b.tags.join(", "))}</dd>` : ""}
      </dl>
      ${b.body ? `<p class="notes">${esc(b.body)}</p>` : ""}
      <div class="detail-actions">
        <button class="btn" data-act="edit">${ic.pencil()}Edit details</button>
        ${b.status === "dropped" ? "" : `<button class="btn" data-act="drop">Abandon</button>`}
        <button class="btn danger${ui.confirmDelete ? " armed" : ""}" data-act="delete">${ic.trash()}${ui.confirmDelete ? "Tap again to delete" : "Delete"}</button>
      </div>
    </section>
  </article>`;
}

/** The owned switch: a square box, filled and checked when on. */
function ownToggle(act, on, label = "I own a copy") {
  return `<button type="button" class="own-toggle" data-act="${act}" aria-pressed="${!!on}"><span class="own-box" aria-hidden="true">${ic.check()}</span>${label}</button>`;
}

// --- Sheets ---

const ADD_STATUSES = ["wishlist", "to_read", "reading", "read"];

export function addView(ui) {
  const statuses = ADD_STATUSES;
  const seg = statuses.map((s) => `<button type="button" class="seg-btn s-${s}" data-add-status="${s}" aria-pressed="${ui.status === s}">${STATUS_LABEL[s]}</button>`).join("");
  return `<div class="sheet-head"><h2 id="sheet-title">Add a book</h2><button class="icon-btn" data-act="close-sheet" aria-label="Close">${ic.close()}</button></div>
    <form class="resolve" data-form="resolve">
      <label for="add-q" class="field-label">Link, ISBN, or title and author</label>
      <div class="resolve-row">
        <input id="add-q" name="q" type="text" inputmode="url" autocomplete="off" autocapitalize="off" spellcheck="false" placeholder="Paste a link or type a title" value="${esc(ui.q)}">
        <button type="button" class="icon-btn boxed" data-act="paste" aria-label="Paste">${ic.paste()}</button>
      </div>
      <button class="btn primary wide" type="submit"${ui.loading ? " disabled" : ""}>${ui.loading ? "Looking it up…" : "Find"}</button>
    </form>
    <div class="seg" role="group" aria-label="Add as">${seg}</div>
    <div class="add-own">${ownToggle("toggle-add-owned", ui.owned)}</div>
    <div id="add-results" aria-live="polite">${addResults(ui)}</div>
    <button class="btn-quiet" data-act="manual">${ic.pencil()}Enter it by hand</button>`;
}

export function addResults(ui) {
  if (ui.error) return `<p class="error">${esc(ui.error)}</p>`;
  if (!ui.result) return "";
  const r = ui.result;
  if (!r.candidates.length) {
    return `<p class="muted">No book found${r.text ? ` for “${esc(r.text.slice(0, 140))}”` : ""}. Try a title, or enter it by hand.</p>`;
  }
  return '<ul class="candidates">' + r.candidates.map((c, i) => {
    const facts = joinFacts([c.first_published, c.page_count ? `${c.page_count} pages` : ""]);
    const m = c.match;
    const inLib = m
      ? `<span class="match s-${m.status}">${m.status === "read" ? `Read${m.date_read ? " " + formatDate(m.date_read) : ""}` : m.status === "wishlist" ? "On your wishlist" : STATUS_LABEL[m.status]}${m.owned ? `${SEP}Owned` : ""}${stars(m.rating)}</span>`
      : "";
    const act = m
      ? `<button class="btn" data-act="open-match" data-book="${esc(m.book_id)}">Open</button>`
      : `<button class="btn primary" data-act="add-candidate" data-i="${i}">Add</button>`;
    return `<li class="candidate"><div><p class="row-title">${esc(c.title)}</p><p class="row-author">${esc(c.author || "Unknown author")}</p>${facts ? `<p class="small">${facts}</p>` : ""}${inLib}</div>${act}</li>`;
  }).join("") + "</ul>";
}

const field = (name, label, value, attrs = "") =>
  `<label class="field"><span class="field-label">${label}</span><input name="${name}" value="${esc(value)}" ${attrs}></label>`;

export function manualView(ui) {
  const f = ui.manual || {};
  return `<div class="sheet-head"><h2 id="sheet-title">Add by hand</h2><button class="icon-btn" data-act="close-sheet" aria-label="Close">${ic.close()}</button></div>
    <form data-form="manual" class="form">
      ${field("title", "Title", f.title, 'required autocomplete="off"')}
      ${field("author", "Author", f.author, 'required autocomplete="off"')}
      <div class="field-pair">${field("first_published", "Year published", f.first_published, 'inputmode="numeric" pattern="[0-9]*"')}${field("page_count", "Pages", f.page_count, 'inputmode="numeric" pattern="[0-9]*"')}</div>
      ${field("isbn", "ISBN", f.isbn, 'inputmode="numeric" autocomplete="off"')}
      <p class="field-label">Add as</p>
      <div class="seg" role="group" aria-label="Add as">${ADD_STATUSES.map((s) => `<button type="button" class="seg-btn s-${s}" data-add-status="${s}" aria-pressed="${ui.status === s}">${STATUS_LABEL[s]}</button>`).join("")}</div>
      ${ownToggle("toggle-add-owned", ui.owned)}
      ${ui.status === "read" ? field("date_finished", "Finished on (leave empty if unknown)", localToday(), `type="date" max="${localToday()}"`) : ""}
      <button class="btn primary wide" type="submit">Add</button>
    </form>`;
}

export function editView(b) {
  return `<div class="sheet-head"><h2 id="sheet-title">Edit details</h2><button class="icon-btn" data-act="close-sheet" aria-label="Close">${ic.close()}</button></div>
    <form data-form="edit" class="form">
      ${field("title", "Title", b.title, "required")}
      ${field("author", "Author", b.author, "required")}
      ${field("sort_author", "Sort under", b.sort_author, 'placeholder="Surname, First"')}
      <div class="field-pair">${field("first_published", "Year published", b.first_published || "", 'inputmode="numeric" pattern="[0-9]*"')}${field("page_count", "Pages", b.page_count || "", 'inputmode="numeric" pattern="[0-9]*"')}</div>
      ${field("isbn", "ISBN", b.isbn, 'inputmode="numeric"')}
      ${field("date_added", "Added on", b.date_added, `type="date" max="${localToday()}"`)}
      ${field("tags", "Tags (comma separated)", (b.tags || []).join(", "))}
      <label class="field"><span class="field-label">Notes</span><textarea name="body" rows="4">${esc(b.body)}</textarea></label>
      <button class="btn primary wide" type="submit">Save</button>
    </form>`;
}

const SYNC_TEXT = {
  idle: "Not synced yet", syncing: "Syncing…", synced: "Synced", offline: "Offline",
  error: "Sync failed", auth: "Key refused", nokey: "No API key",
};

export function syncLabel(sync, pendingCount) {
  if (sync.status === "synced" && pendingCount) return `${pendingCount} to sync`;
  if ((sync.status === "offline" || sync.status === "error") && pendingCount) return `${pendingCount} waiting`;
  return SYNC_TEXT[sync.status] || "";
}

export function settingsView(st, ui) {
  const pending = st.ops.length;
  const dropped = [...st.view.books.values()].filter((b) => b.status === "dropped");
  const undated = [...st.view.reads.values()].filter((r) => !r.date_finished).length;
  const rejected = st.sync.rejected.map((r, i) => `<li><p>${esc(describeOp(r.op, st))}</p><p class="error">${esc(r.error)}</p><button class="btn-quiet" data-act="dismiss-rejected" data-i="${i}">Dismiss</button></li>`).join("");
  return `<div class="sheet-head"><h2 id="sheet-title">Settings</h2><button class="icon-btn" data-act="close-sheet" aria-label="Close">${ic.close()}</button></div>
    <section class="detail-sec"><h3>Sync</h3>
      <p class="sync-line s-sync-${st.sync.status}"><span class="dot" aria-hidden="true"></span>${esc(syncLabel(st.sync, pending))}</p>
      ${st.sync.detail ? `<p class="muted">${esc(st.sync.detail)}</p>` : ""}
      <p class="muted">${pending ? plural(pending, "change") + " waiting to send. " : ""}${st.sync.lastSync ? "Last synced " + new Date(st.sync.lastSync).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }) + "." : ""}</p>
      <button class="btn" data-act="sync-now">Sync now</button>
      ${rejected ? `<h3 class="sub">Changes the server refused</h3><ul class="rejected">${rejected}</ul>` : ""}
    </section>
    <section class="detail-sec"><h3>API key</h3>
      <form data-form="key" class="form inline">
        <label class="field"><span class="field-label">Key for this device</span><input name="key" type="password" autocomplete="off" value="${esc(ui.key)}" placeholder="Paste your key"></label>
        <button class="btn primary" type="submit">Save key</button>
      </form>
    </section>
    <section class="detail-sec"><h3>Library</h3>
      <p class="muted">${joinFacts([plural(st.view.books.size, "book"), plural(undated, "undated read") + " (listed at the end of History)"])}</p>
      ${dropped.length ? `<details class="dropped"><summary>Abandoned books (${dropped.length})</summary><ul class="list">${dropped.map((b) => row(b, "", false)).join("")}</ul></details>` : ""}
    </section>
    <p class="build muted">Build ${esc(ui.build || "…")}</p>`;
}

function describeOp(op, st) {
  const book = st.view.books.get(op.id || (op.book && op.book.id) || (op.read && op.read.book_id));
  const what = { "book.create": "Add", "book.update": "Edit", "book.delete": "Delete", "read.create": "Log read", "read.update": "Change read date", "read.delete": "Delete read" }[op.type] || op.type;
  return `${what}${book ? ": " + book.title : op.book ? ": " + op.book.title : ""}`;
}

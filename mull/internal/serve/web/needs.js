// The Needs you schedule: questions, reviews and decisions, in numbered
// tiers ranked by what an answer unblocks. The tiers and their order come
// from the API; this only draws them. Approving decisions writes (see
// write.js); the other actions copy a prompt or a mull command for the
// human to run.

import { esc, href, CHEVRON, itemStage, stageLabel, shortTitle, plural, clock } from "./html.js";
import { within, runs } from "./revision.js";
import { writeButton, failure } from "./write.js";

// Tiers open by default: the ones that hold work up.
const OPEN = new Set(["blocking", "review", "now"]);

// opts: pid; changes (a Changes, or none); noPlan / noPhase leave out
// "where" links the sheet already says. Tiers and groups carry data-open,
// so the page can keep what the human opened across live re-renders.
export function schedule(ny, opts) {
  const tiers = ny?.tiers || [];
  if (!tiers.length) return `<p class="empty">Nothing needs you here.</p>`;
  return `<div class="sch">${tiers.map(t => tier(t, opts)).join("")}</div>`;
}

function tier(t, opts) {
  const key = `tier:${opts.pid}:${t.key}`;
  const open = OPEN.has(t.key);
  const num = String(t.rank).padStart(2, "0");
  let body, count;
  if (t.key === "decide") {
    const groups = t.groups || [];
    count = `${t.count} in ${plural(groups.length, "group")}`;
    body = groups.map(g => group(g, opts)).join("");
  } else {
    const entries = t.entries || [];
    count = String(t.count);
    body = runs(entries.map((e, i) => row(e, `${t.rank}.${i + 1}`, t.key, opts)), (h, cl) => `<div class="clrun${cl}">${h}</div>`);
    if (entries.length > 1 && t.key !== "review") {
      body += `<div class="foot-act"><button class="btn s" data-copy="${esc(entries.map(e => e.id).join(", "))}">Copy all ${entries.length} for an agent</button></div>`;
    }
  }
  return `<details class="tier t-${esc(t.key)}" data-open="${esc(key)}"${open ? " open" : ""}><summary><span class="n">${num}</span>${CHEVRON}<b>${esc(t.title)}<em>${esc(count)}</em></b><small>${esc(t.hint)}</small></summary>${body}</details>`;
}

// where a row points: its item, else its phase, else its plan.
function rowHref(pid, e) {
  if (e.item) return href.item(pid, e.item);
  if (e.phase) return href.phase(pid, e.phase);
  if (e.plan) return href.plan(pid, e.plan);
  return "";
}

// cloudsOf is what clouds a Needs you row: the entry itself, or for a
// review, the item it reviews.
function cloudsOf(e, changes) {
  if (!changes) return "";
  const review = e.kind === "review" && e.item ? within.item(e.item) : () => false;
  const self = within.entry(e.id);
  return changes.cls(c => self(c) || review(c));
}

function row(e, n, tierKey, opts) {
  const pid = opts.pid, link = rowHref(pid, e), review = tierKey === "review";
  const where = [
    !opts.noPlan && e.plan ? `<a href="${esc(href.plan(pid, e.plan))}">${esc(shortTitle(e.plan_title || e.plan))}</a>` : "",
    !opts.noPhase && e.phase ? `<a href="${esc(href.phase(pid, e.phase))}">${esc(e.phase_title || e.phase)}</a>` : "",
    e.item && !review ? `on <a href="${esc(href.item(pid, e.item))}"><code>${esc(e.item)}</code></a> ${esc(stageLabel(itemStage({ state: e.item_state, ready: e.item_ready })))}` : "",
    review && e.decisions?.length ? `${plural(e.decisions.length, "decision")} inside` : "",
    review ? `in review since ${esc(clock(e.created))}` : "",
    e.source && !review ? `from ${esc(e.source)}` : "",
  ].filter(Boolean).join(" · ");
  const title = review && e.item ? e.item_title || e.text : e.text;
  const text = review && link ? `<a href="${esc(link)}">${esc(title)}</a>` : esc(title);
  const act = review
    ? (link ? `<a class="btn s" href="${esc(link)}">Review</a>` : "")
    : `<button class="btn s${tierKey === "blocking" ? " p" : ""}" data-copy="${esc(e.id)}">Answer</button>`;
  const code = review && e.item ? e.item : e.id;
  return [`<div class="it"${link ? ` data-href="${esc(link)}"` : ""}><span class="n">${esc(n)}</span><code>${esc(code)}</code>` +
    `<div><p${e.detail ? ` title="${esc(e.detail)}"` : ""}>${text}</p>${where ? `<span class="w">${where}</span>` : ""}</div><div class="act">${act}</div></div>`,
    cloudsOf(e, opts.changes)];
}

// group: decisions from one source. A group carried by an open review is
// settled by reviewing it; the others are approved here, all at once or
// one by one.
function group(g, opts) {
  const pid = opts.pid, key = `grp:${pid}:${g.key}`;
  const ids = g.entries.map(e => e.id);
  const ch = opts.changes;
  const cloud = ch ? ch.cls(c => ids.includes(c.id) && c.type === "entry") : "";
  const name = g.item ? `<code>${esc(g.item)}</code> ${esc(g.title)}` : esc(g.title);
  const carried = g.review && g.item;
  const all = `approve:${pid}:${g.key}`;
  const act = carried
    ? `<a class="btn s" href="${esc(href.item(pid, g.item))}">Review</a>`
    : ids.length === 1
      ? approve(pid, all, ids, "Approve", "Approve?", `Approved decision ${ids[0]}.`)
      : approve(pid, all, ids, "Approve all", `Approve ${ids.length}?`, `Approved ${plural(ids.length, "decision")}.`);
  // A group of one is approved from its header alone.
  const own = !carried && ids.length > 1;
  const items = runs(g.entries.map(e => [decision(pid, e, own), ch ? ch.cls(within.entry(e.id)) : ""]),
    (h, cl) => `<ul class="clrun${cl}">${h}</ul>`, h => `<ul>${h}</ul>`);
  // Closed, the group carries the cloud; open, its changed rows do.
  return `<details class="grp${cloud}" data-open="${esc(key)}"><summary>${CHEVRON}<span>${name}${carried ? ` <small>carried by its review</small>` : ""}</span><span class="sp"></span><span class="mono mut">${g.entries.length}</span>${act}</summary>${items}</details>` +
    failure(all, `Approving ${plural(ids.length, "decision")}`);
}

// decision is one decision in a group, with its own approve if approvable
// (not carried by a review, and not alone in its group).
function decision(pid, e, approvable) {
  const key = `approve:${pid}:${e.id}`;
  const act = approvable ? approve(pid, key, [e.id], "Approve", "Approve?", `Approved decision ${e.id}.`) : "";
  return `<li><code>${esc(e.id)}</code><span${e.detail ? ` title="${esc(e.detail)}"` : ""}>${esc(e.text)}</span>${act}${failure(key, `Approving ${e.id}`)}</li>`;
}

const approve = (pid, key, ids, label, ask, done) =>
  writeButton({ pid, key, ops: [{ op: "accept", ids }], label, ask, done });

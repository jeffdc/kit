// Needs you, drawn two ways. On a project's surface it's the calls (GET
// …/surface): high-risk work waiting, questions with the agent's default,
// and landed work to look at, each settled on its row. On a plan's, a
// phase's or Unplanned's sheet it's the schedule (GET …'s needs_you): the
// open questions and reviews there, in numbered tiers ranked by what an
// answer unblocks. The tiers and their order come from the API; this only
// draws them. Agents' decisions no longer come to the human, so a decide
// tier left from before isn't drawn. Settling writes (see write.js); the
// rest copy a prompt that hands the entries to an agent.

import { esc, href, CHEVRON, itemStage, stageLabel, shortTitle, clock, copyId } from "./html.js";
import { within, runs } from "./revision.js";
import { writeButton, textWrite, writeState, moreOpen, moreButton, failure } from "./write.js";

// Tiers open by default: the ones that hold work up.
const OPEN = new Set(["blocking", "review", "now"]);

// tierBox draws one numbered tier: its header (number, title, count, hint)
// and its rows.
const tierBox = ({ key, cls, rank, title, count, hint, open, body }) =>
  `<details class="tier t-${esc(cls)}" data-open="${esc(key)}"${open ? " open" : ""}><summary><span class="n">${String(rank).padStart(2, "0")}</span>${CHEVRON}<b>${esc(title)}<em>${esc(count)}</em></b><small>${esc(hint)}</small></summary>${body}</details>`;

// ---- the surface's calls

// What each kind of call is, in the order they're acted on (the API sends
// them in this order too).
const CALLS = [
  ["review", "High-risk work waiting", "It lands when you accept it"],
  ["question", "Questions", "The agent's default stands if you don't answer"],
  ["look", "Landed: take a look", "Medium risk or sampled, already on its branch"],
];

// calls draws the surface's Needs you: a tier per kind, every row with its
// own writes. landed is the flow's landed list, for a look's summary.
export function calls(pid, list, landed) {
  if (!list.length) return `<p class="empty">Nothing needs you.</p>`;
  const summary = new Map((landed || []).map(l => [l.item, l]));
  const tiers = CALLS.map(([kind, title, hint], i) => {
    const rows = list.filter(c => c.kind === kind);
    if (!rows.length) return "";
    return tierBox({ key: `calls:${pid}:${kind}`, cls: kind, rank: i + 1, title, count: String(rows.length), hint, open: true,
      body: rows.map((c, j) => callRow(pid, c, `${i + 1}.${j + 1}`, summary.get(c.item))).join("") });
  }).join("");
  return `<div class="sch">${tiers}</div>`;
}

// An entry's text names its kind ("Review: …", "Look: …"); the tier says
// that already.
const bare = text => String(text || "").replace(/^(Review|Look):\s*/, "");

function callRow(pid, c, n, landing) {
  const link = c.item ? href.item(pid, c.item) : c.plan ? href.plan(pid, c.plan) : "";
  const title = link ? `<a href="${esc(link)}">${esc(bare(c.text))}</a>` : esc(bare(c.text));
  const plan = c.plan ? `<a href="${esc(href.plan(pid, c.plan))}">${esc(shortTitle(c.plan))}</a>` : "";
  let where, act, form = "", fail = "";
  if (c.kind === "question") {
    const stands = c.lapsed ? `lapsed ${esc(clock(c.due))}: the default stands` : c.due ? `stands ${esc(clock(c.due))}` : "";
    where = [c.default ? `<span class="lab">Default</span> ${esc(c.default)}` : `<span class="lab">No default given</span>`, stands, plan].filter(Boolean).join(" · ");
    const dd = textWrite({ pid, key: `answer:${pid}:${c.entry}`, ops: [{ op: "decide", id: c.entry }], field: "decision",
      label: c.default ? "Decide differently" : "Answer", save: "Save answer",
      placeholder: "Your answer. It joins the decision log, and the agent works from it.", done: `Answered ${c.entry}. It's in the decision log.` });
    const take = c.default ? writeButton({ pid, key: `default:${pid}:${c.entry}`, ops: [{ op: "decide", id: c.entry, decision: c.default }],
      label: "Take the default", ask: "Take it?", done: `Took the default for ${c.entry}: ${c.default}.` }) : "";
    act = take + dd.button;
    form = dd.form;
    fail = failure(`default:${pid}:${c.entry}`, `Taking the default for ${c.entry}`);
  } else {
    const review = c.kind === "review";
    const band = c.why === "sampled" ? "low risk, sampled" : c.why ? `${c.why} risk` : "";
    const why = review ? c.why : [landing?.summary, band].filter(Boolean).join(" · ");
    where = [why ? esc(why) : "", plan, `${review ? "waiting since" : "landed"} ${esc(clock(landing?.at || c.created))}`].filter(Boolean).join(" · ");
    const key = `${review ? "accept" : "fine"}:${pid}:${c.entry}`;
    const ok = review
      ? writeButton({ pid, key, ops: [{ op: "accept", ids: [c.entry] }], label: "Accept", ask: "Accept and land it?", done: `Accepted ${c.item || c.entry}.` })
      : writeButton({ pid, key, ops: [{ op: "accept", ids: [c.entry] }], label: "Looks fine", ask: "", done: `Looked at ${c.item || c.entry}: fine.`, confirm: false });
    const back = textWrite({ pid, key: `sendback:${pid}:${c.entry}`, ops: [{ op: "do", id: c.entry }], field: "reason",
      label: "Send back", save: "Send it back",
      placeholder: review ? "What's wrong. It goes into the item's notes, and the item reopens for another session."
        : "What's wrong. It becomes a follow-up item for an agent; the work stays where it landed.",
      done: review ? `Sent ${c.item || c.entry} back. It reopens for another session.` : `Sent ${c.item || c.entry} back: a follow-up item is filed.` });
    act = back.button + ok;
    form = back.form;
    fail = failure(key, `${review ? "Accepting" : "Marking"} ${c.item || c.entry}`);
  }
  return `<div class="it"${link ? ` data-href="${esc(link)}"` : ""}><span class="n">${esc(n)}</span>${copyId(c.item || c.entry)}` +
    `<div><p>${title}</p><span class="w">${where}</span></div><div class="act">${act}</div>${fail}${form}</div>`;
}

// ---- a sheet's schedule

// opts: pid; changes (a Changes, or none); noPlan / noPhase leave out
// "where" links the sheet already says. Tiers carry data-open, so the page
// can keep what the human opened across live re-renders.
export function schedule(ny, opts) {
  const tiers = (ny?.tiers || []).filter(t => t.key !== "decide");
  if (!tiers.length) return `<p class="empty">Nothing needs you here.</p>`;
  return `<div class="sch">${tiers.map(t => tier(t, opts)).join("")}</div>`;
}

function tier(t, opts) {
  const entries = t.entries || [];
  let body = runs(entries.map((e, i) => row(e, `${t.rank}.${i + 1}`, t.key, opts)), (h, cl) => `<div class="clrun${cl}">${h}</div>`);
  if (entries.length > 1 && t.key !== "review") {
    body += `<div class="foot-act"><button class="btn s" data-copy="${esc(entries.map(e => e.id).join(", "))}">Copy all ${entries.length} for an agent</button></div>`;
  }
  return tierBox({ key: `tier:${opts.pid}:${t.key}`, cls: t.key, rank: t.rank, title: t.title, count: String(t.count), hint: t.hint, open: OPEN.has(t.key), body });
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

// row is one open entry. A review is settled on the project's surface, so
// here it's a link to the item; a question is answered here.
function row(e, n, tierKey, opts) {
  const pid = opts.pid, link = rowHref(pid, e), review = tierKey === "review";
  const where = [
    !opts.noPlan && e.plan ? `<a href="${esc(href.plan(pid, e.plan))}">${esc(shortTitle(e.plan_title || e.plan))}</a>` : "",
    !opts.noPhase && e.phase ? `<a href="${esc(href.phase(pid, e.phase))}">${esc(e.phase_title || e.phase)}</a>` : "",
    e.item && !review ? `on ${copyId(e.item, `<a href="${esc(href.item(pid, e.item))}"><code>${esc(e.item)}</code></a>`)} ${esc(stageLabel(itemStage({ state: e.item_state, ready: e.item_ready })))}` : "",
    review ? `since ${esc(clock(e.created))}` : "",
    e.source && !review ? `from ${esc(e.source)}` : "",
  ].filter(Boolean).join(" · ");
  const title = review && e.item ? e.item_title || e.text : e.text;
  let act, form = "";
  if (review) {
    act = `<a class="btn s" href="${esc(href.project(pid))}" data-needs title="Accept or send it back on the project's surface">On the surface</a>`;
  } else {
    const ans = textWrite({ pid, key: `answer:${pid}:${e.id}`, ops: [{ op: "decide", id: e.id }], field: "decision",
      label: "Answer", save: "Save answer", placeholder: "Your answer. It joins the decision log, and the agent works from it.",
      done: `Answered ${e.id}. It's in the decision log.` });
    const rest = parkOrDrop(pid, e.id, "needs");
    act = `<button class="btn s" data-copy="${esc(e.id)}" title="Copies a prompt to work through it with an agent">Discuss with an agent</button>${ans.button}${rest.buttons}`;
    form = ans.form + rest.forms;
  }
  const code = review && e.item ? e.item : e.id;
  return [`<div class="it"${link ? ` data-href="${esc(link)}"` : ""}><span class="n">${esc(n)}</span>${copyId(code)}` +
    `<div><p${e.detail ? ` title="${esc(e.detail)}"` : ""}>${esc(title)}</p>${where ? `<span class="w">${where}</span>` : ""}</div><div class="act">${act}</div>${form}</div>`,
    cloudsOf(e, opts.changes)];
}

// parkOrDrop are an entry's rarer writes, behind More: park sets it aside
// (it leaves Needs you, and is only counted), drop settles it for good.
// Both take a reason; the drop form says it can't be undone.
function parkOrDrop(pid, id, where) {
  const k = `${where}:${pid}:${id}`;
  const park = textWrite({ pid, key: `park:${k}`, ops: [{ op: "park", id }], field: "reason",
    label: "Park", save: "Park it", done: `Parked ${id}. It's counted as parked, out of Needs you.`,
    placeholder: "Why it can wait. Parked, it leaves Needs you and is only counted." });
  const drop = textWrite({ pid, key: `drop:${k}`, ops: [{ op: "drop", ids: [id] }], field: "reason",
    label: "Drop", save: "Drop it for good", done: `Dropped ${id}.`,
    placeholder: "Why it isn't needed. Dropping can't be undone." });
  const open = moreOpen(`more:${k}`) || writeState(`park:${k}`).open || writeState(`drop:${k}`).open;
  return { buttons: (open ? park.button + drop.button : "") + moreButton(`more:${k}`, open), forms: park.form + drop.form };
}

// Revisions: what changed since the human last looked, as GET …/revision
// lists it. Changes decides what every sheet clouds; the revision schedule
// lists the changes themselves.

import { esc, href, stageLabel, clock, plural, CHEVRON } from "./html.js";

const captured = c => c.type === "item" && c.state === "captured";
const identity = c => `${c.type}:${c.id}:${c.at}:${c.change}`;

// Matchers: which changes fall inside a record or a list. A captured item
// belongs to the backlog, not to the plan or phase it's tagged to.
export const within = {
  all: () => () => true,
  item: id => c => c.type === "item" && c.id === id,
  entry: id => c => c.type === "entry" && c.id === id,
  phase: id => c => ((c.type === "phase" || c.type === "phase_doc") && c.id === id) || (c.phase === id && !captured(c)),
  plan: slug => c => c.plan === slug && !captured(c),
  planRecord: slug => c => (c.type === "plan" || c.type === "plan_doc") && c.id === slug,
  unplanned: () => c => c.type === "item" && !c.plan && !captured(c),
  backlog: () => captured,
};

// Changes is one project's changes since the last look. Changes that
// weren't there the last time the project was drawn are fresh: their
// clouds are drawn in, once.
export class Changes {
  constructor(revision, before) {
    this.revision = revision;
    this.list = revision?.changes || [];
    const old = before ? new Set(before.list.map(identity)) : null;
    this.fresh = new Set(old ? this.list.map(identity).filter(k => !old.has(k)) : []);
  }
  // cls is the class list for something that holds the changes pred
  // matches: " cl", " cl fresh", or nothing.
  cls(pred) { return this.clsOf(this.list.filter(pred)); }
  clsOf(list) {
    if (!list.length) return "";
    return list.some(c => this.fresh.has(identity(c))) ? " cl fresh" : " cl";
  }
  // settle ends the fresh state once the clouds have been drawn in. An
  // event re-renders more than once (the list, then the project), so fresh
  // lasts as long as the draw, not just one render.
  settle() {
    if (this.fresh.size && !this.timer) this.timer = setTimeout(() => this.fresh.clear(), 750);
  }
}

// The revision schedule: one row per changed record, newest first.
export function revisionSchedule(pid, rev, entryHref) {
  const changes = rev?.changes || [];
  if (!changes.length) return "";
  const rows = changes.map(c => {
    const link = c.change === "removed" ? "" : target(pid, c, entryHref);
    const title = esc(recordTitle(c));
    return `<tr${link ? ` data-href="${esc(link)}"` : ""}><td><code>${esc(c.type === "plan" || c.type === "plan_doc" ? "plan" : c.id)}</code></td>` +
      `<td class="ttl">${link ? `<a href="${esc(link)}">${title}</a>` : title}</td><td class="mut">${esc(describe(c))}</td><td class="n mono mut">${esc(clock(c.at))}</td></tr>`;
  }).join("");
  return `<section><details class="revs" data-open="revs:${esc(pid)}"><summary class="hd"><h2>${CHEVRON} Revision ${rev.tip.number}</h2><span class="lab">${plural(changes.length, "change")} since you last looked, at ${esc(clock(rev.seen_at))}</span></summary>
    <div class="tbl-wrap"><table class="t"><thead><tr><th style="width:70px">ID</th><th>Record</th><th>What changed</th><th class="n">At</th></tr></thead><tbody>${rows}</tbody></table></div></details></section>`;
}

function recordTitle(c) {
  if (c.type === "plan_doc") return `${c.title}: plan doc`;
  if (c.type === "phase_doc") return `${c.title}: phase doc`;
  return c.title;
}

function target(pid, c, entryHref) {
  switch (c.type) {
    case "item": return href.item(pid, c.id);
    case "phase": case "phase_doc": return href.phase(pid, c.id);
    case "plan": case "plan_doc": return href.plan(pid, c.id);
    case "entry": return entryHref(c.id) || (c.plan ? href.plan(pid, c.plan) : "");
  }
  return "";
}

const base = path => String(path || "").split("/").filter(Boolean).pop() || "";

// describe says what happened to a record, in a few words.
export function describe(c) {
  if (c.change === "removed") return "removed";
  if (c.type === "plan_doc" || c.type === "phase_doc") return c.change === "added" ? "doc written" : "doc edited";
  if (c.type === "entry") {
    if (c.change === "added") return c.kind === "review" ? "filed for your review" : `new ${c.kind || "entry"}`;
    if (c.state && c.state !== "open") return `settled: ${c.state}`;
    return fields(c) || "edited";
  }
  if (c.change === "added") {
    if (c.type === "item") return c.state === "captured" ? `captured: new ${c.kind || "item"}` : `added, ${stageLabel(c.state)}`;
    if (c.type === "plan") return `new plan, ${c.state}`;
    return "new phase";
  }
  return fields(c) || "edited";
}

function fields(c) {
  return (c.fields || []).map(f => {
    if (f.field === "state" && c.type === "item") {
      if (f.to === "in-review") return "finished; waiting for your review";
      if (f.to === "done") return "accepted, done";
      if (f.to === "active") {
        const claim = c.fields.find(g => g.field === "claim");
        return `started${claim?.to ? ` in ${base(claim.to)}` : ""}`;
      }
      return `${stageLabel(f.from)} → ${stageLabel(f.to)}`;
    }
    if (f.field === "claim") return c.fields.some(g => g.field === "state") ? "" : (f.to ? `claimed in ${base(f.to)}` : "released");
    if (f.from || f.to) return `${f.field}: ${f.from || "none"} → ${f.to || "none"}`;
    return `${f.field} edited`;
  }).filter(Boolean).join(" · ");
}

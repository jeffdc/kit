// Item lists: the shared item table, a project's Unplanned and Backlog
// sheets, and a plan's backlog. Whole rows are links; the title stays the
// keyboard target. Changed items are clouded.

import { esc, href, tag, itemStage, shortTitle, day, base, ago } from "./html.js";
import { within } from "./revision.js";
import { schedule } from "./needs.js";

// Columns: each is [heading, cell(item), class]. Tables pick what they need.
export const col = {
  id: () => ["ID", it => `<code>${esc(it.id)}</code>`, "", "width:70px"],
  title: (pid, label = "Item") => [label, it => {
    const link = href.item(pid, it.id);
    const claim = it.claim ? `<span class="meta">${esc(base(it.claim.worktree))}${it.claim.branch ? ` · ${esc(it.claim.branch)}` : ""} · since ${esc(ago(it.claim.at))}</span>` : "";
    const qs = it.questions ? ` <span class="mut">· ${it.questions} open question${it.questions > 1 ? "s" : ""}</span>` : "";
    return `<a class="ttl" href="${esc(link)}">${esc(it.title)}</a>${it.untriaged ? ` <span class="tag">untriaged</span>` : ""}${qs}${claim}`;
  }],
  kind: () => ["Kind", it => esc(it.kind || "")],
  stage: () => ["Stage", it => tag(itemStage(it))],
  category: () => ["Category", it => esc(it.category || "")],
  plan: (pid, plans) => ["Plan", it => it.plan
    ? `<a href="${esc(href.plan(pid, it.plan))}">${esc(shortTitle(plans.get(it.plan) || it.plan))}</a>${it.phase ? `<span class="meta">${esc(it.phase_title || it.phase)}</span>` : ""}`
    : `<span class="mut">none</span>`],
  phase: () => ["Phase", it => it.phase ? esc(it.phase_title || it.phase) : `<span class="mut">none</span>`],
  from: () => ["From", it => it.source ? esc(it.source) : `<span class="mut">not given</span>`],
  date: () => ["Date", it => `<span class="mono">${esc(day(it.created))}</span>`, "n"],
};

export function itemTable(pid, items, cols, { changes, empty = "None." } = {}) {
  if (!items.length) return `<p class="empty">${esc(empty)}</p>`;
  const head = cols.map(([th, , cls, style]) => `<th${cls ? ` class="${cls}"` : ""}${style ? ` style="${style}"` : ""}>${th}</th>`).join("");
  const rows = items.map(it => {
    const cls = [it.state === "done" || it.state === "dropped" ? "waits" : "", changes ? changes.cls(within.item(it.id)).trim() : ""].filter(Boolean).join(" ");
    return `<tr${cls ? ` class="${cls}"` : ""} data-href="${esc(href.item(pid, it.id))}">${cols.map(([, td, c]) => `<td${c ? ` class="${c}"` : ""}>${td(it)}</td>`).join("")}</tr>`;
  }).join("");
  return `<div class="tbl-wrap"><table class="t"><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>`;
}

export function unplannedBody(pid, page, changes) {
  const table = itemTable(pid, page.items, [col.id(), col.title(pid), col.kind(), col.stage(), col.category()],
    { changes, empty: "Nothing unplanned is committed." });
  const ny = page.needs_you.tiers.length
    ? `<section><div class="hd"><h2>Needs you</h2><span class="lab">On unplanned work</span></div>${schedule(page.needs_you, { pid, changes, noPlan: true })}</section>`
    : "";
  return `<section><div class="hd"><h2>Unplanned work</h2><span class="lab">Committed, no plan · a bug or idea started straight from the backlog lands here</span></div>${table}</section>${ny}`;
}

// Backlog filters, per page (project or plan), kept while the page is open.
const filters = new Map();
export const filterOf = key => {
  if (!filters.has(key)) filters.set(key, { kind: "all", untriaged: false, plan: "all", category: "all" });
  return filters.get(key);
};
export const clearFilter = key => filters.delete(key);

// backlogBody: the captured items, filtered by kind, triage, plan and
// category. inPlan is the plan's slug on a plan's Backlog tab, where the
// plan filter and column are left out.
export function backlogBody(pid, page, changes, { key, inPlan = "" } = {}) {
  const f = filterOf(key);
  const plans = new Map(page.plans.map(p => [p.slug, p.title]));
  const all = page.items;
  const list = all.filter(i =>
    (f.kind === "all" || i.kind === f.kind) &&
    (!f.untriaged || i.untriaged) &&
    (inPlan || f.plan === "all" || (f.plan === "none" ? !i.plan : i.plan === f.plan)) &&
    (f.category === "all" || (i.category || "") === f.category));
  const count = k => all.filter(i => i.kind === k).length;
  const seg = (v, label) => `<button data-filter="${esc(key)}" data-k="kind" data-v="${v}" aria-pressed="${f.kind === v}">${label}</button>`;
  const opt = (v, label, cur) => `<option value="${esc(v)}"${cur === v ? " selected" : ""}>${esc(label)}</option>`;
  const planSelect = inPlan ? "" : `<label>Plan <select data-filter="${esc(key)}" data-k="plan">${opt("all", "Any", f.plan)}${page.plans.map(p => opt(p.slug, shortTitle(p.title), f.plan)).join("")}${opt("none", "No plan", f.plan)}</select></label>`;
  const triage = page.last_triage ? `last triage ${esc(day(page.last_triage))}` : "never triaged";
  const cols = [col.id(), col.title(pid, "Captured"), col.kind(), col.category()];
  if (!inPlan) cols.push(col.plan(pid, plans)); else cols.push(col.phase());
  cols.push(col.from(), col.date());
  const table = list.length
    ? itemTable(pid, list, cols, { changes })
    : all.length
      ? `<p class="empty">Nothing matches these filters. <button class="btn s" data-clear="${esc(key)}">Clear filters</button></p>`
      : `<p class="empty">Nothing captured${inPlan ? " for this plan" : ""}.</p>`;
  const other = inPlan ? ` · <a href="${esc(href.backlog(pid))}">the whole project's backlog</a>` : "";
  return `<section><div class="hd"><h2>${inPlan ? "This plan's backlog" : "Backlog"}</h2><span class="lab">Captured, not committed · ${triage} · triage runs with an agent${other}</span></div>
    ${all.length ? `<div class="filters">
      <span class="seg" role="group" aria-label="Kind">${seg("all", `All ${all.length}`)}${seg("bug", `Bugs ${count("bug")}`)}${seg("idea", `Ideas ${count("idea")}`)}</span>
      <label><input type="checkbox" data-filter="${esc(key)}" data-k="untriaged"${f.untriaged ? " checked" : ""}> Untriaged only (${page.untriaged})</label>
      ${planSelect}
      <label>Category <select data-filter="${esc(key)}" data-k="category">${opt("all", "Every category", f.category)}${page.categories.map(c => opt(c, c || "none", f.category)).join("")}</select></label>
    </div>` : ""}
    ${table}</section>`;
}

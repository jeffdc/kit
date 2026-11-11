// The sheets: all projects, a project's overview, and the sheets still to
// be drawn (plan, phase, item, Unplanned, Backlog), which for now show
// their title block only.

import { esc, href, agoSpan, shortTitle } from "./html.js";
import { titleBlock, projectHead, liveField } from "./title.js";
import { schedule } from "./needs.js";
import { plansColumn } from "./plans.js";
import { revisionSchedule } from "./revision.js";

const crumbHome = `<a href="${href.home()}">All projects</a>`;

export function pageHome(state) {
  const all = state.projects || [];
  const ok = all.filter(p => !p.error), bad = all.filter(p => p.error);
  const sum = f => ok.reduce((n, p) => n + f(p.summary), 0);
  const fields = [
    ["Projects on mull", String(ok.length)],
    ["Blocking", String(sum(s => s.blocking))],
    ["Waiting for review", String(sum(s => s.needs_you.reviews))],
    ["Agents working", String(sum(s => s.agents))],
    ["Status", liveField(state.live)],
    ["Not readable", String(bad.length)],
  ];
  const head = titleBlock({ title: "All projects", fields });
  if (!all.length) {
    return head + `<div class="page"><p class="empty">No projects yet. Run any mull command in a repository with a mull branch to add it.</p></div>`;
  }
  const rows = ok.map(pr => homeRow(pr, state.data.get(pr.id))).join("") + bad.map(errorRow).join("");
  return head + `<div class="page"><section><div class="hd"><h2>Projects</h2><span class="lab">Updates when any repo's mull branch moves</span></div>
    <div class="tbl-wrap"><table class="t"><thead><tr><th>Project</th><th>Active plans</th><th class="n">Blocking</th><th class="n">Reviews</th><th class="n">Questions</th><th class="n">Agents</th><th class="n">Untriaged</th><th class="n">Revision</th><th class="n">Last change</th></tr></thead>
    <tbody>${rows}</tbody></table></div></section></div>`;
}

function homeRow(pr, data) {
  const s = pr.summary, pid = pr.id, link = href.project(pid), rev = pr.revision;
  const tree = data?.tree;
  const plans = s.plans.filter(p => p.state === "active").map(p => {
    const phases = tree?.plans.find(t => t.slug === p.slug)?.phases;
    const now = phases?.filter(ph => ph.state === "now").map(ph => ph.title);
    const meta = !phases ? "" : `<span class="meta">${!phases.length ? "no phases" : `now: ${esc(now.join(", ") || "nothing can start")}`}</span>`;
    return `<div><a href="${esc(href.plan(pid, p.slug))}">${esc(p.title)}</a>${meta}</div>`;
  }).join("") || `<span class="mut">none</span>`;
  const branch = `<code>${esc(s.integration)}</code>${s.ahead_of_release ? ` · ${s.ahead_of_release} ahead of ${esc(s.release)}` : ""}`;
  const changed = rev?.changes ? (data?.changes ? data.changes.clsOf(data.changes.list) : " cl") : "";
  return `<tr class="${changed.trim()}" data-href="${esc(link)}"><td><a class="ttl" href="${esc(link)}" style="font-size:15px">${esc(pid)}</a><span class="meta">${branch}</span></td>` +
    `<td>${plans}</td><td class="n">${s.blocking ? `<span class="stamp">${s.blocking}</span>` : "0"}</td><td class="n">${s.needs_you.reviews}</td><td class="n">${s.needs_you.questions}</td>` +
    `<td class="n">${s.agents}</td><td class="n">${s.untriaged}</td><td class="n">${rev ? `Rev ${rev.number}` : ""}</td>` +
    `<td class="n mut">${agoSpan(s.last_change)}</td></tr>`;
}

function errorRow(pr) {
  const why = pr.error.code === "not_initialized" ? "No mull branch yet: run mull init in it to start." : pr.error.error;
  return `<tr class="waits"><td>${esc(pr.name)}<span class="meta">${esc(pr.path)}</span></td><td colspan="8"><code>${esc(pr.error.code)}</code> ${esc(why)}</td></tr>`;
}

// pageProject is a project's Overview: the revision schedule, then Needs
// you beside the Plans column.
export function pageProject(state, pr) {
  const data = state.data.get(pr.id);
  const head = projectHead(pr, data, "overview", state.live);
  const ov = data?.overview;
  if (!ov) return head + `<div class="page">${data?.overviewError ? failed(data.overviewError) : `<p class="empty">Loading…</p>`}</div>`;
  const ch = data.changes;
  const ny = ov.needs_you;
  const rows = new Map();
  for (const t of ny.tiers) for (const e of t.entries || t.groups?.flatMap(g => g.entries) || []) rows.set(e.id, e);
  const entryHref = id => {
    const e = rows.get(id);
    if (!e) return "";
    return e.item ? href.item(pr.id, e.item) : e.phase ? href.phase(pr.id, e.phase) : e.plan ? href.plan(pr.id, e.plan) : "";
  };
  const s = ov.summary;
  const migrate = s.migration_required
    ? `<p class="note">This project's mull data is format ${s.format}. mull reads it, but refuses every write until it's migrated: <code>mull migrate --dry-run --text</code> shows what would change.</p>`
    : "";
  return head + `<div class="page">${migrate}${revisionSchedule(pr.id, data.revision, entryHref)}<div class="cols">
    <section><div class="hd"><h2>Needs you</h2><span class="lab">${ny.open} open${ny.parked ? ` · ${ny.parked} parked` : ""} · ranked by what it unblocks</span></div>${schedule(ny, { pid: pr.id, changes: ch })}</section>
    <aside>${plansColumn(pr.id, ov, ch)}</aside>
  </div></div>`;
}

// pageUndrawn: a sheet that isn't built yet, with its place in the set.
export function pageUndrawn(state, pr, route) {
  const data = state.data.get(pr.id), pid = pr.id;
  if (route.kind === "unplanned" || route.kind === "backlog") {
    return projectHead(pr, data, route.kind, state.live) + `<div class="page">${undrawn()}</div>`;
  }
  const crumbs = [crumbHome, `<a href="${esc(href.project(pid))}">${esc(pid)}</a>`];
  let title = route.id;
  const tree = data?.tree;
  for (const pl of tree?.plans || []) {
    if (route.kind === "plans" && pl.slug === route.id) title = pl.title;
    for (const ph of pl.phases) {
      if (route.kind === "phases" && ph.id === route.id) {
        title = ph.title;
        crumbs.push(`<a href="${esc(href.plan(pid, pl.slug))}">${esc(shortTitle(pl.title))}</a>`);
      }
      const it = route.kind === "items" && ph.items.find(i => i.id === route.id);
      if (it) {
        title = it.title;
        crumbs.push(`<a href="${esc(href.plan(pid, pl.slug))}">${esc(shortTitle(pl.title))}</a>`, `<a href="${esc(href.phase(pid, ph.id))}">${esc(ph.title)}</a>`);
      }
    }
  }
  return titleBlock({ title, crumbs, long: true }) + `<div class="page">${undrawn()}</div>`;
}

const undrawn = () => `<p class="empty">This sheet isn't drawn yet: plan, phase, item and review sheets, Unplanned and Backlog come with mull item f31c. Until then, <code>mull view --text</code> and <code>mull report &lt;id&gt; --text</code> show them in a terminal.</p>`;

export function pageNotFound(what) {
  return titleBlock({ title: "Not found", crumbs: [crumbHome] }) +
    `<div class="page"><p class="empty">${esc(what)} <a href="${href.home()}">Back to all projects</a>.</p></div>`;
}

export function pageUnreadable(pr) {
  return titleBlock({ title: pr.name, crumbs: [crumbHome] }) +
    `<div class="page"><p class="note"><code>${esc(pr.error.code)}</code> ${esc(pr.error.error)}<br><span class="mut">${esc(pr.path)}</span></p></div>`;
}

export const failed = err => `<p class="note">Couldn't load this from mull serve: ${err.code ? `<code>${esc(err.code)}</code> ` : ""}${esc(err.message)}</p>`;

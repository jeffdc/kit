// Plans on a project's surface: each plan not done, its phases as a hill
// (GET …/surface's plans). Uphill is still being worked out (nothing
// started yet), downhill is executing; those are what Jeff reads, so they
// get rows with their progress. Waiting (blocked by another phase) and done
// are shown quietly, as a line each.

import { esc, href, cells, plural } from "./html.js";
import { within } from "./revision.js";

export function plansSection(pid, plans, changes) {
  const body = plans.length
    ? `<div class="hills">${plans.map(p => plan(pid, p, changes)).join("")}</div>`
    : `<p class="empty">No plans in progress.</p>`;
  return `<section id="plans"><div class="hd"><h2>Plans</h2><span class="lab">${plural(plans.length, "plan")} in progress · uphill is still being worked out, downhill is executing</span></div>${body}</section>`;
}

function plan(pid, p, ch) {
  const cl = pred => ch ? ch.cls(pred) : "";
  const by = hill => p.phases.filter(ph => ph.hill === hill);
  const row = ph => {
    const g = ph.progress, total = g.total - g.dropped, link = href.phase(pid, ph.id);
    return `<div class="pr${cl(within.phase(ph.id))}" data-href="${esc(link)}"><a href="${esc(link)}"><span>${esc(ph.title)}</span></a>${cells(g)}<em>${total ? `${g.done}/${total}` : ""}</em></div>`;
  };
  const quiet = list => `<div class="sub">${list.map(ph => `<a href="${esc(href.phase(pid, ph.id))}">${esc(ph.title)}</a>`).join(", ")}</div>`;
  const rows = [];
  const up = by("uphill"), down = by("downhill"), waiting = by("waiting"), done = by("done");
  if (up.length) rows.push(["Uphill", `<div>${up.map(row).join("")}</div>`]);
  if (down.length) rows.push(["Downhill", `<div>${down.map(row).join("")}</div>`]);
  if (waiting.length) rows.push(["Waiting", quiet(waiting)]);
  if (done.length) rows.push(["Done", `<div class="sub" title="${esc(done.map(ph => ph.title).join("\n"))}">${plural(done.length, "phase")}</div>`]);
  if (!p.phases.length) rows.push(["Phases", `<div class="sub">none yet</div>`]);
  const ship = p.ahead_of_release ? ` · ${plural(p.ahead_of_release, "commit")} to ship` : "";
  return `<div class="plan"><div class="ph"><h3><a class="${cl(within.planRecord(p.slug)).trim()}" href="${esc(href.plan(pid, p.slug))}">${esc(p.title)}</a></h3><span class="sub">${p.target ? `<code>${esc(p.target)}</code>` : ""}${ship}</span></div>
    <div class="dl">${rows.map(([l, v]) => `<span class="lab">${esc(l)}</span>${v}`).join("")}</div></div>`;
}

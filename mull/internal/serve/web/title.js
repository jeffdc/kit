// The title block across the top of every sheet, and a project's tabs.

import { esc, href, agoSpan, clock, plural } from "./html.js";
import { within } from "./revision.js";

// titleBlock: the sheet's name (large) with its breadcrumbs, then boxed
// fields of [label, html]. Long names (plans, phases, items) get a wider
// name box and a smaller face.
export function titleBlock({ title, crumbs = [], fields = [], long = false, cloud = "" }) {
  const plain = String(title);
  const size = long ? (plain.length > 48 ? "xlong" : "long") : "";
  const nav = crumbs.length ? `<nav class="crumbs" aria-label="Breadcrumb">${crumbs.join(`<span class="faint">/</span>`)}</nav>` : "";
  const cls = ["tb", long && "wide", !fields.length && "bare"].filter(Boolean).join(" ");
  return `<header class="${cls}"><div class="pj">${nav}<h1 class="${size}"><span class="${cloud.trim()}">${esc(plain)}</span></h1></div>` +
    fields.map(([label, v]) => `<div class="f"><span class="lab">${esc(label)}</span><div class="v">${v}</div></div>`).join("") + `</header>`;
}

export const liveField = live => live
  ? `<span class="live"><i></i></span>Live`
  : `<span class="live off"><i></i></span>Reconnecting <small>showing the last data</small>`;

// projectFields: branch, ahead count, live state, last change, agents and
// revision, from the project list's summary and revision.
export function projectFields(pr, live) {
  const s = pr.summary, rev = pr.revision;
  const fields = [
    ["Integration branch", `<code>${esc(s.integration)}</code>${s.integration_found ? "" : " <small>not found in git</small>"}`],
    [`Ahead of ${s.release}`, s.ahead_of_release ? plural(s.ahead_of_release, "commit") : `<span class="mut">level</span>`],
    ["Status", liveField(live)],
    ["Last change", `${agoSpan(s.last_change)} <small class="mono">${esc(clock(s.last_change))}</small>`],
    ["Agents working", String(s.agents)],
  ];
  if (rev) {
    const since = esc(clock(rev.seen_at));
    fields.push(["Revision", `Rev ${rev.number} <small>${rev.changes ? `${plural(rev.changes, "change")} since ${since}` : `nothing new since ${since}`}</small>`]);
  } else {
    fields.push(["Revision", `<span class="mut">unknown</span>`]);
  }
  return fields;
}

// projectHead: a project's title block and tabs, shared by its Overview,
// Unplanned and Backlog sheets.
export function projectHead(pr, data, tab, live) {
  const pid = pr.id;
  const head = titleBlock({ title: pid, crumbs: [`<a href="${href.home()}">All projects</a>`], fields: projectFields(pr, live) });
  const counts = data?.tree?.counts || data?.overview?.counts;
  const ch = data?.changes;
  const t = (key, label, link, count, cloud = "") => {
    const on = tab === key;
    return `<a class="${on ? "on" : ""}${cloud}" href="${esc(link)}"${on ? ' aria-current="page"' : ""}>${label}${count !== undefined ? ` <em>${esc(count)}</em>` : ""}</a>`;
  };
  const backlog = counts ? `${counts.backlog}${counts.untriaged ? ` · ${counts.untriaged} untriaged` : ""}` : undefined;
  const changes = pr.revision?.changes || 0;
  return head + `<nav class="tabs" aria-label="Project sections">` +
    t("overview", "Overview", href.project(pid)) +
    t("unplanned", "Unplanned", href.unplanned(pid), counts?.unplanned, ch ? ch.cls(within.unplanned()) : "") +
    t("backlog", "Backlog", href.backlog(pid), backlog, ch ? ch.cls(within.backlog()) : "") +
    `<div class="acts"><button class="btn" data-seen="${esc(pid)}"${changes ? "" : " disabled"} title="${changes ? "Clear the clouds: everything up to this revision is seen" : "Nothing new since you last looked"}">Mark revision seen</button>` +
    `<button class="btn p" data-cmd='mull idea "…" --detail "…"' data-say="Capture from a terminal or an agent session; use mull bug for something that's wrong.">Capture</button></div></nav>`;
}

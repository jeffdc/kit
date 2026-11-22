// The sheet index: every project, drilling down plan → phase → item, with
// Unplanned and Backlog under each project. A row is clouded when something
// inside it changed and it's collapsed, or when it changed itself and none
// of its open children shows the change. Consecutive clouded rows share
// one cloud, as a drafter clouds a region.

import { esc, href, CHEVRON, itemStage, stageLabel, plural, marks, needsTotal, copyButton } from "./html.js";
import { within } from "./revision.js";

const STORE = "mull.tree.v1";
let expanded = new Set();
try {
  const saved = JSON.parse(localStorage.getItem(STORE) || "null");
  if (Array.isArray(saved)) expanded = new Set(saved);
} catch { /* no storage: the tree starts collapsed */ }
const save = () => { try { localStorage.setItem(STORE, JSON.stringify([...expanded])); } catch { /* not kept */ } };

export function toggle(key) {
  if (expanded.has(key)) expanded.delete(key); else expanded.add(key);
  save();
}

const em = s => `<em>${esc(s)}</em>`;

// nodes builds one project's subtree. match picks the changes inside a
// node; a node with children but no data yet (the tree is loading) has none.
function projectNode(pr, data) {
  const pid = pr.id, tree = data?.tree, kids = [];
  if (tree) {
    for (const pl of tree.plans) {
      const done = pl.phases.filter(ph => ph.state === "done"), live = pl.phases.filter(ph => ph.state !== "done");
      const plKids = [];
      if (done.length) {
        const ids = new Set(done.map(ph => ph.id));
        plKids.push({
          key: `built:${pid}:${pl.slug}`, label: "Built", waits: true, meta: em(plural(done.length, "phase")),
          match: c => ids.has(c.type.startsWith("phase") ? c.id : c.phase),
          kids: done.map(ph => phaseNode(pid, ph)),
        });
      }
      plKids.push(...live.map(ph => phaseNode(pid, ph)));
      if (pl.unfiled.length) {
        const ids = new Set(pl.unfiled.map(it => it.id)), n = pl.unfiled.reduce((s, it) => s + (it.needs_you || 0), 0), b = pl.unfiled.reduce((s, it) => s + (it.blocking || 0), 0);
        plKids.push({
          key: `nf:${pid}:${pl.slug}`, label: "No phase", match: c => c.type === "item" && ids.has(c.id),
          meta: marks(b, n, false, href.plan(pid, pl.slug)) + em(plural(pl.unfiled.length, "item")),
          kids: pl.unfiled.map(it => itemNode(pid, it)),
        });
      }
      if (pl.own.needs_you) {
        plKids.push({ key: `po:${pid}:${pl.slug}`, label: "About the plan", href: href.plan(pid, pl.slug), match: () => false,
          meta: marks(pl.own.blocking, pl.own.needs_you, false, href.plan(pid, pl.slug)) });
      }
      kids.push({
        key: `pl:${pid}:${pl.slug}`, label: pl.title, href: href.plan(pid, pl.slug), match: within.plan(pl.slug), kids: plKids,
        meta: marks(pl.blocking, pl.needs_you, false, href.plan(pid, pl.slug)) + (pl.state === "shaping" ? `<span class="tag s-shaping">shaping</span>` : ""),
      });
    }
    const c = tree.counts;
    kids.push({ key: `un:${pid}`, label: "Unplanned", href: href.unplanned(pid), match: within.unplanned(), meta: marks(tree.unplanned.blocking, tree.unplanned.needs_you, false, href.unplanned(pid)) + em(c.unplanned) });
    if (tree.own.needs_you) {
      kids.push({ key: `jo:${pid}`, label: "About the project", href: href.project(pid), match: () => false,
        meta: marks(tree.own.blocking, tree.own.needs_you, false, href.project(pid)) });
    }
    kids.push({ key: `bl:${pid}`, label: "Backlog", href: href.backlog(pid), match: within.backlog(), meta: em(`${c.backlog}${c.untriaged ? ` · ${c.untriaged} new` : ""}`) });
    // Closed plans and finished unplanned work are archived: under Archive,
    // collapsed, newest first, the unplanned work in a group of its own.
    const plans = tree.archive || [], items = tree.archived_unplanned || [];
    if (plans.length || items.length) {
      const inPlan = plans.map(pl => within.plan(pl.slug)), ids = new Set(items.map(it => it.id));
      const arKids = plans.map(pl => ({ key: `pl:${pid}:${pl.slug}`, label: pl.title, href: href.plan(pid, pl.slug), match: within.plan(pl.slug), waits: true,
        meta: em(`closed ${pl.closed.slice(0, 10)}`) }));
      if (items.length) {
        arKids.push({
          key: `au:${pid}`, label: "Unplanned", waits: true, match: c => c.type === "item" && ids.has(c.id), meta: em(plural(items.length, "item")),
          kids: items.map(it => ({ key: `it:${pid}:${it.id}`, label: it.title, code: it.id, href: href.item(pid, it.id), match: within.item(it.id), waits: true,
            meta: em(`${stageLabel(it.state)} ${it.finished.slice(0, 10)}`) })),
        });
      }
      const counts = [plans.length && plural(plans.length, "plan"), items.length && plural(items.length, "item")].filter(Boolean).join(" · ");
      kids.push({
        key: `ar:${pid}`, label: "Archive", waits: true, match: c => inPlan.some(m => m(c)) || (c.type === "item" && ids.has(c.id)), meta: em(counts),
        kids: arKids,
      });
    }
  }
  return {
    key: `p:${pid}`, label: pid, href: href.project(pid), top: true, match: within.all(), kids, expandable: true,
    meta: marks(pr.summary?.blocking || 0, needsTotal(pr.summary?.needs_you), true, href.project(pid)),
  };
}

function phaseNode(pid, ph) {
  const p = ph.progress, total = p.total - p.dropped;
  const progress = ph.state === "done" ? "done" : ph.state === "waits" ? "waits" : ph.state === "awaiting-review" ? "review" : total ? `${p.done}/${total}` : "now";
  return {
    key: `ph:${pid}:${ph.id}`, label: ph.title, href: href.phase(pid, ph.id), match: within.phase(ph.id), waits: ph.state !== "now" && ph.state !== "awaiting-review",
    meta: marks(ph.blocking, ph.needs_you, false, href.phase(pid, ph.id)) + (ph.go_live ? `<span class="tag">Go live</span>` : "") + em(progress),
    kids: ph.items.map(it => itemNode(pid, it)),
  };
}

function itemNode(pid, it) {
  return {
    key: `it:${pid}:${it.id}`, label: it.title, code: it.id, href: href.item(pid, it.id), match: within.item(it.id),
    waits: (it.state === "done" || it.state === "dropped") && !it.needs_you,
    meta: marks(it.blocking || 0, it.needs_you || 0, false, href.item(pid, it.id)) + em(stageLabel(itemStage(it))),
  };
}

// flatten walks the open nodes in order, deciding each row's cloud from
// the changes its parent passed down.
function flatten(node, depth, changes, out) {
  const open = expanded.has(node.key) && node.kids?.length > 0;
  const mine = changes ? changes.list.filter(node.match) : [];
  const shown = open ? mine.filter(c => !node.kids.some(k => k.match(c))) : mine;
  out.push({ node, depth, open, clouds: shown });
  if (open) {
    const sub = changes && { list: mine };
    for (const k of node.kids) flatten(k, depth + 1, sub, out);
  }
}

// keyFor is the tree row a route shows.
function keyFor(route) {
  if (!route.pid) return "home";
  const { pid, kind, id } = route;
  return ({ "": `p:${pid}`, plans: `pl:${pid}:${id}`, phases: `ph:${pid}:${id}`, items: `it:${pid}:${id}`, unplanned: `un:${pid}`, backlog: `bl:${pid}` })[kind] || "";
}

// expandFor opens the tree down to the route's row.
export function expandFor(route, state) {
  if (!route.pid || route.pid === "-") return;
  const { pid, kind, id } = route;
  expanded.add(`p:${pid}`);
  const tree = state.data.get(pid)?.tree;
  for (const pl of tree?.plans || []) {
    for (const ph of pl.phases) {
      const here = (kind === "phases" && ph.id === id) || (kind === "items" && ph.items.some(it => it.id === id));
      if (!here) continue;
      expanded.add(`pl:${pid}:${pl.slug}`);
      if (ph.state === "done") expanded.add(`built:${pid}:${pl.slug}`);
      if (kind === "items") expanded.add(`ph:${pid}:${ph.id}`);
    }
    if (kind === "items" && pl.unfiled?.some(it => it.id === id)) {
      expanded.add(`pl:${pid}:${pl.slug}`);
      expanded.add(`nf:${pid}:${pl.slug}`);
    }
  }
  if (kind === "plans") expanded.add(`pl:${pid}:${id}`);
  if (kind === "plans" && tree?.archive?.some(pl => pl.slug === id)) expanded.add(`ar:${pid}`);
  if (kind === "items" && tree?.archived_unplanned?.some(it => it.id === id)) {
    expanded.add(`ar:${pid}`);
    expanded.add(`au:${pid}`);
  }
  save();
}

export function renderTree(el, state, route) {
  const cur = keyFor(route);
  const readable = (state.projects || []).filter(pr => !pr.error);
  const rows = [{ node: { key: "home", label: "All projects", href: href.home(), meta: em(readable.length) }, depth: 0, clouds: [] }];
  for (const pr of readable) {
    const data = state.data.get(pr.id);
    const n = projectNode(pr, data);
    const at = rows.length;
    flatten(n, 0, data?.changes, rows);
    for (let i = at; i < rows.length; i++) Object.assign(rows[i], { pid: pr.id, changes: data?.changes });
    // A collapsed project whose changes haven't loaded yet is clouded from
    // the project list's count.
    if (!data?.changes && pr.revision?.changes && !rows[at].open) rows[at].clouds = [{}];
  }
  // A run of clouded rows ends at a project's last row, so each cloud
  // belongs to one project (and fades with its Mark revision seen).
  let out = "", run = [];
  const flush = () => {
    if (!run.length) return;
    const fresh = run.some(r => r.fresh);
    out += `<div class="tcl cl${fresh ? " fresh" : ""}" data-pid="${esc(run[0].pid)}">${run.map(r => r.html).join("")}</div>`;
    run = [];
  };
  for (const r of rows) {
    const n = r.node, open = expanded.has(n.key);
    const tg = n.kids?.length || n.expandable
      ? `<button class="tg" data-tg="${esc(n.key)}" aria-expanded="${open}" aria-label="${open ? "Collapse" : "Expand"} ${esc(n.label)}">${CHEVRON}</button>`
      : `<span class="tg leaf"></span>`;
    const label = `${n.code ? `<code>${esc(n.code)}</code>` : ""}${esc(n.label)}`;
    const body = n.href
      ? `<a class="x" href="${esc(n.href)}"${n.key === cur ? ' aria-current="page"' : ""} title="${esc(n.label)}">${label}</a>`
      : `<button class="x" data-tg="${esc(n.key)}" aria-expanded="${open}">${label}</button>`;
    const html = `<div class="node${n.top ? " top" : ""}${n.key === cur ? " cur" : ""}${n.waits ? " waits" : ""}" style="--d:${r.depth}"><span class="guides"></span>${tg}${body}${n.code ? copyButton(n.code) : ""}${n.meta || ""}</div>`;
    if (run.length && run[0].pid !== r.pid) flush();
    if (r.clouds.length) {
      run.push({ html, pid: r.pid, fresh: !!r.changes?.clsOf(r.clouds.filter(c => c.type)).includes("fresh") });
    } else {
      flush();
      out += html;
    }
  }
  flush();
  el.innerHTML = out;
}

// reveal scrolls the tree, never the page, to keep the current row in view.
export function reveal(el) {
  const c = el.querySelector(".node.cur");
  if (!c || el.scrollHeight <= el.clientHeight) return;
  // From the boxes, not offsetTop: a row in a cloud (.cl, positioned)
  // measures offsetTop from its cloud, which read as near the top and
  // scrolled the tree back up.
  const top = c.getBoundingClientRect().top - el.getBoundingClientRect().top + el.scrollTop;
  if (top < el.scrollTop || top > el.scrollTop + el.clientHeight - 30) el.scrollTop = top - el.clientHeight / 2;
}

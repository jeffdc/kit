// A project's Rules sheet: the rulebook (GET …/rules, RULES.md at the
// repository's root), each rule with its ID, title, scope, its check (or
// that it has none: advisory, a hope rather than a guard), and its trend
// since the last look from the surface (GET …/surface). Read-only: rules
// change in the repository, with their checks, in the same commit.

import { esc, plural, copyId } from "./html.js";

// trend says how a checked rule's violations moved since the last look.
function trend(t) {
  if (!t) return `<span class="mut">not run yet</span>`;
  if (t.failing) return `<b>failing</b> <span class="mono">${t.now}</span>`;
  switch (t.trend) {
    case "worse": return `worse <span class="mono">${t.then} → ${t.now}</span>`;
    case "better": return `better <span class="mono">${t.then} → ${t.now}</span>`;
    case "same": return `<span class="mut">same</span> <span class="mono">${t.now}</span>`;
    case "new": return `<span class="mut">first run</span> <span class="mono">${t.now}</span>`;
  }
  return `<span class="mut">not run yet</span>`;
}

// rulesBody draws the rulebook. sf is the surface, or null while it loads
// (or if it failed): the trends are left out then.
export function rulesBody(rb, sf) {
  if (!rb.rules.length) {
    return `<section><div class="hd"><h2>Rules</h2><span class="lab"><code>${esc(rb.path)}</code> at the repository's root</span></div>
      <p class="empty">No rulebook yet. Rules live in <code>${esc(rb.path)}</code>, each with an ID, the files it covers and the check that enforces it; an agent drafts it with you (<code>mull guide intake</code>).</p></section>`;
  }
  const trends = new Map((sf?.rules || []).map(t => [t.id, t]));
  const rows = rb.rules.map(r => {
    const scope = r.scope?.length ? r.scope.map(g => `<code>${esc(g)}</code>`).join(" ") : `<span class="mut">everywhere</span>`;
    const check = r.advisory
      ? `<span class="tag s-advisory">no check</span><span class="meta">advisory: nothing enforces it</span>`
      : `<code>${esc(r.check)}</code>${r.ratchet ? `<span class="meta">ratchet <code>${esc(r.ratchet)}</code></span>` : ""}`;
    return `<tr${r.advisory ? ` class="advisory"` : ""}><td>${copyId(r.id)}</td><td><span class="ttl">${esc(r.title)}</span>${r.why ? `<span class="meta">${esc(r.why)}</span>` : ""}</td>` +
      `<td class="w">${scope}</td><td>${check}</td><td>${r.advisory ? `<span class="mut">none</span>` : sf ? trend(trends.get(r.id)) : ""}</td></tr>`;
  }).join("");
  return `<section><div class="hd"><h2>Rules</h2><span class="lab"><code>${esc(rb.path)}</code> · ${plural(rb.checked, "checked rule")} · ${rb.advisory} advisory, with no check · trends since you looked</span></div>
    <div class="tbl-wrap"><table class="t rulebook"><thead><tr><th style="width:56px">ID</th><th>Rule</th><th class="w">Scope</th><th>Check</th><th>Trend</th></tr></thead><tbody>${rows}</tbody></table></div>
    <p class="sub">The rules change in the repository, each with its check, in one commit. A ratchet lists the known violations, which may only shrink.</p></section>`;
}

// An item's done report, read-only: what was built beside how risky it
// was and what it touched. Done scores the risk from the diff alone: high
// risk waits for the human, medium lands and is shown to them, low lands
// (a sample shown). Accepting or sending work back happens on the
// project's surface, where every call is; this sheet links there.

import { esc, href, tag, plural, clock, copyId } from "./html.js";
import { markdown } from "./doc.js";

const short = sha => esc(String(sha || "").slice(0, 7));

// How a settled review reads: the action as the human took it.
const SETTLED = { accept: "accepted", do: "sent back", drop: "dropped", park: "parked" };

function duration(ms) {
  const s = Math.round(ms / 1000);
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} m ${String(s % 60).padStart(2, "0")} s`;
}

// where the report stands: waiting for the human, landed, settled, or
// failed.
function standing(pid, entry, r) {
  const surface = `<a class="btn s" href="${esc(href.project(pid))}" data-needs>On the surface</a>`;
  if (entry && !entry.settled) {
    return entry.landed
      ? `<span class="sub">Landed (${esc(entry.landed === "sampled" ? "sampled" : `${entry.landed} risk`)}): yours to look at.</span> ${surface}`
      : `<span class="sub">High risk: it waits for you to accept it or send it back.</span> ${surface}`;
  }
  if (entry?.settled) {
    const s = entry.settled;
    return `${tag(s.action === "accept" ? "done" : "committed", SETTLED[s.action] || s.action)} <span class="sub">${esc(clock(s.at))}${s.note ? ` · ${esc(s.note)}` : ""}</span>`;
  }
  return `<span class="sub">${r.passed ? `Landed${r.risk ? ` at ${esc(r.risk)} risk` : ""}, not drawn for a look.` : "Failed its checks, so it didn't land."}</span>`;
}

// reportSection draws page.review (the latest done report, and its review
// entry if it was filed) for an item; page.reports lists every attempt.
export function reportSection(pid, page) {
  const rv = page.review, { entry, report: r } = rv;
  const earlier = page.reports.length > 1 ? `<span class="sub">${page.reports.map(x => `report ${x.n} ${x.passed ? "passed" : "failed"} ${esc(clock(x.at))}`).join(" · ")}</span>` : "";
  const checks = (r.checks?.results || []).map(c => `<div class="check${c.exit ? " bad" : ""}">${tag(c.exit ? "committed" : "pass", c.exit ? `exit ${c.exit}` : "passed")}<code>${esc(c.command)}</code><span class="sub">${esc(duration(c.duration_ms))}</span></div>`).join("")
    || `<div class="check">${tag(r.passed ? "pass" : "committed", r.passed ? "passed" : "failed")}<span class="sub">no checks configured</span></div>`;
  const rules = (r.rule_checks || []).map(c => `<div class="check${c.passed ? "" : " bad"}">${tag(c.passed ? "pass" : "committed", c.passed ? "passed" : "failed")}<span>${copyId(c.id)}${c.count ? ` <span class="sub">${plural(c.count, "known violation")}${c.new?.length ? `, ${c.new.length} new` : ""}</span>` : ""}</span><span class="sub">${esc(duration(c.duration_ms))}</span></div>`).join("");
  const risk = r.risk
    ? `<div><span class="lab">Risk · ${esc(r.risk)}${r.sampled ? ", sampled" : ""}</span>${r.risk_why?.length ? `<ul class="why">${r.risk_why.map(w => `<li>${esc(w)}</li>`).join("")}</ul>` : `<p class="sub" style="margin:6px 0 0">Nothing in the diff raised it.</p>`}</div>`
    : "";
  const lines = [
    r.rules ? ["Rules touched", r.rules] : null,
    r.slowed ? ["What slowed it", r.slowed] : null,
  ].filter(Boolean).map(([l, v]) => `<div><span class="lab">${esc(l)}</span><p class="sub" style="margin:6px 0 0">${esc(v)}</p></div>`).join("");
  const noted = r.decisions?.length
    ? `<div><span class="lab">Decided on the way · ${r.decisions.length}</span>${r.decisions.map(d => `<div class="decn">${copyId(d.id)}<span>${esc(d.text)}</span></div>`).join("")}</div>`
    : "";
  const sharedPaths = new Set((r.shared || []).map(f => f.path));
  const files = (r.changes || []).map(l => {
    const add = l.files.reduce((n, f) => n + f.added, 0), del = l.files.reduce((n, f) => n + f.removed, 0);
    return `<tr class="grp-h"><td><b>${esc(l.layer)}</b></td><td class="n">${plural(l.files.length, "file")}</td><td class="n mono">+${add}</td><td class="n mono">−${del}</td></tr>` +
      l.files.map(f => `<tr><td><code class="mut">${esc(f.status)}</code> <code>${esc(f.path)}</code>${sharedPaths.has(f.path) ? ` <span class="sub">shared</span>` : ""}</td><td></td><td class="n mono">+${f.added}</td><td class="n mono">−${f.removed}</td></tr>`).join("");
  }).join("");
  return `<section class="review">
    <div class="rv-head"><h2>Done report</h2>${standing(pid, entry, r)}</div>
    <div class="pk">
      <section><div class="hd"><h2>What changed</h2><span class="lab">The agent's line</span></div>
        <div class="prose">${markdown(r.summary, pid)}</div>
        ${r.design ? `<div class="prose">${markdown(r.design, pid)}</div>` : ""}
        ${checks}${rules}
        <span class="sub">Report ${r.n} · <code>${short(r.commit)}</code>${r.base ? ` from <code>${short(r.base)}</code>` : ""} · ${esc(clock(r.at))}</span>${earlier}</section>
      <section><div class="hd"><h2>What it touched</h2><span class="lab">From the diff, not the agent</span></div>
        ${risk}${lines}${noted}
        ${files ? `<div><span class="lab">Files by layer</span><div class="tbl-wrap"><table class="t files"><tbody>${files}</tbody></table></div></div>` : ""}
      </section>
    </div></section>`;
}

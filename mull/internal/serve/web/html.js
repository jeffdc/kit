// Shared helpers for building markup: escaping, links, stage tags, cell bars
// and times. Every page renders to an HTML string; anything from the API
// goes through esc.

export const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

export const CHEVRON = `<svg class="ic" viewBox="0 0 10 10" aria-hidden="true"><path d="M3.5 1.5 7 5l-3.5 3.5"/></svg>`;

// Routes mirror the API's paths under /api/projects/{id}.
const seg = encodeURIComponent;
export const href = {
  home: () => "#/",
  project: pid => `#/${seg(pid)}`,
  plan: (pid, slug) => `#/${seg(pid)}/plans/${seg(slug)}`,
  phase: (pid, id) => `#/${seg(pid)}/phases/${seg(id)}`,
  item: (pid, id) => `#/${seg(pid)}/items/${seg(id)}`,
  unplanned: pid => `#/${seg(pid)}/unplanned`,
  backlog: pid => `#/${seg(pid)}/backlog`,
};

// Stage and state words, and the tag style each one takes.
const LABEL = { "in-review": "in review", go_live: "go live" };
const STYLE = { "in-review": "review", active: "active" };
export const stageLabel = s => LABEL[s] || s || "";
export const tag = (state, label) => `<span class="tag s-${esc(STYLE[state] || state)}">${esc(label || stageLabel(state))}</span>`;

// An item's stage as the human reads it: committed work that can start is
// "ready".
export const itemStage = it => it.state === "committed" && (it.ready || it.item_ready) ? "ready" : it.state;

// A plan's title up to its colon, for "where" lines.
export const shortTitle = t => String(t || "").split(":")[0];

export const plural = (n, one, many = one + "s") => `${n} ${n === 1 ? one : many}`;

// cells draws a phase's or plan's progress as one cell per item: done,
// in review (hatched), active (blue), then the rest. Dropped items don't
// count.
export function cells(p, max = 16) {
  const total = (p?.total || 0) - (p?.dropped || 0);
  if (total <= 0) return "";
  const cls = [];
  for (let i = 0; i < p.done; i++) cls.push("d1");
  for (let i = 0; i < p.in_review; i++) cls.push("r1");
  for (let i = 0; i < p.active; i++) cls.push("a1");
  while (cls.length < total) cls.push("");
  const shown = cls.slice(0, max).map(c => `<i class="${c}"></i>`).join("");
  const more = total > max ? `<i class="more">+${total - max}</i>` : "";
  return `<i class="cells" role="img" aria-label="${p.done} of ${total} done">${shown}${more}</i>`;
}

// ago is a time relative to now; the page refreshes [data-ago] spans every
// half minute, so "just now" doesn't go stale.
export function ago(iso) {
  const t = Date.parse(iso);
  if (!t) return "";
  const s = Math.max(0, (Date.now() - t) / 1000);
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  const d = Math.floor(s / 86400);
  return d === 1 ? "yesterday" : `${d} days ago`;
}
export const agoSpan = iso => `<span data-ago="${esc(iso)}">${esc(ago(iso))}</span>`;

// clock is a time of day, with the date when it isn't today.
export function clock(iso) {
  const d = new Date(iso);
  if (isNaN(d)) return "";
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return d.toDateString() === new Date().toDateString() ? time : `${d.toLocaleDateString([], { month: "short", day: "numeric" })}, ${time}`;
}

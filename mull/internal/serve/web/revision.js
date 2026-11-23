// Revisions: what changed since the human last looked, as GET …/revision
// lists it. Changes decides what every sheet clouds (the surface, from the
// same last look, says what changed in words).

const captured = c => c.type === "item" && c.state === "captured";
const identity = c => `${c.type}:${c.id}:${c.at}:${c.change}`;

// Matchers: which changes fall inside a record or a list. A captured item
// belongs to the backlog, not to the plan or phase it's tagged to.
export const within = {
  all: () => () => true,
  item: id => c => c.type === "item" && c.id === id,
  entry: id => c => c.type === "entry" && c.id === id,
  phase: id => c => ((c.type === "phase" || c.type === "phase_doc" || c.type === "phase_rules") && c.id === id) || (c.phase === id && !captured(c)),
  plan: slug => c => c.plan === slug && !captured(c),
  planRecord: slug => c => (c.type === "plan" || c.type === "plan_doc" || c.type === "plan_rules") && c.id === slug,
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

// runs joins consecutive changed pieces under one cloud, as the sheet index
// does, so a list where much changed reads as one clouded region, not a
// cloud per row. parts are [html, cls] (cls from Changes.cls, or ""); cloud
// wraps a run of changed pieces, plain a run of the rest.
export function runs(parts, cloud, plain = h => h) {
  let out = "", run = [], changed = false;
  const flush = () => {
    if (!run.length) return;
    const html = run.map(p => p[0]).join("");
    out += changed ? cloud(html, run.some(p => p[1].includes("fresh")) ? " cl fresh" : " cl") : plain(html);
    run = [];
  };
  for (const [html, cls] of parts) {
    if (!!cls !== changed) { flush(); changed = !!cls; }
    run.push([html, cls]);
  }
  flush();
  return out;
}

// The surface: what needs the human, what's trending worse, where the plans
// stand, and how the work moved. And the hill, which the plan page shares.
import { esc, enc, item, plan, when, section, wr, opener, form, md } from './lib.js';

export function surface(p, s) {
  return [needs(p, s.calls), worse(p, s), plans(p, s.plans), flow(p, s.flow, s.audit)].join('');
}

function needs(p, calls) {
  if (!calls.length) return section('Needs you', '<p class="empty">Nothing needs you.</p>');
  const rows = calls.map(c => {
    const k = c.entry, about = [item(p, c.item), plan(p, c.plan)].filter(Boolean).join(' · ');
    let acts, forms, extra = '';
    if (c.kind === 'question') {
      extra = c.default ? `<span class="w"><span class="lab">Default</span> ${esc(c.default)} · ${c.lapsed ? 'lapsed: the default stands' : c.due ? 'stands ' + esc(when(c.due)) : 'stands if you don’t decide'}</span>` : '';
      acts = (c.default ? wr('Take default', { op: 'accept', ids: [k] }) : '') + opener('Decide', 'd' + k);
      forms = form('d' + k, { op: 'decide', id: k }, 'decision', 'Your decision', 'Decide');
    } else {
      acts = wr(c.kind === 'look' ? 'Looks fine' : 'Accept', { op: 'accept', ids: [k] }) + opener('Send back', 'b' + k);
      forms = form('b' + k, { op: 'send back', id: k }, 'reason', 'Why it goes back', 'Send back');
    }
    return `<div class="it"><span class="lab">${esc(c.kind)}</span>
<div><p>${esc(c.text)}</p>${c.why ? `<span class="w">${esc(c.why)}</span>` : ''}${extra}<span class="w">${about}${about ? ' · ' : ''}<code>${esc(k)}</code> · ${esc(when(c.created))}</span></div>
<div class="act">${acts}</div>${forms}</div>`;
  });
  return section('Needs you', `<div class="sch">${rows.join('')}</div>`, `${calls.length}`);
}

function worse(p, s) {
  const bad = s.rules.filter(r => r.failing || r.trend === 'worse');
  const parts = [];
  if (bad.length) parts.push(part('Rules', bad.map(r => `<div class="ag"><code>${esc(r.id)}</code><span>${esc(r.title)}</span><span class="mono">${r.then} → <b>${r.now}</b>${r.failing ? ' <span class="tag">failing</span>' : ''}</span></div>`).join('')));
  if (s.clusters.length) parts.push(part('Piling up', s.clusters.map(c => `<div class="ag"><span class="mono">${c.count}</span><span>${esc(c.area)}</span><span class="sub">${c.items.map(i => item(p, i)).join(' ')}</span></div>`).join('')));
  if (s.new_edges.length) parts.push(part('New connections', `<p class="sub">${s.new_edges.map(e => `<code>${esc(e)}</code>`).join(', ')}</p>`));
  if (s.problems.length) parts.push(part('Problems', s.problems.map(x => `<div class="ov">${item(p, x.item, x.title)}<div class="sub">${esc(x.text)}</div></div>`).join('')));
  if (!parts.length) return '';
  if (s.advisory.length) parts.push(`<p class="sub">Advisory, no check: ${s.advisory.map(esc).join('; ')}</p>`);
  return section('Trending worse', parts.join(''));
}
const part = (label, body) => `<div class="part"><span class="lab">${label}</span>${body}</div>`;

function plans(p, list) {
  if (!list.length) return section('Plans', '<p class="empty">No open plans.</p>');
  return section('Plans', `<div class="hills">${list.map(h => `<div class="plan">
<div class="ph"><h3>${plan(p, h.slug, h.title)}</h3><span class="acts">${h.state === 'shaping' ? wr('Agree', { op: 'plan agree', slug: h.slug }) : ''}${stateTag(h.state)}</span></div>
<p class="sub">into <code>${esc(h.target)}</code>${h.to_ship ? ` · <b class="mono">${h.to_ship}</b> to ship` : ''}${h.preview ? ` · preview <code>${esc(h.preview)}</code>` : ''}</p>
${hill(h.phases)}</div>`).join('')}</div>`);
}

export const stateTag = st => `<span class="tag s-${esc(st)}">${esc(st)}</span>`;

// The hill: uphill (figuring it out) left, downhill (making it) right;
// waiting and done phases as quiet lines under it.
export function hill(phases) {
  if (!phases.length) return '<p class="sub">No phases yet.</p>';
  const side = h => phases.filter(x => x.hill === h).map(x => `<div class="pr"><span>${esc(x.title)}</span>${cells(x.progress)}</div>`).join('') || '<span class="faint">–</span>';
  const quiet = (h, label) => {
    const l = phases.filter(x => x.hill === h);
    return l.length ? `<p class="sub"><span class="lab">${label}</span> ${l.map(x => esc(x.title) + (x.blocked_by?.length ? ` <span class="faint">(after ${x.blocked_by.map(b => esc(phases.find(y => y.id === b)?.title || b)).join(', ')})</span>` : '')).join(' · ')}</p>` : '';
  };
  return `<div class="hill"><svg viewBox="0 0 200 30" preserveAspectRatio="none" aria-hidden="true"><path d="M0 29 C60 29 70 1 100 1 S140 29 200 29"/></svg>
<div><span class="lab">Uphill</span>${side('uphill')}</div><div><span class="lab">Downhill</span>${side('downhill')}</div></div>${quiet('waiting', 'Waiting')}${quiet('done', 'Done')}`;
}

// One cell per item: done green, in review hatched, active blue.
function cells(g) {
  const n = g.total - g.dropped, k = Math.min(n, 14);
  const c = [];
  for (const [cls, v] of [['d1', g.done], ['r1', g.in_review], ['a1', g.active]]) for (let i = 0; i < v; i++) c.push(cls);
  const out = Array.from({ length: k }, (_, i) => `<i class="${c[i] || ''}"></i>`).join('');
  return `<span class="cells" title="${g.done} of ${n} done">${out}${n > k ? `<i class="more">+${n - k}</i>` : ''}</span><em>${g.done}/${n}</em>`;
}

function flow(p, f, audit) {
  const landed = f.landed.map(l => `<div class="ov">${item(p, l.item, l.title)} <span class="tag r-${esc(l.risk)}">${esc(l.risk)}${l.sampled ? ', sampled' : ''}</span><div class="sub">${esc(l.summary)}</div></div>`).join('');
  const slowed = f.slowed.map(s => `<div class="ov">${item(p, s.item)} <span class="sub">${esc(s.text)}</span></div>`).join('');
  const nums = `<p class="sub"><b class="mono">${f.active}</b> active${f.parallel ? ` (${f.parallel} at once)` : ''} · <b class="mono">${f.queue}</b> queued · <b class="mono">${f.rework}</b> rework${f.median_minutes ? ` · <b class="mono">${f.median_minutes}</b> min start to done (median)` : ''}</p>`;
  const a = audit ? part('Audit ' + esc(when(audit.at)) + (audit.stale ? ', stale' : ''), `<div class="prose sm">${md(audit.summary.split('\n').slice(0, 6).join('\n'))}</div>`) : '';
  return `<details class="fold"><summary class="hd"><h2><svg class="ic" viewBox="0 0 10 10"><path d="M3 1.5l4 3.5-4 3.5"/></svg>Flow</h2><span class="lab">${f.landed.length} landed</span></summary>
${nums}${landed ? part('Landed', landed) : ''}${slowed ? part('What slowed work', slowed) : ''}${a}</details>`;
}

export const projectTabs = (p, on, extra = '') => `<nav class="tabs">${[['', 'Surface'], ['/rules', 'Rules'], ['/queue', 'Queue']].map(([r, l]) => `<a href="#/${enc(p)}${r}"${on === r ? ' class="on"' : ''}>${l}</a>`).join('')}<span class="acts">${extra}</span></nav>`;

// mull serve: the router and the pages. Routes: #/ (projects), #/{id}
// (surface), #/{id}/rules, #/{id}/queue, #/{id}/plan/{slug}, #/{id}/item/{id}.
import { api, esc, enc, item, plan, when, section, wr, md, wire, toast } from './lib.js';
import { surface, hill, stateTag, projectTabs } from './surface.js';

const app = document.getElementById('app'), rail = document.getElementById('rail');
let project = '';

const tb = (crumbs, name, fields) => `<header class="tb"><div class="pj"><div class="crumbs">${crumbs}</div><h1${name.length > 40 ? ' class="long"' : ''}>${esc(name)}</h1></div>
${fields.filter(f => f[1] !== '' && f[1] != null).map(([l, v]) => `<div class="f"><span class="lab">${l}</span><div class="v">${v}</div></div>`).join('')}</header>`;
const home = '<a href="#/">All projects</a>';
const pjCrumb = p => `${home} / <a href="#/${enc(p)}">${esc(p)}</a>`;
const num = n => `<span class="mono">${n}</span>`;
const ask = n => n ? `<span class="ask">${n}</span>` : '<span class="faint">0</span>';

const pages = {
  async list(all) {
    const { projects } = await all;
    const rows = projects.map(x => x.error
      ? `<tr><td><b>${esc(x.name)}</b><span class="meta">${esc(x.path)}</span></td><td colspan="4" class="mut">Can’t read it: ${esc(x.error.error)}</td></tr>`
      : `<tr><td><a class="ttl" href="#/${enc(x.id)}">${esc(x.name)}</a><span class="meta">${esc(x.path)}</span></td><td>${ask(x.summary.calls)}</td><td class="n">${num(x.summary.active)}</td><td class="n">${num(x.summary.queue)}</td>
<td>${x.summary.plans.map(l => `<div>${plan(x.id, l.slug, l.title)} ${stateTag(l.state)}</div>`).join('') || '<span class="faint">none</span>'}</td></tr>`);
    return tb('', 'All projects', [['Projects', num(projects.length)], ['Need you', num(projects.reduce((a, x) => a + (x.summary?.calls || 0), 0))]]) +
      `<div class="page">${projects.length ? `<div class="tbl-wrap"><table class="t"><thead><tr><th>Project</th><th>Needs you</th><th class="n">Active</th><th class="n">Queued</th><th>Open plans</th></tr></thead><tbody>${rows.join('')}</tbody></table></div>` : '<p class="empty">No repositories on mull yet: run <code>mull init</code> in one.</p>'}</div>`;
  },

  async surface(p) {
    const s = await api(`projects/${enc(p)}/surface`);
    return tb(home, p, [['Needs you', ask(s.calls.length)], ['Active', num(s.flow.active)], ['Queued', num(s.flow.queue)], ['Since', esc(when(s.since))]]) +
      projectTabs(p, '', '<button class="btn wr" id="seen">Mark seen</button>') + `<div class="page">${surface(p, s)}</div>`;
  },

  async rules(p) {
    const [r, s] = await Promise.all([api(`projects/${enc(p)}/rules`), api(`projects/${enc(p)}/surface`)]);
    const trend = Object.fromEntries(s.rules.map(t => [t.id, t]));
    const rows = r.rules.map(x => {
      const t = trend[x.id];
      return `<tr><td><code>${esc(x.id)}</code></td><td><span class="ttl">${esc(x.title)}</span>${x.why ? `<span class="meta">${esc(x.why)}</span>` : ''}</td>
<td>${(x.scope || []).map(s => `<code>${esc(s)}</code>`).join(' ') || '<span class="faint">all</span>'}</td>
<td>${x.check ? `<code>${esc(x.check)}</code>${x.ratchet ? ' <span class="tag">ratchet</span>' : ''}` : '<span class="tag s-advisory">no check</span>'}</td>
<td>${t ? `<span class="mono">${t.then} → ${t.now}</span> <span class="tag${t.trend === 'worse' || t.failing ? ' s-review' : ''}">${esc(t.failing ? 'failing' : t.trend)}</span>` : ''}</td></tr>`;
    });
    return tb(pjCrumb(p), 'Rules', [['Checked', num(r.checked)], ['Advisory', num(r.advisory)], ['File', `<code>${esc(r.path)}</code>`]]) + projectTabs(p, '/rules') +
      `<div class="page">${r.rules.length ? `<div class="tbl-wrap"><table class="t"><thead><tr><th>ID</th><th>Rule</th><th>Scope</th><th>Check</th><th>Trend</th></tr></thead><tbody>${rows.join('')}</tbody></table></div>` : `<p class="empty">No rules yet: write them in <code>${esc(r.path)}</code>.</p>`}</div>`;
  },

  async queue(p) {
    const { items } = await api(`projects/${enc(p)}/queue`);
    items.sort((a, b) => (b.created || '').localeCompare(a.created || ''));
    const rows = items.map(i => `<tr><td>${item(p, i.id)}</td><td class="ttl">${esc(i.title)}${i.plan ? `<span class="meta">${plan(p, i.plan)}</span>` : ''}</td><td>${esc(i.kind || '')}</td><td>${esc(i.area || '')}</td><td class="n mut">${esc(when(i.created))}</td></tr>`);
    const cap = `<form class="wform cap"><div class="cap-line"><select name="op" aria-label="Kind"><option>bug</option><option>idea</option></select><input type="text" name="title" required placeholder="Title, in one line" aria-label="Title"><input type="text" name="area" class="area" placeholder="area" aria-label="Area"><button class="btn wr p">Queue it</button></div></form>`;
    return tb(pjCrumb(p), 'Queue', [['Queued', num(items.length)]]) + projectTabs(p, '/queue') +
      `<div class="page">${cap}${items.length ? `<div class="tbl-wrap"><table class="t"><thead><tr><th>ID</th><th>Title</th><th>Kind</th><th>Area</th><th class="n">Queued</th></tr></thead><tbody>${rows.join('')}</tbody></table></div>` : '<p class="empty">The queue is empty.</p>'}</div>`;
  },

  async plan(p, slug) {
    const { plan: pl } = await api(`projects/${enc(p)}/show/${enc(slug)}`);
    const act = pl.state === 'shaping' ? wr('Agree the plan', { op: 'plan agree', slug: pl.slug }) : pl.state === 'active' ? wr('Close the plan', { op: 'plan close', slug: pl.slug }, 'Close it?') : '';
    const order = ['active', 'in-review', 'committed', 'captured', 'done', 'dropped'];
    const stages = order.map(st => [st, pl.items.filter(i => i.state === st)]).filter(([, l]) => l.length)
      .map(([st, l]) => `<div class="part"><span class="lab">${st} · ${l.length}</span>${l.map(i => `<div class="ag">${item(p, i.id)}<span>${esc(i.title)}</span><span class="sub">${esc([i.kind, i.area].filter(Boolean).join(' · '))}</span></div>`).join('')}</div>`);
    return tb(pjCrumb(p), pl.title, [['State', stateTag(pl.state)], ['Target', `<code>${esc(pl.target || 'main')}</code>`], ['Preview', pl.preview ? `<code>${esc(pl.preview)}</code>` : '']]) +
      `<div class="page">${act ? `<div class="acts">${act}</div>` : ''}${pl.summary ? `<p class="lede">${esc(pl.summary)}</p>` : ''}
${section('Phases', hill(pl.phases))}${section('Items', stages.join('') || '<p class="empty">No items yet.</p>', `${pl.items.length}`)}
${pl.doc ? section('The plan', `<div class="prose">${md(pl.doc)}</div>`) : ''}</div>`;
  },

  async item(p, id) {
    const { item: i, reports = [] } = await api(`projects/${enc(p)}/show/${enc(id)}`);
    const r = reports[reports.length - 1];
    const report = r ? section(`Report ${r.n}`, `<div class="prose">${md(r.text || r.summary)}</div>`, `${esc(r.risk || '')} risk · ${esc(when(r.at))}`) : '';
    return tb(pjCrumb(p) + (i.plan ? ' / ' + plan(p, i.plan) : ''), i.title, [['Item', `<code>${esc(i.id)}</code>`], ['State', stateTag(i.state)], ['Kind', esc(i.kind || '')], ['Area', esc(i.area || '')], ['Phase', i.phase ? `<code>${esc(i.phase)}</code>` : '']]) +
      `<div class="page">${i.notes ? section('Notes', `<div class="prose">${md(i.notes)}</div>`) : ''}${report || '<p class="empty">No report yet.</p>'}</div>`;
  },
};

// The rail: every project, always on screen, so switching is one click.
// If the list can't be read, the last one stays.
async function drawRail(all, p) {
  let projects;
  try { ({ projects } = await all); } catch { return; }
  const calls = projects.reduce((a, x) => a + (x.summary?.calls || 0), 0);
  const row = (href, on, name, x) => `<a href="${href}"${on ? ' class="on" aria-current="page"' : ''}${x?.error ? ` title="Can’t read it: ${esc(x.error.error)}"` : ''}>
<span class="nm${x?.error ? ' faint' : ''}">${esc(name)}</span>${x?.summary?.active ? `<i class="live" title="${x.summary.active} active"></i>` : ''}${(x ? x.summary?.calls : calls) ? `<span class="ask">${x ? x.summary.calls : calls}</span>` : ''}</a>`;
  rail.innerHTML = row('#/', !p, 'All projects') + projects.map(x => row(`#/${enc(x.id)}`, x.id === p, x.name, x)).join('');
}

async function route() {
  const [p = '', kind = '', ref = ''] = location.hash.replace(/^#\/?/, '').split('/').map(decodeURIComponent);
  project = p;
  const all = api('projects');
  drawRail(all, p);
  const page = !p ? pages.list(all) : !kind ? pages.surface(p) : kind === 'rules' ? pages.rules(p) : kind === 'queue' ? pages.queue(p)
    : kind === 'plan' ? pages.plan(p, ref) : kind === 'item' ? pages.item(p, ref) : Promise.reject(new Error('No page ' + location.hash));
  try {
    const html = await page;
    app.innerHTML = html;
  } catch (err) {
    app.innerHTML = tb(p ? pjCrumb(p) : home, 'Can’t show this', []) + `<div class="page"><p class="note">${esc(err.message)}</p></div>`;
  }
  document.title = app.querySelector('h1').textContent + ' · mull';
}

wire(app, () => project, route);
app.addEventListener('click', async e => {
  const b = e.target.closest('#seen');
  if (!b) return;
  b.disabled = true;
  try { await api(`projects/${enc(project)}/seen`, {}); toast('Marked seen'); route(); } catch (err) { b.disabled = false; toast('Not saved: ' + err.message); }
});
addEventListener('hashchange', () => { route(); scrollTo(0, 0); });
// Coming back to the tab redraws it, unless a form is open.
document.addEventListener('visibilitychange', () => { if (!document.hidden && !app.querySelector('form.wform:not([hidden]) input:not(:placeholder-shown)')) route(); });
route();

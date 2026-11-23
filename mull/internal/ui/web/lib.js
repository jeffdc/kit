// Shared helpers: the API, HTML escaping, links, simple markdown, and the
// two kinds of action (a write button, and a write that takes text).

export async function api(path, body) {
  const opt = body === undefined ? {} : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
  const r = await fetch('/api/' + path, opt);
  const v = await r.json().catch(() => ({ error: r.statusText || 'no answer' }));
  if (!r.ok) throw new Error(v.lines?.[0]?.error || v.error || r.statusText);
  return v;
}

export const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
export const enc = encodeURIComponent;

// Links: item IDs to the item page, plan slugs to the plan page.
export const item = (p, id, title) => id ? `<a href="#/${enc(p)}/item/${enc(id)}"><code>${esc(id)}</code>${title ? ' ' + esc(title) : ''}</a>` : '';
export const plan = (p, slug, title) => slug ? `<a href="#/${enc(p)}/plan/${enc(slug)}">${esc(title || slug)}</a>` : '';

export function when(t) {
  const d = new Date(t);
  if (!t || isNaN(d)) return '';
  const days = (Date.now() - d) / 864e5;
  if (days > -1 && days < 1 && d.getDate() === new Date().getDate()) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
}

export const section = (title, body, cap = '') => `<section><div class="hd"><h2>${title}</h2>${cap ? `<span class="lab">${cap}</span>` : ''}</div>${body}</section>`;

// A write: one click saves. data-confirm makes it ask once more first.
export function wr(label, op, confirm) {
  return `<button class="btn wr" data-op="${esc(JSON.stringify(op))}"${confirm ? ` data-confirm="${esc(confirm)}"` : ''}>${tick}${esc(label)}</button>`;
}
const tick = '<svg class="ic" viewBox="0 0 10 10"><path d="M1.5 5.5l2.5 2.5 4.5-6"/></svg>';

// A write that takes text: its button opens a form in place (key names the
// form); Save sends op with the text as field.
export const opener = (label, key) => `<button class="btn" data-open="${esc(key)}">${esc(label)}</button>`;
export function form(key, op, field, placeholder, save = 'Save') {
  return `<form class="wform" data-key="${esc(key)}" data-op="${esc(JSON.stringify(op))}" hidden>
<input type="text" name="${field}" required placeholder="${esc(placeholder)}" aria-label="${esc(placeholder)}">
<div class="wform-acts"><button class="btn wr p">${tick}${esc(save)}</button><button type="button" class="btn" data-cancel>Cancel</button></div></form>`;
}

const said = { accept: 'Accepted', decide: 'Decided', 'send back': 'Sent back', bug: 'Bug queued', idea: 'Idea queued', 'plan agree': 'Plan agreed', 'plan close': 'Plan closed' };
let toastTimer;
export function toast(msg) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (t.hidden = true), 3000);
}

// Wires the writes in root, once: posts to the current project's ops (project()
// names it), then calls done.
export function wire(root, project, done) {
  const fail = (at, err) => {
    let f = at.querySelector(':scope > .wfail') || at.nextElementSibling;
    if (!f || !f.classList.contains('wfail')) { f = document.createElement('p'); f.className = 'wfail'; at.after(f); }
    f.innerHTML = `<b>Not saved</b>${esc(err.message)}`;
  };
  const send = async (btn, op, at) => {
    const label = btn.innerHTML;
    btn.disabled = true; btn.classList.add('pending'); btn.textContent = 'Saving…';
    try {
      await api(`projects/${enc(project())}/ops`, { ops: [op] });
      toast(said[op.op] || 'Saved');
      done();
    } catch (err) {
      btn.disabled = false; btn.classList.remove('pending', 'armed'); btn.innerHTML = label;
      fail(at, err);
    }
  };
  root.addEventListener('click', e => {
    const b = e.target.closest('button');
    if (!b) return;
    if (b.dataset.open) {
      const f = root.querySelector(`form[data-key="${CSS.escape(b.dataset.open)}"]`);
      f.hidden = !f.hidden;
      if (!f.hidden) f.querySelector('input').focus();
    } else if ('cancel' in b.dataset) {
      b.closest('form').hidden = true;
    } else if (b.dataset.op && !b.closest('form')) {
      if (b.dataset.confirm && !b.classList.contains('armed')) {
        b.classList.add('armed'); b.dataset.was = b.innerHTML; b.textContent = b.dataset.confirm;
        setTimeout(() => { if (b.classList.contains('armed')) { b.classList.remove('armed'); b.innerHTML = b.dataset.was; } }, 4000);
        return;
      }
      send(b, JSON.parse(b.dataset.op), b.parentElement);
    }
  });
  root.addEventListener('submit', e => {
    e.preventDefault();
    const f = e.target, op = JSON.parse(f.dataset.op || '{}');
    for (const el of f.elements) if (el.name) op[el.name] = el.value.trim();
    send(f.querySelector('button.wr'), op, f.querySelector('.wform-acts') || f);
  });
  root.addEventListener('keydown', e => {
    if (e.key === 'Escape' && e.target.closest('form.wform')) e.target.closest('form').hidden = true;
  });
}

// Markdown, simply: headings, lists, code blocks, quotes and paragraphs;
// inline code, bold, italics and links.
export function md(src) {
  const out = [], lines = String(src || '').replace(/\r/g, '').split('\n');
  let para = [], list = null;
  const flush = () => {
    if (para.length) out.push(`<p>${inline(para.join(' '))}</p>`);
    if (list) out.push(`<${list.tag}>${list.items.map(i => `<li>${inline(i)}</li>`).join('')}</${list.tag}>`);
    para = []; list = null;
  };
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i];
    let m;
    if (/^```/.test(l)) {
      flush();
      const code = [];
      while (++i < lines.length && !/^```/.test(lines[i])) code.push(lines[i]);
      out.push(`<pre><code>${esc(code.join('\n'))}</code></pre>`);
    } else if ((m = l.match(/^(#{1,6})\s+(.*)/))) {
      flush();
      const n = Math.min(m[1].length + 2, 4);
      out.push(`<h${n}>${inline(m[2])}</h${n}>`);
    } else if ((m = l.match(/^\s*([-*+]|\d+[.)])\s+(.*)/))) {
      const tag = /\d/.test(m[1]) ? 'ol' : 'ul';
      if (para.length || (list && list.tag !== tag)) flush();
      list = list || { tag, items: [] };
      list.items.push(m[2]);
    } else if (list && /^\s{2,}\S/.test(l)) {
      list.items[list.items.length - 1] += ' ' + l.trim();
    } else if ((m = l.match(/^>\s?(.*)/))) {
      flush();
      out.push(`<blockquote>${inline(m[1])}</blockquote>`);
    } else if (!l.trim()) {
      flush();
    } else {
      if (list) flush();
      para.push(l.trim());
    }
  }
  flush();
  return out.join('\n');
}

function inline(s) {
  return esc(s)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[\s(])[*_]([^*_\s][^*_]*)[*_]/g, '$1<em>$2</em>')
    .replace(/\[([^\]]+)\]\((https?:[^)\s]+|#[^)\s]*)\)/g, '<a href="$2">$1</a>');
}

const manifest = window.__ZK_MANIFEST || [];
let focusIdx = 0;

export function initCommandPalette() {
  const dialog = document.getElementById('cmd-dialog');
  const trigger = document.getElementById('cmd-trigger');
  const input = document.getElementById('cmd-input');
  const results = document.getElementById('cmd-results');
  if (!dialog || !trigger || !input || !results) return;

  trigger.addEventListener('click', () => openPalette(dialog, input, results));

  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      openPalette(dialog, input, results);
    }
  });

  dialog.addEventListener('close', () => {
    input.value = '';
    results.innerHTML = '';
  });

  dialog.addEventListener('click', (e) => {
    if (e.target === dialog) dialog.close();
  });

  input.addEventListener('input', () => renderResults(input.value, results));

  input.addEventListener('keydown', (e) => {
    const els = results.querySelectorAll('.cmd-item');
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      focusIdx = Math.min(focusIdx + 1, els.length - 1);
      setFocus(els);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      focusIdx = Math.max(focusIdx - 1, 0);
      setFocus(els);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const focused = els[focusIdx];
      if (focused?.dataset.href) {
        dialog.close();
        htmx.ajax('GET', focused.dataset.href, { target: '#content-col', swap: 'outerHTML' });
        history.pushState({}, '', focused.dataset.href);
      }
    }
  });

  results.addEventListener('click', (e) => {
    const item = e.target.closest('.cmd-item');
    if (item?.dataset.href) {
      dialog.close();
      htmx.ajax('GET', item.dataset.href, { target: '#content-col', swap: 'outerHTML' });
      history.pushState({}, '', item.dataset.href);
    }
  });
}

function openPalette(dialog, input, results) {
  dialog.showModal();
  input.focus();
  renderResults('', results);
}

function renderResults(query, container) {
  const q = query.trim().toLowerCase();
  let html = '';

  if (!q) {
    const recent = manifest.slice(0, 8);
    if (recent.length) {
      html += '<div class="cmd-group-label">Recent</div>';
      recent.forEach(n => { html += itemHtml(n); });
    }
  } else {
    const matched = manifest.filter(n =>
      n.title.toLowerCase().includes(q) ||
      n.tags.some(t => t.toLowerCase().includes(q)) ||
      n.path.toLowerCase().includes(q)
    );
    if (matched.length) {
      html += '<div class="cmd-group-label">Notes</div>';
      matched.slice(0, 20).forEach(n => { html += itemHtml(n, q); });
    } else {
      html = '<div class="cmd-empty">No results</div>';
    }
  }

  container.innerHTML = html;
  focusIdx = 0;
  setFocus(container.querySelectorAll('.cmd-item'));
}

function itemHtml(note, query) {
  const title = query ? highlight(note.title || note.path, query) : esc(note.title || note.path);
  const tags = note.tags.map(t => '#' + t).join(' ');
  return `<div class="cmd-item" data-href="/note/${encodeURI(note.path)}">
    <span class="cmd-icon">📄</span>
    <span class="cmd-label">${title}</span>
    <span class="cmd-sub">${esc(tags)}</span>
  </div>`;
}

function setFocus(els) {
  els.forEach((el, i) => el.classList.toggle('focused', i === focusIdx));
}

function highlight(text, query) {
  const escaped = esc(text);
  const re = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
  return escaped.replace(re, '<mark>$1</mark>');
}

function esc(s) {
  const el = document.createElement('span');
  el.textContent = s || '';
  return el.innerHTML;
}

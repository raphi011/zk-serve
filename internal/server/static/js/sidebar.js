const manifest = window.__ZK_MANIFEST || [];
let selectedTags = [];
let searchQuery = '';

let filterInput, filtersEl, sidebarInner, tagsSection;

export function initSidebar() {
  filterInput = document.getElementById('sidebar-filter');
  filtersEl = document.getElementById('active-filters');
  sidebarInner = document.getElementById('sidebar-inner');

  if (!filterInput || !sidebarInner) return;

  filterInput.addEventListener('input', () => {
    searchQuery = filterInput.value;
    render();
  });

  // Event delegation for tag clicks (data-tag attribute).
  document.addEventListener('click', (e) => {
    const tagEl = e.target.closest('[data-tag]');
    if (tagEl && !e.target.closest('.filter-chip')) {
      e.preventDefault();
      e.stopPropagation();
      addTag(tagEl.dataset.tag);
    }
    const chip = e.target.closest('.filter-chip');
    if (chip) {
      removeTag(chip.dataset.tag);
    }
  });

  // Mobile sidebar toggle.
  const menuBtn = document.getElementById('mob-menu-btn');
  const sidebar = document.getElementById('sidebar');
  const backdrop = document.getElementById('sidebar-backdrop');
  if (menuBtn && sidebar && backdrop) {
    menuBtn.addEventListener('click', () => {
      sidebar.classList.toggle('mob-open');
      backdrop.classList.toggle('mob-open');
    });
    backdrop.addEventListener('click', () => {
      sidebar.classList.remove('mob-open');
      backdrop.classList.remove('mob-open');
    });
  }
}

function addTag(tag) {
  if (!selectedTags.includes(tag)) {
    selectedTags.push(tag);
    render();
  }
}

function removeTag(tag) {
  selectedTags = selectedTags.filter(t => t !== tag);
  render();
}

function render() {
  renderFilters();
  const query = searchQuery.trim().toLowerCase();
  const hasTags = selectedTags.length > 0;

  if (!query && !hasTags) {
    sidebarInner.querySelectorAll('.server-tree').forEach(el => el.style.display = '');
    sidebarInner.querySelectorAll('.client-results').forEach(el => el.remove());
    return;
  }

  sidebarInner.querySelectorAll('.server-tree').forEach(el => el.style.display = 'none');

  let results = manifest;
  if (hasTags) {
    results = results.filter(n => selectedTags.every(t => n.tags.includes(t)));
  }
  if (query) {
    results = results.filter(n =>
      n.title.toLowerCase().includes(query) ||
      n.tags.some(t => t.toLowerCase().includes(query)) ||
      n.path.toLowerCase().includes(query)
    );
  }

  sidebarInner.querySelectorAll('.client-results').forEach(el => el.remove());

  const container = document.createElement('div');
  container.className = 'client-results';

  if (results.length === 0) {
    container.innerHTML = '<div class="sidebar-empty">No results</div>';
  } else {
    container.innerHTML = results.map(n => `
      <a class="result-item" href="/note/${encodeURI(n.path)}">
        <div class="result-title">${esc(n.title || n.path)}</div>
        ${n.tags.length ? `<div class="result-tags">${n.tags.map(t =>
          `<span class="result-tag" data-tag="${esc(t)}">${esc(t)}</span>`
        ).join('')}</div>` : ''}
      </a>
    `).join('');
  }

  sidebarInner.appendChild(container);
}

function renderFilters() {
  if (!filtersEl) return;
  if (selectedTags.length === 0) {
    filtersEl.style.display = 'none';
    return;
  }
  filtersEl.style.display = 'flex';
  filtersEl.innerHTML =
    '<span class="active-filters-label">Filter:</span>' +
    selectedTags.map(t =>
      `<span class="filter-chip" data-tag="${esc(t)}">${esc(t)} <span class="remove">×</span></span>`
    ).join('');
}

function esc(s) {
  const el = document.createElement('span');
  el.textContent = s;
  return el.innerHTML;
}

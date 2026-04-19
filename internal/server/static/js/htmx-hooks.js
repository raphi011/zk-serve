import { initToc } from './toc.js';
import { initResize } from './resize.js';

export function initHTMXHooks() {
  document.body.addEventListener('htmx:afterSwap', (e) => {
    if (e.detail.target.id !== 'content-col') return;

    // 1. Update tree active state.
    updateTreeActive();

    // 2. Re-init TOC observer + progress bar.
    initToc();

    // 3. Re-init vertical resize handles (new TOC panel DOM).
    initResize();

    // 4. Re-run mermaid on new content.
    if (window.mermaid) {
      mermaid.run({ nodes: document.querySelectorAll('#content-area .mermaid') });
    }

    // 5. Scroll content area to top.
    const contentArea = document.getElementById('content-area');
    if (contentArea) contentArea.scrollTop = 0;
  });
}

function updateTreeActive() {
  const path = decodeURIComponent(location.pathname).replace(/^\/note\//, '').replace(/^\/folder\//, '');

  // Remove old active.
  document.querySelectorAll('.tree-item.active').forEach(el => el.classList.remove('active'));

  // Set new active.
  const link = document.querySelector(`.tree-item[data-path="${CSS.escape(path)}"]`);
  if (link) {
    link.classList.add('active');
    // Expand parent <details> elements.
    let parent = link.parentElement;
    while (parent) {
      if (parent.tagName === 'DETAILS') parent.open = true;
      parent = parent.parentElement;
    }
  }
}

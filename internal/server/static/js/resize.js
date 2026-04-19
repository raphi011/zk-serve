export function initResize() {
  // Saved widths are restored in the inline <head> script to prevent FOUC.
  setupHandle('sidebar-resize', '--sidebar-width', 'sidebar', 120, 360, false);
  setupHandle('toc-resize', '--toc-width', 'toc-panel', 140, 360, true);
  setupVerticalHandles();
}

function setupHandle(handleId, cssVar, panelId, min, max, invert) {
  const handle = document.getElementById(handleId);
  const panel = document.getElementById(panelId);
  if (!handle || !panel) return;

  handle.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    handle.setPointerCapture(e.pointerId);
    handle.classList.add('dragging');
    const startX = e.clientX;
    const startWidth = panel.getBoundingClientRect().width;

    function onMove(e) {
      const delta = invert ? startX - e.clientX : e.clientX - startX;
      const width = Math.min(max, Math.max(min, startWidth + delta));
      document.documentElement.style.setProperty(cssVar, width + 'px');
    }

    function onUp() {
      handle.classList.remove('dragging');
      handle.removeEventListener('pointermove', onMove);
      handle.removeEventListener('pointerup', onUp);
      const finalWidth = Math.round(panel.getBoundingClientRect().width);
      localStorage.setItem('zk-' + panelId + '-width', finalWidth);
    }

    handle.addEventListener('pointermove', onMove);
    handle.addEventListener('pointerup', onUp);
  });
}

// Vertical resize handles: drag the border between two sections.
// Controls the section below the handle. Drag up → grow (panel expands
// upward into the flexible area above). Drag down → shrink.
function setupVerticalHandles() {
  for (const handle of document.querySelectorAll('.resize-handle-v')) {
    handle.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      handle.setPointerCapture(e.pointerId);
      handle.classList.add('dragging');

      const target = handle.nextElementSibling;
      if (!target) return;

      const startY = e.clientY;
      const startHeight = target.getBoundingClientRect().height;

      function onMove(e) {
        const delta = e.clientY - startY;
        const height = Math.max(40, startHeight - delta);
        target.style.maxHeight = height + 'px';
      }

      function onUp() {
        handle.classList.remove('dragging');
        handle.removeEventListener('pointermove', onMove);
        handle.removeEventListener('pointerup', onUp);
      }

      handle.addEventListener('pointermove', onMove);
      handle.addEventListener('pointerup', onUp);
    });
  }
}

export function initToc() {
  const contentArea = document.getElementById('content-area');
  const tocItems = document.querySelectorAll('#toc-inner .toc-item, .mob-toc-body .toc-item');
  const progressBar = document.getElementById('progress-bar');
  if (!contentArea) return;

  // Collect heading elements from toc hrefs.
  const headingIds = [...new Set([...tocItems].map(a => {
    const href = a.getAttribute('href');
    return href ? href.replace('#', '') : null;
  }).filter(Boolean))];

  const headingEls = headingIds.map(id => document.getElementById(id)).filter(Boolean);

  // TOC scroll tracking via IntersectionObserver.
  if (headingEls.length > 0) {
    let activeId = headingIds[0];

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeId = entry.target.id;
          }
        }
        tocItems.forEach(item => {
          const href = item.getAttribute('href');
          const id = href ? href.replace('#', '') : '';
          item.classList.toggle('active', id === activeId);
        });
      },
      { root: contentArea, rootMargin: '-10% 0px -80% 0px', threshold: 0 }
    );

    headingEls.forEach(el => observer.observe(el));
  }

  // Progress bar — always use JS since we scroll a nested div, not the document.
  // CSS scroll-timeline can't target a non-ancestor scroll container.
  if (progressBar) {
    contentArea.addEventListener('scroll', () => {
      const max = contentArea.scrollHeight - contentArea.clientHeight;
      const pct = max > 0 ? Math.round((contentArea.scrollTop / max) * 100) : 0;
      progressBar.style.width = pct + '%';
    }, { passive: true });
  }

  // Mobile TOC: scroll to heading and auto-close details.
  const mobDetails = document.getElementById('mob-toc-details');
  if (mobDetails) {
    mobDetails.addEventListener('click', (e) => {
      const link = e.target.closest('.toc-item');
      if (link) {
        e.preventDefault();
        const id = link.getAttribute('href')?.replace('#', '');
        const target = id ? document.getElementById(id) : null;
        if (target) contentArea.scrollTop = target.offsetTop - 20;
        mobDetails.open = false;
      }
    });
  }
}

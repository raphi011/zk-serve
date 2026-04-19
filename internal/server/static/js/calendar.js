let selectedDate = null;

export function initCalendar() {
  const cal = document.getElementById('calendar');
  if (!cal) return;

  // Restore selection from URL on initial load.
  const params = new URLSearchParams(location.search);
  const urlDate = params.get('date');
  if (urlDate) {
    selectedDate = urlDate;
    applyDateFilter(urlDate);
    highlightDay(urlDate);
  }

  cal.addEventListener('click', (e) => {
    const dayEl = e.target.closest('.cal-day-active');
    if (!dayEl) return;

    const date = dayEl.dataset.date;
    if (!date) return;

    // Toggle: clicking the selected day again clears the filter.
    if (selectedDate === date) {
      e.preventDefault();
      e.stopPropagation();
      clearDateFilter();
      return;
    }

    // Set new selection.
    e.preventDefault();
    e.stopPropagation();
    clearSelection();
    selectedDate = date;
    dayEl.classList.add('cal-selected');
    applyDateFilter(date);
    pushDateURL(date);
  });

  // Clear calendar selection when sidebar filter or tags are used.
  const filterInput = document.getElementById('sidebar-filter');
  if (filterInput) {
    filterInput.addEventListener('input', () => {
      if (filterInput.value.trim()) {
        clearCalendarSelection();
      }
    });
  }

  // Restore selection after month navigation swap.
  document.body.addEventListener('htmx:afterSwap', (e) => {
    if (e.detail.target.id !== 'calendar') return;
    if (selectedDate) {
      highlightDay(selectedDate);
    }
  });

  // Handle browser back/forward to restore or clear date filter.
  window.addEventListener('popstate', () => {
    const p = new URLSearchParams(location.search);
    const d = p.get('date');
    if (d && d !== selectedDate) {
      selectedDate = d;
      applyDateFilter(d);
      highlightDay(d);
    } else if (!d && selectedDate) {
      selectedDate = null;
      clearSelection();
      htmx.ajax('GET', '/search', { target: '#sidebar-inner', swap: 'innerHTML' });
    }
  });
}

function applyDateFilter(date) {
  htmx.ajax('GET', '/search?date=' + date, { target: '#sidebar-inner', swap: 'innerHTML' });
}

function pushDateURL(date) {
  const url = new URL(location.href);
  url.searchParams.set('date', date);
  history.pushState({}, '', url);
}

function removeDateURL() {
  const url = new URL(location.href);
  if (url.searchParams.has('date')) {
    url.searchParams.delete('date');
    history.pushState({}, '', url);
  }
}

function clearDateFilter() {
  selectedDate = null;
  clearSelection();
  removeDateURL();
  htmx.ajax('GET', '/search', { target: '#sidebar-inner', swap: 'innerHTML' });
}

function highlightDay(date) {
  clearSelection();
  const dayEl = document.querySelector(`.cal-day-active[data-date="${date}"]`);
  if (dayEl) dayEl.classList.add('cal-selected');
}

function clearSelection() {
  document.querySelectorAll('.cal-selected').forEach(el => el.classList.remove('cal-selected'));
}

export function clearCalendarSelection() {
  if (selectedDate) {
    selectedDate = null;
    clearSelection();
    removeDateURL();
  }
}

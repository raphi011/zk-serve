let selectedDate = null;

export function initCalendar() {
  const cal = document.getElementById('calendar');
  if (!cal) return;

  cal.addEventListener('click', (e) => {
    const dayEl = e.target.closest('.cal-day-active');
    if (!dayEl) return;

    const date = dayEl.dataset.date;
    if (!date) return;

    // Toggle: clicking the selected day again clears the filter.
    if (selectedDate === date) {
      e.preventDefault();
      e.stopPropagation();
      selectedDate = null;
      dayEl.classList.remove('cal-selected');
      // Restore full tree.
      htmx.ajax('GET', '/search', { target: '#sidebar-inner', swap: 'innerHTML' });
      return;
    }

    // Set new selection.
    clearSelection();
    selectedDate = date;
    dayEl.classList.add('cal-selected');
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
      const dayEl = document.querySelector(`.cal-day-active[data-date="${selectedDate}"]`);
      if (dayEl) {
        dayEl.classList.add('cal-selected');
      }
    }
  });
}

function clearSelection() {
  document.querySelectorAll('.cal-selected').forEach(el => el.classList.remove('cal-selected'));
}

export function clearCalendarSelection() {
  clearSelection();
  selectedDate = null;
}

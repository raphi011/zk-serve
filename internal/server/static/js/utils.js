// Escape a string for safe insertion into innerHTML.
export function esc(s) {
  const el = document.createElement('span');
  el.textContent = s || '';
  return el.innerHTML;
}

// Shared UI behaviour. Loaded on every page with defer.

function csrfToken() {
  const meta = document.querySelector('meta[name="csrf-token"]');
  return meta ? meta.content : '';
}

function toast(message, type) {
  document.querySelectorAll('.toast').forEach((t) => t.remove());
  const el = document.createElement('div');
  el.className = 'toast toast-' + (type || 'success');
  el.setAttribute('role', 'status');
  el.textContent = message;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

// Server-sent toasts: HX-Trigger: {"showToast": {"message": "...", "type": "..."}}
document.addEventListener('showToast', (e) => toast(e.detail.message, e.detail.type));

// ---------- Book page: select a word in a highlight to add it to Vocabulary ----------

function closeWordPopovers() {
  document.querySelectorAll('.word-popover').forEach((p) => p.remove());
}

function addWordToVocabulary(word, highlightId) {
  const body = { word: word, auto_enrich: true };
  if (highlightId) body.highlight_id = highlightId;

  fetch('/api/vocabulary', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken() },
    body: JSON.stringify(body),
  })
    .then((response) => response.json())
    .then((data) => {
      closeWordPopovers();
      if (!data.error) toast('Added "' + word + '" to vocabulary');
      else if (data.error === 'word already exists') toast('Word already in vocabulary', 'info');
      else toast('Error: ' + data.error, 'error');
    })
    .catch((err) => toast('Error adding word: ' + err, 'error'));
}

function showWordPopover(event, word, highlightEl) {
  const highlight = highlightEl.closest('.highlight');
  const highlightId = highlight ? parseInt(highlight.id.replace('highlight-', ''), 10) : null;

  const popover = document.createElement('div');
  popover.className = 'word-popover';
  popover.innerHTML =
    '<div class="popover-content"><span class="popover-word"></span>' +
    '<button type="button" class="btn btn-small btn-primary" data-add>Add to Vocabulary</button>' +
    '<button type="button" class="btn btn-small" data-cancel>Cancel</button></div>';
  // Selected text is untrusted: only ever set it as text.
  popover.querySelector('.popover-word').textContent = '"' + word + '"';
  popover.querySelector('[data-add]').addEventListener('click', () => addWordToVocabulary(word, highlightId));
  popover.querySelector('[data-cancel]').addEventListener('click', () => popover.remove());
  popover.style.left = event.pageX + 'px';
  popover.style.top = event.pageY + 10 + 'px';
  document.body.appendChild(popover);
}

document.addEventListener('mousedown', (e) => {
  if (!e.target.closest('.word-popover')) closeWordPopovers();
});

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeWordPopovers();
});

document.addEventListener('mouseup', (e) => {
  const highlightEl = e.target.closest('.highlight-text');
  if (!highlightEl) return;
  const word = window.getSelection().toString().trim();
  if (word && word.length < 50 && !word.includes('\n')) showWordPopover(e, word, highlightEl);
});

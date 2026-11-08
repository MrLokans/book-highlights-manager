// Shared UI behaviour, loaded in <head> on every page. All handlers are delegated
// from document, so nothing here waits for the DOM. No inline JS: the CSP forbids it.

// Hide a dismissed demo banner before first paint.
try {
  if (sessionStorage.getItem('demo-banner-dismissed') === 'true') {
    document.documentElement.classList.add('demo-banner-dismissed');
  }
} catch (e) {
  // Storage blocked: the banner just stays visible.
}

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

document.addEventListener('htmx:configRequest', (e) => {
  const token = csrfToken();
  if (token && e.detail.verb !== 'get') e.detail.headers['X-CSRF-Token'] = token;
});

// Run after an htmx response for elements with data-redirect, e.g. deleting the open book.
document.addEventListener('htmx:afterRequest', (e) => {
  const target = e.detail.elt.dataset.redirect;
  if (target && e.detail.successful) window.location.href = target;
});

// Swapped-in fragments can ask for a page reload, e.g. after metadata enrichment.
document.addEventListener('htmx:load', (e) => {
  const el = e.detail.elt.closest('[data-reload-after]');
  if (el) setTimeout(() => window.location.reload(), parseInt(el.dataset.reloadAfter, 10));
});

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

// ---------- Click actions declared with data-* attributes ----------

document.addEventListener('click', (e) => {
  const t = e.target;

  // Delete dropdowns: a toggle opens its own menu; any other click closes open menus.
  const toggle = t.closest('[data-dropdown-toggle]');
  const dropdown = toggle && toggle.closest('.delete-dropdown');
  document.querySelectorAll('.delete-dropdown.open').forEach((d) => {
    if (d !== dropdown) d.classList.remove('open');
  });
  if (dropdown) dropdown.classList.toggle('open');

  const tagFilter = t.closest('[data-tag-filter]');
  if (tagFilter) document.getElementById('tag-input').value = tagFilter.dataset.tagFilter;

  const suggestion = t.closest('[data-tag-suggestion]');
  if (suggestion) {
    const form = suggestion.closest('.tag-add-form');
    form.querySelector('input[name=q]').value = suggestion.dataset.tagSuggestion;
    suggestion.closest('.tag-suggestions').innerHTML = '';
    form.querySelector('button[hx-post]').click();
  }

  const audio = t.closest('[data-audio]');
  if (audio) new Audio(audio.dataset.audio).play();

  if (t.closest('[data-clear-search]')) {
    const input = document.querySelector('.search-box input');
    input.value = '';
    input.dispatchEvent(new Event('input', { bubbles: true }));
  }

  if (t.closest('[data-dismiss-demo-banner]')) {
    document.documentElement.classList.add('demo-banner-dismissed');
    try {
      sessionStorage.setItem('demo-banner-dismissed', 'true');
    } catch (err) {
      // Storage blocked: dismissal lasts for this page only.
    }
  }
});

// Enter in a tag input adds the tag (clicks the add button next to it).
document.addEventListener('keydown', (e) => {
  if (e.key !== 'Enter') return;
  const box = e.target.closest('.tag-add-form, .tag-add-inline');
  if (!box || e.target.tagName !== 'INPUT') return;
  e.preventDefault();
  box.querySelector('button[hx-post]').click();
});

document.addEventListener('change', (e) => {
  if (e.target.matches('[data-autosubmit]')) e.target.form.requestSubmit();
});

// Profile: the confirm-password field must match the new password.
document.addEventListener('input', (e) => {
  if (e.target.id !== 'confirm_password') return;
  const match = e.target.value === document.getElementById('new_password').value;
  e.target.setCustomValidity(match ? '' : 'The two new passwords are different.');
});

(function () {
  function initTagInput(wrap) {
    const targetId = wrap.dataset.target;
    const hidden   = document.getElementById(targetId);
    const isAccent = wrap.dataset.color === 'accent';
    const readonly = wrap.dataset.readonly === 'true';
    if (!hidden) return;

    const values = hidden.value.split(',').map(v => v.trim()).filter(Boolean);
    values.forEach(v => addTag(wrap, hidden, v, isAccent, readonly));

    if (readonly) return;

    const input = document.createElement('input');
    input.className   = 'pv-tag-input';
    input.placeholder = 'Add…';
    wrap.appendChild(input);

    input.addEventListener('keydown', (e) => {
      if ((e.key === 'Enter' || e.key === ',') && input.value.trim()) {
        e.preventDefault();
        addTag(wrap, hidden, input.value.trim(), isAccent, false);
        input.value = '';
        syncHidden(wrap, hidden);
      }
      if (e.key === 'Backspace' && !input.value) {
        const tags = wrap.querySelectorAll('.pv-tag, .pv-ctag');
        if (tags.length) tags[tags.length - 1].remove();
        syncHidden(wrap, hidden);
      }
    });

    wrap.addEventListener('click', () => input.focus());
  }

  function addTag(wrap, hidden, text, accent, readonly) {
    const tag = document.createElement('span');
    tag.className = accent ? 'pv-tag' : 'pv-ctag';
    if (readonly) {
      tag.textContent = text;
    } else {
      tag.innerHTML = `${text} <button type="button"><i class="bi bi-x"></i></button>`;
      tag.querySelector('button').addEventListener('click', (e) => {
        e.stopPropagation();
        tag.remove();
        syncHidden(wrap, hidden);
      });
      const input = wrap.querySelector('.pv-tag-input');
      wrap.insertBefore(tag, input);
      return;
    }
    wrap.appendChild(tag);
  }

  function syncHidden(wrap, hidden) {
    const tags = [...wrap.querySelectorAll('.pv-tag, .pv-ctag')].map(t => t.textContent.trim());
    hidden.value = tags.join(',');
  }

  document.querySelectorAll('.pv-tag-wrap[data-target]').forEach(initTagInput);
})();

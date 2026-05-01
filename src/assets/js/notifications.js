'use strict';

document.addEventListener('DOMContentLoaded', async function () {
  const badge = document.getElementById('notification-badge');
  const itemsEl = document.getElementById('notification-items');
  if (!itemsEl) return;

  const response = await proviant.getNotifications();

  if (response.code !== 200) {
    itemsEl.innerHTML = '<p class="text-secondary-custom small px-3 py-2 mb-0">Could not load notifications.</p>';
    return;
  }

  const data = response.message;

  if (badge && data.total > 0) {
    badge.textContent = data.total > 9 ? '9+' : data.total;
    badge.classList.remove('d-none');
  }

  const iconMap = {
    invitation: 'bi-envelope',
    application_incoming: 'bi-person-plus',
    application_outgoing: 'bi-hourglass-split',
  };

  if (data.total === 0) {
    itemsEl.innerHTML = '<p class="text-secondary-custom small px-3 py-2 mb-0">All clear — nothing pending.</p>';
    return;
  }

  itemsEl.innerHTML = data.items.map((item, i) => {
    const divider = i > 0 ? '<hr class="my-0">' : '';
    return `${divider}<a class="d-block text-decoration-none px-3 py-2 notification-entry" href="/web/user/settings">
      <div class="d-flex align-items-start gap-2">
        <i class="bi ${iconMap[item.type] || 'bi-bell'} text-primary flex-shrink-0 mt-1" aria-hidden="true"></i>
        <div>
          <div class="small text-body">${item.title}</div>
          <div class="text-secondary-custom notification-entry-date">${item.createdAt}</div>
        </div>
      </div>
    </a>`;
  }).join('');
});

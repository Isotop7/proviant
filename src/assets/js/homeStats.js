'use strict';

function renderSkeletons(count) {
  const dashboard = document.getElementById('dashboard');
  for (let i = 0; i < count; i++) {
    const col = document.createElement('div');
    col.className = 'col skeleton-tile';
    col.innerHTML = `<div class="card h-100 p-3">
      <div class="skeleton-block mb-2" style="height:1rem;width:60%"></div>
      <div class="skeleton-block mb-3" style="height:3rem;width:40%"></div>
      <div class="skeleton-block" style="height:.75rem;width:80%"></div>
    </div>`;
    dashboard.appendChild(col);
  }
}

function clearSkeletons() {
  document.querySelectorAll('.skeleton-tile').forEach((el) => el.remove());
}

function showEmptyChart(canvasId, message) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  canvas.style.display = 'none';
  canvas.insertAdjacentHTML(
    'afterend',
    `<p class="text-body-secondary small text-center my-auto py-4">${message}</p>`,
  );
}

function renderTile(title, hero, body, variant, heroClass) {
  const col = document.createElement('div');
  col.className = 'col';
  const cls = heroClass || `display-5 fw-bold ${variant ? 'text-' + variant : 'text-primary'} my-2`;
  col.innerHTML = `
    <div class="card h-100">
      <div class="card-header fw-bold">${title}</div>
      <div class="card-body">
        <p class="${cls} my-2" title="${hero}">${hero}</p>
        <p class="text-body-secondary mb-0">${body}</p>
      </div>
    </div>`;
  return col;
}

function renderListTile(title, items) {
  const col = document.createElement('div');
  col.className = 'col';

  let listHtml;
  if (items.length === 0) {
    listHtml = '<li class="list-group-item text-body-secondary small py-2">No products expiring in the next 7 days</li>';
  } else {
    listHtml = items.map((item) => {
      const date = new Date(item.expireAt + 'T00:00:00');
      const dateLabel = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
      return `<li class="list-group-item d-flex justify-content-between align-items-center px-3 py-2">
        <span class="text-truncate me-2 small">${item.productName}</span>
        <span class="text-nowrap text-body-secondary small">${dateLabel}</span>
      </li>`;
    }).join('');
  }

  col.innerHTML = `
    <div class="card h-100">
      <div class="card-header fw-bold">${title}</div>
      <div class="card-body p-0" style="overflow-y: auto; max-height: 200px;">
        <ul class="list-group list-group-flush">${listHtml}</ul>
      </div>
    </div>`;
  return col;
}

document.addEventListener('DOMContentLoaded', async function () {
  renderSkeletons(6);

  const response = await proviant.getProductStats();
  clearSkeletons();

  if (response.code !== 200) return;
  const s = response.message;

  const dashboard = document.getElementById('dashboard');
  if (dashboard) {
    const tiles = [
      {
        title: 'Active Products',
        hero: s.totalActive,
        body: `${s.totalActive} product${s.totalActive !== 1 ? 's' : ''} currently tracked`,
      },
      {
        title: 'Expired (not archived)',
        hero: `${s.wastePercent.toFixed(1)}%`,
        body: `${s.wasteCount} of ${s.totalActive} active products are past their expiry date`,
        variant: s.wasteCount > 0 ? 'danger' : null,
      },
      {
        title: 'Total Archived',
        hero: s.totalArchived,
        body: `${s.totalArchived} product${s.totalArchived !== 1 ? 's' : ''} archived in total`,
      },
      {
        title: 'Unique Archived',
        hero: s.uniqueArchived,
        body: `${s.uniqueArchived} distinct product${s.uniqueArchived !== 1 ? 's' : ''} have been archived`,
      },
      {
        title: 'Last Added Product',
        hero: s.lastInsertedProduct || '—',
        body: s.lastInsertedProduct ? 'Most recently added to your household' : 'No products added yet',
        heroClass: 'fs-4 fw-bold text-primary text-truncate my-2',
      },
    ];

    tiles.forEach(({ title, hero, body, variant, heroClass }) => {
      dashboard.appendChild(renderTile(title, hero, body, variant, heroClass));
    });

    dashboard.appendChild(renderListTile('Expiring within next 7 Days', s.expiringSoon));

    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = 'Dashboard loaded';
  }

  // Chart 1 — Waste donut (expired vs fresh)
  if (!s.totalActive) {
    showEmptyChart('chartWaste', 'No active products yet');
  } else {
    new Chart(document.getElementById('chartWaste'), {
      type: 'doughnut',
      data: {
        labels: ['Expired', 'Fresh'],
        datasets: [{
          data: [s.wasteCount, s.totalActive - s.wasteCount],
          backgroundColor: ['#DC2626', '#3D7A5C'],
        }],
      },
      options: { plugins: { legend: { position: 'bottom' } } },
    });
  }

  // Chart 2 — Category breakdown (pie)
  if (!Object.keys(s.categories).length) {
    showEmptyChart('chartCategories', 'No category data yet');
  } else {
    new Chart(document.getElementById('chartCategories'), {
      type: 'pie',
      data: {
        labels: Object.keys(s.categories),
        datasets: [{
          data: Object.values(s.categories),
          backgroundColor: ['#3D7A5C', '#5B7FA6', '#7BC67E', '#E8914E', '#9DB5A8', '#DC2626'],
        }],
      },
      options: { plugins: { legend: { position: 'bottom' } } },
    });
  }

  // Chart 3 — Expiry trend (line)
  if (!s.expiryTrend.length) {
    showEmptyChart('chartExpiryTrend', 'No expiry trend data yet');
  } else {
    new Chart(document.getElementById('chartExpiryTrend'), {
      type: 'line',
      data: {
        labels: s.expiryTrend.map((m) => m.month),
        datasets: [{
          label: 'Products expiring',
          data: s.expiryTrend.map((m) => m.count),
          borderColor: '#3D7A5C',
          pointBackgroundColor: '#3D7A5C',
          tension: 0.3,
          fill: false,
        }],
      },
      options: { plugins: { legend: { display: false } } },
    });
  }
});

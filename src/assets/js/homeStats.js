'use strict';

function renderTile(title, hero, body, variant) {
  const col = document.createElement('div');
  col.className = 'col';
  col.innerHTML = `
    <div class="card h-100">
      <div class="card-header fw-bold">${title}</div>
      <div class="card-body">
        <p class="display-5 fw-bold ${variant ? 'text-' + variant : 'text-primary'} my-2">${hero}</p>
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
  const response = await proviant.getProductStats();
  if (response.code !== 200) return;
  const s = response.message;

  const dashboard = document.getElementById('dashboard');
  if (dashboard) {
    const firstChart = dashboard.querySelector('.chart-col');
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
      },
    ];

    tiles.forEach(({ title, hero, body, variant }) => {
      dashboard.insertBefore(renderTile(title, hero, body, variant), firstChart);
    });

    dashboard.insertBefore(
      renderListTile('Expiring within next 7 Days', s.expiringSoon),
      firstChart,
    );
  }

  // Chart 1 — Waste donut (expired vs fresh)
  new Chart(document.getElementById('chartWaste'), {
    type: 'doughnut',
    data: {
      labels: ['Expired', 'Fresh'],
      datasets: [{
        data: [s.wasteCount, s.totalActive - s.wasteCount],
        backgroundColor: ['#dc3545', '#198754'],
      }],
    },
    options: { plugins: { legend: { position: 'bottom' } } },
  });

  // Chart 2 — Category breakdown (pie)
  new Chart(document.getElementById('chartCategories'), {
    type: 'pie',
    data: {
      labels: Object.keys(s.categories),
      datasets: [{ data: Object.values(s.categories) }],
    },
    options: { plugins: { legend: { position: 'bottom' } } },
  });

  // Chart 3 — Expiry trend (line)
  new Chart(document.getElementById('chartExpiryTrend'), {
    type: 'line',
    data: {
      labels: s.expiryTrend.map((m) => m.month),
      datasets: [{
        label: 'Products expiring',
        data: s.expiryTrend.map((m) => m.count),
        borderColor: '#ffc107',
        tension: 0.3,
        fill: false,
      }],
    },
    options: { plugins: { legend: { display: false } } },
  });
});

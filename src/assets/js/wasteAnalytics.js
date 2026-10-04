'use strict';

let monthlyChart = null;
let trendChart = null;

function renderTile(title, hero, variant, heroClass) {
  const col = document.createElement('div');
  col.className = 'col';
  const variantColorMap  = { danger: 'var(--status-expired)', warning: 'var(--status-soon)', success: 'var(--status-fresh)' };
  const variantShadowMap = { danger: 'oklch(0.55 0.20 25)', warning: 'oklch(0.72 0.16 80)', success: 'oklch(0.50 0.12 162)' };

  if (variant && variantColorMap[variant] && !heroClass && parseFloat(hero) > 0) {
    const fill   = variantColorMap[variant];
    const shadow = variantShadowMap[variant];
    col.innerHTML = `
      <div class="metric-tile h-100" style="background:${fill};box-shadow:0 6px 18px ${shadow}55;border-color:transparent;position:relative;overflow:hidden;">
        <div style="position:absolute;right:-10px;top:-10px;width:70px;height:70px;border-radius:999px;background:rgba(255,255,255,0.08);pointer-events:none;"></div>
        <div style="position:relative;">
          <div class="metric-label" style="color:rgba(255,255,255,0.70);">${title}</div>
          <div class="metric-value" style="color:white;" title="${hero}">${hero}</div>
        </div>
      </div>`;
    return col;
  }

  col.innerHTML = `
    <div class="metric-tile h-100" style="${heroClass ? `background:${heroClass};` : ''}">
      <div class="metric-label">${title}</div>
      <div class="metric-value" title="${hero}">${hero}</div>
    </div>`;
  return col;
}

function showEmptyChart(canvasId, message) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  canvas.style.display = 'none';
  // Remove any previous empty message inserted after this canvas to avoid accumulation
  // across period-selector clicks.
  let next = canvas.nextElementSibling;
  while (next && next.classList && next.classList.contains('chart-empty-msg')) {
    const toRemove = next;
    next = next.nextElementSibling;
    toRemove.remove();
  }
  canvas.insertAdjacentHTML(
    'afterend',
    `<p class="text-secondary-custom small text-center my-auto py-4 chart-empty-msg">${message}</p>`,
  );
}

function clearEmptyChart(canvasId) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  canvas.style.display = '';
  let next = canvas.nextElementSibling;
  while (next && next.classList && next.classList.contains('chart-empty-msg')) {
    const toRemove = next;
    next = next.nextElementSibling;
    toRemove.remove();
  }
}

function fmtEur(n) {
  return `€${(n ?? 0).toFixed(2)}`;
}

function fmtKg(n) {
  return `${(n ?? 0).toFixed(2)} kg`;
}

function renderTiles(s) {
  const root = document.getElementById('wasteTiles');
  if (!root) return;
  root.innerHTML = '';

  root.appendChild(renderTile('Consumed', String(s.consumedCount ?? 0), 'success', null));
  root.appendChild(renderTile('Wasted', String(s.wastedCount ?? 0), s.wastedCount > 0 ? 'danger' : null, null));
  root.appendChild(renderTile('Waste %', `${(s.wastedPercent ?? 0).toFixed(1)}%`, s.wastedPercent > 25 ? 'danger' : (s.wastedPercent > 10 ? 'warning' : null), null));
  root.appendChild(renderTile('Wasted €', fmtEur(s.wastedEur), s.wastedEur > 0 ? 'danger' : null, null));
  root.appendChild(renderTile('Wasted CO₂', fmtKg(s.wastedCo2Kg), s.wastedCo2Kg > 0 ? 'warning' : null, null));
}

function renderMonthlyChart(s) {
  const canvas = document.getElementById('chartMonthly');
  if (!canvas) return;
  clearEmptyChart('chartMonthly');
  if (monthlyChart) {
    monthlyChart.destroy();
    monthlyChart = null;
  }
  const labels = (s.monthly || []).map((m) => m.month);
  const consumed = (s.monthly || []).map((m) => m.consumedCount || 0);
  const wasted = (s.monthly || []).map((m) => m.wastedCount || 0);

  if (consumed.every((v) => v === 0) && wasted.every((v) => v === 0)) {
    showEmptyChart('chartMonthly', 'No consume/waste events in this window yet');
    return;
  }

  monthlyChart = new Chart(canvas, {
    type: 'bar',
    data: {
      labels,
      datasets: [
        { label: 'Consumed', data: consumed, backgroundColor: '#3D7A5C' },
        { label: 'Wasted',   data: wasted,   backgroundColor: '#DC2626' },
      ],
    },
    options: {
      plugins: { legend: { position: 'bottom' } },
      scales: { y: { beginAtZero: true, ticks: { precision: 0 } } },
    },
  });
}

function renderTrendChart(s) {
  const canvas = document.getElementById('chartTrend');
  if (!canvas) return;
  clearEmptyChart('chartTrend');
  if (trendChart) {
    trendChart.destroy();
    trendChart = null;
  }
  const labels = (s.trend || []).map((m) => m.month);
  const data = (s.trend || []).map((m) => m.count || 0);

  if (data.every((v) => v === 0)) {
    showEmptyChart('chartTrend', 'No wasted items in the last 6 months');
    return;
  }

  trendChart = new Chart(canvas, {
    type: 'line',
    data: {
      labels,
      datasets: [{
        label: 'Wasted',
        data,
        borderColor: '#DC2626',
        pointBackgroundColor: '#DC2626',
        backgroundColor: 'rgba(220, 38, 38, 0.12)',
        tension: 0.3,
        fill: true,
      }],
    },
    options: {
      plugins: { legend: { display: false } },
      scales: { y: { beginAtZero: true, ticks: { precision: 0 } } },
    },
  });
}

function renderTable(s) {
  const tbody = document.querySelector('#monthlyTable tbody');
  if (!tbody) return;
  tbody.innerHTML = '';
  const rows = s.monthly || [];
  if (rows.length === 0) {
    const tr = document.createElement('tr');
    tr.innerHTML = `<td colspan="6" class="text-center text-secondary-custom py-3">No data yet</td>`;
    tbody.appendChild(tr);
    return;
  }
  for (const m of rows) {
    const total = (m.consumedCount || 0) + (m.wastedCount || 0);
    const pct = total > 0 ? ((m.wastedCount / total) * 100).toFixed(1) : '0.0';
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td><span class="font-monospace">${m.month}</span></td>
      <td class="text-end">${m.consumedCount || 0}</td>
      <td class="text-end">${m.wastedCount || 0}</td>
      <td class="text-end">${pct}%</td>
      <td class="text-end">${fmtEur(m.wastedEur)}</td>
      <td class="text-end">${fmtKg(m.wastedCo2Kg)}</td>`;
    tbody.appendChild(tr);
  }
}

function renderCategories(s) {
  const list = document.getElementById('categoryList');
  if (!list) return;
  list.innerHTML = '';
  const cats = s.mostWastedCategories || [];
  if (cats.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'text-center text-secondary-custom py-3';
    empty.textContent = 'No wasted categories yet';
    list.appendChild(empty);
    return;
  }
  cats.forEach((c, idx) => {
    const prods = Array.isArray(c.products) ? c.products : [];
    const hasProducts = prods.length > 0;
    if (!hasProducts) {
      const item = document.createElement('div');
      item.className = 'list-group-item d-flex justify-content-between align-items-center';
      item.innerHTML = `
        <span class="d-flex align-items-center gap-2">
          <i class="bi bi-tag-fill" style="color:var(--accent);"></i>
          <span>${c.displayName || c.categoryKey}</span>
        </span>
        <span class="d-flex align-items-center gap-3 small">
          <span class="badge-status badge-nodate">${c.count || 0}</span>
          <span class="text-secondary-custom">${fmtEur(c.costEur)}</span>
          <span class="text-secondary-custom">${fmtKg(c.co2Kg)}</span>
        </span>`;
      list.appendChild(item);
      return;
    }
    const item = document.createElement('div');
    item.className = 'accordion-item';
    const targetId = `waste-cat-${idx}`;
    item.innerHTML = `
      <h2 class="accordion-header">
        <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#${targetId}" aria-expanded="false" aria-controls="${targetId}">
          <span class="d-flex align-items-center gap-2 flex-grow-1">
            <i class="bi bi-tag-fill" style="color:var(--accent);"></i>
            <span>${c.displayName || c.categoryKey}</span>
          </span>
          <span class="d-flex align-items-center gap-3 small">
            <span class="badge-status badge-nodate">${c.count || 0}</span>
            <span class="text-secondary-custom">${fmtEur(c.costEur)}</span>
            <span class="text-secondary-custom">${fmtKg(c.co2Kg)}</span>
          </span>
        </button>
      </h2>
      <div id="${targetId}" class="accordion-collapse collapse" data-bs-parent="#categoryList">
        <div class="accordion-body p-2">
          ${renderCategoryProducts(prods)}
        </div>
      </div>`;
    list.appendChild(item);
  });
}

function renderCategoryProducts(products) {
  const rows = products.map((p) => `
    <li class="waste-category-product-list-item d-flex justify-content-between align-items-center">
      <span class="d-flex align-items-center gap-2">
        <i class="bi bi-box" style="color:var(--accent);"></i>
        <span>${p.productName || 'Unknown'}</span>
      </span>
      <span class="d-flex align-items-center gap-3 small">
        <span class="badge-status badge-nodate">${p.count || 0}</span>
        <span class="text-secondary-custom">${fmtEur(p.costEur)}</span>
        <span class="text-secondary-custom">${fmtKg(p.co2Kg)}</span>
      </span>
    </li>`).join('');
  return `<ul class="waste-category-product-list list-unstyled mb-0">${rows}</ul>`;
}

function setActivePeriod(period) {
  document.querySelectorAll('#periodSelector button').forEach((b) => {
    const isActive = b.dataset.period === period;
    b.classList.toggle('active', isActive);
    b.setAttribute('aria-pressed', isActive ? 'true' : 'false');
  });
}

function setActiveSort(sort) {
  document.querySelectorAll('#sortToggle button').forEach((b) => {
    const isActive = b.dataset.sort === sort;
    b.classList.toggle('active', isActive);
    b.setAttribute('aria-pressed', isActive ? 'true' : 'false');
  });
}

let currentSort = 'count';
let currentPeriod = '6months';

async function load(period) {
  if (period) currentPeriod = period;
  const tiles = document.getElementById('wasteTiles');
  if (tiles) tiles.innerHTML = '<div class="col-12 text-center text-secondary-custom py-3"><div class="spinner-border spinner-border-sm" role="status"></div> Loading…</div>';

  const resp = await proviant.getWasteAnalytics(currentPeriod, currentSort);
  if (resp.code !== 200 || !resp.message) {
    if (tiles) tiles.innerHTML = '';
    proviant.showFeedback('error', 'Failed to load', 'Could not load waste analytics. Please try again.');
    return;
  }
  const s = resp.message;
  if (s.sort) currentSort = s.sort;
  setActivePeriod(s.period || currentPeriod);
  setActiveSort(currentSort);
  renderTiles(s);
  renderMonthlyChart(s);
  renderTrendChart(s);
  renderTable(s);
  renderCategories(s);
  const status = document.getElementById('waste-status');
  if (status) status.textContent = 'Waste analytics loaded';
}

document.addEventListener('DOMContentLoaded', () => {
  document.addEventListener('click', (event) => {
    const periodBtn = event.target.closest('#periodSelector button');
    if (periodBtn) {
      const period = periodBtn.dataset.period;
      if (period) load(period);
      return;
    }
    const sortBtn = event.target.closest('#sortToggle button');
    if (sortBtn) {
      const sort = sortBtn.dataset.sort;
      if (sort && sort !== currentSort) {
        currentSort = sort;
        load();
      }
    }
  });
  load('6months');
});

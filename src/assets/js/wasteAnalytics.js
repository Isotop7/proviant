(function() {
'use strict';

let monthlyChart = null;
let trendChart = null;
let lastStats = null;

// A tile of the waste screen: the value carries the status colour, the line
// under it carries the icon and the share. The sheets draw no filled tile
// here — only the dashboard fills a card.
const WASTE_VALUE_COLORS = {
  success: 'var(--status-fresh-text)',
  warning: 'var(--status-soon-text)',
  danger: 'var(--status-expired-text)',
};

function renderTile(title, hero, variant, sub) {
  const col = document.createElement('div');
  col.className = 'col';
  const color = variant ? WASTE_VALUE_COLORS[variant] : '';
  const subHtml = sub
    ? `<div class="kpi__sub"><i class="bi bi-${sub.icon}" aria-hidden="true"></i>${sub.label}</div>`
    : '';
  col.innerHTML = `
    <div class="kpi h-100">
      <div class="kpi__label">${title}</div>
      <div class="kpi__value"${color ? ` style="color:${color}"` : ''} title="${hero}">${hero}</div>
      ${subHtml}
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
    `<p class="fg-secondary small text-center my-auto py-7 chart-empty-msg">${message}</p>`,
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
  const headerCount = document.getElementById('wasteHeaderCount');
  if (headerCount) headerCount.textContent = fmtEur(s.wastedEur);
  if (!root) return;
  root.innerHTML = '';

  const consumed = s.consumedCount ?? 0;
  const wasted = s.wastedCount ?? 0;
  const handled = consumed + wasted;
  const share = (part) => (handled > 0 ? Math.round((part / handled) * 100) : 0);
  const months = Array.isArray(s.monthly) ? s.monthly : [];
  const thisMonth = months.length ? months[months.length - 1] : {};

  root.appendChild(renderTile(proviant.t("waste.consumed"), String(consumed), 'success', {
    icon: 'check2',
    label: proviant.t("js.waste.consumedShare", share(consumed)),
  }));
  root.appendChild(renderTile(proviant.t("waste.wasted"), String(wasted), 'danger', {
    icon: 'arrow-up',
    label: proviant.t("js.waste.wastedShare", share(wasted)),
  }));
  root.appendChild(renderTile(proviant.t("waste.wastePercent"), `${(s.wastedPercent ?? 0).toFixed(1)}%`, null, {
    icon: 'arrow-up',
    label: proviant.t("js.waste.wastedItems", wasted),
  }));
  root.appendChild(renderTile(proviant.t("waste.wastedEur"), fmtEur(s.wastedEur), 'danger', {
    icon: 'arrow-up',
    label: proviant.t("js.waste.wastedThisMonth", thisMonth.wastedCount || 0),
  }));
  root.appendChild(renderTile(proviant.t("js.waste.wastedCo2"), fmtKg(s.wastedCo2Kg), 'warning', null));
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
    showEmptyChart('chartMonthly', proviant.t("js.waste.emptyMonthly"));
    return;
  }

  const t = proviantChart.applyTheme();
  const base = proviantChart.baseOptions(t);
  monthlyChart = new Chart(canvas, {
    type: 'bar',
    data: {
      labels: labels.map((month) => proviantChart.monthLabel(month)),
      datasets: [
        { label: proviant.t("waste.consumed"), data: consumed, backgroundColor: t.statusFresh, borderRadius: 4, maxBarThickness: 28 },
        { label: proviant.t("waste.wasted"),   data: wasted,   backgroundColor: t.statusExpired, borderRadius: 4, maxBarThickness: 28 },
      ],
    },
    options: {
      ...base,
      scales: {
        x: {
          grid: { display: false, drawBorder: false },
          border: { display: false },
          ticks: { color: t.muted, padding: 6, maxRotation: 0, autoSkip: true },
        },
        y: {
          beginAtZero: true,
          grid: { color: t.border, drawBorder: false },
          border: { display: false },
          ticks: { color: t.muted, padding: 8, precision: 0 },
        },
      },
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
    showEmptyChart('chartTrend', proviant.t("js.waste.emptyTrend"));
    return;
  }

  const t = proviantChart.applyTheme();
  const base = proviantChart.baseOptions(t);
  trendChart = new Chart(canvas, {
    type: 'line',
    data: {
      labels: labels.map((month) => proviantChart.monthLabel(month)),
      datasets: [{
        label: proviant.t("waste.wasted"),
        data,
        borderColor: t.statusExpired,
        pointBackgroundColor: t.statusExpired,
        pointBorderColor: t.statusExpired,
        backgroundColor: (context) => proviantChart.areaGradient(context.chart, t.statusExpired, 0.24, 0),
        fill: true,
      }],
    },
    options: {
      ...base,
      plugins: {
        ...base.plugins,
        legend: { display: false },
        tooltip: {
          callbacks: {
            title: (items) => proviantChart.monthTitle(labels[items[0].dataIndex]),
            label: (context) => proviant.tn("js.chart.itemCount", context.parsed.y),
          },
        },
      },
      scales: {
        x: {
          grid: { display: false, drawBorder: false },
          border: { display: false },
          ticks: { color: t.muted, padding: 6, maxRotation: 0, autoSkip: true },
        },
        y: {
          beginAtZero: true,
          grid: { color: t.border, drawBorder: false },
          border: { display: false },
          ticks: { color: t.muted, padding: 8, precision: 0 },
        },
      },
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
    tr.innerHTML = `<td colspan="6" class="text-center fg-secondary py-5">${proviant.t("js.waste.noData")}</td>`;
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
    empty.className = 'text-center fg-secondary py-5';
    empty.textContent = proviant.t("js.waste.noCategories");
    list.appendChild(empty);
    return;
  }
  cats.forEach((c) => {
    const prods = Array.isArray(c.products) ? c.products : [];
    const hasProducts = prods.length > 0;
    if (!hasProducts) {
      const item = document.createElement('div');
      item.className = 'list-group-item d-flex justify-content-between align-items-center';
      item.innerHTML = `
        <span class="d-flex align-items-center gap-3">
          <span class="waste-cat__stripe"></span>
          <i class="bi bi-tag-fill" style="color:var(--accent);"></i>
          <span>
            <span class="waste-cat__name">${c.displayName || c.categoryKey}</span>
            <span class="waste-cat__meta">${proviant.t("js.waste.itemsWasted", c.count || 0)}</span>
          </span>
        </span>
        <span class="d-flex align-items-center gap-5 small">
          <span class="waste-cat__cost">${fmtEur(c.costEur)}</span>
          <span class="fg-secondary">${fmtKg(c.co2Kg)}</span>
        </span>`;
      list.appendChild(item);
      return;
    }
    const item = document.createElement('details');
    item.className = 'accordion-item';
    item.name = 'wasteCategories';
    item.innerHTML = `
      <summary class="accordion-header">
          <span class="d-flex align-items-center gap-3 flex-grow-1">
            <span class="waste-cat__stripe"></span>
            <i class="bi bi-tag-fill" style="color:var(--accent);"></i>
            <span>
              <span class="waste-cat__name">${c.displayName || c.categoryKey}</span>
              <span class="waste-cat__meta">${proviant.t("js.waste.itemsWasted", c.count || 0)}</span>
            </span>
          </span>
          <span class="d-flex align-items-center gap-5 small">
            <span class="waste-cat__cost">${fmtEur(c.costEur)}</span>
            <span class="fg-secondary">${fmtKg(c.co2Kg)}</span>
          </span>
          <svg class="accordion-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round"><path d="m2 5 6 6 6-6"/></svg>
      </summary>
      <div class="accordion-body p-3">
          ${renderCategoryProducts(prods)}
      </div>`;
    list.appendChild(item);
  });
}

function renderCategoryProducts(products) {
  const rows = products.map((p) => `
    <li class="waste-category-product-list-item d-flex justify-content-between align-items-center">
      <span class="d-flex align-items-center gap-3">
        <i class="bi bi-box" style="color:var(--accent);"></i>
        <span>${p.productName || proviant.t("js.waste.unknown")}</span>
      </span>
      <span class="d-flex align-items-center gap-5 small">
        <span class="badge badge--neutral">${p.count || 0}</span>
        <span class="fg-secondary">${fmtEur(p.costEur)}</span>
        <span class="fg-secondary">${fmtKg(p.co2Kg)}</span>
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
  if (tiles) tiles.innerHTML = `<div class="col-12 text-center fg-secondary py-5"><div class="spinner-border spinner-sm" role="status"></div> ${proviant.t("common.loading")}</div>`;

  const resp = await proviant.getWasteAnalytics(currentPeriod, currentSort);
  if (resp.code !== 200 || !resp.message) {
    if (tiles) tiles.innerHTML = '';
    proviant.showFeedback('error', proviant.t("js.waste.loadFailedTitle"), proviant.t("js.waste.loadFailedMessage"));
    return;
  }
  const s = resp.message;
  lastStats = s;
  if (s.sort) currentSort = s.sort;
  setActivePeriod(s.period || currentPeriod);
  setActiveSort(currentSort);
  renderTiles(s);
  renderMonthlyChart(s);
  renderTrendChart(s);
  renderTable(s);
  renderCategories(s);
  const status = document.getElementById('waste-status');
  if (status) status.textContent = proviant.t("js.waste.statusLoaded");
}

document.addEventListener('proviant:themechange', () => {
  if (!lastStats) return;
  renderMonthlyChart(lastStats);
  renderTrendChart(lastStats);
});

document.addEventListener('DOMContentLoaded', () => {
  document.addEventListener('click', (event) => {
    const periodBtn = event.target.closest('#periodSelector button');
    if (periodBtn) {
      const period = periodBtn.dataset.period;
      if (period) void load(period);
      return;
    }
    const sortBtn = event.target.closest('#sortToggle button');
    if (sortBtn) {
      const sort = sortBtn.dataset.sort;
      if (sort && sort !== currentSort) {
        currentSort = sort;
        void load();
      }
    }
  });
  void load('6months');
});
})();
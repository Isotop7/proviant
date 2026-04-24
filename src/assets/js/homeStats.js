'use strict';

function renderSkeletons(count) {
  const dashboard = document.getElementById('dashboard');
  for (let i = 0; i < count; i++) {
    const col = document.createElement('div');
    col.className = 'col skeleton-tile';
    col.innerHTML = `<div class="metric-tile h-100">
      <div class="skeleton-block mb-2" style="height:.75rem;width:60%"></div>
      <div class="skeleton-block" style="height:2rem;width:40%"></div>
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

function renderTile(title, hero, variant, heroClass) {
  const col = document.createElement('div');
  col.className = 'col';
  const variantColorMap  = { danger: 'var(--status-expired)', warning: 'var(--status-soon)', success: 'var(--status-fresh)' };
  const variantShadowMap = { danger: 'oklch(0.55 0.20 25)', warning: 'oklch(0.72 0.16 80)', success: 'oklch(0.50 0.12 162)' };

  // Accent-fill tile for non-zero urgent/warning values
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

  const heroColor = (variant && variantColorMap[variant]) ? variantColorMap[variant] : 'var(--fg)';
  const heroStyle = heroClass ? '' : `style="color:${heroColor}"`;
  const heroInnerClass = heroClass || 'metric-value';
  col.innerHTML = `
    <div class="metric-tile h-100">
      <div class="metric-label">${title}</div>
      <div class="${heroInnerClass}" ${heroStyle} title="${hero}">${hero}</div>
    </div>`;
  return col;
}

function renderListTile(title, items, days) {
  const col = document.createElement('div');
  col.className = 'h-100';

  let listHtml;
  if (items.length === 0) {
    listHtml = `<li class="list-group-item text-body-secondary small py-2">No products expiring in the next ${days} day${days !== 1 ? 's' : ''}</li>`;
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
    <div class="card h-100" style="overflow:hidden;">
      <div class="card-header d-flex align-items-center gap-2 fw-bold">
        <div style="width:28px;height:28px;border-radius:7px;background:var(--accent-subtle);display:flex;align-items:center;justify-content:center;flex-shrink:0;" aria-hidden="true">
          <i class="bi bi-clock-history" style="font-size:12px;color:var(--accent);"></i>
        </div>
        <span class="flex-grow-1">${title}</span>
        <i class="bi bi-info-circle tile-threshold-info" style="font-size:14px;cursor:default;color:var(--fg-3);font-weight:normal;"></i>
      </div>
      <div class="tile-scroll-body">
        <ul class="list-group list-group-flush">${listHtml}</ul>
      </div>
    </div>`;

  return col;
}

document.addEventListener('DOMContentLoaded', async function () {
  renderSkeletons(7);

  const [response, streakResponse, savingsResponse] = await Promise.all([
    proviant.getProductStats(),
    proviant.getStreak(),
    proviant.getSavingsStats(),
  ]);
  clearSkeletons();

  if (response.code !== 200) return;
  const s = response.message;

  const dashboard = document.getElementById('dashboard');
  if (dashboard) {
    const tiles = [
      {
        title: 'Active Products',
        hero: s.totalActive,
      },
      {
        title: 'Expired (not archived)',
        hero: `${s.wastePercent.toFixed(1)}%`,
        variant: s.wasteCount > 0 ? 'danger' : null,
      },
      {
        title: 'Total Archived',
        hero: s.totalArchived,
      },
      {
        title: 'Last Added Product',
        hero: s.lastInsertedProduct || '—',
        heroClass: 'metric-value text-truncate',
      },
    ];

    tiles.forEach(({ title, hero, variant, heroClass }) => {
      dashboard.appendChild(renderTile(title, hero, variant, heroClass));
    });

    // Streak tile
    const streak = (streakResponse.code === 200 && streakResponse.message) ? streakResponse.message : null;
    const currentStreak = streak ? streak.currentStreak : 0;
    const longestStreak = streak ? streak.longestStreak : 0;
    const streakHero = `${currentStreak} day${currentStreak !== 1 ? 's' : ''}`;
    const streakHeroClass = currentStreak > 0 ? 'metric-value' : 'metric-value text-body-secondary';
    const streakSub = longestStreak > 0 ? `Best: ${longestStreak} day${longestStreak !== 1 ? 's' : ''}` : null;
    const streakCol = renderTile('<i class="bi bi-fire"></i> Waste-free streak', streakHero, null, streakHeroClass);
    if (streakSub) {
      const tile = streakCol.querySelector('.metric-tile');
      if (tile) {
        const sub = document.createElement('div');
        sub.style.cssText = 'font-size:var(--text-xs);color:var(--fg-3);margin-top:var(--space-1)';
        sub.textContent = streakSub;
        tile.appendChild(sub);
      }
    }
    dashboard.appendChild(streakCol);

    // Savings tiles
    const savings = (savingsResponse.code === 200 && savingsResponse.message) ? savingsResponse.message : null;
    if (savings) {
      dashboard.appendChild(renderTile(
        'Saved This Month',
        `€${(savings.savedEurThisMonth ?? 0).toFixed(2)}`,
        savings.savedEurThisMonth > 0 ? 'success' : null,
        null,
      ));
      dashboard.appendChild(renderTile(
        'CO₂ Avoided This Month',
        `${(savings.savedCo2KgThisMonth ?? 0).toFixed(2)} kg`,
        savings.savedCo2KgThisMonth > 0 ? 'success' : null,
        null,
      ));
    }
  }

  const dashboardList = document.getElementById('dashboard-list');
  if (dashboardList) {
    const days = s.expiringSoonDays ?? 7;
    const listTile = renderListTile(`Expiring within next ${days} Day${days !== 1 ? 's' : ''}`, s.expiringSoon ?? [], days);
    dashboardList.appendChild(listTile);

    const infoIcon = listTile.querySelector('.tile-threshold-info');
    if (infoIcon) {
      const tooltip = new bootstrap.Tooltip(infoIcon, {
        title: `Shows products expiring within your notification threshold (${days} day${days !== 1 ? 's' : ''}). Change this in User Settings.`,
        placement: 'left',
        trigger: 'manual',
      });
      infoIcon.addEventListener('mouseenter', () => tooltip.show());
      document.addEventListener('click', () => tooltip.hide(), { once: false, capture: true });
    }
  }

  const status = document.getElementById('dashboard-status');
  if (status) status.textContent = 'Dashboard loaded';

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

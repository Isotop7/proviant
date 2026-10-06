(function() {
'use strict';

function cssVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

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
    listHtml = `<li class="list-group-item text-secondary-custom small py-2">No products expiring in the next ${days} day${days !== 1 ? 's' : ''}</li>`;
  } else {
    listHtml = items.map((item) => {
      const date = new Date(item.expireAt + 'T00:00:00');
      const dateLabel = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
      return `<li class="list-group-item d-flex justify-content-between align-items-center px-3 py-2">
        <span class="text-truncate me-2 small">${item.productName}</span>
        <span class="text-nowrap text-secondary-custom small">${dateLabel}</span>
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

let dashboardCharts = [];
let lastStats = null;

function renderCharts(s) {
  dashboardCharts.forEach((chart) => chart.destroy());
  dashboardCharts = [];
  Chart.defaults.color = cssVar('--fg-2');

  // Chart 1 — Waste donut (expired vs fresh)
  clearEmptyChart('chartWaste');
  if (!s.totalActive) {
    showEmptyChart('chartWaste', 'No active products yet');
  } else {
    dashboardCharts.push(new Chart(document.getElementById('chartWaste'), {
      type: 'doughnut',
      data: {
        labels: ['Expired', 'Fresh'],
        datasets: [{
          data: [s.wasteCount, s.totalActive - s.wasteCount],
          backgroundColor: [cssVar('--status-expired'), cssVar('--status-fresh')],
        }],
      },
      options: { plugins: { legend: { position: 'bottom' } } },
    }));
  }

  // Chart 2 — Category breakdown (pie)
  clearEmptyChart('chartCategories');
  if (!Object.keys(s.categories).length) {
    showEmptyChart('chartCategories', 'No category data yet');
  } else {
    dashboardCharts.push(new Chart(document.getElementById('chartCategories'), {
      type: 'pie',
      data: {
        labels: Object.keys(s.categories),
        datasets: [{
          data: Object.values(s.categories),
          backgroundColor: [
            cssVar('--status-fresh'),
            cssVar('--brand-slate'),
            cssVar('--color-info'),
            cssVar('--status-soon'),
            cssVar('--status-nodate'),
            cssVar('--status-expired'),
          ],
        }],
      },
      options: { plugins: { legend: { position: 'bottom' } } },
    }));
  }

  // Chart 3 — Expiry trend (line)
  clearEmptyChart('chartExpiryTrend');
  if (!s.expiryTrend.length) {
    showEmptyChart('chartExpiryTrend', 'No expiry trend data yet');
  } else {
    dashboardCharts.push(new Chart(document.getElementById('chartExpiryTrend'), {
      type: 'line',
      data: {
        labels: s.expiryTrend.map((m) => m.month),
        datasets: [{
          label: 'Products expiring',
          data: s.expiryTrend.map((m) => m.count),
          borderColor: cssVar('--accent'),
          pointBackgroundColor: cssVar('--accent'),
          tension: 0.3,
          fill: false,
        }],
      },
      options: { plugins: { legend: { display: false } } },
    }));
  }
}

document.addEventListener('proviant:themechange', () => {
  if (lastStats) renderCharts(lastStats);
});

document.addEventListener('DOMContentLoaded', async function () {
  renderSkeletons(7);

  try {
    const [response, streakResponse, savingsResponse] = await Promise.all([
      proviant.getProductStats(),
      proviant.getStreak(),
      proviant.getSavingsStats(),
    ]);
    clearSkeletons();

    if (response.code !== 200) {
      const status = document.getElementById('dashboard-status');
      if (status) status.textContent = 'Dashboard failed to load. Please refresh.';
      return;
    }
    const s = response.message;

    const dashboard = document.getElementById('dashboard');
    if (dashboard) {
      const tiles = [
        {
          title: 'Active Products',
          hero: s.totalActive,
        },
        {
          title: 'Expired (not consumed)',
          hero: `${s.wastePercent.toFixed(1)}%`,
          variant: s.wasteCount > 0 ? 'danger' : null,
        },
        {
          title: 'Total Consumed',
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
      const streakHeroClass = currentStreak > 0 ? 'metric-value' : 'metric-value text-secondary-custom';
      const streakCol = renderTile('<i class="bi bi-fire"></i> Waste-free streak', streakHero, null, streakHeroClass);
      const streakTile = streakCol.querySelector('.metric-tile');
      if (streakTile) {
        if (longestStreak > 0) {
          const sub = document.createElement('div');
          sub.style.cssText = 'font-size:var(--text-xs);color:var(--fg-3);margin-top:var(--space-1)';
          sub.textContent = `Best: ${longestStreak} day${longestStreak !== 1 ? 's' : ''}`;
          streakTile.appendChild(sub);
        }
        // Milestone progress: next milestone = 7, 30, 100, 365
        const milestones = [7, 30, 100, 365];
        const nextMilestone = milestones.find((m) => m > currentStreak);
        if (nextMilestone) {
          const prevMilestone = [...milestones].reverse().find((m) => m <= currentStreak) || 0;
          const intoMilestone = currentStreak - prevMilestone;
          const span = Math.max(1, nextMilestone - prevMilestone);
          const milestonePct = Math.min(100, Math.round((intoMilestone / span) * 100));
          const prog = document.createElement('div');
          prog.style.cssText = 'margin-top:var(--space-2);font-size:var(--text-xs);color:var(--fg-3);';
          prog.innerHTML = `
            <div class="d-flex justify-content-between mb-1">
              <span>Next milestone: ${nextMilestone}d</span>
              <span>${intoMilestone}/${nextMilestone - prevMilestone}d</span>
            </div>
            <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${milestonePct}" style="height:6px;">
              <div class="progress-bar" style="width:${milestonePct}%;background:var(--status-fresh);"></div>
            </div>`;
          streakTile.appendChild(prog);
        } else {
          const prog = document.createElement('div');
          prog.style.cssText = 'margin-top:var(--space-2);font-size:var(--text-xs);color:var(--status-fresh);';
          prog.innerHTML = `<i class="bi bi-trophy-fill me-1"></i>All milestones reached`;
          streakTile.appendChild(prog);
        }
      }
      dashboard.appendChild(streakCol);

      // Monthly waste goal tile
      const settingsResponse = await proviant.getHouseholdSettings();
      if (settingsResponse.code === 200 && settingsResponse.message) {
        const hs = settingsResponse.message;
        if (hs.monthlyWasteGoalType === 'count' || hs.monthlyWasteGoalType === 'percent') {
          const goalCol = document.createElement('div');
          goalCol.className = 'col';
          const wasteThisMonth = s.wasteCount;
          let pct = 0;
          let label = '';
          if (hs.monthlyWasteGoalType === 'count') {
            const goal = hs.monthlyWasteGoalCount || 0;
            pct = goal > 0 ? Math.min(100, Math.round((wasteThisMonth / goal) * 100)) : 0;
            label = `${wasteThisMonth} / ${goal} wasted`;
          } else {
            const goal = hs.monthlyWasteGoalPercent || 0;
            pct = goal > 0 ? Math.min(100, Math.round((s.wastePercent / goal) * 100)) : 0;
            label = `${s.wastePercent.toFixed(1)}% / ${goal}%`;
          }
          const exceeded = pct >= 100;
          const barColor = exceeded ? 'var(--status-expired)' : (pct >= 75 ? 'var(--status-soon)' : 'var(--status-fresh)');
          goalCol.innerHTML = `
            <div class="metric-tile h-100">
              <div class="metric-label"><i class="bi bi-bullseye me-1"></i>Monthly goal</div>
              <div class="metric-value" title="${label}" style="color:${exceeded ? 'var(--status-expired)' : 'var(--fg)'}">${label}</div>
              <div class="progress mt-2" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct}" style="height:6px;">
                <div class="progress-bar" style="width:${pct}%;background:${barColor};"></div>
              </div>
            </div>`;
          dashboard.appendChild(goalCol);
        }
      }

      // Activity feed tile
      const activityResponse = await proviant.getActivityFeed(10);
      if (activityResponse.code === 200 && activityResponse.message && activityResponse.message.activities) {
        const activities = activityResponse.message.activities;
        const activityCol = document.createElement('div');
        activityCol.className = 'col-12';

        const actionIconMap = {
          add: 'bi-plus-circle',
          consume: 'bi-check-circle',
          waste: 'bi-trash3',
          restore: 'bi-arrow-counterclockwise',
          cook: 'bi-fire',
          amount_change: 'bi-pencil',
          import: 'bi-upload',
        };
        const actionLabelMap = {
          add: 'added',
          consume: 'consumed',
          waste: 'wasted',
          restore: 'restored',
          cook: 'cooked',
          amount_change: 'adjusted',
          import: 'imported',
        };

        function relativeTime(timestamp) {
          const now = Date.now();
          const then = new Date(timestamp).getTime();
          const diff = Math.floor((now - then) / 1000);
          if (diff < 60) return 'just now';
          if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
          if (diff < 86400) return `${Math.floor(diff / 3600)} hour${Math.floor(diff / 3600) !== 1 ? 's' : ''} ago`;
          return `${Math.floor(diff / 86400)} day${Math.floor(diff / 86400) !== 1 ? 's' : ''} ago`;
        }

        let listHtml;
        if (activities.length === 0) {
          listHtml = `<li class="list-group-item text-secondary-custom small py-2">No recent activity in this household</li>`;
        } else {
          listHtml = activities.map((a) => {
            const icon = actionIconMap[a.action] || 'bi-circle';
            const label = actionLabelMap[a.action] || a.action;
            return `<li class="list-group-item d-flex justify-content-between align-items-center px-3 py-2">
              <span class="d-flex align-items-center gap-2 text-truncate me-2 small">
                <i class="bi ${icon}" style="font-size:12px;color:var(--accent);flex-shrink:0;"></i>
                <span class="text-truncate"><strong>${a.userName || 'Someone'}</strong> ${label} ${a.productName || 'a product'}</span>
              </span>
              <span class="text-nowrap text-secondary-custom small" style="flex-shrink:0;">${relativeTime(a.timestamp)}</span>
            </li>`;
          }).join('');
        }

        activityCol.innerHTML = `
          <div class="card h-100" style="overflow:hidden;">
            <div class="card-header d-flex align-items-center gap-2 fw-bold">
              <div style="width:28px;height:28px;border-radius:7px;background:var(--accent-subtle);display:flex;align-items:center;justify-content:center;flex-shrink:0;" aria-hidden="true">
                <i class="bi bi-activity" style="font-size:12px;color:var(--accent);"></i>
              </div>
              <span class="flex-grow-1">Recent Activity</span>
            </div>
            <div class="tile-scroll-body">
              <ul class="list-group list-group-flush">${listHtml}</ul>
            </div>
          </div>`;

        const activityRow = document.createElement('div');
        activityRow.className = 'col-12';
        activityRow.appendChild(activityCol);
        dashboard.appendChild(activityRow);
      }

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

    lastStats = s;
    renderCharts(s);
  } catch {
    clearSkeletons();
    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = 'Dashboard failed to load. Please refresh.';
  }
});
})();

(function() {
'use strict';

// The tiles the script builds go in front of the ones the template renders:
// the template's list cell is the anchor, so the dashboard keeps the order the
// screen sheet draws it in.
function tileAnchor() {
  const dashboard = document.getElementById('dashboard');
  return dashboard ? dashboard.querySelector('[data-tile="expiring-list"]') : null;
}

function insertTile(cell) {
  const dashboard = document.getElementById('dashboard');
  if (!dashboard) return;
  const anchor = tileAnchor();
  if (anchor) dashboard.insertBefore(cell, anchor);
  else dashboard.appendChild(cell);
}

function renderSkeletons(count) {
  for (let i = 0; i < count; i++) {
    const col = document.createElement('div');
    col.className = 'bento__cell bento__cell--2 skeleton-tile';
    col.innerHTML = `<div class="kpi h-100">
      <div class="skeleton mb-3" style="height:.75rem;width:60%"></div>
      <div class="skeleton" style="height:2rem;width:40%"></div>
    </div>`;
    insertTile(col);
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

function renderHeroTile(tileId, title, hero, chips) {
  const col = document.createElement('div');
  col.className = 'bento__cell bento__cell--3';
  col.dataset.tile = tileId;
  const chipHtml = chips.map(function (chip) {
    const modifier = chip.variant && chip.variant !== 'fresh' ? ' kpi__chip--' + chip.variant : '';
    return `<span class="kpi__chip${modifier}"><i class="bi bi-${chip.icon}" aria-hidden="true"></i>${chip.label}</span>`;
  }).join('');
  col.innerHTML = `
    <div class="kpi kpi--hero h-100">
      <div class="kpi__label">${title}</div>
      <div class="kpi__value">${hero}</div>
      ${chipHtml ? `<div class="kpi__chips">${chipHtml}</div>` : ''}
    </div>`;
  return col;
}

// The variants a tile's value can carry: the status quad plus the accent for
// the money tiles.
const TILE_VALUE_VARIANTS = ['danger', 'warning', 'success', 'accent'];

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function renderTile(tileId, title, hero, variant, heroClass, span, sub) {
  const col = document.createElement('div');
  col.className = 'bento__cell bento__cell--' + (span || 2);
  col.dataset.tile = tileId;
  // A solid fill is reserved for the expired state — the design system's
  // bento sheet fills exactly one card. Every other variant colours the
  // value, which is what the status text variants exist for.
  const fillClass = variant === 'danger' ? 'kpi--fill-expired' : '';
  const valueClass = TILE_VALUE_VARIANTS.includes(variant) ? ' kpi__value--' + variant : '';
  const subHtml = sub
    ? `<div class="kpi__sub${sub.variant ? ' kpi__sub--' + sub.variant : ''}"><i class="bi bi-${sub.icon}" aria-hidden="true"></i>${sub.text}</div>`
    : '';

  if (fillClass && !heroClass && Number.parseFloat(hero) > 0) {
    col.innerHTML = `
      <div class="kpi ${fillClass} h-100">
        <div class="kpi__label">${title}</div>
        <div class="kpi__value" title="${escapeHtml(hero)}">${escapeHtml(hero)}</div>
        ${subHtml}
      </div>`;
    return col;
  }

  const heroInnerClass = heroClass || 'kpi__value';
  col.innerHTML = `
    <div class="kpi h-100">
      <div class="kpi__label">${title}</div>
      <div class="${heroInnerClass}${valueClass}" title="${escapeHtml(hero)}">${escapeHtml(hero)}</div>
      ${subHtml}
    </div>`;
  return col;
}

// The explanation of a tile: an info trigger behind its eyebrow that carries
// the text in its tooltip.
function attachTileHint(cell, text) {
  if (!text) return;
  const label = cell.querySelector('.kpi__label');
  if (!label) return;

  const trigger = document.createElement('button');
  trigger.type = 'button';
  trigger.className = 'tile-threshold-info';
  trigger.setAttribute('aria-label', text);
  const icon = document.createElement('i');
  icon.className = 'bi bi-info-circle';
  icon.setAttribute('aria-hidden', 'true');
  trigger.appendChild(icon);
  label.appendChild(trigger);

  attachInfoTooltip(trigger, text, 'top');
}

// A tooltip opens on hover and on keyboard focus.
function attachInfoTooltip(trigger, text, placement) {
  if (!trigger || typeof bootstrap === 'undefined') return;
  bootstrap.Tooltip.getOrCreateInstance(trigger, {
    title: text,
    placement: placement || 'left',
    trigger: 'hover focus',
  });
}

// The expiry status of a product, derived from its date with the app's own
// thresholds: critical within three days, soon within seven.
function expiryStatus(expireAt) {
  const days = (new Date(expireAt + 'T00:00:00') - new Date()) / 86400000;
  if (days < 0) return 'expired';
  if (days < 3) return 'critical';
  if (days < 7) return 'soon';
  return 'fresh';
}

const EXPIRY_STATUS = {
  expired: { label: 'status.expired', icon: 'x-circle-fill' },
  critical: { label: 'status.critical', icon: 'exclamation-circle-fill' },
  soon: { label: 'status.soon', icon: 'clock-fill' },
  fresh: { label: 'status.fresh', icon: 'check-circle-fill' },
};

function renderListTile(title, items, days) {
  const col = document.createElement('div');
  col.className = 'h-100';

  let listHtml;
  if (items.length === 0) {
    listHtml = `<li class="list-group-item fg-secondary small py-3">${proviant.tn("js.home.noExpiringProducts", days)}</li>`;
  } else {
    listHtml = items.map((item) => {
      const status = expiryStatus(item.expireAt);
      const meta = EXPIRY_STATUS[status];
      const date = new Date(item.expireAt + 'T00:00:00');
      const dateLabel = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
      return `<li class="list-group-item d-flex align-items-center gap-5 px-5 py-3">
        <span class="list-group-item__stripe list-group-item__stripe--${status}" aria-hidden="true"></span>
        <span class="list-group-item__body">
          <span class="list-group-item__title text-truncate">${item.productName}</span>
        </span>
        <span class="list-group-item__trailing">
          <span class="expiry-text status-${status}">${dateLabel}</span>
          <span class="badge badge--${status}"><i class="bi bi-${meta.icon}" aria-hidden="true"></i>${proviant.t(meta.label)}</span>
        </span>
      </li>`;
    }).join('');
  }

  const countChip = items.length
    ? `<span class="badge badge--critical"><i class="bi bi-exclamation-circle-fill" aria-hidden="true"></i>${proviant.tn("js.home.expiringItemCount", items.length)}</span>`
    : '';

  col.innerHTML = `
    <div class="card h-100" style="overflow:hidden;">
      <div class="card-header d-flex flex-row align-items-center gap-3 fw-bold">
        <div class="icon-chip" aria-hidden="true">
          <i class="bi bi-clock-history"></i>
        </div>
        <span class="flex-grow-1">${title}</span>
        ${countChip}
        <button type="button" class="tile-threshold-info"><i class="bi bi-info-circle" aria-hidden="true"></i></button>
      </div>
      <div class="tile-scroll-body">
        <ul class="list-group list-group-flush">${listHtml}</ul>
      </div>
    </div>`;

  return col;
}

let dashboardCharts = [];
let lastStats = null;

function setChartValue(id, value) {
  const element = document.getElementById(id);
  if (element) element.textContent = value;
}

// A category name that is too long for the axis is cut with an ellipsis.
function shortLabel(name, max) {
  return name.length > max ? `${name.slice(0, max - 1)}…` : name;
}

// The category keys reach the dashboard as slugs and as the two buckets the
// API fills itself ("Other" for the tail, "Uncategorized" for products without
// a category).
function categoryLabel(name) {
  if (name === 'Other') return proviant.t("js.chart.other");
  if (name === 'Uncategorized') return proviant.t("js.chart.uncategorized");
  const words = name.replace(/[-_]+/g, ' ').trim();
  if (!words) return name;
  return words.charAt(0).toUpperCase() + words.slice(1);
}

function renderCharts(s) {
  dashboardCharts.forEach((chart) => chart.destroy());
  dashboardCharts = [];
  const t = proviantChart.applyTheme();

  // Chart 1 — Waste donut (expired vs fresh)
  clearEmptyChart('chartWaste');
  const expired = s.wasteCount || 0;
  const active = s.totalActive || 0;
  setChartValue('wasteRateValue', active ? `${s.wastePercent.toFixed(1)}%` : '—');
  if (!active) {
    showEmptyChart('chartWaste', proviant.t("js.home.noActiveProducts"));
  } else {
    const base = proviantChart.baseOptions(t);
    dashboardCharts.push(new Chart(document.getElementById('chartWaste'), {
      type: 'doughnut',
      data: {
        labels: [proviant.t("status.expired"), proviant.t("status.fresh")],
        datasets: [{
          data: [expired, active - expired],
          backgroundColor: [t.statusExpired, t.statusFresh],
          borderWidth: 0,
          borderRadius: 4,
          hoverOffset: 6,
        }],
      },
      options: {
        ...base,
        cutout: '70%',
        // A doughnut has no axes — the base options would draw a pair of them.
        scales: {},
        plugins: {
          ...base.plugins,
          legend: {
            ...base.plugins.legend,
            labels: { ...base.plugins.legend.labels, generateLabels: proviantChart.shareLegend },
          },
          proviantCenterText: {
            value: String(expired),
            label: proviant.t("js.home.expiredCenterLabel"),
            color: t.fg,
            labelColor: t.muted,
          },
          tooltip: {
            callbacks: {
              label: (context) => {
                const value = context.parsed;
                const share = active > 0 ? Math.round((value / active) * 100) : 0;
                return `${context.label}: ${value} (${share}%)`;
              },
            },
          },
        },
      },
    }));
  }

  // Chart 2 — Category breakdown (ranked bars)
  clearEmptyChart('chartCategories');
  const categories = Object.keys(s.categories || {})
    .map((name) => [name, s.categories[name] || 0])
    .sort((a, b) => b[1] - a[1]);
  setChartValue(
    'categoryCountValue',
    categories.length ? proviant.tn("js.home.categoryCount", categories.length) : '—',
  );
  if (!categories.length) {
    showEmptyChart('chartCategories', proviant.t("js.home.noCategoryData"));
  } else {
    const base = proviantChart.baseOptions(t);
    dashboardCharts.push(new Chart(document.getElementById('chartCategories'), {
      type: 'bar',
      data: {
        labels: categories.map(([name]) => shortLabel(categoryLabel(name), 18)),
        datasets: [{
          data: categories.map(([, count]) => count),
          backgroundColor: t.accent,
          borderRadius: 6,
          borderSkipped: false,
          maxBarThickness: 22,
        }],
      },
      options: {
        ...base,
        indexAxis: 'y',
        plugins: {
          ...base.plugins,
          legend: { display: false },
          proviantBarValues: { enabled: true, color: t.fg2, insideColor: t.accentOn },
          tooltip: {
            callbacks: {
              title: (items) => categoryLabel(categories[items[0].dataIndex][0]),
              label: (context) => proviant.tn("js.chart.itemCount", context.parsed.x),
            },
          },
        },
        scales: {
          x: {
            beginAtZero: true,
            grid: { color: t.border, drawBorder: false },
            border: { display: false },
            ticks: { color: t.muted, padding: 8, precision: 0 },
          },
          y: {
            grid: { display: false, drawBorder: false },
            border: { display: false },
            ticks: { color: t.fg2, padding: 6, autoSkip: false },
          },
        },
      },
    }));
  }

  // Chart 3 — Expiry trend (line)
  clearEmptyChart('chartExpiryTrend');
  const trend = s.expiryTrend || [];
  const expiring = trend.reduce((sum, month) => sum + (month.count || 0), 0);
  setChartValue(
    'expiryTrendValue',
    trend.length ? proviant.tn("js.chart.itemCount", expiring) : '—',
  );
  if (!trend.length) {
    showEmptyChart('chartExpiryTrend', proviant.t("js.home.noExpiryTrendData"));
  } else {
    const base = proviantChart.baseOptions(t);
    dashboardCharts.push(new Chart(document.getElementById('chartExpiryTrend'), {
      type: 'line',
      data: {
        labels: trend.map((m) => proviantChart.monthLabel(m.month)),
        datasets: [{
          label: proviant.t("js.home.productsExpiring"),
          data: trend.map((m) => m.count || 0),
          borderColor: t.accent,
          pointBackgroundColor: t.accent,
          pointBorderColor: t.accent,
          backgroundColor: (context) => proviantChart.areaGradient(context.chart, t.accent, 0.28, 0),
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
              title: (items) => proviantChart.monthTitle(trend[items[0].dataIndex].month),
              label: (context) => proviant.tn("js.chart.itemCount", context.parsed.y),
            },
          },
        },
        scales: {
          x: {
            grid: { display: false, drawBorder: false },
            border: { display: false },
            ticks: { color: t.muted, padding: 6, maxRotation: 0, autoSkip: true, maxTicksLimit: 12 },
          },
          y: {
            beginAtZero: true,
            grid: { color: t.border, drawBorder: false },
            border: { display: false },
            ticks: { color: t.muted, padding: 8, precision: 0 },
          },
        },
      },
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
      if (status) status.textContent = proviant.t("js.home.loadFailed");
      return;
    }
    const s = response.message;

    const dashboard = document.getElementById('dashboard');
    if (dashboard) {
      // The pantry count leads the dashboard as the filled hero tile, the way
      // the screen sheets draw it.
      const expiringSoon = s.expiringSoon || [];
      const expiredCount = s.wasteCount || 0;
      const freshCount = Math.max(0, s.totalActive - expiredCount - expiringSoon.length);
      insertTile(renderHeroTile(
        'hero',
        proviant.t("js.home.activeProducts"),
        s.totalActive,
        [
          {
            variant: 'fresh',
            icon: 'check2',
            label: proviant.t("js.home.freshCount", freshCount),
          },
          {
            variant: 'soon',
            icon: 'clock',
            label: proviant.t("js.home.expiringSoonCount", expiringSoon.length),
          },
          {
            variant: 'expired',
            icon: 'exclamation',
            label: proviant.t("js.home.expiredCount", expiredCount),
          },
        ],
      ));

      const tiles = [
        {
          tile: 'waste-rate',
          title: proviant.t("js.home.expiredNotConsumed"),
          hero: `${s.wastePercent.toFixed(1)}%`,
          variant: expiredCount > 0 ? 'danger' : null,
          span: 3,
          sub: {
            text: proviant.t("js.home.expiredOfTotal", expiredCount, s.totalActive),
            icon: 'exclamation',
          },
        },
        {
          tile: 'consumed',
          title: proviant.t("js.home.totalConsumed"),
          hero: s.totalArchived,
          span: 2,
          sub: {
            text: proviant.t("js.home.archivedNotWasted"),
            icon: 'check2-circle',
            variant: 'fresh',
          },
        },
        {
          tile: 'last-added',
          title: proviant.t("js.home.lastAddedProduct"),
          hero: s.lastInsertedProduct || '—',
          heroClass: 'kpi__value text-truncate',
          span: 2,
          sub: {
            text: proviant.t("js.home.recentlyAdded"),
            icon: 'plus-circle',
            variant: 'muted',
          },
        },
      ];

      tiles.forEach(({ tile, title, hero, variant, heroClass, span, sub }) => {
        insertTile(renderTile(tile, title, hero, variant, heroClass, span, sub));
      });

      // Streak tile
      const streak = (streakResponse.code === 200 && streakResponse.message) ? streakResponse.message : null;
      const currentStreak = streak ? streak.currentStreak : 0;
      const longestStreak = streak ? streak.longestStreak : 0;
      const streakHero = proviant.tn("js.home.streakDays", currentStreak);
      const streakHeroClass = currentStreak > 0 ? 'kpi__value' : 'kpi__value fg-secondary';
      const streakCol = renderTile('streak', '<i class="bi bi-fire"></i> ' + proviant.t("js.home.wasteFreeStreak"), streakHero, null, streakHeroClass, 2);
      const streakTile = streakCol.querySelector('.kpi');
      if (streakTile) {
        if (longestStreak > 0) {
          const sub = document.createElement('div');
          sub.className = 'kpi__sub kpi__sub--muted';
          sub.textContent = proviant.tn("js.home.bestStreak", longestStreak);
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
          prog.style.cssText = 'margin-top:var(--space-2);font-size:var(--text-xs);color:var(--meta);';
          prog.innerHTML = `
            <div class="d-flex justify-content-between mb-1">
              <span>${proviant.t("js.home.nextMilestone", nextMilestone)}</span>
              <span>${intoMilestone}/${nextMilestone - prevMilestone}d</span>
            </div>
            <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${milestonePct}" style="height:6px;">
              <div class="progress-bar" style="width:${milestonePct}%;background:var(--status-fresh);"></div>
            </div>`;
          streakTile.appendChild(prog);
        } else {
          const prog = document.createElement('div');
          prog.style.cssText = 'margin-top:var(--space-2);font-size:var(--text-xs);color:var(--status-fresh);';
          prog.innerHTML = `<i class="bi bi-trophy-fill me-1"></i>${proviant.t("js.home.allMilestonesReached")}`;
          streakTile.appendChild(prog);
        }
      }
      insertTile(streakCol);

      // Monthly waste goal tile
      const settingsResponse = await proviant.getHouseholdSettings();
      if (settingsResponse.code === 200 && settingsResponse.message) {
        const hs = settingsResponse.message;
        if (hs.monthlyWasteGoalType === 'count' || hs.monthlyWasteGoalType === 'percent') {
          const goalCol = document.createElement('div');
          goalCol.className = 'bento__cell bento__cell--2';
          goalCol.dataset.tile = 'goal';
          const wasteThisMonth = s.wasteCount;
          let pct = 0;
          let label = '';
          if (hs.monthlyWasteGoalType === 'count') {
            const goal = hs.monthlyWasteGoalCount || 0;
            pct = goal > 0 ? Math.min(100, Math.round((wasteThisMonth / goal) * 100)) : 0;
            label = proviant.t("js.home.wastedOfGoal", wasteThisMonth, goal);
          } else {
            const goal = hs.monthlyWasteGoalPercent || 0;
            pct = goal > 0 ? Math.min(100, Math.round((s.wastePercent / goal) * 100)) : 0;
            label = `${s.wastePercent.toFixed(1)}% / ${goal}%`;
          }
          const exceeded = pct >= 100;
          const barColor = exceeded ? 'var(--status-expired)' : (pct >= 75 ? 'var(--status-soon)' : 'var(--status-fresh)');
          goalCol.innerHTML = `
            <div class="kpi h-100">
              <div class="kpi__label"><i class="bi bi-bullseye me-1"></i>${proviant.t("js.home.monthlyGoal")}</div>
              <div class="kpi__value${exceeded ? ' kpi__value--danger' : ''}" title="${label}">${label}</div>
              <div class="progress mt-3" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct}" style="height:6px;">
                <div class="progress-bar" style="width:${pct}%;background:${barColor};"></div>
              </div>
            </div>`;
          insertTile(goalCol);
        }
      }

      // Activity feed tile
      const activityResponse = await proviant.getActivityFeed(10);
      if (activityResponse.code === 200 && activityResponse.message && activityResponse.message.activities) {
        const activities = activityResponse.message.activities;
        const activityCol = document.createElement('div');
        activityCol.className = 'h-100';

        const actionIconMap = {
          add: 'bi-plus-circle',
          consume: 'bi-check-circle',
          waste: 'bi-trash3',
          restore: 'bi-arrow-counterclockwise',
          cook: 'bi-fire',
          amount_change: 'bi-pencil',
          import: 'bi-upload',
        };
        const actionKeyMap = {
          add: 'js.home.activityAdded',
          consume: 'js.home.activityConsumed',
          waste: 'js.home.activityWasted',
          restore: 'js.home.activityRestored',
          cook: 'js.home.activityCooked',
          amount_change: 'js.home.activityAdjusted',
          import: 'js.home.activityImported',
        };

        function relativeTime(timestamp) {
          const now = Date.now();
          const then = new Date(timestamp).getTime();
          const diff = Math.floor((now - then) / 1000);
          if (diff < 60) return proviant.t("js.home.justNow");
          if (diff < 3600) return proviant.t("js.home.minutesAgo", Math.floor(diff / 60));
          if (diff < 86400) return proviant.tn("js.home.hoursAgo", Math.floor(diff / 3600));
          return proviant.tn("js.home.daysAgo", Math.floor(diff / 86400));
        }

        let listHtml;
        if (activities.length === 0) {
          listHtml = `<li class="list-group-item fg-secondary small py-3">${proviant.t("js.home.noRecentActivity")}</li>`;
        } else {
          listHtml = activities.map((a) => {
            const icon = actionIconMap[a.action] || 'bi-circle';
            const productName = a.productName || proviant.t("js.home.aProduct");
            const actionText = actionKeyMap[a.action]
              ? proviant.t(actionKeyMap[a.action], productName)
              : `${a.action} ${productName}`;
            // The reason belongs to a waste; every other action has none.
            const causeText = a.wasteCause
              ? ` <span class="fg-secondary">· ${proviant.wasteCauseLabel(a.wasteCause)}</span>`
              : '';
            return `<li class="list-group-item d-flex justify-content-between align-items-center px-5 py-3">
              <span class="d-flex align-items-center gap-3 text-truncate me-2 small">
                <i class="bi ${icon}" style="font-size:12px;color:var(--accent);flex-shrink:0;"></i>
                <span class="text-truncate"><strong>${a.userName || proviant.t("js.home.someone")}</strong> ${actionText}${causeText}</span>
              </span>
              <span class="text-nowrap fg-secondary small" style="flex-shrink:0;">${relativeTime(a.timestamp)}</span>
            </li>`;
          }).join('');
        }

        activityCol.innerHTML = `
          <div class="card h-100" style="overflow:hidden;">
            <div class="card-header d-flex flex-row align-items-center gap-3 fw-bold">
              <div style="width:28px;height:28px;border-radius:7px;background:var(--accent-soft);display:flex;align-items:center;justify-content:center;flex-shrink:0;" aria-hidden="true">
                <i class="bi bi-activity" style="font-size:12px;color:var(--accent);"></i>
              </div>
              <span class="flex-grow-1">${proviant.t("js.home.recentActivity")}</span>
            </div>
            <div class="tile-scroll-body">
              <ul class="list-group list-group-flush">${listHtml}</ul>
            </div>
          </div>`;

        const activityRow = document.createElement('div');
        activityRow.className = 'bento__cell bento__cell--6';
        activityRow.dataset.tile = 'activity';
        activityRow.appendChild(activityCol);
        insertTile(activityRow);
      }

      // Savings tiles
      const savings = (savingsResponse.code === 200 && savingsResponse.message) ? savingsResponse.message : null;
      if (savings) {
        // The events from before the count model are not in the totals: their
        // figures were scaled by the net content, not by a count of packs.
        const legacyEvents = savings.legacyEvents || 0;
        const legacyHint = legacyEvents > 0
          ? proviant.tn("js.home.legacySavingsNote", legacyEvents)
          : null;

        const eurTile = renderTile(
          'savings-eur',
          proviant.t("js.home.savedThisMonth"),
          `€${(savings.savedEurThisMonth ?? 0).toFixed(2)}`,
          savings.savedEurThisMonth > 0 ? 'accent' : null,
          null,
        );
        insertTile(eurTile);
        attachTileHint(eurTile, legacyHint);

        const co2Tile = renderTile(
          'savings-co2',
          proviant.t("js.home.co2AvoidedThisMonth"),
          `${(savings.savedCo2KgThisMonth ?? 0).toFixed(2)} kg`,
          savings.savedCo2KgThisMonth > 0 ? 'accent' : null,
          null,
          2,
          {
            text: proviant.t("js.home.versusWastingIt"),
            icon: 'cloud-arrow-down',
            variant: 'accent',
          },
        );
        insertTile(co2Tile);
        attachTileHint(co2Tile, legacyHint);
      }
    }

    const dashboardList = document.getElementById('dashboard-list');
    if (dashboardList) {
      const days = s.expiringSoonDays ?? 7;
      const listTile = renderListTile(proviant.tn("js.home.expiringWithin", days), s.expiringSoon ?? [], days);
      dashboardList.appendChild(listTile);

      const infoTrigger = listTile.querySelector('.tile-threshold-info');
      if (infoTrigger) {
        const text = proviant.tn("js.home.thresholdTooltip", days);
        infoTrigger.setAttribute('aria-label', text);
        attachInfoTooltip(infoTrigger, text, 'left');
      }
    }

    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = proviant.t("js.home.loaded");

    lastStats = s;

    // Every tile stands in the grid now: the layout engine arranges them.
    document.dispatchEvent(new CustomEvent('proviant:dashboardready'));

    renderCharts(s);
  } catch (error) {
    console.error('proviant: dashboard load failed', error);
    clearSkeletons();
    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = proviant.t("js.home.loadFailed");
  }
});
})();

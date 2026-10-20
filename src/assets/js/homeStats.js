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
    col.innerHTML = `<div class="kpi h-100">
      <div class="skeleton mb-2" style="height:.75rem;width:60%"></div>
      <div class="skeleton" style="height:2rem;width:40%"></div>
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
    `<p class="text-body-secondary small text-center my-auto py-4 chart-empty-msg">${message}</p>`,
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
  // A solid fill is reserved for the expired state — the design system's
  // bento sheet fills exactly one card. Every other variant colours the
  // value, which is what the status text variants exist for.
  const fillMap = { danger: 'var(--status-expired)' };
  const inkMap  = { danger: 'var(--status-expired-on)' };
  const textMap = {
    danger: 'var(--status-expired-text)',
    warning: 'var(--status-soon-text)',
    success: 'var(--status-fresh-text)',
  };

  // Accent-fill tile for non-zero urgent values
  if (variant && fillMap[variant] && !heroClass && Number.parseFloat(hero) > 0) {
    const fill = fillMap[variant];
    const ink  = inkMap[variant];
    col.innerHTML = `
      <div class="kpi h-100" style="background:${fill};box-shadow:0 6px 18px color-mix(in oklab, ${fill} 33%, transparent);border-color:transparent;position:relative;overflow:hidden;">
        <div style="position:absolute;right:-10px;top:-10px;width:70px;height:70px;border-radius:999px;background:color-mix(in oklab, ${ink} 8%, transparent);pointer-events:none;"></div>
        <div style="position:relative;">
          <div class="kpi__label" style="color:color-mix(in oklab, ${ink} 88%, transparent);">${title}</div>
          <div class="kpi__value" style="color:${ink};" title="${hero}">${hero}</div>
        </div>
      </div>`;
    return col;
  }

  const heroColor = (variant && textMap[variant]) ? textMap[variant] : 'var(--fg)';
  const heroStyle = heroClass ? '' : `style="color:${heroColor}"`;
  const heroInnerClass = heroClass || 'kpi__value';
  col.innerHTML = `
    <div class="kpi h-100">
      <div class="kpi__label">${title}</div>
      <div class="${heroInnerClass}" ${heroStyle} title="${hero}">${hero}</div>
    </div>`;
  return col;
}

function renderListTile(title, items, days) {
  const col = document.createElement('div');
  col.className = 'h-100';

  let listHtml;
  if (items.length === 0) {
    listHtml = `<li class="list-group-item text-body-secondary small py-2">${proviant.tn("js.home.noExpiringProducts", days)}</li>`;
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
        <div style="width:28px;height:28px;border-radius:7px;background:var(--accent-soft);display:flex;align-items:center;justify-content:center;flex-shrink:0;" aria-hidden="true">
          <i class="bi bi-clock-history" style="font-size:12px;color:var(--accent);"></i>
        </div>
        <span class="flex-grow-1">${title}</span>
        <i class="bi bi-info-circle tile-threshold-info" style="font-size:14px;cursor:default;color:var(--meta);font-weight:normal;"></i>
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
    showEmptyChart('chartWaste', proviant.t("js.home.noActiveProducts"));
  } else {
    dashboardCharts.push(new Chart(document.getElementById('chartWaste'), {
      type: 'doughnut',
      data: {
        labels: [proviant.t("status.expired"), proviant.t("status.fresh")],
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
    showEmptyChart('chartCategories', proviant.t("js.home.noCategoryData"));
  } else {
    dashboardCharts.push(new Chart(document.getElementById('chartCategories'), {
      type: 'pie',
      data: {
        labels: Object.keys(s.categories),
        datasets: [{
          data: Object.values(s.categories),
          // The design system has no categorical palette: the category
          // breakdown borrows the status language. Six slots, cycled by
          // Chart.js when a household has more categories than that.
          backgroundColor: [
            cssVar('--accent'),
            cssVar('--status-critical'),
            cssVar('--info'),
            cssVar('--status-soon'),
            cssVar('--status-neutral'),
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
    showEmptyChart('chartExpiryTrend', proviant.t("js.home.noExpiryTrendData"));
  } else {
    dashboardCharts.push(new Chart(document.getElementById('chartExpiryTrend'), {
      type: 'line',
      data: {
        labels: s.expiryTrend.map((m) => m.month),
        datasets: [{
          label: proviant.t("js.home.productsExpiring"),
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
      if (status) status.textContent = proviant.t("js.home.loadFailed");
      return;
    }
    const s = response.message;

    const dashboard = document.getElementById('dashboard');
    if (dashboard) {
      const tiles = [
        {
          title: proviant.t("js.home.activeProducts"),
          hero: s.totalActive,
        },
        {
          title: proviant.t("js.home.expiredNotConsumed"),
          hero: `${s.wastePercent.toFixed(1)}%`,
          variant: s.wasteCount > 0 ? 'danger' : null,
        },
        {
          title: proviant.t("js.home.totalConsumed"),
          hero: s.totalArchived,
        },
        {
          title: proviant.t("js.home.lastAddedProduct"),
          hero: s.lastInsertedProduct || '—',
          heroClass: 'kpi__value text-truncate',
        },
      ];

      tiles.forEach(({ title, hero, variant, heroClass }) => {
        dashboard.appendChild(renderTile(title, hero, variant, heroClass));
      });

      // Streak tile
      const streak = (streakResponse.code === 200 && streakResponse.message) ? streakResponse.message : null;
      const currentStreak = streak ? streak.currentStreak : 0;
      const longestStreak = streak ? streak.longestStreak : 0;
      const streakHero = proviant.tn("js.home.streakDays", currentStreak);
      const streakHeroClass = currentStreak > 0 ? 'kpi__value' : 'kpi__value text-body-secondary';
      const streakCol = renderTile('<i class="bi bi-fire"></i> ' + proviant.t("js.home.wasteFreeStreak"), streakHero, null, streakHeroClass);
      const streakTile = streakCol.querySelector('.kpi');
      if (streakTile) {
        if (longestStreak > 0) {
          const sub = document.createElement('div');
          sub.style.cssText = 'font-size:var(--text-xs);color:var(--meta);margin-top:var(--space-1)';
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
              <div class="kpi__value" title="${label}" style="color:${exceeded ? 'var(--status-expired)' : 'var(--fg)'}">${label}</div>
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
          listHtml = `<li class="list-group-item text-body-secondary small py-2">${proviant.t("js.home.noRecentActivity")}</li>`;
        } else {
          listHtml = activities.map((a) => {
            const icon = actionIconMap[a.action] || 'bi-circle';
            const productName = a.productName || proviant.t("js.home.aProduct");
            const actionText = actionKeyMap[a.action]
              ? proviant.t(actionKeyMap[a.action], productName)
              : `${a.action} ${productName}`;
            return `<li class="list-group-item d-flex justify-content-between align-items-center px-3 py-2">
              <span class="d-flex align-items-center gap-2 text-truncate me-2 small">
                <i class="bi ${icon}" style="font-size:12px;color:var(--accent);flex-shrink:0;"></i>
                <span class="text-truncate"><strong>${a.userName || proviant.t("js.home.someone")}</strong> ${actionText}</span>
              </span>
              <span class="text-nowrap text-body-secondary small" style="flex-shrink:0;">${relativeTime(a.timestamp)}</span>
            </li>`;
          }).join('');
        }

        activityCol.innerHTML = `
          <div class="card h-100" style="overflow:hidden;">
            <div class="card-header d-flex align-items-center gap-2 fw-bold">
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
        activityRow.className = 'col-12';
        activityRow.appendChild(activityCol);
        dashboard.appendChild(activityRow);
      }

      // Savings tiles
      const savings = (savingsResponse.code === 200 && savingsResponse.message) ? savingsResponse.message : null;
      if (savings) {
        dashboard.appendChild(renderTile(
          proviant.t("js.home.savedThisMonth"),
          `€${(savings.savedEurThisMonth ?? 0).toFixed(2)}`,
          savings.savedEurThisMonth > 0 ? 'success' : null,
          null,
        ));
        dashboard.appendChild(renderTile(
          proviant.t("js.home.co2AvoidedThisMonth"),
          `${(savings.savedCo2KgThisMonth ?? 0).toFixed(2)} kg`,
          savings.savedCo2KgThisMonth > 0 ? 'success' : null,
          null,
        ));
        // The events from before the count model are not in the totals: their
        // figures were scaled by the net content, not by a count of packs.
        const legacyEvents = savings.legacyEvents || 0;
        if (legacyEvents > 0) {
          const note = document.createElement('div');
          note.className = 'col-12 text-body-secondary small mt-1';
          note.textContent = proviant.tn("js.home.legacySavingsNote", legacyEvents);
          dashboard.appendChild(note);
        }
      }
    }

    const dashboardList = document.getElementById('dashboard-list');
    if (dashboardList) {
      const days = s.expiringSoonDays ?? 7;
      const listTile = renderListTile(proviant.tn("js.home.expiringWithin", days), s.expiringSoon ?? [], days);
      dashboardList.appendChild(listTile);

      const infoIcon = listTile.querySelector('.tile-threshold-info');
      if (infoIcon) {
        const tooltip = new bootstrap.Tooltip(infoIcon, {
          title: proviant.tn("js.home.thresholdTooltip", days),
          placement: 'left',
          trigger: 'manual',
        });
        infoIcon.addEventListener('mouseenter', () => tooltip.show());
        document.addEventListener('click', () => tooltip.hide(), { once: false, capture: true });
      }
    }

    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = proviant.t("js.home.loaded");

    lastStats = s;
    renderCharts(s);
  } catch {
    clearSkeletons();
    const status = document.getElementById('dashboard-status');
    if (status) status.textContent = proviant.t("js.home.loadFailed");
  }
});
})();

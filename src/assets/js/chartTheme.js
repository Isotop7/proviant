// The chart language of the app: one theme for every Chart.js canvas
// (design-system/DESIGN.md §5 "Diagramme / Charts"). Loaded after
// chart.umd.min.js and before the script that draws a page's charts.
(function() {
'use strict';

const FONT_STACK = '"Jost", system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif';

function token(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

// The derived slots are color-mix() expressions, which a canvas cannot parse;
// a probe element resolves them into their computed color.
function resolve(value) {
  if (!value || !value.includes('color-mix')) return value;
  const probe = document.createElement('span');
  probe.style.display = 'none';
  probe.style.color = value;
  if (!probe.style.color) return value;
  document.body.appendChild(probe);
  const computed = getComputedStyle(probe).color;
  probe.remove();
  return computed || value;
}

// A colour with an alpha channel, for the area fill under a line.
function withAlpha(color, alpha) {
  const value = String(color).trim();
  if (value.charAt(0) === '#') {
    const hex = value.slice(1);
    const full = hex.length === 3 ? hex.split('').map((c) => c + c).join('') : hex;
    if (full.length < 6) return value;
    const r = Number.parseInt(full.slice(0, 2), 16);
    const g = Number.parseInt(full.slice(2, 4), 16);
    const b = Number.parseInt(full.slice(4, 6), 16);
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  const match = value.match(/^rgba?\(([^)]+)\)$/i);
  if (match) {
    const parts = match[1].split(/[,\s/]+/).filter(Boolean);
    if (parts.length >= 3) return `rgba(${parts[0]}, ${parts[1]}, ${parts[2]}, ${alpha})`;
  }
  return value;
}

function prefersReducedMotion() {
  return Boolean(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
}

function tokens() {
  return {
    accent: resolve(token('--accent')),
    accentOn: resolve(token('--accent-on')),
    fg: resolve(token('--fg')),
    fg2: resolve(token('--fg-2')),
    muted: resolve(token('--muted')),
    border: resolve(token('--border')),
    surface: resolve(token('--surface')),
    statusFresh: resolve(token('--status-fresh')),
    statusExpired: resolve(token('--status-expired')),
  };
}

// The tokens of the current theme, ready for Chart.js.
function applyTheme() {
  const t = tokens();
  Chart.defaults.font.family = FONT_STACK;
  Chart.defaults.font.size = 12;
  Chart.defaults.color = t.muted;
  Chart.defaults.borderColor = t.border;
  Chart.defaults.elements.arc.borderWidth = 0;
  Chart.defaults.elements.line.borderWidth = 2;
  Chart.defaults.elements.line.tension = 0.35;
  Chart.defaults.elements.point.radius = 0;
  Chart.defaults.elements.point.hoverRadius = 5;
  Chart.defaults.plugins.tooltip.backgroundColor = t.surface;
  Chart.defaults.plugins.tooltip.borderColor = t.border;
  Chart.defaults.plugins.tooltip.borderWidth = 1;
  Chart.defaults.plugins.tooltip.titleColor = t.fg;
  Chart.defaults.plugins.tooltip.bodyColor = t.fg2;
  Chart.defaults.plugins.tooltip.padding = 10;
  Chart.defaults.plugins.tooltip.cornerRadius = 8;
  Chart.defaults.plugins.tooltip.displayColors = false;
  Chart.defaults.plugins.tooltip.titleFont = { family: FONT_STACK, size: 12, weight: '600' };
  Chart.defaults.plugins.tooltip.bodyFont = { family: FONT_STACK, size: 12 };
  Chart.defaults.plugins.legend.labels.font = { family: FONT_STACK, size: 13 };
  // The screenshot tool turns the global animation off before Chart.js loads,
  // so the timing is only set when the default is still an object.
  if (prefersReducedMotion()) {
    Chart.defaults.animation = false;
  } else if (Chart.defaults.animation && typeof Chart.defaults.animation === 'object') {
    Chart.defaults.animation.duration = 600;
    Chart.defaults.animation.easing = 'easeOutQuart';
  }
  return t;
}

// The options every chart in the app shares: the canvas fills its frame, the
// legend sits below it, the value axis starts at zero.
function baseOptions(t) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index', intersect: false },
    plugins: {
      legend: {
        position: 'bottom',
        labels: {
          usePointStyle: true,
          pointStyle: 'circle',
          boxWidth: 8,
          boxHeight: 8,
          padding: 12,
          color: t.fg2,
          font: { family: FONT_STACK, size: 13 },
        },
      },
    },
    scales: {
      x: {
        grid: { display: false, drawBorder: false },
        border: { display: false },
        ticks: { color: t.muted, padding: 6 },
      },
      y: {
        beginAtZero: true,
        grid: { color: t.border, drawBorder: false },
        border: { display: false },
        ticks: { color: t.muted, padding: 8, precision: 0 },
      },
    },
  };
}

// The vertical gradient under a line chart, cached per chart area so the
// canvas does not build a new gradient for every data point.
function areaGradient(chart, color, topAlpha, bottomAlpha) {
  const area = chart.chartArea;
  const top = area ? area.top : 0;
  const bottom = area ? area.bottom : chart.height;
  const cached = chart.proviantAreaGradient;
  if (cached && cached.top === top && cached.bottom === bottom && cached.color === color) {
    return cached.gradient;
  }
  const gradient = chart.ctx.createLinearGradient(0, top, 0, bottom);
  gradient.addColorStop(0, withAlpha(color, topAlpha));
  gradient.addColorStop(1, withAlpha(color, bottomAlpha));
  chart.proviantAreaGradient = { top, bottom, color, gradient };
  return gradient;
}

// The number in the middle of a doughnut.
const centerTextPlugin = {
  id: 'proviantCenterText',
  afterDraw(chart) {
    const config = chart.options.plugins.proviantCenterText;
    if (!config || !config.value) return;
    const meta = chart.getDatasetMeta(0);
    if (!meta || !meta.data.length) return;
    const arc = meta.data[0];
    const { ctx } = chart;
    const radius = arc.outerRadius;
    const valueSize = Math.max(16, Math.min(30, radius * 0.42));
    ctx.save();
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = config.color || chart.options.color;
    ctx.font = `600 ${valueSize}px ${FONT_STACK}`;
    ctx.fillText(config.value, arc.x, arc.y - (config.label ? valueSize * 0.42 : 0));
    if (config.label) {
      ctx.fillStyle = config.labelColor || chart.options.color;
      ctx.font = `500 12px ${FONT_STACK}`;
      ctx.fillText(config.label, arc.x, arc.y + valueSize * 0.78);
    }
    ctx.restore();
  },
};

// The value at the end of a bar, so a ranked chart reads without a tooltip.
const barValuePlugin = {
  id: 'proviantBarValues',
  afterDatasetsDraw(chart) {
    const config = chart.options.plugins.proviantBarValues;
    if (!config || config.enabled !== true) return;
    const meta = chart.getDatasetMeta(0);
    if (!meta || !meta.data.length) return;
    const { ctx, chartArea } = chart;
    if (!chartArea) return;
    const values = chart.data.datasets[0].data;
    ctx.save();
    ctx.font = `600 12px ${FONT_STACK}`;
    ctx.textBaseline = 'middle';
    meta.data.forEach((bar, index) => {
      const value = values[index];
      if (value === undefined || value === null) return;
      const label = config.formatter ? config.formatter(value) : String(value);
      const width = ctx.measureText(label).width;
      const outside = bar.x + 8 + width <= chartArea.right;
      ctx.fillStyle = outside ? (config.color || token('--fg-2')) : (config.insideColor || token('--accent-on'));
      ctx.textAlign = outside ? 'left' : 'right';
      ctx.fillText(label, outside ? bar.x + 8 : bar.x - 8, bar.y);
    });
    ctx.restore();
  },
};

// The legend of a doughnut carries the count and the share of every slice, so
// the arc does not have to be read against the axis. Chart.js draws the legend
// text in the item's own fontColor, so it has to be carried along.
function shareLegend(chart) {
  const dataset = chart.data.datasets[0];
  const total = dataset.data.reduce((sum, value) => sum + (value || 0), 0);
  const fontColor = chart.options.plugins.legend.labels.color;
  return chart.data.labels.map((label, index) => {
    const value = dataset.data[index] || 0;
    const share = total > 0 ? ` (${Math.round((value / total) * 100)}%)` : '';
    return {
      text: `${label} · ${value}${share}`,
      fillStyle: dataset.backgroundColor[index],
      strokeStyle: dataset.backgroundColor[index],
      fontColor,
      lineWidth: 0,
      pointStyle: 'circle',
      hidden: false,
      index,
      datasetIndex: 0,
    };
  });
}

// "2026-10" → "Oct 26" for an axis, "October 2026" for a tooltip.
function monthLabel(month) {
  const date = parseMonth(month);
  if (!date) return month;
  const name = date.toLocaleDateString(undefined, { month: 'short', timeZone: 'UTC' });
  return `${name} '${String(date.getUTCFullYear()).slice(2)}`;
}

function monthTitle(month) {
  const date = parseMonth(month);
  if (!date) return month;
  return date.toLocaleDateString(undefined, { month: 'long', year: 'numeric', timeZone: 'UTC' });
}

function parseMonth(month) {
  const match = /^(\d{4})-(\d{2})$/.exec(String(month || ''));
  if (!match) return null;
  return new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, 1));
}

const proviantChart = {
  tokens,
  applyTheme,
  baseOptions,
  areaGradient,
  withAlpha,
  monthLabel,
  monthTitle,
  shareLegend,
  centerTextPlugin,
  barValuePlugin,
};

window.proviantChart = proviantChart;

if (typeof Chart !== 'undefined') {
  Chart.register(centerTextPlugin, barValuePlugin);
}
})();

# Proviant Design Guidelines

> Canonical reference for any agent building or updating the Proviant prototype suite. Follow these rules to keep all 14 screens visually consistent and ship-ready.

Reference file: `screens/02-dashboard.html` (bento dashboard) is the canonical layout. Every other screen inherits the same token system, navbar, and component vocabulary.

---

## 1. Product context

**Proviant** is a household food-expiration tracker. Web app in Go + server-rendered Bootstrap 5 templates. Core surfaces: dashboard metrics, product CRUD with barcode scan, recipes, household collaboration, calendar/notification sync, API tokens. Target audience: household food managers (25–55) who cook at home and want to waste less.

Design posture: **warm, modern, calm.** Kitchen-counter analog, not Silicon Valley utility. Trust + clarity over novelty. Restraint over ornament.

---

## 2. Required head block (every screen)

Every prototype HTML must include, in this order, inside `<head>`:

```html
<style>
@font-face {
  font-family: 'Vend Sans';
  font-style: normal;
  font-weight: 300 700;
  font-display: swap;
  src: url('https://cdn.jsdelivr.net/npm/@fontsource-variable/vend-sans/files/vend-sans-latin-wght-normal.woff2') format('woff2-variations');
}
@font-face {
  font-family: 'Vend Sans';
  font-style: italic;
  font-weight: 300 700;
  font-display: swap;
  src: url('https://cdn.jsdelivr.net/npm/@fontsource-variable/vend-sans/files/vend-sans-latin-wght-italic.woff2') format('woff2-variations');
}

  @media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
      scroll-behavior: auto !important;
    }
  }

</style>

<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Page title — Proviant</title>
<!-- Chart.js only on screens that render charts (02-dashboard) -->
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.5.0/dist/chart.umd.min.js"></script>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.13.1/font/bootstrap-icons.min.css">
<style>/* :root + CSS (see section 3) */</style>
```

Vend Sans is the proprietary font; load it from `@fontsource-variable` on jsDelivr. Do not use Inter/Roboto/Arial as display face.

---

## 3. Token system (`:root`)

The complete token block. **Paste verbatim into every file's `<style>`** so `var(--*)` references resolve. The only values that change between themes are the accent hues — leave all other tokens alone.

```css
:root {
  /* ── Brand ── */
  --accent:           oklch(42% .14 155deg);
  --accent-fg:        oklch(0% 0 100deg / .9);
  --accent-hover:     oklch(48% .14 155deg);
  --accent-active:    oklch(36% .14 155deg);
  --accent-subtle:    oklch(90% .06 155deg);
  --accent-muted:     oklch(94% .03 155deg);

  /* ── Text ── */
  --fg:               oklch(16% .022 145deg);
  --fg-2:             oklch(42% .018 145deg);
  --fg-3:             oklch(60% .012 145deg);
  --fg-4:             oklch(72% .008 145deg);

  /* ── Backgrounds ── */
  --bg:               oklch(97% .006 145deg);
  --surface: var(--bg);
  --bg-subtle:        oklch(95% .006 145deg);
  --bg-2:             oklch(92% .006 145deg);

  /* ── Borders ── */
  --border:           oklch(87% .008 145deg);
  --border-strong:    oklch(72% .014 145deg);
  --border-subtle:    oklch(93% .005 145deg);

  /* ── Status ── */
  --status-expired:   oklch(55% .20 25deg);
  --status-critical:  oklch(64% .18 45deg);
  --status-soon:      oklch(70% .16 85deg);
  --status-fresh:     oklch(55% .12 150deg);
  --status-nodate:    oklch(60% .010 70deg);

  /* ── Brand palette ── */
  --brand-slate:      oklch(40% .06 240deg);
  --brand-slate-subtle: oklch(96% .006 240deg);
  --color-success:    oklch(60% .14 152deg);
  --color-warning:    oklch(70% .16 85deg);
  --color-danger:     oklch(55% .20 25deg);
  --color-info:       oklch(55% .14 240deg);

  /* ── Typography ── */
  --font-ui:           'Vend Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
  --font-mono:         'DM Mono', ui-monospace, monospace;
  --text-xs:          0.6875rem;
  --text-sm:          0.8125rem;
  --text-base:        0.875rem;
  --text-md:          1rem;
  --text-lg:          1.125rem;
  --text-xl:          1.25rem;
  --text-2xl:         1.5rem;
  --text-3xl:         1.875rem;
  --text-4xl:         2.25rem;
  --leading-tight:    1.25;
  --leading-normal:   1.5;
  --leading-relaxed:  1.625;
  --tracking-tight:  -0.025em;
  --tracking-normal:  0;
  --tracking-wide:     0.04em;
  --tracking-wider:    0.06em;

  /* ── Radii ── */
  --radius-pill:      9999px;
  --radius-md:        0.625rem;
  --radius-lg:        1rem;
  --radius-sm:        0.375rem;

  /* ── Shadows ── */
  --shadow-xs:        0 1px 2px oklch(0% 0 0deg / .05);
  --shadow-sm:        0 1px 3px oklch(0% 0 0deg / .08), 0 1px 2px oklch(0% 0 0deg / .04);
  --shadow-md:        0 4px 12px oklch(0% 0 0deg / .10), 0 2px 4px oklch(0% 0 0deg / .06);
  --shadow-lg:        0 12px 32px oklch(0% 0 0deg / .12), 0 4px 8px oklch(0% 0 0deg / .06);

  /* ── Spacing ── */
  --space-1:  0.25rem;
  --space-2:  0.5rem;
  --space-3:  0.75rem;
  --space-4:  1rem;
  --space-5:  1.25rem;
  --space-6:  1.5rem;
  --space-8:  2rem;
  --space-10: 2.5rem;
  --space-12: 3rem;
  --space-16: 4rem;
  --space-20: 5rem;
  --space-24: 6rem;

  /* ── Transitions ── */
  --duration-fast:    100ms;
  --duration-base:    150ms;
  --duration-slow:    300ms;
  --duration-slower:  450ms;
  --ease-bounce:      cubic-bezier(.34, 1.56, .64, 1);
  --ease-in:          cubic-bezier(.4, 0, 1, 1);
  --ease-out:         cubic-bezier(0, 0, .2, 1);
  --ease-in-out:      cubic-bezier(.4, 0, .2, 1);
}
```

**Re-skinning**: change only the `--accent` (and its hover/active/subtle/muted) values. The status tokens can also be retuned for different domain semantics.

---

## 4. Global CSS resets (every file)

```css
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html { font-family: var(--font-ui); font-size: 16px; color: var(--fg); background: var(--bg); -webkit-font-smoothing: antialiased; scroll-behavior: smooth; }
body { min-height: 100vh; line-height: var(--leading-normal); }
*:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }

a { color: var(--accent); text-decoration: none; }
a:hover { color: var(--accent-hover); }

::selection { background: var(--accent-subtle); color: var(--fg); }
::-webkit-scrollbar { width: 6px; height: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--border-strong); border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: var(--fg-3); }
```

---

## 5. Component library (every file)

### 5.1 Buttons

```css
.btn { display: inline-flex; align-items: center; justify-content: center; gap: var(--space-2); font-family: var(--font-ui); font-weight: 600; border: none; cursor: pointer; border-radius: var(--radius-md); transition: all var(--duration-base); white-space: nowrap; text-decoration: none; }
.btn-primary { background: var(--accent); color: #fff; padding: 0 var(--space-5); height: 44px; font-size: var(--text-sm); box-shadow: var(--shadow-sm); }
.btn-primary:hover { background: var(--accent-hover); transform: translateY(-1px); box-shadow: var(--shadow-md); }
.btn-primary:active { background: var(--accent-active); transform: translateY(0); }
.btn-secondary { background: var(--bg-2); color: var(--fg); padding: 0 var(--space-5); height: 44px; font-size: var(--text-sm); border: 1.5px solid var(--border-strong); box-shadow: var(--shadow-sm); }
.btn-secondary:hover { background: var(--bg-subtle); border-color: var(--fg-3); }
.btn-ghost { background: transparent; color: var(--fg-2); padding: 0 var(--space-3); height: 36px; font-size: var(--text-sm); }
.btn-ghost:hover { background: var(--bg-subtle); color: var(--fg); }
.btn-icon { width: 40px; height: 40px; padding: 0; border-radius: var(--radius-md); background: transparent; color: var(--fg-2); border: none; cursor: pointer; transition: all var(--duration-base); display: inline-flex; align-items: center; justify-content: center; }
.btn-icon:hover { background: var(--bg-subtle); color: var(--fg); }
```

**Critical rule**: button text on accent backgrounds uses the **literal hex `#fff`**, not `var(--accent-fg)`. Browser variable resolution can fail and render black text. Use `#fff` directly.

### 5.2 Cards

```css
.card { background: var(--bg); border-radius: var(--radius-lg); border: 1px solid var(--border-subtle); box-shadow: var(--shadow-sm); overflow: hidden; }
.card-body { padding: var(--space-5); }
.accent-gradient { background: linear-gradient(135deg, var(--accent-subtle) 0%, var(--bg-subtle) 100%); border: 1.5px solid oklch(0.82 0.04 155); }
```

### 5.3 Status badges

```css
.badge { display: inline-flex; align-items: center; gap: 5px; font-size: var(--text-xs); font-weight: 600; padding: 3px 8px; border-radius: var(--radius-pill); }
.badge-dot { width: 6px; height: 6px; border-radius: 50%; }
.badge-expired { background: oklch(0.93 0.08 25 / 0.12); color: var(--status-expired); }
.badge-expired .badge-dot { background: var(--status-expired); }
.badge-critical { background: oklch(0.93 0.08 45 / 0.12); color: var(--status-critical); }
.badge-critical .badge-dot { background: var(--status-critical); }
.badge-soon { background: oklch(0.93 0.07 85 / 0.12); color: oklch(0.55 0.12 85); }
.badge-soon .badge-dot { background: var(--status-soon); }
.badge-fresh { background: oklch(0.93 0.06 155 / 0.12); color: oklch(0.38 0.10 155); }
.badge-fresh .badge-dot { background: var(--status-fresh); }
```

Usage:
```html
<span class="badge badge-expired"><span class="badge-dot"></span>Expired</span>
```

### 5.4 List item (standard row)

```css
.list-item { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle); cursor: pointer; transition: background var(--duration-fast); }
.list-item:last-child { border-bottom: none; }
.list-item:hover { background: var(--bg-subtle); }
.list-item .li-icon { width: 36px; height: 36px; border-radius: var(--radius-md); background: var(--bg-subtle); display: flex; align-items: center; justify-content: center; color: var(--fg-3); flex-shrink: 0; }
.list-item .li-text { flex: 1; min-width: 0; }
.list-item .li-name { font-weight: 600; font-size: var(--text-sm); color: var(--fg); }
.list-item .li-meta { font-size: var(--text-xs); color: var(--fg-3); margin-top: 1px; }
.list-item .li-date { font-size: var(--text-sm); color: var(--fg-2); white-space: nowrap; }
```

### 5.5 Section title (eyebrow)

```css
.section-title { font-size: var(--text-xs); font-weight: 700; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: var(--space-4); }
```

---

## 6. Layout system

### 6.1 Page widths

```css
main { padding: var(--space-8) var(--space-6) var(--space-24); max-width: 1100px; margin: 0 auto; }
main.has-bottomtab { padding-bottom: calc(60px + var(--space-8)); }
```

The `has-bottomtab` class is added when the page has the mobile bottom tab bar.

### 6.2 Top nav (every screen except login)

```html
<nav class="topnav">
  <a href="02-dashboard.html" class="logo">
    <div class="logo-mark">P</div>
    proviant
  </a>
  <div class="nav-actions hide-mobile" style="margin:0 auto;position:absolute;left:50%;transform:translateX(-50%)">
    <a href="02-dashboard.html" class="nav-link active">Dashboard</a>
    <a href="03-products.html" class="nav-link">Products</a>
    <a href="05-recipes.html" class="nav-link">Recipes</a>
    <a href="06-settings.html" class="nav-link">Settings</a>
  </div>
  <div class="nav-actions" style="display:flex;align-items:center;gap:var(--space-2);margin-left:auto;position:relative">
    <button class="btn-icon" aria-label="Notifications" aria-expanded="false" onclick="toggleNotifCenter()" style="position:relative">
      <i class="bi bi-bell" style="font-size:18px"></i>
    </button>
    <div class="notif-center" id="notifCenter">
      <!-- 3 notif-item rows -->
    </div>
  </div>
</nav>
```

CSS:
```css
nav.topnav { position: sticky; top: 0; z-index: 100; height: 60px; background: oklch(97% 0.006 145deg / .85); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: center; padding: 0 var(--space-5); gap: var(--space-4); }
nav.topnav .logo { font-weight: 700; font-size: var(--text-lg); color: var(--fg); letter-spacing: -0.02em; display: flex; align-items: center; gap: var(--space-2); }
nav.topnav .logo-mark { width: 28px; height: 28px; background: var(--accent); border-radius: var(--radius-sm); display: flex; align-items: center; justify-content: center; font-size: 15px; font-weight: 700; color: #fff; }
nav.topnav .nav-links { display: flex; align-items: center; gap: var(--space-1); }
nav.topnav .nav-link { font-size: var(--text-sm); font-weight: 500; color: var(--fg-2); padding: var(--space-2) var(--space-3); border-radius: var(--radius-sm); transition: all var(--duration-base); }
nav.topnav .nav-link:hover { background: var(--bg-subtle); color: var(--fg); }
nav.topnav .nav-link.active { color: var(--accent); background: var(--accent-muted); font-weight: 600; }
nav.topnav .nav-actions { display: flex; align-items: center; gap: var(--space-1); }
nav.topnav .nav-actions .btn-icon { width: 36px; height: 36px; }
```

### 6.3 Notification center

```css
.notif-center { position: absolute; top: calc(100% + 8px); right: 0; width: 320px; background: var(--bg); border: 1px solid var(--border-subtle); border-radius: var(--radius-lg); box-shadow: var(--shadow-lg); z-index: 200; display: none; }
.notif-center.open { display: block; }
.notif-center-header { display: flex; align-items: center; justify-content: space-between; padding: var(--space-4); border-bottom: 1px solid var(--border-subtle); }
.notif-item { display: flex; gap: var(--space-3); padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle); }
.notif-item:last-child { border-bottom: none; }
.notif-item .ni-icon { width: 32px; height: 32px; border-radius: var(--radius-sm); background: var(--bg-subtle); display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.notif-item .ni-text { font-size: var(--text-sm); color: var(--fg); flex: 1; }
.notif-item .ni-time { font-size: var(--text-xs); color: var(--fg-3); margin-top: 2px; }
.notif-center-footer { padding: var(--space-3) var(--space-4); text-align: center; border-top: 1px solid var(--border-subtle); }
```

Toggle script (paste before `</body>`):
```js
function toggleNotifCenter() {
  var nc = document.getElementById('notifCenter');
  var btn = document.querySelector('[aria-label="Notifications"]');
  var isOpen = nc.classList.toggle('open');
  if (btn) btn.setAttribute('aria-expanded', isOpen);
  if (isOpen) {
    setTimeout(function() {
      document.addEventListener('click', function closeNotif(e) {
        if (!nc.contains(e.target) && !e.target.closest('[aria-label="Notifications"]')) {
          nc.classList.remove('open');
          if (btn) btn.setAttribute('aria-expanded', 'false');
          document.removeEventListener('click', closeNotif);
        }
      });
    }, 0);
  }
}
```

### 6.4 Mobile bottom tab bar (every screen except login)

```html
<nav class="bottomtab">
  <div class="tab-items">
    <a href="02-dashboard.html" class="tab-item active"><i class="bi bi-house-fill"></i>Home</a>
    <a href="03-products.html" class="tab-item"><i class="bi bi-box-seam"></i>Products</a>
    <a href="04-add-product.html" class="tab-item"><i class="bi bi-plus-circle-fill"></i>Add</a>
    <a href="08-shopping-list.html" class="tab-item"><i class="bi bi-cart3"></i>Shopping</a>
    <a href="06-settings.html" class="tab-item"><i class="bi bi-gear"></i>Settings</a>
  </div>
</nav>
```

```css
nav.bottomtab { display: none; position: fixed; bottom: 0; left: 0; right: 0; height: 60px; background: oklch(100% 0 0deg / .92); backdrop-filter: blur(12px); border-top: 1px solid var(--border-subtle); z-index: 100; padding: 0 var(--space-2); }
nav.bottomtab .tab-items { display: flex; height: 100%; }
nav.bottomtab .tab-item { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px; font-size: 10px; font-weight: 600; color: var(--fg-3); text-decoration: none; transition: color var(--duration-fast); }
nav.bottomtab .tab-item i { font-size: 20px; }
nav.bottomtab .tab-item.active { color: var(--accent); }
nav.bottomtab .tab-item:hover { color: var(--fg-2); }
```

Set the active item by adding `class="tab-item active"` to the link for the current page.

### 6.5 Bento grid (signature dashboard pattern)

The 6-column bento grid is the dashboard's signature. Other screens can use it for compact dashboards (e.g. user-profile, settings home).

```css
.bento { display: grid; grid-template-columns: repeat(6, 1fr); grid-auto-rows: minmax(0, auto); gap: var(--space-4); margin-bottom: var(--space-8); }
.bento > .bento-card { background: var(--bg); border-radius: var(--radius-lg); border: 1px solid var(--border-subtle); box-shadow: var(--shadow-sm); overflow: hidden; transition: transform var(--duration-base), box-shadow var(--duration-base); display: flex; flex-direction: column; }
.bento > .bento-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-md); }
.bento > .bento-card.accent { background: linear-gradient(135deg, var(--accent-subtle) 0%, var(--bg-subtle) 100%); border-color: oklch(0.82 0.04 155); }
.bento > .bento-card.dark { background: oklch(20% .02 155deg); color: oklch(97% .006 145deg); border-color: oklch(28% .02 155deg); }
.bento > .bento-card.dark .section-title { color: oklch(70% .02 145deg); }
.bento-span-2 { grid-column: span 2; }
.bento-span-3 { grid-column: span 3; }
.bento-span-4 { grid-column: span 4; }
.bento-span-6 { grid-column: span 6; }
.bento-row-2 { grid-row: span 2; }
.bento-head { display: flex; align-items: center; justify-content: space-between; padding: var(--space-4) var(--space-5); border-bottom: 1px solid var(--border-subtle); }
.bento-head h2 { font-size: var(--text-xs); font-weight: 700; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.08em; margin: 0; }
.bento-body { padding: var(--space-5); flex: 1; display: flex; flex-direction: column; gap: var(--space-3); }
```

Bento card variants (use as needed):
- `.bento-hero-stat` — large dark stat block (3 cols × 2 rows)
- `.bento-stat` — small stat (2 cols, ~120px tall)
- `.bento-chart` — chart container
- `.bento-donut` — donut chart container
- `.bento-list` — list of `.bl-item` rows
- `.bento-cta` — gradient call-to-action card

Responsive collapse:
```css
@media (max-width: 960px) {
  .bento { grid-template-columns: repeat(2, 1fr); }
  .bento-span-2, .bento-span-3, .bento-span-4 { grid-column: span 2; }
}
@media (max-width: 560px) {
  .bento { grid-template-columns: 1fr; }
  .bento-span-2, .bento-span-3, .bento-span-4, .bento-span-6 { grid-column: span 1; }
  .bento-hero-stat .bsh-value { font-size: 2.5rem; }
}
```

### 6.6 Other layout primitives

```css
.metric-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: var(--space-4); margin-bottom: var(--space-8); }
.metric-card { background: var(--bg); border-radius: var(--radius-lg); border: 1px solid var(--border-subtle); padding: var(--space-5); box-shadow: var(--shadow-sm); transition: transform var(--duration-base), box-shadow var(--duration-base); }
.metric-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-md); }
.metric-card .mcv { font-size: var(--text-3xl); font-weight: 700; color: var(--fg); letter-spacing: -0.03em; line-height: 1; }
.metric-card .mcl { font-size: var(--text-sm); color: var(--fg-2); margin-top: var(--space-1); }
.metric-card .mcd { font-size: var(--text-xs); font-weight: 600; margin-top: var(--space-2); display: inline-flex; align-items: center; gap: 3px; }
.mcd.up { color: var(--status-fresh); }
.mcd.down { color: var(--status-expired); }
```

---

## 7. Responsive breakpoints (canonical)

```css
@media (max-width: 768px) {
  nav.topnav { padding: 0 var(--space-4); }
  nav.topnav .hide-mobile { display: none; }
  nav.bottomtab { display: flex; }
  main { padding: var(--space-6) var(--space-4) calc(60px + var(--space-6)); }
  main.has-bottomtab { padding-bottom: calc(60px + var(--space-6)); }
}
@media (max-width: 480px) {
  .metric-row { grid-template-columns: 1fr 1fr; }
}
```

**Conventions**:
- ≤768px = tablet/phone breakpoint (nav links hide, bottom tab appears)
- ≤480px = phone breakpoint (2-col metric grids become 1-col stacks)
- ≥769px = desktop (nav links visible, bottom tab hidden)

---

## 8. Page header pattern

Every authenticated page (everything except login) opens `<main class="has-bottomtab">` with this centered header:

```html
<div style="text-align:center;margin-bottom:var(--space-8)">
  <h1 style="font-size:var(--text-2xl);font-weight:700;letter-spacing:-0.02em;color:var(--fg)">Page title</h1>
  <p style="color:var(--fg-2);font-size:var(--text-sm);margin-top:var(--space-1)">One-line subhead</p>
</div>
```

This is the canonical pattern: centered, single column, large title + small subhead. Match the dashboard's "Good morning, Maya" rhythm.

---

## 9. Chart.js integration

Dashboard reads CSS variable values for chart colors. Use this pattern:

```js
document.addEventListener('DOMContentLoaded', function() {
  var css = getComputedStyle(document.documentElement);
  var accent = css.getPropertyValue('--accent').trim();
  var fresh = css.getPropertyValue('--status-fresh').trim();
  var expired = css.getPropertyValue('--status-expired').trim();
  var critical = css.getPropertyValue('--status-critical').trim();
  var soon = css.getPropertyValue('--status-soon').trim();
  var fg3 = css.getPropertyValue('--fg-3').trim();
  var border = css.getPropertyValue('--border').trim();
  var fg = css.getPropertyValue('--fg').trim();

  Chart.defaults.color = fg3;
  Chart.defaults.borderColor = border;
  Chart.defaults.font.family = "'Vend Sans', sans-serif";
  Chart.defaults.font.size = 11;

  // new Chart(...) instances here
});
```

This way the chart always reflects the current theme without hardcoded hex.

---

## 10. Hard rules (do not break)

1. **No emoji as feature icons.** Use `bi bi-*` from bootstrap-icons. The only allowed emoji are user-content (e.g. "use it up" recipe card ingredient chips where emojis are user data, not UI chrome).
2. **No invented metrics.** Real household numbers (24 items, $48 saved, 12% more) are fine; "10× faster!" or "99.9% uptime" are not.
3. **No filler copy.** No "Feature One" or "Lorem ipsum". Real product names: Atlantic Salmon, Greek Yogurt, Sharp Cheddar, Sourdough Bread, Baby Spinach, Whole Milk, Free-range Eggs.
4. **No decorative accent.** Use `--accent` for play/active/CTA only. Status colors for product status. Never paint large background panels purple/violet.
5. **No light backgrounds on primary surfaces.** Dark forest is the identity; keep it.
6. **No raw `var(--accent-fg)` for text on accent.** Use `color: #fff` literal.
7. **No duplicate `:root` blocks** in a file. One canonical token block at the top of `<style>`.
8. **No breaking the navbar pattern.** All authenticated pages use the same `<nav class="topnav">` block with the same classes. Don't invent a new one.
9. **All buttons white text on accent.** `color: #fff` literal, not `var(--accent-fg)`.
10. **Vend Sans only** for UI/display. Mono = `DM Mono`. Never Inter, Roboto, Arial, or system default as the display face.

---

## 11. Anti-patterns to avoid

- ❌ Aggressive purple/violet gradient backgrounds
- ❌ Generic emoji feature icons (✨ 🚀 🎯)
- ❌ Rounded card with a left coloured border accent
- ❌ Hand-drawn SVG humans / faces / scenery
- ❌ Inter / Roboto / Arial as a *display* face
- ❌ Invented metrics ("10× faster", "99.9% uptime")
- ❌ Filler copy — "Feature One / Feature Two", lorem ipsum
- ❌ An icon next to every heading
- ❌ Warm beige / cream / peach / pink / orange-brown page backgrounds
- ❌ Designer chrome in product UI (no platform toggles, no demo controls)

---

## 12. File inventory

| File | Source | Use case |
|---|---|---|
| `index.html` | (root) | Gallery / launcher linking to all 14 screens |
| `screens/01-login.html` | `login.tmpl` | Auth — no nav, no bottom tab |
| `screens/02-dashboard.html` | `home.tmpl` | **Canonical reference** — bento grid + charts |
| `screens/03-products.html` | `productsView.tmpl` | Product list (default) + grid view toggle |
| `screens/04-add-product.html` | `addProduct.tmpl` | Barcode scan + manual form |
| `screens/05-recipes.html` | `recipes.tmpl` | Recipe cards, filter by ingredient |
| `screens/06-settings.html` | `userSettings.tmpl` | All settings sections + modals |
| `screens/07-edit-product.html` | edit variant | Edit existing product |
| `screens/08-shopping-list.html` | `shoppingList.tmpl` | Low-stock items, stepper |
| `screens/09-onboarding.html` | `onboarding.tmpl` | 3-step wizard |
| `screens/10-accept-invite.html` | `acceptInvite.tmpl` | Household invite accept |
| `screens/11-unsubscribe.html` | `unsubscribe.tmpl` | Email unsubscribe success |
| `screens/12-verify-email.html` | `verifyEmail.tmpl` | Email verification state |
| `screens/13-error.html` | `error.tmpl` | Standalone error page |
| `screens/14-user-profile.html` | `user.tmpl` | User profile + household members |

---

## 13. Update workflow for an agent

When updating an existing screen to this design system:

1. **Start with the canonical head block** (section 2). Replace the existing `<head>` content.
2. **Paste the full `:root` block** (section 3) verbatim. One block only.
3. **Add the global resets** (section 4).
4. **Add the component CSS** (section 5) — pick what the page needs.
5. **Add the navbar** (section 6.2) if the page is authenticated.
6. **Add the bottom tab bar** (section 6.4) if the page is authenticated.
7. **Use the page header pattern** (section 8) for the page title.
8. **Replace any existing icons** with `bi bi-*` from bootstrap-icons.
9. **Strip duplicate CSS** — many legacy files have duplicate `:root` blocks, duplicate `nav.topnav` rules, etc.
10. **Hard refresh** to bust cache.

When generating a new screen from scratch: follow steps 1–8 in order, then compose the page using the bento grid (section 6.5) for dashboard-style content or the list-item pattern (section 5.4) for tabular content.

---

## 14. Checklist before shipping

- [ ] Vend Sans loaded from `@fontsource-variable`
- [ ] `:root` block present, no duplicates
- [ ] `prefers-reduced-motion` block present
- [ ] `:focus-visible` outline defined
- [ ] No emoji as icons (use `bi bi-*`)
- [ ] No invented metrics
- [ ] Real product names + household context
- [ ] All buttons on accent backgrounds use `color: #fff`
- [ ] Navbar matches canonical pattern (section 6.2)
- [ ] Bottom tab bar on every authenticated screen
- [ ] Bootstrap-icons CDN included
- [ ] Charts (if any) read colors from CSS variables
- [ ] Mobile responsive at 768px and 480px breakpoints

# Proviant UI Kit

## Overview
Full interactive prototype of the Proviant web app. Built with React + Babel, all in the browser — no build step.

## Design System
**v2 design system:** `../../proviant-design-system.html`  
**Migration audit:** `../../DESIGN_SYSTEM_AUDIT.md`  
**Token source:** `../../colors_and_type.css`

All CSS custom properties (colors, type, spacing, shadow, radius) are defined in `colors_and_type.css` and loaded by `index.html`. Do not hardcode values in component files — always reference tokens.

---

## Screens

| Screen | File | Status |
|--------|------|--------|
| Login / Signup | `Login.jsx` | Auth tabs, toast notifications, loading state |
| Dashboard | `Dashboard.jsx` | Metric tiles, donut chart, category bars, sparkline |
| Products | `Products.jsx` | Card grid, search, status/category filter, bulk select, archive/delete with modals |
| Create Product | `CreateProduct.jsx` | Form with live status preview, simulated barcode scan → Open Food Facts lookup |
| Archive | `Archive.jsx` | Archived products, restore/delete |
| Settings | `Settings.jsx` | Profile, notification providers (Email/Ntfy/Telegram), household |

---

## Component Files

| File | Purpose |
|------|---------|
| `shared.jsx` | `StatusBadge`, `Btn`, `Input`, `Select`, `Modal`, sample data |
| `Login.jsx` | Auth screen |
| `Nav.jsx` | Sidebar navigation |
| `Dashboard.jsx` | Overview / home screen |
| `Products.jsx` | Product list + `ProductCard` |
| `CreateProduct.jsx` | Add product form |
| `Archive.jsx` | Archive screen |
| `Settings.jsx` | Settings + `NotifCard` |
| `App.jsx` | Root, routing, state, localStorage |

---

## Shared Components (`shared.jsx`)

### `<StatusBadge status="expired|critical|soon|fresh|nodate" small? />`
Pill badge with status dot and label. Uses `STATUS` map for colours (all sourced from `colors_and_type.css` tokens).

**DS v2 change pending:** Icon (`bi-x-circle-fill` etc.) → plain 6px dot. See `DESIGN_SYSTEM_AUDIT.md §2`.

### `<Btn variant="primary|secondary|ghost|danger" size="sm|md|lg" icon="bi-*" onClick disabled loading />`
Consistent button. Maps to DS v2 button variants.

**DS v2 changes pending:** explicit height (28/36/44px), primary box-shadow, hover `translateY(-1px)`. See `DESIGN_SYSTEM_AUDIT.md §3`.

### `<Input label required placeholder value onChange icon error helper type />`
Labelled input with focus ring, icon support, and error/helper text.

**DS v2 change pending:** label uppercase + tracking, height 38px. See `DESIGN_SYSTEM_AUDIT.md §4`.

### `<Select label value onChange options />`
Styled native `<select>`.

### `<Modal title icon iconColor onClose footer />`
Overlay modal with icon header, body slot, and optional footer actions.

**DS v2 changes pending:** border-radius → 16px, width → 460px, footer bg + border-top. See `DESIGN_SYSTEM_AUDIT.md §6`.

---

## Design Tokens (from `colors_and_type.css`)

### Colors
```
--bg              parchment white page background
--bg-subtle       slightly darker surface (sidebar, inputs)
--surface         card / panel surface (white)
--fg              primary text
--fg-2            secondary text
--fg-3            tertiary / placeholder
--border          default border
--border-subtle   dividers
--accent          primary brand action (green)
--accent-subtle   tinted green background
```

### Status Colors
```
--status-expired        + -subtle, -border, -fg, -hover
--status-critical       + -subtle, -border, -fg, -hover
--status-soon           + -subtle, -border, -fg, -hover
--status-fresh          + -subtle, -border, -fg, -hover
--status-nodate         + -subtle, -border, -fg, -hover
```

### Typography
```
--font-ui         'VendSans', system-ui, sans-serif   ← UI font
--font-mono       'DM Mono', 'Courier New', monospace  ← barcodes, IDs, dates
--text-xs         12px
--text-sm         13px
--text-base       14px
--text-md         16px
--text-lg         18px
--text-xl         20px
--text-2xl        24px
```

### Spacing (4px base)
```
--space-1  4px   --space-4  16px   --space-8  32px
--space-2  8px   --space-5  20px   --space-10 40px
--space-3  12px  --space-6  24px   --space-12 48px
```

### Radii
```
--radius-sm    4px   buttons sm, qty steppers
--radius-md    8px   buttons, inputs, cards (current)
--radius-lg    12px  modals, panels (DS v2 default for cards)
--radius-pill  999px badges, chips
```
DS v2 adds: `--radius-xl: 16px` (modals), `--radius-2xl: 24px` (hero cards) — add to `colors_and_type.css`.

### Shadows
```
--shadow-xs   barely-there (stat cards)
--shadow-sm   default card
--shadow-md   hover state
--shadow-lg   dropdowns
--shadow-xl   modals
```

---

## State
All state persists to `localStorage` so the prototype survives page refreshes.

---

## Icons
Bootstrap Icons CDN — `bi-*` classes.  
CDN: `https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.3/font/bootstrap-icons.css`

---

## Running
Open `index.html` in a browser. No server needed for local use.  
For cross-origin font loading (VendSans), serve from localhost:
```
npx serve .
# or
python3 -m http.server 8080
```

---

## Pending DS v2 Migrations

See `../../DESIGN_SYSTEM_AUDIT.md` for full details and code snippets.

| Priority | Item | File |
|----------|------|------|
| 🔴 High | StatusBadge: icon → dot | `shared.jsx` |
| 🔴 High | Filter chips replace `<select>` dropdowns | `Products.jsx` |
| 🔴 High | Badge classes in Go templates | Go template SCSS |
| 🟠 Medium | Btn: explicit height + hover shadow | `shared.jsx` |
| 🟠 Medium | Input: label uppercase + height 38px | `shared.jsx` |
| 🟠 Medium | Modal: radius 16px, width 460px, footer border | `shared.jsx` |
| 🟠 Medium | Nav: active state → white bg + shadow | `Nav.jsx` |
| 🟠 Medium | Warning banner when expired > 0 | `Dashboard.jsx` |
| 🟡 Low | Card border-radius → `var(--radius-lg)` | `Products.jsx` |
| 🟡 Low | Page title font token | `Dashboard.jsx`, `Products.jsx` |
| 🟡 Low | Add `--radius-xl`, `--radius-2xl` tokens | `colors_and_type.css` |

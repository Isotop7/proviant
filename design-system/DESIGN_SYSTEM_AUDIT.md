# Proviant — Design System Audit & Migration Plan
> Generated: 2026-05-01  
> Source of truth: `proviant-design-system.html` (Design System v2)  
> Current implementation: `ui_kits/proviant/`, `colors_and_type.css`, Go template views  

---

## Purpose

Feed this document to an AI agent to:
1. Diff the current implementation against the Design System v2 spec
2. Identify specific files/lines that need changing
3. Generate a prioritised migration plan with concrete code changes

---

## 1. Token Layer — `colors_and_type.css`

### 1.1 Font family

| Token | Current value | DS v2 spec | Action |
|-------|--------------|------------|--------|
| `--font-ui` | `'Vend Sans', system-ui, sans-serif` | `'VendSans', system-ui, sans-serif` | Verify font-family name matches the actual `@font-face` family name in the project. If the font loads as `VendSans` (no space), update the token. Check in browser devtools: `getComputedStyle(document.body).fontFamily`. |
| `--font-mono` | `'DM Mono', 'Courier New', monospace` | `'DM Mono', 'Courier New', monospace` | ✅ Match — no change needed |

### 1.2 Color tokens — naming & values

The current `colors_and_type.css` uses `oklch()` values. DS v2 introduces `--pv-` prefixed hex tokens alongside. **Do not replace oklch with hex** — oklch is better. The action here is to reconcile naming and add missing tokens.

| Issue | Current | DS v2 | Action |
|-------|---------|-------|--------|
| Missing `--pv-` prefix | All tokens unprefixed | DS v2 uses `--pv-green-*`, `--pv-neutral-*` | Add `--pv-` aliases for all tokens (keep originals for BC). See §7 for the full alias block to add. |
| Background uses `--bg` | `oklch(0.98 0.012 75)` | `--pv-neutral-50: #f7f6f3` | These are functionally equivalent (warm off-white). Alias: `--pv-neutral-50: var(--bg)`. No visual change needed. |
| No `--neutral-150` border step | `--border-subtle: oklch(0.93 0.006 75)` | `--pv-neutral-150` used for card borders, table dividers | Add: `--pv-neutral-150: oklch(0.91 0.008 75)` |
| `--accent` is teal-green (hue 162) | `oklch(0.50 0.12 162)` | DS v2 uses hue ~145 (greener) | **Review with designer.** Current accent reads as teal; DS v2 spec is more pure green. This is a brand decision. If changing: update `--accent` to `oklch(0.50 0.12 145)` and audit all accent usages. |
| Missing `--pv-radius-xl` and `--pv-radius-2xl` | `--radius-lg: 12px` is the largest | DS v2 adds `--radius-xl: 16px`, `--radius-2xl: 24px` | Add: `--radius-xl: 16px; --radius-2xl: 24px;` to `:root`. Used by modals and hero cards. |

### 1.3 Missing tokens to add

Add the following to `colors_and_type.css` `:root`:

```css
/* DS v2 additions */
--radius-xl:   16px;   /* modals, bottom sheets */
--radius-2xl:  24px;   /* hero cards, quick-action banners */
--radius-full: 9999px; /* badges, chips — alias for --radius-pill */

/* Neutral step for table borders */
--border-medium: oklch(0.91 0.008 75);  /* between --border-subtle and --border */

/* Shadow additions */
--shadow-xs: 0 1px 2px oklch(0.18 0.02 70 / 0.05);  /* already exists — verify */
```

---

## 2. Component: `StatusBadge` — `shared.jsx`

### Current implementation
```jsx
// shared.jsx line 1-10
const STATUS = {
  expired:  { bg: 'oklch(0.95 0.05 25)', fg: 'oklch(0.45 0.20 25)', border: 'oklch(0.80 0.12 25)', ... },
  critical: { bg: 'oklch(0.96 0.05 45)', fg: 'oklch(0.50 0.18 45)', ... },
  ...
};
// StatusBadge renders as: inline-flex pill with icon + text, fontSize 11px, padding 3px 10px
```

### DS v2 spec
- Background: light tinted (✅ already correct — oklch subtle values match)
- Border: 1px solid tinted border (✅ already present)
- **Issue 1:** Uses `bi-x-circle-fill`, `bi-exclamation-circle-fill` etc. as leading icons → DS v2 uses a plain 6px dot instead of an icon. Simpler, less visual noise.
- **Issue 2:** `fontSize: 11px` → DS v2 spec is `var(--text-xs)` which is `12px` (current) or `11px` (DS v2 doc). **No change needed** — values align.
- **Issue 3:** No `soon` status dot colour in the chip system — add `--status-soon-dot` alias.

### Migration steps
```jsx
// Replace icon with dot in StatusBadge:
// BEFORE:
<i className={`bi ${s.icon}`} style={{ fontSize: small ? 10 : 11 }} />

// AFTER:
<span style={{
  width: 6, height: 6, borderRadius: '50%',
  background: s.dotColor,   // add dotColor to STATUS object
  flexShrink: 0, display: 'inline-block'
}} />
```

Add `dotColor` to each STATUS entry in `shared.jsx`:
```js
expired:  { ..., dotColor: 'var(--status-expired)' },
critical: { ..., dotColor: 'var(--status-critical)' },
soon:     { ..., dotColor: 'var(--status-soon)' },
fresh:    { ..., dotColor: 'var(--status-fresh)' },
nodate:   { ..., dotColor: 'var(--status-nodate)' },
```

---

## 3. Component: `Btn` — `shared.jsx`

### Current implementation
```jsx
// shared.jsx ~line 40-70
// - border-radius: 8px (hardcoded)
// - fontFamily: 'var(--font-ui)'
// - no box-shadow on primary
// - fontSize: sm=12, default=13, lg=15
// - padding: sm='5px 12px', default='8px 16px', lg='11px 22px'
// - height: implicit (no explicit height)
```

### DS v2 spec
| Property | Current | DS v2 | Action |
|----------|---------|-------|--------|
| Height | implicit (~30px sm, ~36px default, ~42px lg) | explicit: 28px sm, 36px default, 44px lg | Set explicit `height` + `lineHeight: 1` on button |
| Primary box-shadow | none | `0 1px 0 rgba(0,0,0,0.12), inset 0 1px 0 rgba(255,255,255,0.12)` | Add to primary variant |
| Primary hover | no transform | `translateY(-1px)` + green glow | Add `:hover` equivalent via `onMouseEnter`/`onMouseLeave` state |
| border-radius | `8px` hardcoded | `var(--radius-md)` | Change to `var(--radius-md)` for consistency |
| lg border-radius | n/a | `var(--radius-lg)` | Add lg-specific radius |
| `ghost` border | `transparent` border (no border prop) | `border: '1px solid transparent'` explicit | Explicitly set so width doesn't shift on hover |

### Migration
```jsx
// In Btn, update base style:
const base = {
  ...
  borderRadius: size === 'lg' ? 'var(--radius-lg)' : 'var(--radius-md)',
  height: size === 'sm' ? 28 : size === 'lg' ? 44 : 36,
  lineHeight: 1,
  border: '1px solid transparent',
};

// Update primary variant:
primary: {
  background: 'var(--accent)',
  color: 'white',
  borderColor: 'var(--accent-hover)',
  boxShadow: '0 1px 0 rgba(0,0,0,0.12), inset 0 1px 0 rgba(255,255,255,0.12)',
},
```

---

## 4. Component: `Input` — `shared.jsx`

### Current vs DS v2
| Property | Current | DS v2 | Action |
|----------|---------|-------|--------|
| Height | implicit (~34px) | 38px default, 44px for `input-lg` | Add `height: 38` to input style |
| Label style | `fontSize:12, fontWeight:500` | `fontSize: var(--text-xs)`, `fontWeight: 600`, `letterSpacing: 0.3px`, `textTransform: 'uppercase'` | Update label style |
| Focus ring | `0 0 0 3px oklch(0.52 0.10 145 / 0.15)` | `0 0 0 3px rgba(30,158,86,0.12)` | Functionally identical — keep oklch version ✅ |
| Helper text | `fontSize:12` | `var(--text-xs)` | Update to token |

### Migration
```jsx
// Label update in Input component:
<label style={{
  display: 'block',
  fontSize: 'var(--text-xs)',
  fontWeight: 600,
  color: 'var(--fg-2)',
  marginBottom: 6,
  letterSpacing: '0.3px',
  textTransform: 'uppercase',
}}>

// Input height:
<input style={{
  ...
  height: 38,
  padding: icon ? '0 12px 0 34px' : '0 12px',
}} />
```

---

## 5. Component: `Nav` — `Nav.jsx`

### Current implementation
```jsx
// Nav.jsx
// Active state: background 'var(--accent-subtle)', color 'var(--accent)'
// No background container wrapping nav items
// Uses left-border accent? No — uses background tint only
// User section: avatar + name + email + logout button
```

### DS v2 spec
| Property | Current | DS v2 | Action |
|----------|---------|-------|--------|
| Nav item container | no wrapper bg | `background: var(--bg-subtle)` container with `border-radius: var(--radius-lg)`, padding `var(--space-2)` | Wrap items in a container div |
| Active item style | `accent-subtle` bg, `accent` color | white bg + `shadow-xs` | Change active: `background: 'white', boxShadow: 'var(--shadow-xs)'`, color `var(--fg)` |
| Active item font | `fontWeight: 600` | `fontWeight: 600` ✅ | No change |
| Nav badge (Products count) | not present | red pill badge `margin-left: auto` | Add badge for `products` nav item showing expired+critical count |
| Wordmark | `<img src="../../assets/header.png">` | `P` logomark + "proviant" text | Keep image — it's the real brand asset. No change needed. |

### Migration
```jsx
// In Nav.jsx, wrap items section:
<div style={{
  background: 'var(--bg-subtle)',
  borderRadius: 'var(--radius-lg)',
  padding: 'var(--space-2)',
  display: 'flex', flexDirection: 'column', gap: 2,
}}>
  {items.map(item => {
    const active = page === item.id;
    return (
      <button key={item.id} style={{
        ...
        background: active ? 'white' : 'transparent',
        boxShadow: active ? 'var(--shadow-xs)' : 'none',
        color: active ? 'var(--fg)' : 'var(--fg-2)',
      }}>
        ...
        {/* Badge example for products: */}
        {item.id === 'products' && expiredCount > 0 && (
          <span style={{
            marginLeft: 'auto', background: 'var(--status-expired)',
            color: 'white', fontSize: 10, fontWeight: 700,
            padding: '1px 5px', borderRadius: 999, minWidth: 18, textAlign: 'center',
          }}>{expiredCount}</span>
        )}
      </button>
    );
  })}
</div>
```

---

## 6. Component: `Modal` — `shared.jsx`

### Current implementation
```jsx
// Modal: borderRadius 12px, width 400px, no footer separator
// Icon: 40×40 box, border-radius 10
// Close button: fontSize 18, no hover bg
// Footer: padding '0 20px 20px', flex row, justify flex-end
```

### DS v2 spec
| Property | Current | DS v2 | Action |
|----------|---------|-------|--------|
| Border radius | `12px` | `var(--radius-xl)` = 16px | Update to `borderRadius: 'var(--radius-xl)'` |
| Width | `400px` | `460px` max-width | Update `width: 460` |
| Footer bg | white (inherits) | `var(--bg-subtle)` + `border-top: 1px solid var(--border-subtle)` | Add footer separator |
| Tab switcher | not in modal | underline tabs for Manual/Camera mode | In `CreateProduct.jsx` modal/flow, replace segment button with proper tab underline style |
| Backdrop blur | `backdrop-filter: blur(2px)` ✅ | same | No change |
| Modal body padding | `14px 20px` | `var(--space-6)` = 24px | Increase to `24px 20px` |

### Migration
```jsx
// Modal footer:
{footer && (
  <div style={{
    padding: '12px 20px',
    display: 'flex', gap: 8, justifyContent: 'flex-end',
    borderTop: '1px solid var(--border-subtle)',
    background: 'var(--bg-subtle)',
  }}>
    {footer}
  </div>
)}
```

---

## 7. Component: `ProductCard` — `Products.jsx`

### Current implementation
```jsx
// Card grid view (not table view)
// Status bar: 3px top border in status colour
// Image area: 72px hatched placeholder
// Checkbox: absolute top-right
// Body: name (13px 600), category (11px), StatusBadge, expires + barcode in mono
```

### DS v2 spec
The DS v2 spec focuses on the **table/list view**, not the card grid. The card grid is a valid additional view mode.

| Property | Current | DS v2 | Action |
|----------|---------|-------|--------|
| Card border-radius | `8px` | `var(--radius-lg)` = 12px | Update to `borderRadius: 'var(--radius-lg)'` |
| Status bar | 3px top coloured stripe | DS v2 doesn't use stripe — uses badge only | Optional: remove stripe, add left-side status dot to product name instead |
| Hover shadow | `var(--shadow-md)` ✅ | same | No change |
| Hover transform | `translateY(-1px)` ✅ | same | No change |
| Image placeholder | hatched SVG bg | striped with mono label ✅ | No change — matches |
| Product name font | `13px 600` | `var(--text-sm) 500` | Change weight from 600→500, use token |

---

## 8. Component: `Dashboard` — `Dashboard.jsx`

### Current vs DS v2

| Issue | Current | DS v2 | Action |
|-------|---------|-------|--------|
| Metric tile bg | `white` | `var(--surface)` | Update to token (same visual, better semantics) |
| Metric tile border-radius | `8px` hardcoded | `var(--radius-lg)` | Update |
| Metric tile padding | `16px 18px` | `var(--space-5) var(--space-6)` | Update to tokens |
| "Expired" tile | shown as metric card, coloured icon | DS v2: solid red card when expired > 20% | Conditionally apply danger-card style: `background: 'var(--status-expired)', color: 'white'` on the expired tile when `expiredPct > 0.2` |
| Warning banner | not present | inline warning banner when expired > 0 | Add below stats: see §8a below |
| Last added | separate row below tiles | part of tile grid as 4th tile | Move to tile grid |
| Sparkline | 12-month fake data | Expiry Trend (same concept) ✅ | No change needed |
| Category breakdown | horizontal bar chart ✅ | same | No change |

### §8a Warning banner (add to `Dashboard.jsx`)
```jsx
{expired > 0 && (
  <div style={{
    display: 'flex', alignItems: 'center', gap: 12,
    padding: '10px 16px',
    background: 'var(--status-expired-subtle)',
    border: '1px solid var(--status-expired-border)',
    borderRadius: 'var(--radius-md)',
    marginBottom: 28,
    fontSize: 13, color: 'var(--status-expired)',
  }}>
    <i className="bi bi-exclamation-triangle-fill" style={{ fontSize: 16, flexShrink: 0 }} />
    <span style={{ flex: 1 }}>
      {expired} product{expired > 1 ? 's have' : ' has'} expired and {expired > 1 ? 'are' : 'is'} not archived.
    </span>
    <button onClick={() => { setFilterStatus('expired'); setPage('products'); }}
      style={{ background: 'none', border: 'none', cursor: 'pointer', fontWeight: 600,
               color: 'var(--status-expired)', textDecoration: 'underline', fontSize: 13 }}>
      View expired →
    </button>
  </div>
)}
```

---

## 9. Component: Filter Chips — `Products.jsx`

### Current implementation
```jsx
// Products.jsx uses <select> dropdowns for status + category filter
// No chip/pill filter UI in the prototype (unlike the real app screenshots)
```

### DS v2 spec
DS v2 recommends pill-style filter chips replacing the `<select>` dropdowns for the status filter row.

| Action | Details |
|--------|---------|
| Replace status `<select>` | Use horizontal scrollable chip row |
| Chip base style | `border: '1px solid var(--border)', borderRadius: 999, padding: '4px 12px', fontSize: 12, fontWeight: 500, cursor: 'pointer'` |
| Active chip (All) | `background: 'var(--accent)', borderColor: 'var(--accent)', color: 'white'` |
| Active chip (Expired) | `background: 'var(--status-expired-subtle)', borderColor: 'var(--status-expired-border)', color: 'var(--status-expired)'` |
| Active chip (Critical) | `background: 'var(--status-critical-subtle)', borderColor: 'var(--status-critical-border)', color: 'var(--status-critical)'` |
| Active chip (Soon) | `background: 'var(--status-soon-subtle)', borderColor: 'var(--status-soon-border)', color: 'var(--status-soon)'` |
| Active chip (Fresh) | `background: 'var(--status-fresh-subtle)', borderColor: 'var(--status-fresh-border)', color: 'var(--status-fresh)'` |
| Keep category `<select>` | Location/category stays as a dropdown — too many options for chips |

---

## 10. Typography — global

| Location | Current | DS v2 | Action |
|----------|---------|-------|--------|
| `index.html` body | `font-size: 14px` | `var(--text-base)` = `0.875rem` (14px) ✅ | No change |
| Page titles (`h1`) | `fontSize: 22, fontWeight: 700, letterSpacing: '-0.02em'` | `var(--text-xl)` = 20px, weight 700, tracking `-0.4px` | Minor: update to `fontSize: 'var(--text-xl)'` |
| Card labels | `fontSize: 12, fontWeight: 500` | `var(--text-xs)`, weight 600, uppercase, `letterSpacing: 0.5px` | Update weight + letter-spacing for all section labels |
| Barcode/ID text | `fontFamily: 'var(--font-mono)', fontSize: 10–11` | same ✅ | No change |
| Page sub-copy | `fontSize: 13, color: 'var(--fg-2)'` | `var(--text-sm)`, `var(--fg-2)` ✅ | No change |

---

## 11. Go Template Views (real app)

These are separate from the prototype. The agent should check the Go template HTML for the following:

### 11.1 Bootstrap SCSS variable overrides to add/verify
```scss
// In your main _variables.scss or similar:
$primary:                 #1e9e56;           // or oklch(0.50 0.12 145) if using postcss-oklch
$body-bg:                 #f7f6f3;
$body-color:              #363229;
$font-family-sans-serif:  'VendSans', system-ui, sans-serif;
$font-family-monospace:   'DM Mono', 'Courier New', monospace;
$border-radius:           8px;
$border-radius-sm:        4px;
$border-radius-lg:        12px;
$border-radius-xl:        16px;             // Bootstrap 5.3+
$btn-border-radius:       8px;
$btn-border-radius-sm:    4px;
$btn-border-radius-lg:    12px;
$input-border-radius:     8px;
$card-border-radius:      12px;
$modal-content-border-radius: 16px;
```

### 11.2 Badge classes — replace Bootstrap `.badge.bg-danger` etc.
Current: `<span class="badge bg-danger">Expired</span>` (solid red fill)  
DS v2: custom badge classes with tinted background

```html
<!-- BEFORE -->
<span class="badge bg-danger rounded-pill">
  <i class="bi bi-x-circle-fill me-1"></i>Expired
</span>

<!-- AFTER -->
<span class="badge-pv badge-pv--expired">Expired</span>
```

Add to your custom SCSS:
```scss
.badge-pv {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.6875rem;
  font-weight: 600;
  padding: 3px 9px;
  border-radius: 9999px;
  border: 1px solid;
  white-space: nowrap;

  &::before {
    content: '';
    width: 6px; height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  &--expired  { background: var(--status-expired-subtle);  color: var(--status-expired);  border-color: var(--status-expired-border);  &::before { background: var(--status-expired); } }
  &--critical { background: var(--status-critical-subtle); color: var(--status-critical); border-color: var(--status-critical-border); &::before { background: var(--status-critical); } }
  &--soon     { background: var(--status-soon-subtle);     color: var(--status-soon);     border-color: var(--status-soon-border);     &::before { background: var(--status-soon); } }
  &--fresh    { background: var(--status-fresh-subtle);    color: var(--status-fresh);    border-color: var(--status-fresh-border);    &::before { background: var(--status-fresh); } }
  &--nodate   { background: var(--status-nodate-subtle);   color: var(--status-nodate);   border-color: var(--status-nodate-border);   &::before { background: var(--status-nodate); } }
}
```

### 11.3 Filter chip classes
```scss
.filter-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.75rem;
  font-weight: 500;
  padding: 4px 12px;
  border-radius: 9999px;
  border: 1px solid var(--bs-border-color);
  background: white;
  color: var(--bs-secondary-color);
  cursor: pointer;
  transition: border-color 120ms ease, background 120ms ease;
  white-space: nowrap;

  &:hover { border-color: var(--bs-border-color-translucent); background: var(--bs-gray-100); }
  &.active           { background: var(--bs-primary); border-color: var(--bs-primary); color: white; }
  &.active-expired   { background: var(--status-expired-subtle);  color: var(--status-expired);  border-color: var(--status-expired-border); }
  &.active-critical  { background: var(--status-critical-subtle); color: var(--status-critical); border-color: var(--status-critical-border); }
  &.active-soon      { background: var(--status-soon-subtle);     color: var(--status-soon);     border-color: var(--status-soon-border); }
  &.active-fresh     { background: var(--status-fresh-subtle);    color: var(--status-fresh);    border-color: var(--status-fresh-border); }
}
```

---

## 12. Screens not yet in prototype (create these)

| Screen | Priority | Notes |
|--------|----------|-------|
| Empty state — Products list | High | When pantry is empty or filter returns 0 results. Already exists in `Products.jsx` ✅ |
| Empty state — Recipes | Medium | No recipes found; prompt to add more products |
| Login screen redesign | Medium | Split Sign In / Sign Up into tabs; add brand atmosphere |
| Mobile product row | High | Desktop prototype uses card grid; mobile prototype uses table rows with expiry delta (−19 days). Verify table row view exists in Go templates. |
| Warning/alert banner | Medium | `Dashboard.jsx` §8a above |

---

## 13. Migration Priority Order

### Phase 1 — Token & global (no visual regression, safe)
1. Add `--radius-xl`, `--radius-2xl`, `--radius-full`, `--border-medium` to `colors_and_type.css`
2. Add Bootstrap SCSS variable overrides (§11.1)
3. Verify `VendSans` vs `Vend Sans` font-family name — fix if mismatched
4. Add `--pv-` alias block (optional, for future tooling compatibility)

### Phase 2 — Badge system (high visual impact, low risk)
5. Add `.badge-pv` SCSS class (§11.2) to Go template SCSS
6. Replace all Bootstrap `.badge.bg-danger/warning/success` in Go templates with `.badge-pv--*`
7. Update `StatusBadge` in `shared.jsx` to use dot instead of icon (§2)

### Phase 3 — Component polish (medium effort)
8. Update `Btn` height, shadow, hover states (§3)
9. Update `Input` label style + height (§4)
10. Update `Modal` border-radius, width, footer separator (§6)
11. Update `Nav` active state + item container (§5)
12. Add filter chips to `Products.jsx` toolbar (§9)

### Phase 4 — Dashboard & warnings (new features)
13. Add warning banner to `Dashboard.jsx` (§8a)
14. Add expired-count nav badge to `Nav.jsx` (§5)
15. Conditionally style expired metric tile red (§8)

### Phase 5 — Layout & type (lowest risk, incremental)
16. Update border-radius on all cards to `var(--radius-lg)` (§7)
17. Update page title + label typography to match scale (§10)
18. Add `--pv-shadow-*` aliases; verify shadow usage across all components

---

## 14. Files to check (agent checklist)

```
colors_and_type.css          — token additions (§1)
ui_kits/proviant/shared.jsx  — StatusBadge dot, Btn height/shadow, Input label, Modal footer (§2–4, §6)
ui_kits/proviant/Nav.jsx     — active state, container, badge (§5)
ui_kits/proviant/Dashboard.jsx — tile styles, warning banner (§8)
ui_kits/proviant/Products.jsx  — filter chips, card radius (§7, §9)
[Go templates]/_variables.scss — Bootstrap overrides (§11.1)
[Go templates]/[product views] — badge class replacement (§11.2)
[Go templates]/[product views] — filter chip class addition (§11.3)
```

---

## 15. Design system source

Full component specs, token values, and before/after comparisons:  
→ `proviant-design-system.html` (open in browser)

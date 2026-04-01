# Proviant — Design System MASTER

> Source of truth for all UI decisions. Page-specific overrides live in `pages/`.

---

## 1. Product Identity

**Type:** Inventory & Stock Management + Smart Home Dashboard hybrid  
**Audience:** Home users managing household food inventory — practical, low friction, glanceable  
**Tone:** Clean, organized, reliable, slightly fresh (food/health context)  
**Mode:** Dark-first (current `data-bs-theme="dark"` stays)

---

## 2. Style

**Primary style:** Flat Design + Minimalism  
**Secondary:** Subtle micro-interactions (tactile feedback, status transitions)  
**Anti-patterns to avoid:**
- Cyberpunk / neon effects — wrong domain
- Complex onboarding flows
- Slow animations (>400ms)
- Layout-shifting transforms on press
- Emoji used as structural icons (use Bootstrap Icons only)

---

## 3. Color Tokens

### 3.1 Dark Mode (active — `data-bs-theme="dark"`)

The most important color rule for Proviant: **status colors must be instantly readable at a glance**. Expiry state is the primary information signal.

```scss
// ── Status colors (CRITICAL — keep high contrast on dark bg) ──
$success:   #4ADE80;   // Fresh / OK          — green-400 (WCAG AA on dark)
$warning:   #FBBF24;   // Expiring soon        — amber-400
$danger:    #F87171;   // Expired / overdue    — red-400
$info:      #67E8F9;   // Informational        — cyan-300

// ── Brand ──
$primary:   #577590;   // Slate Blue (keep — works well in dark)
$secondary: #94A3B8;   // Cool Gray — slightly lighter than current for contrast

// ── Surfaces ──
$body-bg:         #0F172A;   // Slate-900 — rich dark, not pure black (avoids OLED smear)
$surface:         #1E293B;   // Slate-800 — cards, nav, modals
$surface-raised:  #334155;   // Slate-700 — elevated elements, dropdowns
$border-color:    rgba(255, 255, 255, 0.08);  // hairline, subtle

// ── Text ──
$body-color:        #E2E8F0;  // Slate-200 — primary text (contrast 12:1 on bg)
$text-muted:        #94A3B8;  // Slate-400 — secondary/metadata text
$text-emphasis:     #F8FAFC;  // Slate-50  — headings, strong labels
```

### 3.2 Semantic Token Map (Bootstrap overrides)

| Token | Value | Usage |
|-------|-------|-------|
| `--bs-primary` | `#577590` | Brand actions, links |
| `--bs-success` | `#4ADE80` | Fresh products, OK states |
| `--bs-warning` | `#FBBF24` | Expiring soon (≤7 days) |
| `--bs-danger`  | `#F87171` | Expired, destructive actions |
| `--bs-info`    | `#67E8F9` | Scanned date, neutral info |

### 3.3 Status Badge Rules (Expiry — core UI logic)

```
Expired (date passed)        → bg-danger   (#F87171)
Expiring soon (≤7 days)      → bg-warning  (#FBBF24) + dark text
Fresh (>7 days)              → bg-success  (#4ADE80) + dark text
No date set                  → bg-secondary (#94A3B8)
```

---

## 4. Typography

**Existing stack is solid — keep DM Sans Variable.**

```scss
$font-family-base:    'DM Sans Variable', system-ui, sans-serif;
$font-family-heading: 'DM Sans Variable', sans-serif;  // same — consistent
$font-family-logo:    'Pacifico', cursive;              // keep for brand mark only

// Scale (Bootstrap defaults are fine; enforce minimums)
$font-size-base:  1rem;        // 16px — never go below on body text
$line-height-base: 1.6;        // change from current 2.0 → 1.6 (2.0 is too loose)
$h1-font-size:    2rem;        // 32px
$h2-font-size:    1.5rem;      // 24px
$h3-font-size:    1.25rem;     // 20px
$h4-font-size:    1.125rem;    // 18px
$font-weight-normal:  400;
$font-weight-medium:  500;
$font-weight-bold:    700;
```

**Key fix:** `$line-height-base: 2` is too aggressive — it makes content feel very spaced out and wastes vertical space on product lists. Change to `1.6`.

### Font Weight Hierarchy

| Element | Weight | Size |
|---------|--------|------|
| Page headings | 700 | 24–32px |
| Card titles (product name) | 600 | 18px |
| Body / labels | 400 | 16px |
| Metadata (barcode, dates) | 400 | 14px |
| Badge text | 500 | 12–13px |
| Logo (Pacifico) | 400 | any |

---

## 5. Spacing Scale

Use Bootstrap's 4pt/8px grid. Custom spacing must be multiples of 4.

```
4px   — tight internal (icon gap, badge padding)
8px   — default gap between inline elements
16px  — card internal padding, form field gap
24px  — section spacing, card gap in grid
32px  — major section breaks
48px  — page-level vertical rhythm
```

**Current issue:** `#content-wrapper { padding-top: 100px }` — this is fine for fixed navbar clearance, but verify on mobile (navbar collapses, so this may be too much).

---

## 6. Layout & Responsive

| Breakpoint | Width | Layout |
|-----------|-------|--------|
| xs (mobile) | <576px | Single column, full-width cards |
| sm | 576px | Single column |
| md | 768px | 2-column product grid |
| lg | 992px | 3-column product grid |
| xl | 1200px | 3-column, wider container |

**Rules:**
- `viewport-meta` — already correct (`width=device-width, initial-scale=1`) — do NOT add `user-scalable=no`
- Container max-width: Bootstrap default `container` — keep as-is
- Product cards: `row-cols-1 row-cols-md-2 row-cols-lg-3` — change from current `row-cols-md-3` to include 2-col at md
- No horizontal scroll — verify on 375px

---

## 7. Component Patterns

### 7.1 Product Card

**Current issues to fix:**
- Inline `<style>` in `productCard.tmpl` — move to SCSS
- `col-md-1 / col-md-2 / col-md-9` split for checkbox/image/content works well — keep layout
- Image `max-height: 160px` — fine, but add a fallback placeholder style for missing images
- ID badge (`#{{.Model.ID}}`) — low priority info, consider `text-muted` styling instead of primary

**Recommended card structure (semantic hierarchy):**
```
Card
├── [checkbox]  (top-left, accessible label required)
├── [image]     (fixed 80×80, object-fit: contain, bg: surface)
├── [content]
│   ├── Product name (h4, weight 600)
│   ├── Barcode (font-mono, text-muted, 13px)
│   └── Category badges
└── [list-group footer]
    ├── Expiry date     → status badge (danger/warning/success)
    ├── Scanned date    → info badge
    └── Notification    → muted badge
```

**Checkbox accessibility fix (required):**
```html
<!-- Current (bad): aria-label="..." with empty string -->
<input class="form-check-input" type="checkbox" id="checkbox-{{ .Model.ID }}"
       aria-label="Select {{ .ProductName }}">
```

### 7.2 Status Badges

```html
<!-- Expiry badge — dynamic class from template -->
<span class="badge rounded-pill {{ expiryBadgeClass .ExpireAt }}">
  {{ .ExpireAt | humanDate }}
</span>
```

Template helper should return:
- `bg-danger text-white` — expired
- `bg-warning text-dark` — ≤7 days
- `bg-success text-dark` — fresh

### 7.3 Navigation

**Current issues:**
- Icon-only nav on desktop (≥lg) — violates `nav-label-icon` rule
- Nav items show either text (mobile) or icon (desktop) — should show both, or add `title` tooltip on icon

**Fix:** Add `data-bs-toggle="tooltip"` + `title` attribute to icon-only nav links on desktop.

```html
<i class="mx-3 d-none d-lg-inline bi bi-box-seam-fill"
   data-bs-toggle="tooltip" title="All Products"></i>
```

**Active state:** Current nav has no active state indicator. Add `aria-current="page"` and Bootstrap's `.active` class to the current page link via template.

### 7.4 Forms (Create/Edit Product)

- Labels must be visible — not placeholder-only
- Required fields: add `*` with `.text-danger` and `aria-required="true"`
- Error messages: place below the field (`invalid-feedback` class)
- Submit button: disable + show spinner during async submit
- Input height: min 44px (Bootstrap's default form controls meet this)

### 7.5 Empty States

Use consistent structure:
```html
<div class="text-center py-5">
  <i class="bi bi-[icon] hero-icon text-muted"></i>
  <p class="fs-4 mt-3">[Short message]</p>
  <p class="text-muted">[Helpful explanation]</p>
  <a class="btn btn-primary mt-3" href="...">[Primary action]</a>
</div>
```

### 7.6 Home Dashboard Tiles

Current: `card text-body-secondary` with `display-5` hero number.  
Recommendation: Keep structure, add status coloring to hero numbers (e.g., expired count in danger color).

---

## 8. Animation & Motion

```scss
// Durations
--transition-fast:    150ms;  // hover, badge state
--transition-base:    250ms;  // card interactions, modals
--transition-slow:    350ms;  // page-level transitions

// Easing
--ease-out: cubic-bezier(0.0, 0.0, 0.2, 1);  // enter
--ease-in:  cubic-bezier(0.4, 0.0, 1, 1);     // exit
```

**Card press (already implemented — keep):**
```scss
.card {
  transition: transform 0.06s ease;  // current is good
}
.card.card-clicked {
  transform: scale(0.98);            // subtle — correct
}
```

**Nav icon hover (already implemented — keep):**
```scss
.navbar-nav .nav-link i {
  transition: color 0.2s ease;       // fine
}
```

**Add: reduced motion support**
```scss
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 9. Accessibility Checklist

- [ ] Checkbox `aria-label` must include product name (not empty string)
- [ ] Icon-only nav links need `title` tooltip or visible label
- [ ] Active nav item needs `aria-current="page"`
- [ ] Status info (expiry) must not rely on color alone — date text is already shown (good)
- [ ] Form labels: use `<label for="...">` — not placeholder-only
- [ ] Focus rings: Bootstrap 5 default focus-visible rings — do NOT remove
- [ ] Image `alt` text on product images: use product name
- [ ] Logo image: `alt="Proviant"` — already correct
- [ ] Contrast: all badge text must meet 4.5:1 on badge bg color
  - `bg-warning text-dark`: amber #FBBF24 + dark #1E293B = ✓ passes
  - `bg-danger text-white`: red #F87171 + white — verify at ~4.5:1
  - `bg-success text-dark`: green #4ADE80 + dark = ✓ passes

---

## 10. Pre-Delivery Checklist

- [ ] No inline `<style>` blocks in templates — all styles in SCSS
- [ ] `line-height-base` changed from `2` to `1.6`
- [ ] Status badge colors follow expiry traffic-light rules
- [ ] Checkbox `aria-label` includes product name
- [ ] Nav active state set via template
- [ ] `prefers-reduced-motion` media query added to SCSS
- [ ] Mobile layout: product grid is 1-col on xs/sm, 2-col on md, 3-col on lg+
- [ ] Content padding-top on mobile verified (navbar collapse height)
- [ ] No horizontal scroll at 375px viewport width

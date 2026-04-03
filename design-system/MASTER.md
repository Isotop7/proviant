# Design System — Proviant

> **LOGIC:** When building a specific page, first check `design-system/pages/[page-name].md`.
> If that file exists, its rules **override** this Master file.
> If not, strictly follow the rules below.

---

**Project:** Proviant — Food Expiration Tracking / Pantry Management
**Updated:** 2026-04-03
**Theme:** Light mode — `data-bs-theme="light"` on all pages
**Palette:** Fresh Garden — matched to `src/assets/icons/hero.png`

---

## 1. Color Tokens

Extracted directly from the Proviant icon: basket green outline, leaf greens, carrot orange, apple green, and logo steel blue.

### Surfaces

| Role | Hex | SCSS var |
|------|-----|----------|
| Page background | `#F5FAF5` | `$body-bg` |
| Card | `#FFFFFF` | `$card-bg` |
| Elevated / info blocks | `#EBF5EB` | `$card-cap-bg`, `$light`, `--bs-tertiary-bg` |

### Text

| Role | Hex |
|------|-----|
| Primary | `#1A3028` — dark forest |
| Strong / emphasis | `#2D4F3C` |
| Muted | `#6A9580` — sage |

### Brand & Status

| Role | Hex | SCSS var | Source |
|------|-----|----------|--------|
| Primary / brand | `#3D7A5C` | `$primary` | basket outline green |
| Secondary / links | `#5B7FA6` | `$secondary`, `$link-color` | logo steel blue |
| Info | `#7BC67E` | `$info` | leaf / vegetable green |
| Success | `#2D9B4F` | `$success` | produce green |
| Warning | `#E8914E` | `$warning` | carrot orange |
| Danger | `#DC2626` | `$danger` | red |

### Borders & Shadows

| Role | Value |
|------|-------|
| Border | `#C8DDD0` — muted green |
| Shadow | `rgba(26, 48, 40, 0.10)` |

### Status Badge Rules — Expiry (core logic)

| State | Condition | Badge classes |
|-------|-----------|---------------|
| Expired | date passed | `bg-danger text-white` |
| Expiring soon | ≤ 7 days | `bg-warning text-dark` |
| Fresh | > 7 days | `bg-success text-dark` |
| No date | — | `bg-secondary` |

---

## 2. Typography

```scss
$font-family-base:  'DM Sans Variable', system-ui, sans-serif;  // self-hosted woff2
$font-family-logo:  'Pacifico', cursive;    // brand mark only (.logo-font)
$font-size-base:    1rem;                   // 16px — never go below on body text
$line-height-base:  1.6;
```

| Element | Weight | Size |
|---------|--------|------|
| Page headings | 700 | 24–32px |
| Card titles | 600 | 18px |
| Body / labels | 400 | 16px |
| Metadata (barcode, dates) | 400 | 14px |
| Badge text | 500 | 12–13px |

---

## 3. Spacing Scale (8pt grid)

Multiples of 4px only. Bootstrap's default spacing utilities are sufficient.

| Size | Value | Usage |
|------|-------|-------|
| xs | 4px | icon gaps, badge padding |
| sm | 8px | inline element gaps |
| md | 16px | card padding, form field gaps |
| lg | 24px | section spacing, card gaps |
| xl | 32px | major breaks |
| 2xl | 48px | page-level rhythm |

---

## 4. Layout & Responsive

| Breakpoint | Product grid |
|-----------|-------------|
| xs / sm (<768px) | 1 column |
| md (768px) | 2 columns |
| lg+ (992px+) | 3 columns |

- `viewport-meta` — `width=device-width, initial-scale=1` — never add `user-scalable=no`
- No horizontal scroll at 375px — verify after any layout change

---

## 5. Component Patterns

### Product Card

Semantic hierarchy:
```
Card
├── [checkbox]   aria-label="Select {ProductName}" (required)
├── [image]      max-height 120px, object-fit: contain
├── [content]    product name (h4, 600) · barcode (mono, muted, 13px) · category badges
└── [list-group]
    ├── Expiry     → status badge (danger / warning / success)
    ├── Scanned    → info badge
    └── Notified   → secondary badge
```

### Navbar

- `border-bottom: 6px solid var(--bs-primary)` — basket green accent bar
- `bg-body-secondary` background
- Logo: Pacifico font, `var(--bs-info)` color (leaf green)
- Desktop icon-only links must include `data-bs-toggle="tooltip" title="..."` (already implemented)
- Active link: `.active` class + `aria-current="page"` (set via template)

### Empty States

```html
<div class="text-center py-5">
  <i class="bi bi-[icon] hero-icon text-muted"></i>
  <p class="fs-4 mt-3">[Short message]</p>
  <p class="text-muted">[Explanation]</p>
  <a class="btn btn-primary mt-3" href="...">[Primary action]</a>
</div>
```

### Forms

- Visible `<label for="...">` per input — no placeholder-only labels
- Required fields: `*` with `text-danger` + `aria-required="true"`
- Errors: `invalid-feedback` class below the field
- Submit: disable + spinner while async in progress

### Popups (Modals, Toasts, Inline Alerts)

> Full spec: `design-system/pages/popups.md` — implementation guide: `design-system/pages/popups-implementation-guide.md`

| Component | Key rules |
|-----------|-----------|
| **Feedback Modal** | Single `#proviantFeedbackModal` in `base.tmpl`; `proviant.showFeedback(type, title, msg, onClose?)` everywhere; icon + color per semantic type |
| **Confirmation Modal** | `modal fade` + `modal-dialog-centered`; close + Cancel + semantic-color action |
| **Toast** | Auth page only; `position-fixed top-0 start-50 translate-middle-x`; `text-bg-{semantic}`; `data-bs-delay="4000"` |
| **Inline Alert** | Field/section errors only; `d-none fade`; semantic color set by JS; never for page-level async results |

---

### Email Templates (inline styles only)

| Element | Value |
|---------|-------|
| Body bg | `#F5FAF5` |
| Card container | `#FFFFFF` + `box-shadow: 0 4px 16px rgba(26,48,40,0.10)` |
| Info block | `#EBF5EB` + `border: 1px solid #C8DDD0` |
| Brand icon badge | `#3D7A5C` bg, `#FFFFFF` text |
| Danger icon badge | `#DC2626` bg, `#FFFFFF` text |
| Body text | `#1A3028` |
| Muted text | `#6A9580` |
| Strong text | `#2D4F3C` |
| CTA button | `#3D7A5C` bg, `#FFFFFF` text, `border-radius: 8px` |
| Footer divider | `#C8DDD0` |

---

## 6. Animation & Motion

| Token | Value | Usage |
|-------|-------|-------|
| Fast | 150ms | hover, badge state |
| Base | 250ms | card interactions, modals |
| Slow | 350ms | page transitions |

- Card press: `transform: scale(0.98)` via `.card-clicked` (already in SCSS)
- Reduced motion: `@media (prefers-reduced-motion: reduce)` already in SCSS — keep

---

## 7. Accessibility Checklist

- [ ] Checkbox `aria-label` includes product name
- [ ] Nav icon links have tooltip `title` attribute
- [ ] Active nav item has `aria-current="page"`
- [ ] Status info (expiry) shows date text — not color alone
- [ ] Form labels use `<label for="...">` — not placeholder-only
- [ ] Focus rings: Bootstrap 5 default `focus-visible` — do NOT remove
- [ ] Product image `alt` uses product name
- [ ] Logo `alt="Proviant home"` (already correct)
- [ ] Warning badge: `text-dark` on `#E8914E` — verify ≥ 4.5:1 contrast

---

## 8. Anti-Patterns

- ❌ `data-bs-theme="dark"` — app is light-mode only
- ❌ Raw hex in templates — use Bootstrap utility classes or `var(--bs-*)`
- ❌ `text-white` on white-background cards
- ❌ Emojis as icons — Bootstrap Icons (`bi-*`) only
- ❌ Inline `<style>` blocks in templates — all styles in `main.scss`
- ❌ `$line-height-base > 1.6` — too loose, wastes vertical space on product lists

---

## 9. Pre-Delivery Checklist

- [ ] `data-bs-theme="light"` on HTML element
- [ ] Surfaces: page `#F5FAF5`, card `#FFFFFF`, elevated `#EBF5EB`
- [ ] Status badges follow expiry traffic-light rules (§1)
- [ ] Bootstrap utilities used (not raw hex)
- [ ] All icons from Bootstrap Icons
- [ ] `cursor-pointer` on all clickable elements
- [ ] Hover/active transitions 150–300ms
- [ ] Text contrast ≥ 4.5:1 on white card surfaces
- [ ] Responsive: 375px single-col verified
- [ ] `make css` run after any `.scss` change
- [ ] SW cache version bumped after JS/CSS changes
- [ ] New DB models added to `AutoMigrate` in `src/proviant.go`

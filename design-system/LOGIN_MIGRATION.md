# Proviant — Login Screen Migration Guide
> Design reference: `ui_kits/proviant/Login v2.html`  
> Current implementation: Go templates + Bootstrap 5 SCSS  
> Prototype (old): `ui_kits/proviant/Login.jsx`  
> Generated: 2026-05-01

---

## Overview

The login screen is being redesigned from a **centred single-column card** to a **split-panel layout**:

- **Left (desktop only):** Fixed-width green brand panel with logo, headline, feature list
- **Right:** Form panel with underline-tab mode switcher, labelled inputs, password strength meter, inline validation, and improved toast

Local authentication only — no OAuth/social providers.

---

## 1. What changes vs. current implementation

| Area | Current (Go template) | v2 Design | Action |
|------|----------------------|-----------|--------|
| Layout | Centred card, max-width ~400px, white bg | Split panel: 440px green left + flexible right | Replace outer wrapper |
| Tab switcher | Segmented pill control (bg-subtle container, white active pill) | Underline tabs (border-bottom accent, no bg container) | Restyle tab buttons |
| Form card | White card with border + shadow wrapping the form | No card wrapper — form sits directly on warm bg | Remove `.card` wrapper |
| Input labels | `font-size:12px, font-weight:500, color:var(--fg)` | `font-size:11px, font-weight:600, uppercase, letter-spacing:0.5px, color:var(--fg-2)` | Update label styles |
| Input height | Implicit ~34px (Bootstrap default) | Explicit **44px** | Add `height: 44px` |
| Password field | No show/hide toggle | Eye toggle button (right-side absolute) | Add toggle button |
| Password strength | Not present | 4-bar strength meter on signup (Weak/Fair/Good/Strong) | Add strength indicator (frontend only — no backend needed) |
| Validation | On submit: single toast if any field empty | Per-field inline errors below each input, validated on submit | Add per-field error state |
| Toast | Left-border only, icon + text | Left coloured bar + title + body text + close button | Update toast structure |
| Forgot password | Not present | Ghost link right-aligned below password field (sign-in mode only) | Add link — routes to `/forgot-password` |
| Terms note | Not present | Small centred text below submit on signup | Add terms text with links |
| Brand panel | Not present | Fixed 440px green panel, hidden on mobile | Add new `<aside>` element |
| Mobile | Centred card on `var(--bg)` | No brand panel; compact logo at top of form | Use `@media` to hide panel |
| Wordmark | `<img src="/static/img/header.png">` centred above card | Inline `P` logomark + "proviant" text (brand panel) + mobile fallback | See §4 |

---

## 2. HTML structure — full diff

### Current structure (simplified)
```html
<div class="login-root"> <!-- min-height:100vh, flex, center, bg:var(--bg) -->
  <!-- toast (fixed, top-right) -->

  <div class="login-card"> <!-- max-width:400px, white, border, shadow, border-radius:12px, padding:28px -->
    <div class="wordmark"> <!-- centred img header.png + tagline -->
    </div>

    <div class="mode-tabs"> <!-- bg-subtle pill switcher -->
      <button class="tab active">Sign in</button>
      <button class="tab">Create account</button>
    </div>

    <form>
      <!-- signup only: name input -->
      <div class="field">...</div> <!-- email -->
      <div class="field">...</div> <!-- password -->
      <button type="submit" class="btn-primary full-width">Sign in</button>
    </form>
  </div>
</div>
```

### v2 structure
```html
<div class="login-root"> <!-- display:flex, min-height:100vh -->

  <!-- Brand panel — hidden on mobile (<768px) -->
  <aside class="login-brand">
    <!-- Decorative circles via ::before ::after + .brand-circle-sm -->
    <div class="brand-top">
      <div class="brand-logo">
        <div class="brand-logo-mark">P</div>
        <div class="brand-logo-name">proviant</div>
      </div>
      <h1 class="brand-headline">Less waste.<br>More meals.</h1>
      <p class="brand-sub">Track what's in your pantry…</p>
    </div>

    <div class="brand-features">
      <div class="brand-feature">
        <div class="brand-feature-icon"><i class="bi bi-upc-scan"></i></div>
        <div class="brand-feature-text">Scan barcodes to add products instantly</div>
      </div>
      <!-- repeat for: bi-bell, bi-journal-richtext, bi-graph-down-arrow -->
    </div>

    <div class="brand-bottom">© 2026 Proviant · Free to use · Open source</div>
  </aside>

  <!-- Form panel -->
  <div class="login-form-panel"> <!-- flex:1, flex center, overflow-y:auto -->
    <div class="login-form-inner"> <!-- max-width:380px -->

      <!-- Mobile logo (visible only on mobile) -->
      <div class="mobile-logo">
        <div class="mobile-logo-mark">P</div>
        <div class="mobile-logo-name">proviant</div>
      </div>

      <!-- Underline tab switcher -->
      <div class="auth-tabs">
        <button class="auth-tab active">Sign in</button>
        <button class="auth-tab">Create account</button>
      </div>

      <!-- Heading (changes per mode) -->
      <div class="form-heading">
        <div class="form-heading-title">Welcome back</div>
        <div class="form-heading-sub">Sign in to your Proviant account</div>
      </div>

      <form method="POST" action="/login" novalidate>
        {{ if eq .Mode "signup" }}
        <!-- Name field (signup only) -->
        <div class="field">
          <label class="field-label">Display name</label>
          <div class="field-input-wrap">
            <input class="field-input {{ if .Errors.name }}error{{ end }}"
              type="text" name="name" placeholder="Your name"
              value="{{ .Form.name }}" autocomplete="name">
            <i class="bi bi-person field-icon"></i>
          </div>
          {{ if .Errors.name }}
          <div class="field-error">
            <i class="bi bi-exclamation-circle-fill"></i> {{ .Errors.name }}
          </div>
          {{ end }}
        </div>
        {{ end }}

        <!-- Email field -->
        <div class="field">
          <label class="field-label">Email</label>
          <div class="field-input-wrap">
            <input class="field-input {{ if .Errors.email }}error{{ end }}"
              type="email" name="email" placeholder="you@example.com"
              value="{{ .Form.email }}" autocomplete="email">
            <i class="bi bi-envelope field-icon"></i>
          </div>
          {{ if .Errors.email }}
          <div class="field-error">
            <i class="bi bi-exclamation-circle-fill"></i> {{ .Errors.email }}
          </div>
          {{ end }}
        </div>

        <!-- Password field -->
        <div class="field">
          <label class="field-label">Password</label>
          <div class="field-input-wrap">
            <input class="field-input {{ if .Errors.password }}error{{ end }}"
              type="password" name="password" placeholder="••••••••"
              autocomplete="{{ if eq .Mode "signin" }}current-password{{ else }}new-password{{ end }}"
              id="password-input">
            <i class="bi bi-lock field-icon"></i>
            <!-- Show/hide toggle -->
            <button type="button" class="pwd-toggle" aria-label="Toggle password visibility"
              onclick="togglePassword()">
              <i class="bi bi-eye" id="pwd-eye-icon"></i>
            </button>
          </div>
          {{ if .Errors.password }}
          <div class="field-error">
            <i class="bi bi-exclamation-circle-fill"></i> {{ .Errors.password }}
          </div>
          {{ end }}
        </div>

        {{ if eq .Mode "signin" }}
        <!-- Forgot password (sign-in only) -->
        <div class="forgot-link">
          <a href="/forgot-password">Forgot password?</a>
        </div>
        {{ end }}

        <!-- CSRF token -->
        <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
        <!-- Mode indicator -->
        <input type="hidden" name="mode" value="{{ .Mode }}">

        <button type="submit" class="btn-submit">
          <i class="bi {{ if eq .Mode "signin" }}bi-box-arrow-in-right{{ else }}bi-person-plus{{ end }}"></i>
          {{ if eq .Mode "signin" }}Sign in{{ else }}Create account{{ end }}
        </button>

        {{ if eq .Mode "signup" }}
        <p class="terms-note">
          By creating an account you agree to our
          <a href="/terms">Terms of Service</a> and
          <a href="/privacy">Privacy Policy</a>.
        </p>
        {{ end }}
      </form>
    </div>
  </div>
</div>
```

---

## 3. CSS to add

Add the following to your login page stylesheet (or a dedicated `login.scss` partial). All values reference existing tokens from `colors_and_type.css`.

```scss
// ── Layout ──────────────────────────────────────────────
.login-root {
  display: flex;
  min-height: 100vh;
}

// ── Brand panel ─────────────────────────────────────────
.login-brand {
  width: 440px;
  flex-shrink: 0;
  background: var(--accent);
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 48px 48px 44px;

  &::before {
    content: '';
    position: absolute;
    width: 340px; height: 340px;
    border-radius: 50%;
    background: rgba(255,255,255,0.07);
    top: -80px; right: -100px;
  }
  &::after {
    content: '';
    position: absolute;
    width: 220px; height: 220px;
    border-radius: 50%;
    background: rgba(255,255,255,0.05);
    bottom: 40px; left: -60px;
  }
}

.brand-circle-sm {
  position: absolute;
  width: 120px; height: 120px;
  border-radius: 50%;
  background: rgba(255,255,255,0.06);
  bottom: 160px; right: 30px;
}

.brand-top { position: relative; z-index: 1; }

.brand-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 52px;
}

.brand-logo-mark {
  width: 44px; height: 44px;
  background: rgba(255,255,255,0.18);
  border-radius: 12px;
  display: flex; align-items: center; justify-content: center;
  font-size: 20px; font-weight: 700; color: #fff;
  letter-spacing: -0.5px;
  border: 1px solid rgba(255,255,255,0.25);
}

.brand-logo-name {
  font-size: 22px; font-weight: 700;
  color: #fff; letter-spacing: -0.5px;
}

.brand-headline {
  font-size: 32px; font-weight: 700;
  color: #fff; line-height: 1.15;
  letter-spacing: -0.8px; margin-bottom: 16px;
}

.brand-sub {
  font-size: 15px;
  color: rgba(255,255,255,0.72);
  line-height: 1.6; max-width: 320px;
}

.brand-features {
  position: relative; z-index: 1;
  display: flex; flex-direction: column; gap: 16px;
}

.brand-feature {
  display: flex; align-items: center; gap: 14px;
}

.brand-feature-icon {
  width: 36px; height: 36px;
  border-radius: 10px;
  background: rgba(255,255,255,0.14);
  border: 1px solid rgba(255,255,255,0.18);
  display: flex; align-items: center; justify-content: center;
  font-size: 16px; color: #fff; flex-shrink: 0;
}

.brand-feature-text {
  font-size: 13px;
  color: rgba(255,255,255,0.85);
  font-weight: 500; line-height: 1.4;
}

.brand-bottom {
  position: relative; z-index: 1;
  font-size: 11px;
  color: rgba(255,255,255,0.45);
  letter-spacing: 0.2px;
}

// ── Form panel ──────────────────────────────────────────
.login-form-panel {
  flex: 1;
  display: flex; align-items: center; justify-content: center;
  padding: 48px 32px;
  background: var(--bg);
  overflow-y: auto;
}

.login-form-inner {
  width: 100%; max-width: 380px;
}

// ── Tab switcher ────────────────────────────────────────
.auth-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-subtle);
  margin-bottom: 32px;
}

.auth-tab {
  padding: 10px 20px;
  font-family: var(--font-ui);
  font-size: 14px; font-weight: 500;
  color: var(--fg-3);
  border: none;
  border-bottom: 2px solid transparent;
  background: transparent;
  cursor: pointer;
  transition: color 150ms ease, border-color 150ms ease;
  margin-bottom: -1px;
  text-decoration: none;  // if using <a> tags

  &:hover { color: var(--fg-2); }

  &.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
    font-weight: 600;
  }
}

// ── Form heading ────────────────────────────────────────
.form-heading { margin-bottom: 28px; }

.form-heading-title {
  font-size: 22px; font-weight: 700;
  color: var(--fg); letter-spacing: -0.5px; margin-bottom: 4px;
}

.form-heading-sub {
  font-size: 13px; color: var(--fg-2); line-height: 1.5;
}

// ── Fields ──────────────────────────────────────────────
.field {
  display: flex; flex-direction: column;
  gap: 6px; margin-bottom: 16px;
}

.field-label {
  font-size: 11px; font-weight: 600;
  color: var(--fg-2);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.field-input-wrap { position: relative; }

.field-icon {
  position: absolute;
  left: 12px; top: 50%; transform: translateY(-50%);
  color: var(--fg-3); font-size: 15px;
  pointer-events: none;
  transition: color 150ms ease;
}

.field-input-wrap:focus-within .field-icon {
  color: var(--accent);
}

.field-input {
  font-family: var(--font-ui);
  font-size: 14px; color: var(--fg);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 0 12px 0 38px;
  height: 44px; width: 100%;
  outline: none;
  transition: border-color 150ms ease, box-shadow 150ms ease;

  &::placeholder { color: var(--fg-3); }

  &:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px oklch(from var(--accent) l c h / 0.12);
  }

  &.error {
    border-color: var(--status-expired);
    box-shadow: 0 0 0 3px oklch(from var(--status-expired) l c h / 0.10);
  }

  // Extra right padding when password toggle is present
  &.has-toggle { padding-right: 40px; }
}

.field-error {
  display: flex; align-items: center; gap: 5px;
  font-size: 12px; color: var(--status-expired);
}

// ── Password toggle ─────────────────────────────────────
.pwd-toggle {
  position: absolute;
  right: 12px; top: 50%; transform: translateY(-50%);
  background: none; border: none; cursor: pointer;
  color: var(--fg-3); font-size: 15px; padding: 0;
  display: flex; align-items: center;
  transition: color 150ms;

  &:hover { color: var(--fg-2); }
}

// ── Password strength (signup only) ─────────────────────
.strength-meter {
  display: flex; gap: 3px; margin-top: 6px;
}

.strength-bar {
  flex: 1; height: 3px;
  border-radius: 2px;
  background: var(--border);
  transition: background 300ms ease;
}

.strength-label {
  font-size: 11px; font-weight: 500; margin-top: 4px;
}

// ── Forgot password ─────────────────────────────────────
.forgot-link {
  display: flex; justify-content: flex-end;
  margin-top: -10px; margin-bottom: 16px;

  a {
    font-size: 12px; color: var(--accent);
    text-decoration: none; font-weight: 500;
    &:hover { text-decoration: underline; }
  }
}

// ── Submit button ───────────────────────────────────────
.btn-submit {
  display: flex; align-items: center; justify-content: center;
  gap: 8px; width: 100%; height: 44px;
  border-radius: 10px;
  border: 1px solid var(--accent-active);
  background: var(--accent); color: white;
  font-family: var(--font-ui);
  font-size: 14px; font-weight: 600;
  cursor: pointer; letter-spacing: -0.1px;
  margin-top: 8px;
  box-shadow: 0 1px 0 rgba(0,0,0,0.12), inset 0 1px 0 rgba(255,255,255,0.12);
  transition: background 150ms ease, box-shadow 150ms ease, transform 120ms ease;

  &:hover:not(:disabled) {
    background: var(--accent-hover);
    box-shadow: 0 3px 10px oklch(from var(--accent) l c h / 0.35), 0 1px 0 rgba(0,0,0,0.12);
    transform: translateY(-1px);
  }

  &:active:not(:disabled) {
    transform: translateY(0);
    box-shadow: none;
  }

  &:disabled { opacity: 0.7; cursor: not-allowed; }
}

// ── Terms note ──────────────────────────────────────────
.terms-note {
  margin-top: 16px;
  font-size: 11px; color: var(--fg-3);
  line-height: 1.6; text-align: center;

  a { color: var(--fg-2); text-decoration: underline; }
}

// ── Mobile logo ─────────────────────────────────────────
.mobile-logo {
  display: none;
  align-items: center; gap: 10px;
  margin-bottom: 32px;
}

.mobile-logo-mark {
  width: 38px; height: 38px;
  background: var(--accent); border-radius: 10px;
  display: flex; align-items: center; justify-content: center;
  color: white; font-size: 18px; font-weight: 700;
}

.mobile-logo-name {
  font-size: 20px; font-weight: 700;
  color: var(--fg); letter-spacing: -0.4px;
}

// ── Responsive ──────────────────────────────────────────
@media (max-width: 768px) {
  .login-brand    { display: none; }
  .mobile-logo    { display: flex; }
  .login-form-panel {
    align-items: flex-start;
    padding: 40px 24px;
  }
  .login-form-inner { max-width: 100%; }
}

// ── Toast (updated structure) ────────────────────────────
.pv-toast-v2 {
  position: fixed; top: 20px; right: 20px;
  background: var(--surface);
  border-radius: 10px;
  box-shadow: var(--shadow-lg);
  padding: 12px 16px;
  display: flex; align-items: center; gap: 10px;
  z-index: 9999; min-width: 280px; max-width: 360px;
}

.pv-toast-v2-bar {
  width: 3px; align-self: stretch;
  border-radius: 2px; flex-shrink: 0;

  &.success { background: var(--status-fresh); }
  &.error   { background: var(--status-expired); }
  &.info    { background: var(--accent); }
}

.pv-toast-v2-body { flex: 1; }

.pv-toast-v2-title {
  font-size: 13px; font-weight: 600; color: var(--fg); margin-bottom: 1px;
}

.pv-toast-v2-msg {
  font-size: 12px; color: var(--fg-2);
}
```

---

## 4. JavaScript — password toggle + strength meter

Add this small inline script to the login page template. No external dependencies needed.

```javascript
// Password show/hide toggle
function togglePassword() {
  const input = document.getElementById('password-input');
  const icon  = document.getElementById('pwd-eye-icon');
  if (input.type === 'password') {
    input.type = 'text';
    icon.className = 'bi bi-eye-slash';
  } else {
    input.type = 'password';
    icon.className = 'bi bi-eye';
  }
}

// Password strength meter (signup only)
// Attach to the password input if mode === signup
const pwdInput = document.getElementById('password-input');
const strengthBars = document.querySelectorAll('.strength-bar');
const strengthLabel = document.querySelector('.strength-label');

const strengthColors = [
  'var(--status-expired)',   // Weak
  'var(--status-critical)',  // Fair
  'var(--status-soon)',      // Good
  'var(--status-fresh)',     // Strong
];
const strengthLabels = ['Weak', 'Fair', 'Good', 'Strong'];

if (pwdInput && strengthBars.length) {
  pwdInput.addEventListener('input', () => {
    const v = pwdInput.value;
    let score = 0;
    if (v.length >= 8)          score++;
    if (/[A-Z]/.test(v))        score++;
    if (/[0-9]/.test(v))        score++;
    if (/[^A-Za-z0-9]/.test(v)) score++;

    strengthBars.forEach((bar, i) => {
      bar.style.background = i < score
        ? strengthColors[score - 1]
        : 'var(--border)';
    });

    if (strengthLabel) {
      strengthLabel.textContent = score > 0 ? strengthLabels[score - 1] : '';
      strengthLabel.style.color = score > 0 ? strengthColors[score - 1] : 'var(--fg-3)';
    }
  });
}
```

Add the strength meter HTML below the password field (signup mode only):
```html
<div class="strength-meter" id="strength-meter" style="display:none;">
  <div class="strength-bar"></div>
  <div class="strength-bar"></div>
  <div class="strength-bar"></div>
  <div class="strength-bar"></div>
</div>
<div class="strength-label" id="strength-label"></div>
```

Show/hide the meter via Go template conditional:
```html
{{ if eq .Mode "signup" }}
  <div class="strength-meter">...</div>
{{ end }}
```

---

## 5. Go handler changes

### Route structure
The login page should handle both `signin` and `signup` modes via a `mode` query param or form field:

```
GET  /login          → renders mode=signin
GET  /login?mode=signup → renders mode=signup
POST /login          → processes form, reads mode from hidden input
POST /register       → alternative: separate endpoint for signup
```

### Template data struct
```go
type LoginPageData struct {
    Mode      string            // "signin" | "signup"
    Form      map[string]string // repopulated form values on error
    Errors    map[string]string // field-level errors: "email", "password", "name"
    CSRFToken string
    Flash     *FlashMessage     // optional: server-side toast (success/error)
}

type FlashMessage struct {
    Type    string // "success" | "error" | "info"
    Title   string
    Message string
}
```

### Validation rules (server-side — mirror the frontend)
```go
func validateLogin(form url.Values) map[string]string {
    errs := map[string]string{}
    if form.Get("email") == "" {
        errs["email"] = "Email is required"
    } else if !isValidEmail(form.Get("email")) {
        errs["email"] = "Enter a valid email address"
    }
    if form.Get("password") == "" {
        errs["password"] = "Password is required"
    }
    return errs
}

func validateSignup(form url.Values) map[string]string {
    errs := validateLogin(form)
    if form.Get("name") == "" {
        errs["name"] = "Display name is required"
    }
    if len(form.Get("password")) < 8 {
        errs["password"] = "Password must be at least 8 characters"
    }
    return errs
}
```

### Re-render with errors (do not redirect on validation failure)
```go
func handleLogin(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        r.ParseForm()
        mode := r.FormValue("mode")
        var errs map[string]string
        if mode == "signup" {
            errs = validateSignup(r.Form)
        } else {
            errs = validateLogin(r.Form)
        }
        if len(errs) > 0 {
            // Re-render with errors + repopulate form (except password)
            renderTemplate(w, "login.html", LoginPageData{
                Mode:      mode,
                Errors:    errs,
                Form:      map[string]string{"email": r.FormValue("email"), "name": r.FormValue("name")},
                CSRFToken: generateCSRF(),
            })
            return
        }
        // ... authenticate / create user ...
    }
}
```

---

## 6. Tab switching — client-side vs. server-side

The prototype switches modes client-side (React state). In the Go template, you have two options:

### Option A — Server-side (recommended, simpler)
Two separate links styled as tabs. Mode is a URL param.
```html
<div class="auth-tabs">
  <a href="/login" class="auth-tab {{ if eq .Mode "signin" }}active{{ end }}">Sign in</a>
  <a href="/login?mode=signup" class="auth-tab {{ if eq .Mode "signup" }}active{{ end }}">Create account</a>
</div>
```

### Option B — Client-side JS (no page reload)
```javascript
document.querySelectorAll('.auth-tab').forEach(tab => {
  tab.addEventListener('click', () => {
    const mode = tab.dataset.mode;
    document.querySelectorAll('.auth-tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
    // Show/hide signup-only fields
    document.querySelectorAll('.signup-only').forEach(el => {
      el.style.display = mode === 'signup' ? '' : 'none';
    });
    document.querySelector('[name="mode"]').value = mode;
    // Update heading text
    document.querySelector('.form-heading-title').textContent =
      mode === 'signin' ? 'Welcome back' : 'Get started free';
    document.querySelector('.form-heading-sub').textContent =
      mode === 'signin' ? 'Sign in to your Proviant account' : 'Create your household account';
    // Toggle forgot-password link
    const forgot = document.querySelector('.forgot-link');
    if (forgot) forgot.style.display = mode === 'signin' ? 'flex' : 'none';
    // Toggle terms note
    const terms = document.querySelector('.terms-note');
    if (terms) terms.style.display = mode === 'signup' ? 'block' : 'none';
    // Toggle submit button label
    document.querySelector('.btn-submit .btn-label').textContent =
      mode === 'signin' ? 'Sign in' : 'Create account';
  });
});
```

Add `data-mode="signin"` / `data-mode="signup"` attributes to tab buttons, and `class="signup-only"` to the name field wrapper and strength meter.

---

## 7. Forgot password page

The "Forgot password?" link routes to `/forgot-password`. That page is out of scope for this migration but should follow the same layout:
- Same split-panel layout (reuse `.login-root`, `.login-brand`, `.login-form-panel`)
- Single email field, no tabs
- Heading: "Reset your password" / sub: "Enter your email and we'll send you a link"

---

## 8. Accessibility checklist

- [ ] All inputs have associated `<label>` elements (use `for`/`id` pair in Go templates, not just proximity)
- [ ] Password toggle button has `aria-label="Toggle password visibility"` and `aria-pressed` state
- [ ] Error messages are linked to their input via `aria-describedby`
- [ ] Submit button shows loading state accessible text (e.g. `aria-label="Signing in, please wait"` when loading)
- [ ] Brand panel is `aria-hidden="true"` — it's decorative, not content

Example accessible field:
```html
<div class="field">
  <label class="field-label" for="email">Email</label>
  <div class="field-input-wrap">
    <input class="field-input {{ if .Errors.email }}error{{ end }}"
      type="email" id="email" name="email"
      placeholder="you@example.com"
      value="{{ .Form.email }}"
      autocomplete="email"
      aria-describedby="{{ if .Errors.email }}email-error{{ end }}"
      aria-invalid="{{ if .Errors.email }}true{{ end }}">
    <i class="bi bi-envelope field-icon" aria-hidden="true"></i>
  </div>
  {{ if .Errors.email }}
  <div class="field-error" id="email-error" role="alert">
    <i class="bi bi-exclamation-circle-fill" aria-hidden="true"></i>
    {{ .Errors.email }}
  </div>
  {{ end }}
</div>
```

---

## 9. Files to create/modify

| File | Action | Notes |
|------|--------|-------|
| `templates/login.html` | **Modify** | Replace layout with split-panel structure (§2) |
| `static/css/login.scss` | **Create** | Add all CSS from §3 as a new partial |
| `static/css/main.scss` | **Modify** | `@use 'login'` or `@import 'login'` |
| `templates/login.html` | **Modify** | Add inline JS from §4 at bottom of file |
| `handlers/auth.go` (or equivalent) | **Modify** | Add `LoginPageData` struct + field validation (§5) |

---

## 10. What NOT to change

- Backend authentication logic (password hashing, session creation, etc.)
- CSRF token generation
- Redirect behaviour after successful login
- Any middleware or route protection
- The `/forgot-password` endpoint (out of scope)

---

## Reference

- Design prototype: `ui_kits/proviant/Login v2.html` — open in browser to see the full interactive implementation
- Token source: `colors_and_type.css`
- Full design system: `proviant-design-system.html`
- Full audit: `DESIGN_SYSTEM_AUDIT.md`

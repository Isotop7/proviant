# Page Override: User Settings (`/web/user/settings`)

> Overrides apply on top of `design-system/MASTER.md`. Only differences from Master are listed.

---

## Priority Focus

A settings page is task-oriented: users arrive with a goal (change email, rename household, invite someone). Layout must group related controls tightly, surface destructive actions with clear visual warning, and give unambiguous async feedback on every form submit.

---

## Page Score (current state as of 2026-04-17): 6 / 10

### What works
- Feature completeness — all flows (invite, apply, leave, create, notify) are present
- Bootstrap Icons used consistently — no emoji icons
- Admin-gated sections use `{{ if .IsAdmin }}` guards correctly
- Responsive grid usage (col / col-auto rows)
- Notification Settings: list-group toggle rows with `role="switch"` + `visually-hidden` labels
- Notification Settings: inline progressive disclosure — each channel expands its config directly below its row
- Notification Settings: conceptual split into **Delivery Channels** and **Scheduled Reports**

### Issues found

| # | Severity | Rule violated | Description |
|---|----------|---------------|-------------|
| 1 | **Critical** | `form-labels` (MASTER §5 Forms) | `<span class="input-group-text">` is **not** a `<label>`. Screen readers won't announce the label when the input receives focus. Affects Username, Mail Address, Password. |
| 2 | **Critical** | `heading-hierarchy` (UX §1) | Page heading is `h2` (should be `h1`); sections jump to `h4`, then `h5` — the `h3` level is skipped entirely. |
| 3 | **High** | `card-bg` surface pattern (MASTER §1) | Sections float directly on `#F5FAF5` body background. Card surfaces (`#FFFFFF`) should wrap each logical section to create depth and grouping. |
| 4 | **High** | Inline Alert misuse (MASTER §5 Popups) | `#updateAlert`, `#passwordAlert`, `#notificationAlert` are used for page-level async results — these must use `proviant.showFeedback()`. Inline alerts are for field-level errors only. |
| 5 | **High** | `destructive-emphasis` (UX §8) | "Leave Household" sits inline between neutral action rows with no visual separation or danger zone container. |
| 6 | **Medium** | `field-grouping` + `progressive-disclosure` (UX §8) | The Household section contains 9 sub-features (current info, rename, members, leave, create, apply, pending requests, outgoing applications, invitations) rendered as a flat list of rows — cognitively overwhelming. |
| 7 | **Medium** | `spacing-scale` (MASTER §3) | Section spacing is inconsistent: mix of `g-5 p-3`, `gx-5 px-3 mb-3`, `gx-5 p-3 my-3`. No systematic 8pt rhythm. |
| 8 | **Medium** | `required-indicators` (MASTER §5 Forms) | Required fields are not marked. |
| 9 | **Low** | `cursor-pointer` (MASTER §9 checklist) | Not applied to interactive elements. |
| 10 | **Low** | `submit-feedback` (UX §8) | Buttons do not disable + show spinner during async operations. |

---

## Recommended Layout Structure

### Page anatomy

```
<h1>User Settings</h1>  ← page title (h1, not h2)
<span class="badge bg-primary">ID: #{{ .User.ID }}</span>

[card] Personal Details          ← most frequent edit; above the fold
  ├── <label> Username
  ├── <label> Email
  └── [btn-outline-primary] Update  ← in card-header, right-aligned

[card] Password                  ← same "account identity" group as Personal Details
  ├── <label> New password
  ├── <label> Confirm password
  └── [btn-sm btn-outline-primary] "Update"  ← in card-header, right-aligned (same pattern as Personal Details)

[card] Notification Settings     ← preference, secondary to identity
  ├── [h3] Delivery Channels
  │   ├── list-group row: Email toggle
  │   │   └── [if emailEnabled] #emailSettings (bg-light row)
  │   │       ├── registered address confirmation + link to Personal Details
  │   │       └── Expiry Threshold number input
  │   ├── list-group row: ntfy.sh toggle
  │   │   └── [if ntfyEnabled] #ntfySettings (bg-light row)
  │   │       ├── ntfy.sh URL (required)
  │   │       ├── Topic (required)
  │   │       └── Access Token (optional)
  └── [h3] Scheduled Reports
      └── list-group row: Monthly Waste Report toggle
          └── [if monthlyWasteReportEnabled] #monthlyWasteReportSettings (bg-light row)
              └── bullet summary of what the report covers

[card] Household                 ← most complex; users scroll here deliberately
  ├── Current household info (elevated bg-light block)
  ├── [if IsAdmin] Rename section
  ├── [if Members] Members list
  ├── ─── Join / Invite ───────────────────────────────
  ├── Apply to join a household
  ├── [if MyApplications] Your pending applications
  ├── Invite by email
  ├── [if Invitations] Sent invitations
  ├── [if IsAdmin && PendingApplications] Pending join requests
  ├── ─── Danger zone ──────────────────────────────────
  ├── Create New Household   ← destructive-ish, separates user from current household
  └── Leave Household        ← danger, visually separated at bottom
```

**Rationale (`content-priority` rule):** Personal Details and Password are grouped first because they share the "account identity" concern and are the most frequently edited. Notification Settings sits in the middle as a personal preference. Household is last — it is the most complex section with the most sub-features and the Danger Zone; users who need it scroll deliberately, while everyday users are not slowed down by it.

### Card structure template

```html
<div class="card border-0 shadow-sm mb-4">
  <div class="card-header bg-white d-flex justify-content-between align-items-center py-3 px-4">
    <h2 class="h5 mb-0 fw-semibold">Section Title</h2>
    <!-- optional: action button — always btn-sm btn-outline-primary, label "Update", right-aligned in header -->
    <button type="button" class="btn btn-sm btn-outline-primary">Update</button>
  </div>
  <div class="card-body px-4 py-4">
    <!-- content -->
  </div>
</div>
```

> Use `h2` for card section headings (visually styled as `.h5`), `h3` for sub-sections within a card.

**Card action button rule:** When a card has a single primary submit action, place it as `btn-sm btn-outline-primary` in the `card-header`, right-aligned. Label it **"Update"** — never "Save", "Submit", or "Update [Section Name]". Cards without a primary action (e.g. Household) omit the button from the header entirely.

### Notification Settings — toggle row pattern (baseline)

Each notification channel or report is a `list-group-item` row with title + description on the left, `form-check form-switch` on the right. When the toggle is enabled, the next sibling `list-group-item.bg-light` expands inline (remove `d-none`); when disabled it collapses.

```html
<div class="list-group list-group-flush border rounded-3">
  <!-- Toggle row -->
  <div class="list-group-item px-3 py-3">
    <div class="d-flex align-items-center justify-content-between gap-3">
      <div>
        <div class="fw-medium small">[Channel Name]</div>
        <div class="text-body-secondary" style="font-size:13px;">[One-line description]</div>
      </div>
      <div class="form-check form-switch mb-0 flex-shrink-0">
        <input class="form-check-input" type="checkbox" role="switch" id="toggle[Name]" [checked]>
        <label class="visually-hidden" for="toggle[Name]">[Channel Name]</label>
      </div>
    </div>
  </div>
  <!-- Expandable config row (hidden by default) -->
  <div id="[name]Settings" class="list-group-item px-3 py-3 bg-light [d-none if disabled]">
    <!-- channel-specific fields -->
  </div>
</div>
```

**Rules:**
- `role="switch"` is required on every `form-check-input` toggle — screen readers announce it correctly
- `visually-hidden` label on the input; the row title is the visible label
- Config row uses `bg-light` to visually distinguish it as a sub-level
- JS: one `toggle[Name]Settings()` function per channel, wired in the `change` event delegation block in `userSettings.js`
- **Do not** use a standalone section or card for channel-specific config — always inline below its toggle row

**Section split:**
- `h3.h6.fw-semibold` **Delivery Channels** — real-time alert routing (Email, ntfy.sh)
- `h3.h6.fw-semibold` **Scheduled Reports** — periodic digests (Monthly Waste Report, future reports)
- Expiry Threshold lives inside `#emailSettings` (applies to alert channels, not reports)

### Danger zone block

```html
<div class="mt-4 pt-4 border-top border-danger border-opacity-25">
  <h3 class="h6 text-danger mb-3">
    <i class="bi bi-exclamation-triangle me-1"></i>Danger Zone
  </h3>
  <!-- Leave + Create New Household actions here -->
</div>
```

### Correct form label pattern

```html
<!-- Replace input-group-text pseudo-labels with real label+input pairs -->
<div class="row g-3">
  <div class="col-md-6">
    <label for="inputUsername" class="form-label fw-medium">Username <span class="text-danger" aria-hidden="true">*</span></label>
    <input type="text" class="form-control" id="inputUsername"
           value="{{ .User.Username }}" aria-required="true" autocomplete="username">
    <div class="invalid-feedback"></div>
  </div>
  <div class="col-md-6">
    <label for="inputMailAddress" class="form-label fw-medium">Email Address <span class="text-danger" aria-hidden="true">*</span></label>
    <input type="email" class="form-control" id="inputMailAddress"
           value="{{ .User.MailAddress }}" aria-required="true" autocomplete="email">
    <div class="invalid-feedback"></div>
  </div>
</div>
```

### Async feedback rule

**Do not** use `alert alert-danger fade` divs for async POST results.
Use the global feedback modal instead:

```js
// success
proviant.showFeedback('success', 'Saved', 'Personal details updated successfully.');
// error
proviant.showFeedback('danger', 'Error', data.message || 'Could not save changes.');
```

Reserve `alert-danger` inline divs for field validation errors only, placed directly below the offending field.

---

## Spacing Rules (page-specific)

| Context | Value |
|---------|-------|
| Between major sections (cards) | `mb-4` (24px) |
| Card inner padding | `px-4 py-4` (24px) |
| Between form fields | `g-3` (16px gap) |
| Between sub-sections within a card | `mt-4 pt-4 border-top` |
| Inline input-group gaps | `mb-3` (16px) |

---

## Heading Hierarchy

| Level | Element | Visual class | Usage |
|-------|---------|--------------|-------|
| `h1` | Page title "User Settings" | `h2` (visually smaller) | Once, at top |
| `h2` | Section card titles | `.h5 fw-semibold` | Personal Details, Household, etc. |
| `h3` | Sub-section within a card | `.h6 fw-medium` | Members, Pending Requests, Danger Zone |

---

## Household Section: Information Architecture

The household section is the most complex area. Apply progressive disclosure:

1. **Always visible**: current household badge + name + admin badge
2. **Admin-only, always visible**: Rename input
3. **Collapsible group "Members"**: member list — expanded by default on desktop, collapsed on mobile
4. **Tabbed or separated group "Join & Invite"**: Apply, Your Applications, Invite, Sent Invitations
5. **Admin-only, below separator "Requests"**: Pending join requests
6. **Danger zone (bottom, visually separated)**: Create New / Leave

This prevents cognitive overload by surfacing everyday info (current household, members) and hiding edge-case flows (join, invite, leave) below a visual separator.

---

## Color Deviations (none — follow MASTER)

All color decisions follow `MASTER.md §1`. No page-specific overrides needed.

---

## Component Checklist (userSettings page)

- [ ] Page title uses `<h1>` (visually `.h2` or similar, not `.display-*`)
- [ ] All form inputs have `<label for="...">` — no `input-group-text` pseudo-labels
- [ ] Required fields marked with `*` + `text-danger` + `aria-required="true"`
- [ ] Each major section wrapped in `card border-0 shadow-sm`
- [ ] Async results use `proviant.showFeedback()` — not inline alert divs
- [ ] Inline `invalid-feedback` divs present below each validated field
- [ ] "Leave Household" and "Create New Household" inside a Danger Zone block
- [ ] Buttons disable + show spinner during async operations
- [x] Notification toggles use `role="switch"` + `visually-hidden` label
- [x] Each channel's config expands inline as `list-group-item bg-light` directly below its toggle row
- [x] Notification Settings split into **Delivery Channels** and **Scheduled Reports** sub-sections
- [x] Expiry Threshold lives inside `#emailSettings` (not a standalone section)
- [x] ntfy.sh config block hidden by default, revealed on toggle
- [ ] `cursor-pointer` on all `<button>` and `<a>` elements that lack it via CSS
- [ ] `autocomplete` attributes on Username (`username`), Email (`email`), Password (`new-password`)
- [ ] `aria-required="true"` on required fields
- [ ] `aria-live="polite"` on alert regions or use `role="alert"` for dynamic errors
- [ ] Household member "Remove" buttons have `aria-label="Remove [username]"` (not generic "Remove member")

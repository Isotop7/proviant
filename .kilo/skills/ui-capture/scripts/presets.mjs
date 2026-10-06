// Preset table for the ui-capture skill.
//
// Lives in its own module so uicapture.mjs can build its `--help` preset list
// from the same data browser-capture.mjs shoots. A hand-copied list drifts:
// nine implemented presets were missing from --help the last time it was
// written out by hand.
//
// Every preset is a real route (src/router/router.go:431-466). `ready` is the
// selector that proves the page finished rendering, not just that it loaded —
// Chart.js and the fetch-driven tiles all paint after DOMContentLoaded.

export const PRESETS = {
  'login': { path: '/web/auth', login: false, ready: '#authForm' },
  'dashboard': { path: '/web', login: true, ready: '#dashboard-status:has-text("Dashboard loaded")' },
  'products': { path: '/web/products', login: true, ready: '#productRows .list-row' },
  'add-product': { path: '/web/products/scan', login: true, ready: '#viewfinder' },
  'receipt-scan': { path: '/web/products/scan-receipt', login: true, ready: '#receiptUploadForm' },
  'recipes': {
    path: '/web/recipes',
    login: true,
    // Either the suggestion cards landed or the error container became visible;
    // waiting on the container alone would screenshot the pre-fetch blank state.
    ready: '#recipe-suggestions-container .recipe-card, #recipe-error-container:not(.d-none)',
  },
  'waste': { path: '/web/waste-analytics', login: true, ready: '#waste-status:has-text("Waste analytics loaded")' },
  'shopping-list': { path: '/web/shopping-list', login: true, ready: '#shoppingListContainer .sl-list-row' },
  'settings': { path: '/web/user/settings', login: true, ready: 'h1:has-text("User Settings")' },
  'user': { path: '/web/user', login: true, ready: 'h3:has-text("User")' },
  'onboarding': { path: '/web/onboarding', login: true, ready: '#step1' },
  'product-detail': { path: '/web/products/1/view', login: true, ready: 'h1.page-header-title' },
  'product-edit': { path: '/web/products/1/edit', login: true, ready: '#editProductForm' },

  // Product filter tabs. The seed leaves product 7 past its expiry and writes
  // ~6 months of archived rows, so both of these lists are populated.
  'products-expired': { path: '/web/products?status=expired', login: true, ready: '#productRows .list-row' },
  'products-consumed': { path: '/web/products?status=archived', login: true, ready: '#productRows .list-row' },
  'products-grid': { path: '/web/products?view=grid', login: true, ready: '.product-card' },

  // Public, token-gated pages. Without a token they render an error card, which
  // documents nothing; seed.go fixes DemoInviteToken / DemoVerifyToken /
  // DemoUnsubscribeToken so each page reaches its populated branch.
  'forgot-password': { path: '/web/forgot-password', login: false, ready: '#forgot-password-form' },
  'reset-password': { path: '/web/reset-password?token=demo-verify-token', login: false, ready: '#reset-password-form' },
  'invite-accept': { path: '/web/invite/accept?token=demo-invite-token', login: false, ready: '.alert-info' },
  // Success card only. verifyEmail.js renders .alert-danger with "Verification
  // Failed" when the POST rejects, and accepting it here would photograph the
  // failure branch at exit 0 if the token ever drifts from seed.go. Timing out
  // instead fails the run loudly, like the other token presets.
  'verify-email': { path: '/web/verify-email?token=demo-verify-token', login: false, ready: '.alert-success' },
  'unsubscribe': { path: '/web/unsubscribe?token=demo-unsubscribe-token', login: false, ready: 'h1:has-text("Unsubscribed")' },

  // Catch-all handler (router.go:474) renders error.tmpl for any unknown /web path.
  'not-found': { path: '/web/this-page-does-not-exist', login: true, ready: '.empty-title:has-text("Something went wrong")' },
};

// Order the `all` walk visits. Deliberately not Object.keys(PRESETS): this is a
// narrative tour, and it skips the token-gated and error pages.
export const ALL_WALK = [
  'dashboard',
  'products',
  'product-detail',
  'product-edit',
  'add-product',
  'products-expired',
  'products-consumed',
  'recipes',
  'waste',
  'shopping-list',
  'settings',
  'user',
];

// --help renders this, so the documented set can never lag the implemented one.
export function presetNames() {
  return Object.keys(PRESETS);
}
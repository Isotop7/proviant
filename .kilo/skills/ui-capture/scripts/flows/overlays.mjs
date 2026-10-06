// Overlay states that no single-route preset can reach: modals, dropdowns and
// the shared feedback dialog.
//
//   node .kilo/skills/ui-capture/scripts/uicapture.mjs \
//     --script .kilo/skills/ui-capture/scripts/flows/overlays.mjs
//
// The runner logs in as demo/demo before the flow starts.

export default async function flow({ page, shot, goto, settle }) {
  // ── Products: add-product modal, manual entry then the camera tab ──────────
  // Manual Input is the default tab (products.tmpl:430), so the first still is
  // the manual form. The camera tab then shows the degraded headless state —
  // html5-qrcode finds no camera, which is the honest capture.
  await goto('/web/products');
  await settle('#productRows .list-row');
  await page.click('#btnOpenAddProductModal');
  await settle('#addProductModal.show', 'visible');
  await shot('products-add-modal-manual');

  // The camera tab cannot work headless — html5-qrcode raises NotFoundError and
  // the app reports it through the shared feedback dialog. That stack is a real
  // user-facing state, so it gets its own shot before the flow moves on.
  await page.click('#tabCamera');
  await settle('#proviantFeedbackModal.show', 'visible');
  await shot('products-add-modal-camera-unavailable');
  await page.click('#proviantFeedbackBtn');
  await settle('#proviantFeedbackModal:not(.show)');
  await page.click('#addProductModal .btn-modal-close');
  await settle('#addProductModal:not(.show)');

  // ── Products: export dropdown ──────────────────────────────────────────────
  await page.click('[aria-label="Product export options bar"] .dropdown-toggle');
  await settle('.export-menu.show', 'visible');
  await shot('products-export-menu');
  await page.keyboard.press('Escape');
  await settle('.export-menu:not(.show)');

  // ── Products: cook modal, reached by selecting rows then the bulk button ───
  const rows = page.locator('#productRows input.row-checkbox');
  await rows.nth(0).check();
  await rows.nth(1).check();
  await settle('#bulkCount:has-text("2 selected")');
  await shot('products-bulk-selection');

  await page.click('[data-bulk-action="cook"]');
  await settle('#cookModal.show', 'visible');
  await settle('#cookModalItems .cook-item-row');
  await shot('products-cook-modal');
  await page.keyboard.press('Escape');
  await settle('#cookModal:not(.show)');

  // ── Products: destructive confirm dialog ───────────────────────────────────
  await page.click('[data-bulk-action="delete"]');
  await settle('#proviantConfirmModal.show', 'visible');
  await shot('products-confirm-waste');
  await page.click('#proviantConfirmModal [data-bs-dismiss="modal"]');
  await settle('#proviantConfirmModal:not(.show)');

  // ── Notifications dropdown (sidebar bell) ──────────────────────────────────
  await page.click('button[aria-label="Notifications"]');
  await settle('#notification-dropdown.show', 'visible');
  await settle('#notification-items .list-group-item, #notification-items .dropdown-item, #notification-items .text-secondary-custom:not(:has-text("Loading"))');
  await shot('nav-notifications-dropdown');
  await page.keyboard.press('Escape');

  // ── Product detail: add-to-shopping-list restock suggestion ─────────────────
  // The seeded archive gives products 1, 2 and 14 two consumed samples each, so
  // the suggestion endpoint answers with a real quantity rather than falling
  // back to a direct add.
  await goto('/web/products/1/view');
  await settle('h1.page-header-title');
  await page.click('.btn-add-to-shopping-list');
  await settle('#restockSuggestionModal.show', 'visible');
  await shot('product-restock-suggestion');
  await page.keyboard.press('Escape');
  await settle('#restockSuggestionModal:not(.show)');

  // ── Settings: create API token modal ───────────────────────────────────────
  await goto('/web/user/settings');
  await settle('h1:has-text("User Settings")');
  await page.click('#btnCreatePAT');
  await settle('#createPatModal.show', 'visible');
  await shot('settings-create-token-modal');
  await page.keyboard.press('Escape');
  await settle('#createPatModal:not(.show)');
}
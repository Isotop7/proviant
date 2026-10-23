// Overlay states that no single-route preset can reach: modals, dropdowns and
// the shared feedback dialog.
//
//   node .agents/skills/ui-capture/scripts/uicapture.mjs \
//     --script .agents/skills/ui-capture/scripts/flows/overlays.mjs
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
  await settle('#addProductModal[open]', 'visible');
  await shot('products-add-modal-manual');

  // The camera tab cannot work headless — html5-qrcode finds no camera, so the
  // app reports it as a toast. That stack is a real user-facing state, so it
  // gets its own shot before the flow moves on. The camera section may or may
  // not have opened its own dialog on top, so Escape is the reliable way out:
  // it closes the topmost one.
  await page.click('#tabCamera');
  await settle('#proviantToastStack .toast', 'visible');
  await shot('products-add-modal-camera-unavailable');
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  await settle('#addProductModal:not([open])');

  // ── Products: export menu ──────────────────────────────────────────────────
  await page.click('[aria-label="Product export options bar"] [data-bs-toggle="menu"]');
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
  await settle('#cookModal[open]', 'visible');
  await settle('#cookModalItems .cook-item-row');
  await shot('products-cook-modal');
  await page.keyboard.press('Escape');
  await settle('#cookModal:not([open])');

  // ── Products: destructive confirm dialog ───────────────────────────────────
  await page.click('[data-bulk-action="delete"]');
  await settle('#proviantConfirmModal[open]', 'visible');
  await shot('products-confirm-waste');
  // The footer button, not the header close: an error toast can sit on top of
  // the header's close button.
  await page.click('#proviantConfirmModal .dialog-footer [data-bs-dismiss="dialog"]');
  await settle('#proviantConfirmModal:not([open])');

  // ── Notifications menu (sidebar bell) ──────────────────────────────────────
  // An error toast stays until it is dismissed, and it sits right on top of
  // the notification bell. Clear it first.
  const errorToast = page.locator('#proviantToastStack .toast [data-bs-dismiss="toast"]');
  if (await errorToast.count()) {
    await errorToast.first().click();
    await settle('#proviantToastStack .toast', 'hidden');
  }
  await page.click('button[aria-label="Notifications"]');
  await settle('#notification-dropdown.show', 'visible');
  await settle('#notification-items .notification-entry');
  await shot('nav-notifications-dropdown');
  await page.keyboard.press('Escape');

  // ── Product edit: add-to-shopping-list restock suggestion ───────────────────
  // The seeded archive gives products 1, 2 and 14 two consumed samples each, so
  // the suggestion endpoint answers with a real quantity rather than falling
  // back to a direct add.
  await goto('/web/products/1/edit');
  await settle('#editProductForm');
  await page.click('.btn-add-to-shopping-list');
  await settle('#restockSuggestionModal[open]', 'visible');
  await shot('product-restock-suggestion');
  await page.keyboard.press('Escape');
  await settle('#restockSuggestionModal:not([open])');

  // ── Settings: create API token dialog ───────────────────────────────────────
  await goto('/web/user/settings');
  await settle('h1:has-text("User Settings")');
  await page.click('#btnCreatePAT');
  await settle('#createPatModal[open]', 'visible');
  await shot('settings-create-token-modal');
  await page.keyboard.press('Escape');
  await settle('#createPatModal:not([open])');
}
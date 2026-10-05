// Mobile-only overlay: the products filter sheet.
//
//   node .kilo/skills/ui-capture/scripts/uicapture.mjs \
//     --preset all --viewport mobile --script .kilo/skills/ui-capture/scripts/flows/mobile-filters.mjs
//
// The trigger is inside a `d-block d-md-none` dropdown (products.tmpl:91), so
// this flow only produces a shot at `--viewport mobile`. At desktop width the
// dropdown toggle is display:none and the click would time out.

export default async function flow({ page, shot, goto, settle, viewport }) {
  if (viewport.width >= 768) {
    throw new Error('mobile-filters.mjs needs --viewport mobile; the filter sheet trigger is hidden at md and up');
  }

  await goto('/web/products');
  await settle('#productRows .list-row');

  // The "..." dropdown in the toolbar, then the Filters entry inside it.
  await page.click('#toolbarRight .dropdown-toggle');
  await settle('#mobileExportMenu.show', 'visible');
  await shot('products-mobile-actions-menu');

  await page.click('#mobileExportMenu [data-bs-target="#mobileFiltersOffcanvas"]');
  await settle('#mobileFiltersOffcanvas.show', 'visible');
  await shot('products-mobile-filter-sheet');
}
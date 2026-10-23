// Mobile-only overlay: the products filter drawer.
//
//   node .agents/skills/ui-capture/scripts/uicapture.mjs \
//     --preset all --viewport mobile --script .agents/skills/ui-capture/scripts/flows/mobile-filters.mjs
//
// The trigger is inside a `d-block md:d-none` menu (products.tmpl:91), so
// this flow only produces a shot at `--viewport mobile`. At desktop width the
// menu toggle is display:none and the click would time out.

export default async function flow({ page, shot, goto, settle, viewport }) {
  if (viewport.width >= 768) {
    throw new Error('mobile-filters.mjs needs --viewport mobile; the filter sheet trigger is hidden at md and up');
  }

  await goto('/web/products');
  await settle('#productRows .list-row');

  // The "..." menu in the toolbar, then the Filters entry inside it.
  await page.click('#toolbarRight [data-bs-toggle="menu"]');
  await settle('#mobileExportMenu.show', 'visible');
  await shot('products-mobile-actions-menu');

  await page.click('#mobileExportMenu [data-bs-target="#mobileFiltersOffcanvas"]');
  await settle('#mobileFiltersOffcanvas[open]', 'visible');
  await shot('products-mobile-filter-sheet');
}
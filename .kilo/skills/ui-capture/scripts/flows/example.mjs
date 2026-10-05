// Reference custom flow for the ui-capture skill.
//
// Run it with:
//
//   node .kilo/skills/ui-capture/scripts/uicapture.mjs \
//     --script .kilo/skills/ui-capture/scripts/flows/example.mjs
//
// The flow receives helpers only and must import nothing, so the same file
// runs unchanged under the Playwright container's Node and under host Node.
//
//   { page, shot, goto, settle, baseURL, viewport, login, clip }
//
//   shot(name)        settle, then write <NN>-<name>.png into the out dir
//   goto(path)        navigate to a route on the already-authenticated session
//   settle(selector)  wait for a selector that proves the page finished rendering
//   login()           re-authenticate (the runner already logs in before a flow)

export default async function flow({ page, shot, goto, settle }) {
  // The products list, logged in as the seeded demo user.
  await goto('/web/products');
  await settle('#productRows .list-row');
  await shot('products');

  // Open the add-product modal from the toolbar — a state that no single-route
  // preset can reach, which is exactly what a flow is for.
  await page.click('#btnOpenAddProductModal');
  await settle('#addProductModal.show', 'visible');
  await shot('add-product-modal');

  // Manual-entry tab, the capturable surface of the scanner headless.
  await page.click('#tabManualInput');
  await shot('add-product-manual-entry');

  // Close and move on to the waste analytics charts.
  await page.keyboard.press('Escape');
  await goto('/web/waste-analytics');
  await settle('#waste-status:has-text("Waste analytics loaded")');
  await shot('waste-analytics');
}
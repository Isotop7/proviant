// The "frequently bought" strip in the add-product dialog: the chips the
// household's staples fill the dialog with, and the form a tap fills.
//
//   node .agents/skills/ui-capture/scripts/uicapture.mjs \
//     --script .agents/skills/ui-capture/scripts/flows/frequent-products.mjs
//
// The runner logs in as demo/demo before the flow starts.

export default async function flow({ page, shot, goto, settle }) {
  await goto('/web/products');
  await settle('#productRows .list-row');
  await page.click('#btnOpenAddProductModal');
  await settle('#addProductModal[open]', 'visible');
  await settle('#frequentProducts:not(.d-none)', 'visible');
  await settle('#frequentProductsList .frequent-product');
  await shot('products-frequently-bought-strip');

  await page.click('#frequentProductsList .frequent-product');
  await settle('#productData:not(.d-none)', 'visible');
  await settle('#modalStepForm', 'visible');
  await shot('products-frequently-bought-filled');
}

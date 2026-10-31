// The partial consume/waste dialog: one action surface with two outcomes.
//
//   node .agents/skills/ui-capture/scripts/uicapture.mjs \
//     --script .agents/skills/ui-capture/scripts/flows/partial-action.mjs
//
// The runner logs in as demo/demo before the flow starts.

export default async function flow({ page, shot, goto, settle }) {
  await goto('/web/products');
  await settle('#productRows .list-row');
  await page.click('#productRows .btn-partial-action');
  await settle('#partialActionModal[open]', 'visible');
  await settle('#partialActionStock:not(:empty)');
  await shot('partial-action-modal-list');
  await page.keyboard.press('Escape');
  await settle('#partialActionModal:not([open])');

  await goto('/web/products?view=grid');
  await settle('.product-card');
  await page.click('.product-card .btn-partial-action');
  await settle('#partialActionModal[open]', 'visible');
  await settle('#partialActionStock:not(:empty)');
  await shot('partial-action-modal-grid');
  await page.keyboard.press('Escape');
  await settle('#partialActionModal:not([open])');
}

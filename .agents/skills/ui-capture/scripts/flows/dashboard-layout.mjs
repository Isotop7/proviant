// Custom flow: switch the dashboard into the edit mode, drag a tile and
// resize it, then reload to prove the arrangement survives on the server.
//
//   node .agents/skills/ui-capture/scripts/uicapture.mjs \
//     --script .agents/skills/ui-capture/scripts/flows/dashboard-layout.mjs

export default async function flow({ page, shot, goto, settle }) {
  await goto('/web');
  await settle('#dashboard-edit-layout');

  // The grips and the reset button belong to the edit mode.
  const viewMode = await page.evaluate(() => ({
    gripsHidden: [...document.querySelectorAll('.bento__handle')].every((h) => h.hidden),
    resetHidden: document.getElementById('dashboard-reset-layout').hidden,
  }));
  await shot('dashboard-view-mode');

  await page.click('#dashboard-edit-layout');
  await settle('[data-tile="hero"] .bento__handle--move', 'visible');

  const editMode = await page.evaluate(() => ({
    gripsVisible: [...document.querySelectorAll('.bento__handle--move')].every((h) => !h.hidden),
    resetVisible: !document.getElementById('dashboard-reset-layout').hidden,
    label: document.querySelector('.dashboard-tools__label').textContent.trim(),
  }));
  await shot('dashboard-edit-mode');

  const order = () =>
    page.$$eval('[data-tile]', (tiles) =>
      tiles
        .map((tile) => ({
          id: tile.dataset.tile,
          order: Number(tile.style.getPropertyValue('--tile-order')),
        }))
        .sort((a, b) => a.order - b.order)
        .map((tile) => tile.id),
    );

  const before = await order();

  // Drag the "last added" tile by its grip onto the first slot.
  const grip = await page.$('[data-tile="last-added"] .bento__handle--move');
  const gripBox = await grip.boundingBox();
  const targetBox = await (await page.$('[data-tile="hero"]')).boundingBox();
  await page.mouse.move(gripBox.x + gripBox.width / 2, gripBox.y + gripBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + 20, targetBox.y + 10, { steps: 12 });

  // The landing shadow marks the slot the tile will take.
  const landing = await page.evaluate(() => {
    const slot = document.querySelector('.bento__slot');
    const grid = document.getElementById('dashboard');
    if (!slot) return null;
    const shadow = slot.getBoundingClientRect();
    const bounds = grid.getBoundingClientRect();
    return {
      visible: slot.classList.contains('bento__slot--visible'),
      width: Math.round(shadow.width),
      height: Math.round(shadow.height),
      insideGrid:
        shadow.left >= bounds.left - 1 &&
        shadow.top >= bounds.top - 1 &&
        shadow.right <= bounds.right + 1 &&
        shadow.bottom <= bounds.bottom + 1,
    };
  });

  await page.mouse.up();
  await page.waitForTimeout(400);
  await shot('dashboard-after-drag');

  const after = await order();
  const slotGone = await page.evaluate(() => document.querySelector('.bento__slot') === null);

  // Resize the hero tile with the corner grip: one column wider, one row taller.
  const resize = await page.$('[data-tile="hero"] .bento__handle--resize');
  const resizeBox = await resize.boundingBox();
  const heroBox = await (await page.$('[data-tile="hero"]')).boundingBox();
  await page.mouse.move(resizeBox.x + resizeBox.width / 2, resizeBox.y + resizeBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    resizeBox.x + resizeBox.width / 2 + heroBox.width / 3,
    resizeBox.y + resizeBox.height / 2 + 120,
    { steps: 12 },
  );
  await page.mouse.up();
  await page.waitForTimeout(400);
  await shot('dashboard-after-resize');

  const sizes = await page.$$eval('[data-tile="hero"]', (tiles) =>
    tiles.map((tile) => ({
      w: tile.style.getPropertyValue('--tile-w'),
      h: tile.style.getPropertyValue('--tile-h'),
    })),
  );

  // The arrangement lives on the server: a fresh load has to show it again.
  await page.reload();
  await settle('#dashboard-edit-layout');
  const reloaded = await order();
  const reloadedSizes = await page.$$eval('[data-tile="hero"]', (tiles) =>
    tiles.map((tile) => ({
      w: tile.style.getPropertyValue('--tile-w'),
      h: tile.style.getPropertyValue('--tile-h'),
    })),
  );
  await shot('dashboard-after-reload');

  console.log(
    JSON.stringify({ viewMode, editMode, landing, slotGone, before, after, sizes, reloaded, reloadedSizes }, null, 2),
  );
}

// Custom flow: drag a dashboard tile and resize it, then reload to prove the
// arrangement survives on the server.
//
//   node .agents/skills/ui-capture/scripts/uicapture.mjs \
//     --script .agents/skills/ui-capture/scripts/flows/dashboard-layout.mjs

export default async function flow({ page, shot, goto, settle }) {
  await goto('/web');
  await settle('[data-tile="hero"] .bento__handle--move');
  await shot('dashboard-before');

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
  await page.mouse.up();
  await page.waitForTimeout(400);
  await shot('dashboard-after-drag');

  const after = await order();

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
  await settle('[data-tile="hero"] .bento__handle--move');
  const reloaded = await order();
  const reloadedSizes = await page.$$eval('[data-tile="hero"]', (tiles) =>
    tiles.map((tile) => ({
      w: tile.style.getPropertyValue('--tile-w'),
      h: tile.style.getPropertyValue('--tile-h'),
    })),
  );
  await shot('dashboard-after-reload');

  console.log(JSON.stringify({ before, after, sizes, reloaded, reloadedSizes }, null, 2));
}

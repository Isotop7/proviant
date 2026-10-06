export default async function flow({ page, shot, goto, settle }) {
  await goto('/web');
  await settle('#dashboard-status:has-text("Dashboard loaded")');

  const metrics = await page.evaluate(() => {
    const img = document.querySelector('.pv-sidebar .brand .brand-logo');
    const span = document.querySelector('.pv-sidebar .brand .brand-name');
    const brand = document.querySelector('.pv-sidebar .brand');
    const cs = getComputedStyle(span);
    const ics = getComputedStyle(img);
    const bcs = getComputedStyle(brand);
    const r = (el) => {
      const b = el.getBoundingClientRect();
      return { top: b.top, bottom: b.bottom, left: b.left, height: b.height, centerY: (b.top + b.bottom) / 2 };
    };
    return {
      brand: { display: bcs.display, alignItems: bcs.alignItems, rect: r(brand) },
      img: { display: ics.display, transform: ics.transform, rect: r(img) },
      span: { display: cs.display, lineHeight: cs.lineHeight, fontSize: cs.fontSize, rect: r(span) },
      centerDelta: r(img).centerY - r(span).centerY
    };
  });
  console.log('[metrics]', JSON.stringify(metrics, null, 2));

  const brand = page.locator('.pv-sidebar .brand');
  await brand.screenshot({ path: '/tmp/brand-zoom.png' });
  await shot('brand-full');
}

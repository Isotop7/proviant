export default async function flow({ page, shot, goto, settle }) {
  // Light mode — dashboard
  await goto('/web');
  await settle('#dashboard-status:has-text("Dashboard loaded")');
  await shot('light-dashboard');

  // Light mode — settings (card headers)
  await goto('/web/user/settings');
  await settle('h1:has-text("User Settings")');
  await shot('light-settings');

  // Switch to dark mode via localStorage + attribute
  await page.evaluate(() => {
    localStorage.setItem('proviant_theme', 'dark');
    document.documentElement.setAttribute('data-bs-theme', 'dark');
  });

  // Dark mode — dashboard
  await goto('/web');
  await settle('#dashboard-status:has-text("Dashboard loaded")');
  await shot('dark-dashboard');

  // Dark mode — settings (card headers)
  await goto('/web/user/settings');
  await settle('h1:has-text("User Settings")');
  await shot('dark-settings');

  // Dark mode — products list
  await goto('/web/products');
  await settle('#productRows .list-row');
  await shot('dark-products');
}

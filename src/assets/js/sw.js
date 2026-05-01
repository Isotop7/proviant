'use strict';

const CACHE_NAME = 'proviant-v58';

// Static shell to pre-cache on install
const PRECACHE_URLS = [
  '/manifest.json',
  '/assets/css/main.css',
  '/assets/js/proviant.js',
  '/assets/js/bootstrap.bundle.min.js',
  '/assets/js/chart.umd.min.js',
  '/assets/js/main.js',
  '/assets/js/notifications.js',
  '/assets/icons/proviant_logo_192.png',
  '/assets/icons/proviant_logo_512.png',
  '/assets/icons/hero.png',
  '/assets/icons/favicon.png',
];

// Minimal offline fallback page (no asset dependencies)
const OFFLINE_PAGE = `<!doctype html>
<html lang="en" data-bs-theme="light">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>proviant — offline</title>
  <style>
    *{box-sizing:border-box;margin:0;padding:0}
    body{
      background:#212529;color:#e2e8f0;
      font-family:system-ui,-apple-system,sans-serif;
      display:flex;align-items:center;justify-content:center;
      min-height:100dvh;text-align:center;padding:2rem;
    }
    .icon{font-size:5rem;line-height:1;margin-bottom:1.5rem}
    h1{font-size:1.75rem;font-weight:700;margin-bottom:.75rem}
    p{color:#94a3b8;margin-bottom:1.5rem;line-height:1.6}
    button{
      background:#4e9ab8;color:#fff;border:none;border-radius:.5rem;
      padding:.75rem 1.5rem;font-size:1rem;font-weight:500;cursor:pointer;
    }
    button:hover{background:#3d87a3}
  </style>
</head>
<body>
  <div>
    <div class="icon" aria-hidden="true">
      <svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" fill="#4e9ab8" viewBox="0 0 16 16">
        <path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14m0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16"/>
        <path d="M7.002 11a1 1 0 1 1 2 0 1 1 0 0 1-2 0M7.1 4.995a.905.905 0 1 1 1.8 0l-.35 3.507a.552.552 0 0 1-1.1 0z"/>
      </svg>
    </div>
    <h1>You're offline</h1>
    <p>Proviant needs a connection to load your products.<br>Check your network and try again.</p>
    <button onclick="location.reload()">Try again</button>
  </div>
</body>
</html>`;

// ── Install: pre-cache static shell ──────────────────────────────────────────
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE_URLS))
  );
  self.skipWaiting();
});

// ── Activate: clean up old caches ────────────────────────────────────────────
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))
      )
    )
  );
  self.clients.claim();
});

// ── Fetch ─────────────────────────────────────────────────────────────────────
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Never intercept API calls, auth endpoints, or non-GET requests
  if (
    request.method !== 'GET' ||
    url.pathname.startsWith('/api/') ||
    url.pathname.startsWith('/auth/')
  ) {
    return;
  }

  // Network-first for JavaScript files to avoid stale cached scripts
  if (url.pathname.endsWith('.js')) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
          }
          return response;
        })
        .catch(() => caches.match(request))
    );
    return;
  }

  // Cache-first for other static assets — they are versioned via cache name.
  // Product images are barcode-keyed and effectively immutable per barcode.
  if (
    url.pathname.startsWith('/assets/') ||
    url.pathname.startsWith('/product-images/') ||
    url.pathname === '/manifest.json'
  ) {
    event.respondWith(
      caches.match(request).then(
        (cached) =>
          cached ||
          fetch(request).then((response) => {
            if (response.ok) {
              const clone = response.clone();
              caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
            }
            return response;
          })
      )
    );
    return;
  }

  // Network-first for all navigation — cache successful responses for offline fallback
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
          }
          return response;
        })
        .catch(() =>
          caches.match(request).then(
            (cached) =>
              cached ||
              new Response(OFFLINE_PAGE, {
                status: 200,
                headers: { 'Content-Type': 'text/html; charset=utf-8' },
              })
          )
        )
    );
  }
});

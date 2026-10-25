'use strict';

const CACHE_NAME = 'proviant-v169';

// The offline screen is a server-rendered page: the worker precaches it and
// serves the cached copy when a navigation cannot reach the network.
const OFFLINE_URL = '/offline';

const PRECACHE_URLS = [
  '/manifest.json',
  '/assets/css/main.css',
  '/assets/js/proviant.js',
  '/assets/js/offlineQueue.js',
  '/assets/js/theme.js',
  '/assets/js/bootstrap.bundle.min.js',
  '/assets/js/bootstrap-globals.js',
  '/assets/js/chart.umd.min.js',
  '/assets/js/main.js',
  '/assets/js/notifications.js',
  '/assets/icons/proviant_logo_192.png',
  '/assets/icons/proviant_logo_512.png',
  '/assets/icons/hero.png',
  '/assets/icons/favicon.png',
  OFFLINE_URL,
];

// The precache request for the offline page carries the browser's language,
// because the worker's own requests carry no Accept-Language of their own.
function precacheRequest(url) {
  if (url === OFFLINE_URL) {
    return new Request(url, {
      cache: 'no-cache',
      headers: { 'Accept-Language': navigator.language || 'en' },
    });
  }
  return new Request(url, { cache: 'no-cache' });
}

// ── Install: pre-cache static shell ──────────────────────────────────────────
// Precache fetches bypass the HTTP cache: its heuristic freshness can serve a
// pre-deploy main.css and pin stale bytes into a brand-new cache, defeating
// every CACHE_NAME bump until a hard reload.
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) =>
      cache.addAll(PRECACHE_URLS.map(precacheRequest))
    )
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

// ── Push: handle push notifications ──────────────────────────────────────────
self.addEventListener('push', function(event) {
  if (!event.data) return;
  const data = event.data.json();
  const title = data.title || 'Proviant';
  const options = {
    body: data.body || '',
    icon: '/assets/icons/proviant_logo_192.png',
    badge: '/assets/icons/proviant_logo_192.png',
    tag: data.tag || 'proviant-notification',
    data: data.url || '/',
  };
  event.waitUntil(
    self.registration.showNotification(title, options)
  );
});

// ── Notification Click: handle click on notification ─────────────────────────
self.addEventListener('notificationclick', function(event) {
  event.notification.close();
  event.waitUntil(
    clients.openWindow(event.notification.data || '/')
  );
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
            void caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
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
              void caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
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
            void caches.open(CACHE_NAME).then((cache) => cache.put(request, clone));
          }
          return response;
        })
        .catch(() =>
          caches.match(request).then(
            (cached) =>
              cached ||
              caches.match(OFFLINE_URL).then(
                (offlinePage) =>
                  offlinePage ||
                  new Response(
                    '<!doctype html><meta charset="utf-8"><title>proviant</title><p>Offline.</p>',
                    {
                      status: 503,
                      headers: { 'Content-Type': 'text/html; charset=utf-8' },
                    }
                  )
              )
          )
        )
    );
  }
});

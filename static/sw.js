// Homelab Dashboard — Service Worker
// Strategy: cache-first for static assets, network-first for dynamic routes

const CACHE_VERSION = 'homelab-v1';
const STATIC_CACHE  = CACHE_VERSION + '-static';
const DYNAMIC_CACHE = CACHE_VERSION + '-dynamic';

const STATIC_ASSETS = [
  '/',
  '/static/manifest.json',
];

const OFFLINE_HTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="theme-color" content="#030712">
  <title>Homelab — Offline</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      min-height: 100dvh;
      display: flex; flex-direction: column; align-items: center; justify-content: center;
      background: #020617; color: #94a3b8; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      text-align: center; padding: 2rem;
    }
    .icon { font-size: 3rem; margin-bottom: 1.5rem; }
    h1 { color: #f1f5f9; font-size: 1.25rem; font-weight: 600; margin-bottom: 0.5rem; }
    p  { font-size: 0.875rem; color: #64748b; margin-bottom: 2rem; }
    button {
      background: #0ea5e9; color: white; border: none; border-radius: 12px;
      padding: 0.75rem 1.5rem; font-size: 0.875rem; font-weight: 600;
      cursor: pointer; transition: opacity 0.2s;
    }
    button:hover { opacity: 0.85; }
  </style>
</head>
<body>
  <div class="icon">🏠</div>
  <h1>You're offline</h1>
  <p>Homelab Dashboard needs a connection to show live data.</p>
  <button onclick="location.reload()">Try again</button>
</body>
</html>`;

// ── Install ──────────────────────────────────────────
self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(STATIC_CACHE)
      .then(cache => cache.addAll(STATIC_ASSETS))
      .then(() => self.skipWaiting())
  );
});

// ── Activate ─────────────────────────────────────────
self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys().then(keys =>
      Promise.all(
        keys
          .filter(k => k !== STATIC_CACHE && k !== DYNAMIC_CACHE)
          .map(k => caches.delete(k))
      )
    ).then(() => self.clients.claim())
  );
});

// ── Fetch ─────────────────────────────────────────────
self.addEventListener('fetch', event => {
  const { request } = event;
  const url = new URL(request.url);

  // Only intercept same-origin requests
  if (url.origin !== self.location.origin) return;

  // API routes → network-first, no cache
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(networkOnly(request));
    return;
  }

  // Static assets → cache-first
  if (url.pathname.startsWith('/static/') || url.pathname.startsWith('/icons/')) {
    event.respondWith(cacheFirst(request));
    return;
  }

  // HTML pages → network-first with offline fallback
  event.respondWith(networkFirstWithFallback(request));
});

// ── Strategies ────────────────────────────────────────

async function cacheFirst(request) {
  const cached = await caches.match(request);
  if (cached) return cached;
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(STATIC_CACHE);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    return new Response('Not found', { status: 404 });
  }
}

async function networkFirstWithFallback(request) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(DYNAMIC_CACHE);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await caches.match(request);
    if (cached) return cached;
    return new Response(OFFLINE_HTML, {
      status: 200,
      headers: { 'Content-Type': 'text/html; charset=utf-8' },
    });
  }
}

async function networkOnly(request) {
  try {
    return await fetch(request);
  } catch {
    return new Response(JSON.stringify({ error: 'offline' }), {
      status: 503,
      headers: { 'Content-Type': 'application/json' },
    });
  }
}

// ── Background sync placeholder ───────────────────────
self.addEventListener('sync', event => {
  if (event.tag === 'homelab-refresh') {
    event.waitUntil(self.clients.matchAll().then(clients =>
      clients.forEach(c => c.postMessage({ type: 'SYNC_REFRESH' }))
    ));
  }
});

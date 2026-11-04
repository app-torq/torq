// torq service worker (specs/004-installable-pwa, research W2/W3).
//
// It keeps only this build's app shell on the device, for an instant start
// and the offline screen. Default-deny: it answers exactly two kinds of
// requests, same-origin GETs of shell files and page navigations outside
// /api/, and lets everything else go to the network untouched. API
// responses, photos, exports and anything else user-specific are never
// cached, including API routes added later.
//
// The build fills in SHELL and BUILD (vite.config.ts), so every build is a
// byte-different worker and browsers install it as an update.

const SHELL = __TORQ_SHELL__
const BUILD = __TORQ_BUILD__
const CACHE = 'torq-shell-' + BUILD
const shellPaths = new Set(SHELL)

self.addEventListener('install', (event) => {
  // No skipWaiting here: the page decides when to switch (research W5).
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(SHELL)))
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k.startsWith('torq-shell-') && k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') self.skipWaiting()
})

self.addEventListener('fetch', (event) => {
  const req = event.request
  if (req.method !== 'GET') return
  const url = new URL(req.url)
  if (url.origin !== self.location.origin) return
  if (url.pathname.startsWith('/api/') || url.pathname === '/healthz') return

  if (req.mode === 'navigate') {
    // Every app route is the same single page.
    event.respondWith(fromCache('/index.html', req))
    return
  }
  if (shellPaths.has(url.pathname)) event.respondWith(fromCache(url.pathname, req))
})

function fromCache(path, req) {
  return caches.open(CACHE).then((cache) => cache.match(path).then((hit) => hit || fetch(req)))
}

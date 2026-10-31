// Dog-ear service worker: makes the app installable, opens instantly and keeps
// the shell and book covers available offline. Library data always comes from
// the server; /api and PocketBase's /_/ pages are never cached.
const VERSION = 'v2'
const SHELL = `dogear-shell-${VERSION}`
const COVERS = `dogear-covers-${VERSION}`
const MAX_COVERS = 300

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(SHELL)
      .then((cache) => cache.addAll(['/', '/manifest.webmanifest', '/icon-192.png', '/icon-512.png']))
      .then(() => self.skipWaiting()),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== SHELL && k !== COVERS).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

async function trimCovers() {
  const cache = await caches.open(COVERS)
  const keys = await cache.keys()
  await Promise.all(keys.slice(0, Math.max(0, keys.length - MAX_COVERS)).map((k) => cache.delete(k)))
}

self.addEventListener('fetch', (event) => {
  const request = event.request
  if (request.method !== 'GET') return
  const url = new URL(request.url)

  // Book covers never change for a given ID.
  if (url.hostname === 'covers.openlibrary.org' || url.hostname === 'books.google.com') {
    event.respondWith(
      caches.match(request).then(
        (hit) =>
          hit ||
          fetch(request).then((response) => {
            if (response.ok || response.type === 'opaque') {
              const copy = response.clone()
              caches
                .open(COVERS)
                .then((cache) => cache.put(request, copy))
                .then(trimCovers)
            }
            return response
          }),
      ),
    )
    return
  }

  if (url.origin !== self.location.origin) return
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/_/')) return

  // Pages: network first so a new deploy shows up, falling back to the shell.
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) {
            const copy = response.clone()
            caches.open(SHELL).then((cache) => cache.put('/', copy))
          }
          return response
        })
        .catch(() => caches.match('/')),
    )
    return
  }

  // Hashed build assets never change; everything else refreshes in the background.
  const immutable = url.pathname.startsWith('/_next/static/')
  event.respondWith(
    caches.match(request).then((hit) => {
      const network = fetch(request).then((response) => {
        if (response.ok) {
          const copy = response.clone()
          caches.open(SHELL).then((cache) => cache.put(request, copy))
        }
        return response
      })
      if (hit) {
        if (!immutable) network.catch(() => undefined)
        return hit
      }
      return network
    }),
  )
})

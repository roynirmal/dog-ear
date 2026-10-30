'use client'

import { useEffect, useSyncExternalStore } from 'react'

type InstallPromptEvent = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: string }> }

// Browsers fire beforeinstallprompt once, often before React mounts the button
// that wants it, so keep it at module level.
let deferred: InstallPromptEvent | null = null
const listeners = new Set<() => void>()
const notify = () => listeners.forEach((listener) => listener())

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (event) => {
    event.preventDefault()
    deferred = event as InstallPromptEvent
    notify()
  })
  window.addEventListener('appinstalled', () => {
    deferred = null
    notify()
  })
}

/** Registers the service worker in production builds. */
export function ServiceWorker() {
  useEffect(() => {
    if (process.env.NODE_ENV !== 'production' || !('serviceWorker' in navigator)) return
    navigator.serviceWorker.register('/sw.js').catch(() => undefined)
  }, [])
  return null
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function standalone() {
  return (
    window.matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
  )
}

/**
 * How Dog-ear can be added to the home screen here: a one-tap prompt (Chrome,
 * Edge, Android), manual steps (iPhone and iPad Safari), or nothing when it is
 * already installed or the browser cannot install apps.
 */
export function useInstall(): { mode: 'prompt' | 'ios' | null; install: () => Promise<void> } {
  const mode = useSyncExternalStore(
    subscribe,
    () => {
      if (standalone()) return null
      if (deferred) return 'prompt'
      const ios =
        /iphone|ipad|ipod/i.test(navigator.userAgent) ||
        (navigator.maxTouchPoints > 1 && /mac/i.test(navigator.userAgent))
      return ios ? 'ios' : null
    },
    () => null,
  )
  return {
    mode,
    install: async () => {
      if (!deferred) return
      await deferred.prompt()
      await deferred.userChoice.catch(() => undefined)
      deferred = null
      notify()
    },
  }
}

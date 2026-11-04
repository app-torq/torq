import { useSyncExternalStore } from 'react'

// Platform detection and the browser's install prompt for the install hint
// (specs/004-installable-pwa, research W11).

/** Running from the home screen or as an installed app window. */
export function isStandalone(): boolean {
  return window.matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
}

/** iPhone or iPad, including iPads that report a desktop Mac. */
export function isIOS(): boolean {
  const ua = navigator.userAgent
  return /iPhone|iPad|iPod/.test(ua) || (ua.includes('Macintosh') && navigator.maxTouchPoints > 1)
}

export function isTouchDevice(): boolean {
  return window.matchMedia('(pointer: coarse)').matches
}

type InstallPromptEvent = Event & { prompt: () => Promise<unknown> }

let deferred: InstallPromptEvent | null = null
let capturing = false
const listeners = new Set<() => void>()
const notify = () => { for (const fn of listeners) fn() }

/** Keeps Chromium's install prompt so the hint can offer an Install button. */
export function captureInstallPrompt(): void {
  if (capturing) return
  capturing = true
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault()
    deferred = e as InstallPromptEvent
    notify()
  })
  window.addEventListener('appinstalled', () => {
    deferred = null
    notify()
  })
}

/** Returns a function that shows the browser's install dialog, if it offers one. */
export function useInstallPrompt(): (() => void) | null {
  const event = useSyncExternalStore(
    (fn) => { listeners.add(fn); return () => { listeners.delete(fn) } },
    () => deferred,
  )
  if (!event) return null
  return () => {
    void event.prompt()
    deferred = null
    notify()
  }
}

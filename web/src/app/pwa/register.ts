import { useSyncExternalStore } from 'react'
import { api } from '../../lib/api'
import { installDirtyTracking, isDirty } from './dirty'
import { captureInstallPrompt } from './install'
import { createUpdater, type Updater } from './updates'

// Wires the service worker and the update watch into the running app
// (specs/004-installable-pwa, research W5, W12).

let updater: Updater | undefined
const listeners = new Set<() => void>()
const notify = () => { for (const fn of listeners) fn() }

export function startPWA(): void {
  installDirtyTracking()
  captureInstallPrompt()
  const start = (registration?: ServiceWorkerRegistration) => {
    const u = createUpdater({
      fetchVersion: () => api.get<{ version: string }>('/instance').then((i) => i.version),
      reload: () => { window.location.reload() },
      now: () => Date.now(),
      isDirty,
      registration,
    })
    updater = u
    u.subscribe(notify)
    void u.checkNow({ force: true })
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') void u.checkNow()
    })
    setInterval(() => void u.checkNow({ force: true }), 60 * 60_000)
    notify()
  }

  // Workers need a secure context; on plain HTTP torq works as before and
  // only the version watch runs.
  if (!(import.meta.env.PROD && window.isSecureContext && 'serviceWorker' in navigator)) {
    start()
    return
  }
  // The very first worker claims the page too; that is not an update.
  let hadController = navigator.serviceWorker.controller !== null
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (hadController) updater?.onControllerChange()
    hadController = true
  })
  navigator.serviceWorker.register('/sw.js', { scope: '/' }).then(start, (err: unknown) => {
    console.warn('torq: service worker registration failed', err)
    start()
  })
}

export function getUpdater(): Updater | undefined {
  return updater
}

/** Whether a new torq version is waiting to be loaded. */
export function useUpdatePending(): boolean {
  return useSyncExternalStore(
    (fn) => { listeners.add(fn); return () => { listeners.delete(fn) } },
    () => updater?.pending() ?? false,
  )
}

// Brings open and installed apps to the server's current version
// (specs/004-installable-pwa, research W5). Two signals feed one flag:
//   - the server's version from GET /instance changed since the page loaded,
//     which also works on plain HTTP without a service worker;
//   - a new service worker is installed and waiting.
// A pending update is applied on the next navigation when nothing is unsaved;
// otherwise the update banner offers "Reload now".

const minInterval = 60_000

export interface UpdaterDeps {
  fetchVersion: () => Promise<string>
  reload: () => void
  now: () => number
  isDirty: () => boolean
  registration?: {
    update: () => Promise<unknown>
    readonly waiting: { postMessage: (msg: unknown) => void } | null
  } | undefined
}

export type Updater = ReturnType<typeof createUpdater>

export function createUpdater(deps: UpdaterDeps) {
  let baseline: string | undefined
  let lastCheck = -Infinity
  let isPending = false
  let applying = false
  const subscribers = new Set<() => void>()

  const setPending = () => {
    if (isPending) return
    isPending = true
    for (const fn of subscribers) fn()
  }

  const applyNow = () => {
    const waiting = deps.registration?.waiting
    if (waiting) {
      // The worker activates and claims this page; onControllerChange reloads.
      applying = true
      waiting.postMessage({ type: 'SKIP_WAITING' })
      return
    }
    deps.reload()
  }

  return {
    pending: () => isPending,

    subscribe(fn: () => void): () => void {
      subscribers.add(fn)
      return () => { subscribers.delete(fn) }
    },

    async checkNow(opts: { force?: boolean } = {}): Promise<void> {
      const now = deps.now()
      if (!opts.force && now - lastCheck < minInterval) return
      lastCheck = now
      try {
        await deps.registration?.update()
      } catch {
        // Offline or the worker script is unreachable; try again later.
      }
      if (deps.registration?.waiting) setPending()
      try {
        const version = await deps.fetchVersion()
        if (baseline === undefined) baseline = version
        else if (version !== baseline) setPending()
      } catch {
        // Server unreachable: nothing to compare.
      }
    },

    onNavigate(): void {
      if (isPending && !deps.isDirty()) applyNow()
    },

    applyNow,

    /** A new worker took control: ours after applyNow, or another tab's. */
    onControllerChange(): void {
      if (applying) deps.reload()
      else setPending()
    },
  }
}

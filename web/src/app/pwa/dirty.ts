// Tracks whether the current page has unsaved input, so an app update never
// reloads under someone typing (specs/004-installable-pwa, research W6). Any
// input since the last navigation or successful save counts; forms need no
// code of their own.

let dirty = false
const subscribers = new Set<() => void>()

function set(value: boolean) {
  if (dirty === value) return
  dirty = value
  for (const fn of subscribers) fn()
}

function onInput(e: Event) {
  // Search fields hold nothing to lose.
  if (e.target instanceof HTMLInputElement && e.target.type === 'search') return
  set(true)
}

export function isDirty(): boolean {
  return dirty
}

/** Called on navigation and after every successful save. */
export function markClean(): void {
  set(false)
}

export function subscribe(fn: () => void): () => void {
  subscribers.add(fn)
  return () => { subscribers.delete(fn) }
}

export function installDirtyTracking(doc: Document = document): () => void {
  doc.addEventListener('input', onInput, true)
  return () => { doc.removeEventListener('input', onInput, true) }
}

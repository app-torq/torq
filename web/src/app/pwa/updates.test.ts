import { describe, expect, it, vi } from 'vitest'
import { createUpdater, type UpdaterDeps } from './updates'

function setup(over: Partial<UpdaterDeps> = {}) {
  let version = 'v1'
  let now = 0
  let dirty = false
  const deps: UpdaterDeps = {
    fetchVersion: vi.fn(() => Promise.resolve(version)),
    reload: vi.fn(),
    now: () => now,
    isDirty: () => dirty,
    ...over,
  }
  const u = createUpdater(deps)
  return {
    u, deps,
    setVersion: (v: string) => { version = v },
    advance: (ms: number) => { now += ms },
    setDirty: (d: boolean) => { dirty = d },
  }
}

describe('updater', () => {
  it('remembers the first version and flags a different one', async () => {
    const s = setup()
    await s.u.checkNow()
    expect(s.u.pending()).toBe(false)
    s.advance(61_000)
    await s.u.checkNow()
    expect(s.u.pending()).toBe(false)
    s.setVersion('v2')
    s.advance(61_000)
    await s.u.checkNow()
    expect(s.u.pending()).toBe(true)
  })

  it('checks at most once a minute unless forced', async () => {
    const s = setup()
    await s.u.checkNow()
    await s.u.checkNow()
    expect(s.deps.fetchVersion).toHaveBeenCalledTimes(1)
    await s.u.checkNow({ force: true })
    expect(s.deps.fetchVersion).toHaveBeenCalledTimes(2)
  })

  it('ignores an unreachable server', async () => {
    const s = setup({ fetchVersion: vi.fn(() => Promise.reject(new TypeError('offline'))) })
    await expect(s.u.checkNow()).resolves.toBeUndefined()
    expect(s.u.pending()).toBe(false)
  })

  it('applies a pending update on navigation when nothing is unsaved', async () => {
    const s = setup()
    await s.u.checkNow()
    s.setVersion('v2')
    await s.u.checkNow({ force: true })
    s.u.onNavigate()
    expect(s.deps.reload).toHaveBeenCalledTimes(1)
  })

  it('keeps unsaved input: no reload on navigation, the banner shows instead', async () => {
    const s = setup()
    await s.u.checkNow()
    s.setVersion('v2')
    await s.u.checkNow({ force: true })
    s.setDirty(true)
    s.u.onNavigate()
    expect(s.deps.reload).not.toHaveBeenCalled()
    expect(s.u.pending()).toBe(true)
  })

  it('asks a waiting service worker to take over, then reloads when it does', async () => {
    const postMessage = vi.fn()
    const registration = { update: vi.fn(() => Promise.resolve()), waiting: { postMessage } }
    const s = setup({ registration })
    await s.u.checkNow()
    expect(registration.update).toHaveBeenCalled()
    expect(s.u.pending()).toBe(true) // a waiting worker is an update too
    s.u.applyNow()
    expect(postMessage).toHaveBeenCalledWith({ type: 'SKIP_WAITING' })
    expect(s.deps.reload).not.toHaveBeenCalled()
    s.u.onControllerChange()
    expect(s.deps.reload).toHaveBeenCalledTimes(1)
  })

  it('flags an update another tab activated instead of reloading under the user', () => {
    const s = setup()
    s.u.onControllerChange()
    expect(s.deps.reload).not.toHaveBeenCalled()
    expect(s.u.pending()).toBe(true)
  })

  it('notifies subscribers when an update becomes pending', async () => {
    const s = setup()
    const fn = vi.fn()
    s.u.subscribe(fn)
    await s.u.checkNow()
    s.setVersion('v2')
    await s.u.checkNow({ force: true })
    expect(fn).toHaveBeenCalledTimes(1)
  })
})

import { afterEach, describe, expect, it, vi } from 'vitest'
import { installDirtyTracking, isDirty, markClean, subscribe } from './dirty'

describe('dirty tracking', () => {
  let uninstall = () => {}
  afterEach(() => { uninstall(); markClean() })

  it('marks the page dirty on input and clean on markClean', () => {
    uninstall = installDirtyTracking(document)
    const input = document.createElement('input')
    document.body.append(input)
    expect(isDirty()).toBe(false)
    input.dispatchEvent(new Event('input', { bubbles: true }))
    expect(isDirty()).toBe(true)
    markClean()
    expect(isDirty()).toBe(false)
    input.remove()
  })

  it('ignores search fields, which hold no work to lose', () => {
    uninstall = installDirtyTracking(document)
    const search = document.createElement('input')
    search.type = 'search'
    document.body.append(search)
    search.dispatchEvent(new Event('input', { bubbles: true }))
    expect(isDirty()).toBe(false)
    search.remove()
  })

  it('notifies subscribers on changes only', () => {
    uninstall = installDirtyTracking(document)
    const fn = vi.fn()
    const unsubscribe = subscribe(fn)
    document.dispatchEvent(new Event('input'))
    document.dispatchEvent(new Event('input'))
    markClean()
    expect(fn).toHaveBeenCalledTimes(2)
    unsubscribe()
  })
})

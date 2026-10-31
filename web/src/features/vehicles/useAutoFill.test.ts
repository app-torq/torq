import { act, renderHook } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useAutoFill } from './useAutoFill'

type V = { make: string; model: string; power: string }

function setup(initial: V) {
  return renderHook(() => {
    const [values, setValues] = useState(initial)
    return { values, setValues, auto: useAutoFill(values, setValues) }
  })
}

afterEach(() => { vi.useRealTimers() })

describe('useAutoFill', () => {
  it('fills only changed fields, highlights them briefly, and undoes', () => {
    vi.useFakeTimers()
    const { result } = setup({ make: 'BMW', model: '', power: '' })
    act(() => { result.current.auto.fill({ make: 'BMW', model: '3 Series', power: '110' }) })
    expect(result.current.values).toEqual({ make: 'BMW', model: '3 Series', power: '110' })
    expect(result.current.auto.isHighlighted('model')).toBe(true)
    expect(result.current.auto.isHighlighted('make')).toBe(false)
    expect(result.current.auto.canUndo).toBe(true)
    act(() => { vi.advanceTimersByTime(2000) })
    expect(result.current.auto.isHighlighted('model')).toBe(false)
    act(() => { result.current.auto.undo() })
    expect(result.current.values).toEqual({ make: 'BMW', model: '', power: '' })
    expect(result.current.auto.canUndo).toBe(false)
  })

  it('drops manually edited fields from the undo snapshot', () => {
    const { result } = setup({ make: '', model: '', power: '' })
    act(() => { result.current.auto.fill({ make: 'Audi', model: 'A4' }) })
    act(() => {
      result.current.setValues((v) => ({ ...v, model: 'A4 Avant' }))
      result.current.auto.touch('model')
    })
    act(() => { result.current.auto.undo() })
    expect(result.current.values).toEqual({ make: '', model: 'A4 Avant', power: '' })
  })

  it('replaces the snapshot with the next fill and ignores no-op fills', () => {
    const { result } = setup({ make: 'Opel', model: 'Astra', power: '' })
    act(() => { result.current.auto.fill({ model: 'Corsa' }) })
    act(() => { result.current.auto.fill({ power: '66' }) })
    let changed: string[] = []
    act(() => { changed = result.current.auto.fill({ power: '66' }) })
    expect(changed).toEqual([])
    act(() => { result.current.auto.undo() })
    expect(result.current.values).toEqual({ make: 'Opel', model: 'Corsa', power: '' })
  })
})

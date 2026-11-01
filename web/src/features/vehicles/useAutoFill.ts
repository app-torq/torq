import { useCallback, useEffect, useRef, useState } from 'react'

const HIGHLIGHT_MS = 2000

/**
 * Applies catalog fills to a form-values object with a brief highlight of the
 * changed fields and a one-step undo (research C11). Only fields whose value
 * actually changes are recorded; a manual edit of a filled field removes it
 * from the undo snapshot.
 */
export function useAutoFill<T extends Record<string, string>>(values: T, setValues: (update: (prev: T) => T) => void) {
  const [snapshot, setSnapshot] = useState<Partial<T> | null>(null)
  const [highlighted, setHighlighted] = useState<ReadonlySet<keyof T>>(new Set())
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => () => { clearTimeout(timer.current) }, [])

  const flash = useCallback((fields: (keyof T)[]) => {
    clearTimeout(timer.current)
    setHighlighted(new Set(fields))
    if (fields.length > 0) timer.current = setTimeout(() => { setHighlighted(new Set()) }, HIGHLIGHT_MS)
  }, [])

  /** Fills every given field whose value differs; returns the changed fields. */
  const fill = useCallback(
    (next: Partial<T>): (keyof T)[] => {
      const current = values
      const changed = (Object.keys(next) as (keyof T)[]).filter((k) => next[k] !== undefined && next[k] !== current[k])
      if (changed.length === 0) return changed
      const before: Partial<T> = {}
      const after: Partial<T> = {}
      for (const k of changed) {
        before[k] = current[k]
        after[k] = next[k]
      }
      setValues((prev) => ({ ...prev, ...after }))
      setSnapshot(before)
      flash(changed)
      return changed
    },
    [values, setValues, flash],
  )

  const undo = useCallback(() => {
    if (!snapshot) return
    const restore = snapshot
    setValues((prev) => ({ ...prev, ...restore }))
    setSnapshot(null)
    flash([])
  }, [snapshot, setValues, flash])

  const touch = useCallback((field: keyof T) => {
    setSnapshot((s) => {
      if (!s || !(field in s)) return s
      const rest = Object.fromEntries(Object.entries(s).filter(([k]) => k !== field)) as Partial<T>
      return Object.keys(rest).length > 0 ? rest : null
    })
  }, [])

  return {
    fill,
    undo,
    touch,
    canUndo: snapshot !== null,
    isHighlighted: (field: keyof T) => highlighted.has(field),
  }
}

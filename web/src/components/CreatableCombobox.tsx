import { useId, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { catalogKey } from '../lib/catalogKey'
import { Field, fieldInputClass } from './Field'

interface Props<T> {
  label: ReactNode
  /** Current text; it is always what gets saved (free text is allowed). */
  value: string
  onChange: (text: string) => void
  /** A suggestion was chosen. */
  onSelect: (item: T) => void
  /** Text committed by choosing or leaving the field; `changed` compares it with the text at focus time. */
  onCommit?: (text: string, changed: boolean) => void
  onFocusChange?: (focused: boolean) => void
  items: readonly T[] | undefined
  itemText: (item: T) => string
  renderItem?: (item: T) => ReactNode
  /** Whether the catalog knows this level; without it the field is a plain text input (FR-011). */
  catalogAvailable: boolean
  loading?: boolean
  error?: string | undefined
  hint?: ReactNode
  optional?: boolean
  required?: boolean
  highlighted?: boolean
}

type Option<T> = { kind: 'custom'; text: string } | { kind: 'item'; item: T }

/**
 * Searchable, creatable combobox following the WAI-ARIA 1.2 combobox pattern
 * (research C10): ↑/↓/Home/End move, Enter chooses, Escape closes and keeps
 * the text, Tab moves on. When the typed text matches no suggestion exactly,
 * "Use custom: '…'" is offered first (FR-014).
 */
export function CreatableCombobox<T>({
  label, value, onChange, onSelect, onCommit, onFocusChange, items, itemText, renderItem, catalogAvailable,
  loading, error, hint, optional, required, highlighted,
}: Props<T>) {
  const { t } = useTranslation('vehicles')
  const listId = useId()
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const [focusValue, setFocusValue] = useState(value)

  const text = value.trim()
  const suggestions = items ?? []
  const exact = suggestions.some((i) => catalogKey(itemText(i)) === catalogKey(text))
  const options: Option<T>[] = [
    ...(text !== '' && !exact ? [{ kind: 'custom' as const, text }] : []),
    ...suggestions.map((item) => ({ kind: 'item' as const, item })),
  ]
  const showList = open && catalogAvailable && options.length > 0
  const activeIndex = Math.min(active, options.length - 1)

  const commit = (committed: string) => {
    onCommit?.(committed, catalogKey(committed) !== catalogKey(focusValue))
    setFocusValue(committed)
  }

  const choose = (o: Option<T>) => {
    if (o.kind === 'custom') {
      onChange(o.text)
      commit(o.text)
    } else {
      onSelect(o.item)
      commit(itemText(o.item))
    }
    setOpen(false)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        if (!open) setOpen(true)
        else setActive((a) => Math.min(a + 1, options.length - 1))
        break
      case 'ArrowUp':
        e.preventDefault()
        setActive((a) => Math.max(a - 1, 0))
        break
      case 'Home':
        if (showList) { e.preventDefault(); setActive(0) }
        break
      case 'End':
        if (showList) { e.preventDefault(); setActive(options.length - 1) }
        break
      case 'Enter': {
        const o = options[activeIndex]
        if (showList && o) {
          e.preventDefault()
          choose(o)
        }
        break
      }
      case 'Escape':
        if (open) { e.preventDefault(); setOpen(false) }
        break
      case 'Tab':
        setOpen(false)
        break
    }
  }

  const optionId = (i: number) => `${listId}-opt-${String(i)}`

  return (
    <Field label={label} error={error} hint={hint} optional={optional ?? false}>
      {(a) => (
        <div className="relative">
          <input
            {...a}
            type="text"
            role="combobox"
            autoComplete="off"
            required={required}
            aria-autocomplete="list"
            aria-expanded={showList}
            aria-controls={listId}
            aria-activedescendant={showList ? optionId(activeIndex) : undefined}
            value={value}
            onChange={(e) => {
              onChange(e.target.value)
              setOpen(true)
              setActive(0)
            }}
            onFocus={() => {
              setFocusValue(value)
              onFocusChange?.(true)
            }}
            onBlur={() => {
              setOpen(false)
              onFocusChange?.(false)
              if (value !== focusValue) commit(value.trim())
            }}
            onClick={() => { setOpen(true) }}
            onKeyDown={onKeyDown}
            className={fieldInputClass(highlighted)}
          />
          <ul
            id={listId}
            role="listbox"
            hidden={!showList}
            className="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-md border border-zinc-700 bg-zinc-900 py-1 text-sm shadow-lg shadow-black/40"
          >
            {showList && options.map((o, i) => (
              <li
                key={o.kind === 'custom' ? '__custom' : `${itemText(o.item)}-${String(i)}`}
                id={optionId(i)}
                role="option"
                aria-selected={i === activeIndex}
                onMouseDown={(e) => { e.preventDefault() }}
                onClick={() => { choose(o) }}
                onMouseEnter={() => { setActive(i) }}
                className={`cursor-pointer px-3 py-2 ${i === activeIndex ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-300'}`}
              >
                {o.kind === 'custom' ? (
                  <span className="italic text-amber-300">{t('combobox.useCustom', { value: o.text })}</span>
                ) : (
                  (renderItem ?? itemText)(o.item)
                )}
              </li>
            ))}
            {showList && loading && <li role="presentation" className="px-3 py-2 text-xs text-zinc-400">{t('combobox.loading')}</li>}
          </ul>
        </div>
      )}
    </Field>
  )
}

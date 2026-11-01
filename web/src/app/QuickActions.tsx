import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight } from 'lucide-react'
import { useFormat } from '../lib/useFormat'
import { vehicleName, type Vehicle } from '../lib/types'
import { Dialog } from '../components/Dialog'
import { inputClass } from '../components/Field'
import { useVehicles } from '../features/vehicles/hooks'
import { OdometerQuickUpdate } from '../features/vehicles/OdometerQuickUpdate'
import { quickActions, type QuickActionDef } from './paletteActions'

/** Opens the palette on Ctrl/⌘+K. */
export function useQuickActionsHotkey(open: () => void) {
  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        open()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => { window.removeEventListener('keydown', onKey) }
  }, [open])
}

export function QuickActions({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  return (
    <Dialog open={open} onClose={onClose} title={t('actions.quickActions')}>
      {open && <Palette onClose={onClose} />}
    </Dialog>
  )
}

type Step = { kind: 'actions' } | { kind: 'vehicle'; action: QuickActionDef } | { kind: 'odometer'; vehicle: Vehicle }

function Palette({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const fmt = useFormat()
  const navigate = useNavigate()
  const vehicles = useVehicles('false')
  const [step, setStep] = useState<Step>({ kind: 'actions' })
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  const items = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (step.kind === 'actions') {
      return quickActions
        .map((a) => ({ key: a.id, label: t(a.labelKey), icon: a.icon, run: () => { choose(a) } }))
        .filter((i) => i.label.toLowerCase().includes(q))
    }
    if (step.kind === 'vehicle') {
      return (vehicles.data?.items ?? [])
        .map((v) => ({ key: String(v.id), label: `${vehicleName(v)} · ${fmt.km(v.current_odometer_km)}`, icon: undefined, run: () => { pickVehicle(step.action, v) } }))
        .filter((i) => i.label.toLowerCase().includes(q))
    }
    return []
    // eslint-disable-next-line react-hooks/exhaustive-deps -- choose/pickVehicle are stable for this render
  }, [step, query, vehicles.data, t, fmt])

  function choose(action: QuickActionDef) {
    if (!action.needsVehicle) {
      if (action.to) void navigate(action.to(null))
      onClose()
      return
    }
    const list = vehicles.data?.items ?? []
    const only = list.length === 1 ? list[0] : undefined
    if (only) {
      pickVehicle(action, only)
      return
    }
    setStep({ kind: 'vehicle', action })
    setQuery('')
    setActive(0)
    inputRef.current?.focus()
  }

  function pickVehicle(action: QuickActionDef, v: Vehicle) {
    if (action.kind === 'odometer') {
      setStep({ kind: 'odometer', vehicle: v })
      return
    }
    if (action.to) void navigate(action.to(v.id))
    onClose()
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive((a) => Math.min(a + 1, items.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive((a) => Math.max(a - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      items[active]?.run()
    }
  }

  if (step.kind === 'odometer') {
    return (
      <div className="space-y-3">
        <p className="text-sm text-zinc-400">{vehicleName(step.vehicle)}</p>
        <OdometerQuickUpdate vehicle={step.vehicle} onDone={onClose} autoFocus />
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <input
        ref={inputRef}
        autoFocus
        className={inputClass}
        placeholder={step.kind === 'vehicle' ? t('quick.pickVehicle') : t('quick.placeholder')}
        aria-label={step.kind === 'vehicle' ? t('quick.pickVehicle') : t('quick.placeholder')}
        value={query}
        onChange={(e) => { setQuery(e.target.value); setActive(0) }}
        onKeyDown={onKeyDown}
      />
      <ul role="listbox" className="max-h-72 overflow-y-auto">
        {items.length === 0 && <li className="px-3 py-2 text-sm text-zinc-400">{t('quick.noResults')}</li>}
        {items.map((it, i) => (
          <li key={it.key} role="option" aria-selected={i === active}>
            <button
              type="button"
              onMouseEnter={() => { setActive(i) }}
              onClick={it.run}
              className={`flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm ${i === active ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-300'}`}
            >
              {it.icon && <it.icon size={16} className="text-zinc-400" />}
              <span className="flex-1">{it.label}</span>
              <ChevronRight size={14} className="text-zinc-600" />
            </button>
          </li>
        ))}
      </ul>
      <p className="text-xs text-zinc-400">{t('quick.hint')}</p>
    </div>
  )
}

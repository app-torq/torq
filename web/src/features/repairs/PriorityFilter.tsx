import { useTranslation } from 'react-i18next'
import { ChevronDown } from 'lucide-react'
import { Checkbox } from '../../components/Field'
import { PriorityBadge, priorityName, useRepairPriorities } from './priorities'

/**
 * Narrows projects to one or more priorities. value holds priority ids as
 * text and "none" for projects without a priority; empty means all.
 */
export function PriorityFilter({ value, onChange }: { value: string[]; onChange: (v: string[]) => void }) {
  const { t } = useTranslation('repairs')
  const priorities = useRepairPriorities()
  const items = priorities.data?.items ?? []
  const toggle = (key: string, on: boolean) => {
    onChange(on ? [...value, key] : value.filter((v) => v !== key))
  }
  const single = value.length === 1 ? items.find((p) => String(p.id) === value[0]) : undefined
  let summary = t('filters.priorityCount', { count: value.length })
  if (value.length === 0) summary = t('filters.allPriorities')
  else if (value[0] === 'none' && value.length === 1) summary = t('priorities.none')
  else if (single) summary = priorityName(single, t)
  return (
    <details className="group relative">
      <summary className="focus-ring flex h-10 cursor-pointer list-none items-center gap-2 rounded-md border border-zinc-700 bg-zinc-950 px-3 text-sm text-zinc-100 [&::-webkit-details-marker]:hidden">
        <span className="text-zinc-400">{t('fields.priority')}:</span>
        {summary}
        <ChevronDown size={14} aria-hidden className="text-zinc-400 transition-transform group-open:rotate-180" />
      </summary>
      <fieldset className="absolute z-20 mt-1 flex w-64 flex-col items-start gap-2 rounded-md border border-zinc-700 bg-zinc-900 p-3 shadow-lg">
        <legend className="sr-only">{t('fields.priority')}</legend>
        {items.map((p) => (
          <Checkbox key={p.id} label={<PriorityBadge priority={p} display="both" />} checked={value.includes(String(p.id))} onChange={(on) => { toggle(String(p.id), on) }} />
        ))}
        <Checkbox label={t('priorities.none')} checked={value.includes('none')} onChange={(on) => { toggle('none', on) }} />
        {value.length > 0 && (
          <button type="button" className="focus-ring text-xs text-sky-300 hover:underline" onClick={() => { onChange([]) }}>
            {t('filters.clear')}
          </button>
        )}
      </fieldset>
    </details>
  )
}

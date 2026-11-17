import { useTranslation } from 'react-i18next'
import { ChevronDown } from 'lucide-react'
import { Checkbox } from '../../components/Field'
import { useRepairTags } from './tags'

/** Narrows projects to those that have all selected tags (names); empty means all. */
export function TagFilter({ value, onChange }: { value: string[]; onChange: (v: string[]) => void }) {
  const { t } = useTranslation('repairs')
  const tags = useRepairTags()
  const items = tags.data?.items ?? []
  if (items.length === 0 && value.length === 0) return null
  const selected = (name: string) => value.some((v) => v.toLowerCase() === name.toLowerCase())
  const toggle = (name: string, on: boolean) => {
    onChange(on ? [...value, name] : value.filter((v) => v.toLowerCase() !== name.toLowerCase()))
  }
  let summary = t('filters.tagCount', { count: value.length })
  if (value.length === 0) summary = t('filters.allTags')
  else if (value.length === 1) summary = value[0] ?? ''
  return (
    <details className="group relative">
      <summary className="focus-ring flex h-10 cursor-pointer list-none items-center gap-2 rounded-md border border-zinc-700 bg-zinc-950 px-3 text-sm text-zinc-100 [&::-webkit-details-marker]:hidden">
        <span className="text-zinc-400">{t('fields.tags')}:</span>
        {summary}
        <ChevronDown size={14} aria-hidden className="text-zinc-400 transition-transform group-open:rotate-180" />
      </summary>
      <fieldset className="absolute z-20 mt-1 flex max-h-80 w-64 flex-col items-start gap-2 overflow-y-auto rounded-md border border-zinc-700 bg-zinc-900 p-3 shadow-lg">
        <legend className="sr-only">{t('fields.tags')}</legend>
        <p className="text-xs text-zinc-400">{t('filters.tagsHint')}</p>
        {items.map((tag) => (
          <Checkbox key={tag.id} label={tag.name} checked={selected(tag.name)} onChange={(on) => { toggle(tag.name, on) }} />
        ))}
        {value.length > 0 && (
          <button type="button" className="focus-ring text-xs text-sky-300 hover:underline" onClick={() => { onChange([]) }}>
            {t('filters.clear')}
          </button>
        )}
      </fieldset>
    </details>
  )
}

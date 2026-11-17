import { useId, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { Field, inputBaseClass } from '../../components/Field'
import { normalizeTag, useRepairTags } from './tags'

/**
 * Chip input for project tags. Existing tags are suggested, and a typed name
 * that matches one in any case takes its spelling, so "brembo" and "Brembo"
 * stay one tag. Enter or comma adds a tag.
 */
export function TagInput({ value, onChange, error }: { value: string[]; onChange: (tags: string[]) => void; error?: string | undefined }) {
  const { t } = useTranslation('repairs')
  const tags = useRepairTags()
  const listId = useId()
  const [draft, setDraft] = useState('')
  const known = tags.data?.items ?? []
  const has = (list: string[], name: string) => list.some((x) => x.toLowerCase() === name.toLowerCase())
  const commit = () => {
    const next = [...value]
    for (const part of draft.split(',')) {
      const name = normalizeTag(part.replaceAll('"', ''))
      if (name === '' || has(next, name)) continue
      next.push(known.find((k) => k.name.toLowerCase() === name.toLowerCase())?.name ?? name)
    }
    if (next.length !== value.length) onChange(next)
    setDraft('')
  }
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }
  return (
    <Field label={t('fields.tags')} hint={t('fields.tagsHint')} error={error} optional>
      {(a) => (
        <div className="flex flex-wrap items-center gap-1.5 rounded-md border border-zinc-700 bg-zinc-950 p-1.5">
          {value.map((tag) => (
            <span key={tag} className="inline-flex items-center gap-1 rounded border border-zinc-700 px-1.5 py-0.5 text-xs text-zinc-200">
              {tag}
              <button type="button" aria-label={`${t('common:actions.remove')} ${tag}`} className="text-zinc-400 hover:text-zinc-100" onClick={() => { onChange(value.filter((x) => x !== tag)) }}>
                <X size={12} />
              </button>
            </span>
          ))}
          <input
            {...a}
            list={listId}
            value={draft}
            maxLength={40}
            onChange={(e) => { setDraft(e.target.value) }}
            onKeyDown={onKey}
            onBlur={commit}
            className={`${inputBaseClass} min-w-32 flex-1 border-0 bg-transparent p-1`}
          />
          <datalist id={listId}>
            {known.filter((k) => !has(value, k.name)).map((k) => <option key={k.id} value={k.name} />)}
          </datalist>
        </div>
      )}
    </Field>
  )
}

import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { ListResponse, RepairTag } from '../../lib/types'

export function useRepairTags() {
  return useQuery({
    queryKey: queryKeys.tags,
    queryFn: () => api.get<ListResponse<RepairTag>>('/repair-tags'),
  })
}

/** Tags are plain, uncolored chips so they stay apart from the colored priorities. */
export function TagChip({ name }: { name: string }) {
  return <span className="inline-flex items-center rounded border border-zinc-700 px-1.5 py-0.5 text-xs text-zinc-300">{name}</span>
}

/** The first limit tags and "+ n more" for the rest (named in its tooltip); limit 0 shows all. */
export function TagList({ tags, limit = 0 }: { tags: string[]; limit?: number }) {
  const { t } = useTranslation('repairs')
  if (tags.length === 0) return null
  const shown = limit > 0 ? tags.slice(0, limit) : tags
  const rest = tags.slice(shown.length)
  return (
    <ul aria-label={t('fields.tags')} className="flex flex-wrap items-center gap-1">
      {shown.map((tag) => <li key={tag}><TagChip name={tag} /></li>)}
      {rest.length > 0 && (
        <li className="text-xs text-zinc-400" title={rest.join(', ')}>
          {t('tags.more', { count: rest.length })}
        </li>
      )}
    </ul>
  )
}

/** Normalizes typed tag text the way the server does. */
export function normalizeTag(name: string): string {
  return name.trim().split(/\s+/).join(' ')
}

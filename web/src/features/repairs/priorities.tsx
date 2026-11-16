import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { priorityClasses } from '../../lib/priorityColors'
import type { ListResponse, PriorityDisplay, RepairPriority } from '../../lib/types'
import { useSession } from '../../app/session'

export function useRepairPriorities() {
  return useQuery({
    queryKey: queryKeys.priorities,
    queryFn: () => api.get<ListResponse<RepairPriority>>('/repair-priorities'),
  })
}

/** The custom name, or the translated name of a built-in priority. */
export function priorityName(p: RepairPriority, t: TFunction): string {
  return p.custom_name ?? t(`repairs:priorities.builtin.${p.builtin_key ?? ''}`)
}

/**
 * A priority as color and name, color only or name only (the user's choice,
 * or display when given). With color only, the name stays the tooltip and the
 * accessible name.
 */
export function PriorityBadge({ priority, display }: { priority: RepairPriority; display?: PriorityDisplay }) {
  const { t } = useTranslation('repairs')
  const { user } = useSession()
  const mode = display ?? user.priority_display
  const name = priorityName(priority, t)
  const c = priorityClasses(priority.color)
  if (mode === 'color') {
    return <span role="img" aria-label={t('priorities.label', { name })} title={name} className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${c.dot}`} />
  }
  if (mode === 'name') {
    return <span className="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium text-zinc-200 ring-1 ring-inset ring-zinc-700">{name}</span>
  }
  return (
    <span className={`inline-flex items-center gap-1.5 rounded px-1.5 py-0.5 text-xs font-medium ring-1 ring-inset ${c.chip}`}>
      <span aria-hidden className={`h-2 w-2 shrink-0 rounded-full ${c.dot}`} />
      {name}
    </span>
  )
}

/** Looks up a project's priority; undefined for none or while loading. */
export function usePriorityLookup(): (id: number | null) => RepairPriority | undefined {
  const priorities = useRepairPriorities()
  const byId = new Map(priorities.data?.items.map((p) => [p.id, p]))
  return (id) => (id === null ? undefined : byId.get(id))
}

/** The badge of a project's priority, or nothing when it has none. */
export function ProjectPriority({ id }: { id: number | null }) {
  const priority = usePriorityLookup()(id)
  return priority ? <PriorityBadge priority={priority} /> : null
}

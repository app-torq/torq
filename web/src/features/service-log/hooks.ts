import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { ListResponse, ServiceCategory } from '../../lib/types'

export function useCategories(includeArchived = false) {
  return useQuery({
    queryKey: [...queryKeys.categories, includeArchived],
    queryFn: () => api.get<ListResponse<ServiceCategory>>('/service-categories', { include_archived: includeArchived ? 'true' : undefined }),
    staleTime: 60_000,
  })
}

/** Display name: translated for built-ins, as entered for custom categories. */
export function categoryLabel(c: ServiceCategory | undefined, t: TFunction): string {
  if (!c) return ''
  if (c.builtin_key) return t(`service:categories.${c.builtin_key}`)
  return c.custom_name ?? ''
}

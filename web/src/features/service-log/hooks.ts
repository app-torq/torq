import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { ListResponse, ServiceCategory, VehicleType } from '../../lib/types'

export function useCategories(includeArchived = false) {
  return useQuery({
    queryKey: [...queryKeys.categories, includeArchived],
    queryFn: () => api.get<ListResponse<ServiceCategory>>('/service-categories', { include_archived: includeArchived ? 'true' : undefined }),
    staleTime: 60_000,
  })
}

/**
 * Whether a picker offers c for a vehicle of type (specs/006-motorcycles):
 * categories for all types always, type-specific built-ins only for their
 * type, everything while no vehicle is chosen.
 */
export function appliesTo(c: ServiceCategory, type: VehicleType | undefined): boolean {
  return c.vehicle_type === null || type === undefined || c.vehicle_type === type
}

/** Display name: translated for built-ins, as entered for custom categories. */
export function categoryLabel(c: ServiceCategory | undefined, t: TFunction): string {
  if (!c) return ''
  if (c.builtin_key) return t(`service:categories.${c.builtin_key}`)
  return c.custom_name ?? ''
}

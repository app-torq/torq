// Offline vehicle catalog lookups (specs/002-vehicle-catalog-lookup,
// contracts/rest-api.md). Catalog data only changes when torq is upgraded,
// so results never go stale within a session.
import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { CatalogEngine, CatalogGeneration, CatalogMeta, CatalogModel, KbaVariant } from '../../lib/types'

export const HSN_RE = /^\d{4}$/
export const TSN_RE = /^[0-9A-Z]{3}$/

interface Items<T> {
  items: T[]
}

const forever = { staleTime: Infinity, gcTime: 30 * 60 * 1000 } as const

/** Query options for an HSN/TSN lookup; the quick-fill bar fetches them on demand. */
export function kbaLookupOptions(hsn: string, tsn: string) {
  return queryOptions({
    queryKey: queryKeys.catalogKba(hsn, tsn),
    queryFn: () => api.get<Items<KbaVariant>>('/catalog/kba', { hsn, tsn }).then((r) => r.items),
    ...forever,
  })
}

export function useCatalogMakes(q: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.catalogList('makes', [], q),
    queryFn: () => api.get<Items<{ name: string }>>('/catalog/makes', { q }).then((r) => r.items),
    enabled,
    ...forever,
  })
}

export function useCatalogModels(make: string, q: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.catalogList('models', [make], q),
    queryFn: () => api.get<Items<CatalogModel>>('/catalog/models', { make, q }).then((r) => r.items),
    enabled: enabled && make.trim() !== '',
    ...forever,
  })
}

export function useCatalogGenerations(make: string, model: string, q: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.catalogList('generations', [make, model], q),
    queryFn: () => api.get<Items<CatalogGeneration>>('/catalog/generations', { make, model, q }).then((r) => r.items),
    enabled: enabled && make.trim() !== '' && model.trim() !== '',
    ...forever,
  })
}

export function useCatalogEngines(make: string, model: string, generation: string, q: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.catalogList('engines', [make, model, generation], q),
    queryFn: () => api.get<Items<CatalogEngine>>('/catalog/engines', { make, model, generation, q }).then((r) => r.items),
    enabled: enabled && make.trim() !== '' && model.trim() !== '' && generation.trim() !== '',
    ...forever,
  })
}

export function useCatalogMeta() {
  return useQuery({
    queryKey: queryKeys.catalogMeta,
    queryFn: () => api.get<CatalogMeta>('/catalog/meta'),
    ...forever,
  })
}

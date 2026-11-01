// Offline vehicle catalog lookups (specs/002-vehicle-catalog-lookup,
// contracts/rest-api.md). Names come in the user's language. The catalog only
// changes on an upgrade or an admin's online update, after which the About
// page invalidates the cached lookups.
import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSession } from '../../app/session'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { CatalogEngine, CatalogGeneration, CatalogMeta, CatalogModel, CatalogUpdate, KbaVariant, Locale } from '../../lib/types'

export const HSN_RE = /^\d{4}$/
export const TSN_RE = /^[0-9A-Z]{3}$/

interface Items<T> {
  items: T[]
}

const forever = { staleTime: Infinity, gcTime: 30 * 60 * 1000 } as const

/** Language of catalog names: the signed-in user's locale. */
export function useCatalogLang(): Locale {
  return useSession().user.locale
}

/** Query options for an HSN/TSN lookup; the quick-fill bar fetches them on demand. */
export function kbaLookupOptions(hsn: string, tsn: string, lang: Locale) {
  return queryOptions({
    queryKey: queryKeys.catalogKba(hsn, tsn, lang),
    queryFn: () => api.get<Items<KbaVariant>>('/catalog/kba', { hsn, tsn, lang }).then((r) => r.items),
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
  const lang = useCatalogLang()
  return useQuery({
    queryKey: queryKeys.catalogList('models', [lang, make], q),
    queryFn: () => api.get<Items<CatalogModel>>('/catalog/models', { make, q, lang }).then((r) => r.items),
    enabled: enabled && make.trim() !== '',
    ...forever,
  })
}

export function useCatalogGenerations(make: string, model: string, q: string, enabled = true) {
  const lang = useCatalogLang()
  return useQuery({
    queryKey: queryKeys.catalogList('generations', [lang, make, model], q),
    queryFn: () => api.get<Items<CatalogGeneration>>('/catalog/generations', { make, model, q, lang }).then((r) => r.items),
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
    // Poll while an online update runs.
    refetchInterval: (query) => (query.state.data?.update.state === 'running' ? 3000 : false),
  })
}

/** Starts an online catalog update (admins only). */
export function useCatalogUpdate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<{ update: CatalogUpdate }>('/catalog/update'),
    onSuccess: ({ update }) => {
      qc.setQueryData<CatalogMeta>(queryKeys.catalogMeta, (prev) => (prev ? { ...prev, update } : prev))
    },
  })
}

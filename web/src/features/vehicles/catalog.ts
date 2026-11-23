// Offline vehicle catalog lookups (specs/002-vehicle-catalog-lookup,
// contracts/rest-api.md). Names come in the user's language. The catalog only
// changes on an upgrade or an admin's online update, after which the About
// page invalidates the cached lookups.
import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSession } from '../../app/session'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { CatalogEngine, CatalogGeneration, CatalogMeta, CatalogModel, CatalogUpdate, KbaVariant, Locale, VehicleType } from '../../lib/types'
import type { Formatter } from '../../lib/format'

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

// The cascade is filtered by vehicle type (specs/006-motorcycles).

export function useCatalogMakes(type: VehicleType, q: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.catalogList('makes', [type], q),
    queryFn: () => api.get<Items<{ name: string }>>('/catalog/makes', { vehicle_type: type, q }).then((r) => r.items),
    enabled,
    ...forever,
  })
}

export function useCatalogModels(type: VehicleType, make: string, q: string, enabled = true) {
  const lang = useCatalogLang()
  return useQuery({
    queryKey: queryKeys.catalogList('models', [type, lang, make], q),
    queryFn: () => api.get<Items<CatalogModel>>('/catalog/models', { vehicle_type: type, make, q, lang }).then((r) => r.items),
    enabled: enabled && make.trim() !== '',
    ...forever,
  })
}

export function useCatalogGenerations(type: VehicleType, make: string, model: string, q: string, enabled = true) {
  const lang = useCatalogLang()
  return useQuery({
    queryKey: queryKeys.catalogList('generations', [type, lang, make, model], q),
    queryFn: () => api.get<Items<CatalogGeneration>>('/catalog/generations', { vehicle_type: type, make, model, q, lang }).then((r) => r.items),
    enabled: enabled && make.trim() !== '' && model.trim() !== '',
    ...forever,
  })
}

/** Engines of a car generation, or the versions of a motorcycle model (which has no generations). */
export function useCatalogEngines(type: VehicleType, make: string, model: string, generation: string, q: string, enabled = true) {
  const gen = type === 'motorcycle' ? '' : generation
  return useQuery({
    queryKey: queryKeys.catalogList('engines', [type, make, model, gen], q),
    queryFn: () => api.get<Items<CatalogEngine>>('/catalog/engines', { vehicle_type: type, make, model, generation: gen, q }).then((r) => r.items),
    enabled: enabled && make.trim() !== '' && model.trim() !== '' && (type === 'motorcycle' || gen.trim() !== ''),
    ...forever,
  })
}

/** Years as "2014–2020", "2014–" (still current) or "2019". */
export const yearRange = (from: number | null, to: number | null) =>
  from == null ? '' : to == null ? `${String(from)}–` : from === to ? String(from) : `${String(from)}–${String(to)}`

/** A motorcycle version in the user's units: "821 cm³ · 82 kW · 2014–2020". */
export function versionLabel(e: CatalogEngine, fmt: Formatter): string {
  return [
    e.displacement_cc != null ? fmt.displacement(e.displacement_cc) : null,
    e.power_kw != null ? fmt.powerShort(e.power_kw * 1000) : null,
    yearRange(e.year_from, e.year_to) || null,
  ].filter(Boolean).join(' · ') || e.name
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

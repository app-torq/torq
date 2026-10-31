import { expect, type APIRequestContext, type Page } from '@playwright/test'

export const ADMIN = { username: 'marek', password: 'workshop-manual-1', display: 'Marek' }
const headers = { 'X-Torq-Request': '1' }

/** Signs in through the API; the cookie is shared with the page. */
export async function login(page: Page) {
  const res = await page.request.post('/api/v1/auth/login', { data: { username: ADMIN.username, password: ADMIN.password }, headers })
  expect(res.ok()).toBeTruthy()
}

export async function apiPost<T>(request: APIRequestContext, path: string, data: unknown): Promise<T> {
  const res = await request.post(`/api/v1${path}`, { data, headers })
  expect(res.ok(), await res.text()).toBeTruthy()
  return (await res.json()) as T
}

export async function createVehicle(request: APIRequestContext, km: number, model = 'C 220 CDI'): Promise<number> {
  const { vehicle } = await apiPost<{ vehicle: { id: number } }>(request, '/vehicles', {
    make: 'Mercedes-Benz', model, chassis_code: 'S203', model_year: 2006, engine_code: 'OM646', current_odometer_km: km,
  })
  return vehicle.id
}

interface SeedKba {
  hsn: string
  tsn: string
  make: string
  model: string
  trade_name: string
}

/**
 * Picks HSN/TSN keys from the committed catalog seed so the catalog scenarios
 * never hard-code catalog content: one key with a single variant that names a
 * model, one key with several variants, and one key that is absent.
 */
export async function catalogKeys() {
  const { readFileSync } = await import('node:fs')
  const { gunzipSync } = await import('node:zlib')
  const seed = JSON.parse(gunzipSync(readFileSync(new URL('../../internal/catalog/data/catalog_seed.json.gz', import.meta.url))).toString()) as { kba: SeedKba[] }
  const byKey = new Map<string, SeedKba[]>()
  for (const r of seed.kba) {
    const k = `${r.hsn}/${r.tsn}`
    byKey.set(k, [...(byKey.get(k) ?? []), r])
  }
  const entries = [...byKey.entries()]
  const single = entries.find(([, v]) => v.length === 1 && v[0]?.model !== '' && /^[A-Z]{3}$/.test(v[0]?.tsn ?? ''))
  const multi = entries.find(([, v]) => v.length === 3)
  let absent = '0000/ZZZ'
  for (let n = 0; byKey.has(absent); n++) absent = `${String(9000 + n)}/ZZZ`
  if (!single || !multi) throw new Error('catalog seed lacks single- or multi-variant keys')
  return { single: { key: single[0], variant: single[1][0] as SeedKba }, multi: { key: multi[0], variants: multi[1] }, absent }
}

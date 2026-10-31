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

import { expect, test } from '@playwright/test'
import { createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

// Quickstart V3 (specs/002-vehicle-catalog-lookup): extended profile.
test('V12: technical profile shows kW (PS), cm³ and fuel; invalid values are refused; exports carry the fields', async ({ page }) => {
  await login(page)
  const legacyId = await createVehicle(page.request, 120000, 'Legacy')
  const id = await createVehicle(page.request, 150000, 'Profile')

  await page.goto(`/vehicles/${String(id)}/edit`)
  await page.getByLabel('Displacement').fill('2148')
  await page.getByLabel('Power').fill('110')
  await page.getByLabel('Fuel type').selectOption('diesel')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('110 kW (150 PS)')).toBeVisible()
  await expect(page.getByText('2,148 cm³')).toBeVisible()
  await expect(page.getByText('Diesel').first()).toBeVisible()

  // German number format and labels.
  const pref = await page.request.patch('/api/v1/me/preferences', { data: { locale: 'de', number_format: 'comma' }, headers })
  expect(pref.ok(), await pref.text()).toBeTruthy()
  await page.goto(`/vehicles/${String(id)}/specs`)
  await expect(page.getByText('2.148 cm³')).toBeVisible()
  await expect(page.getByText('110 kW (150 PS)')).toBeVisible()

  // PS preferred: shown first, still stored as kW.
  const ps = await page.request.patch('/api/v1/me/preferences', { data: { power_unit: 'ps' }, headers })
  expect(ps.ok(), await ps.text()).toBeTruthy()
  await page.goto(`/vehicles/${String(id)}/specs`)
  await expect(page.getByText('150 PS (110 kW)')).toBeVisible()
  const back = await page.request.patch('/api/v1/me/preferences', { data: { locale: 'en', number_format: 'point', power_unit: 'kw' }, headers })
  expect(back.ok()).toBeTruthy()

  // A vehicle without the new fields shows them as not set.
  await page.goto(`/vehicles/${String(legacyId)}`)
  const row = page.locator('div', { has: page.getByText('HSN / TSN', { exact: true }) }).last()
  await expect(row).toContainText('—')

  // Invalid values: field codes from the API.
  for (const [field, value, code] of [
    ['hsn', '071', 'vehicle.hsn_invalid'], ['tsn', 'AB', 'vehicle.tsn_invalid'],
    ['power_kw', 0, 'vehicle.power_out_of_range'], ['fuel_type', 'kerosene', 'vehicle.fuel_type_invalid'],
  ] as const) {
    const res = await page.request.patch(`/api/v1/vehicles/${String(id)}`, { data: { [field]: value }, headers })
    expect(res.status()).toBe(422)
    expect(((await res.json()) as { error: { fields: Record<string, string> } }).error.fields[field]).toBe(code)
  }

  // Exports.
  const json = (await (await page.request.get(`/api/v1/vehicles/${String(id)}/history?format=json`)).json()) as { vehicle: Record<string, unknown> }
  expect(json.vehicle).toMatchObject({ displacement_cc: 2148, power_kw: 110, fuel_type: 'diesel', hsn: null, tsn: null })
  const csv = await (await page.request.get(`/api/v1/vehicles/${String(id)}/history?format=csv`)).text()
  expect(csv.split('\n')[0]).toContain('hsn,tsn,displacement_cc,power_kw,fuel_type')
  expect(csv.split('\n')[1]).toMatch(/^vehicle,.*,2148,110,diesel\r?$/)
})

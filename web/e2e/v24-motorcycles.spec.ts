import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import { login } from './helpers'

interface Items<T> {
  items: T[]
}

async function get<T>(request: APIRequestContext, path: string, query: Record<string, string>): Promise<T[]> {
  const res = await request.get(`/api/v1${path}?${new URLSearchParams(query).toString()}`)
  expect(res.ok()).toBeTruthy()
  return ((await res.json()) as Items<T>).items
}

/** Types into a catalog combobox and picks the first option whose text starts with option. */
async function pick(page: Page, label: string, text: string, option: string) {
  const box = page.getByRole('combobox', { name: label })
  await box.fill(text)
  const opt = page.getByRole('option', { name: new RegExp(`^${option.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) }).first()
  await expect(opt).toBeVisible()
  await opt.click()
}

// Quickstart M-V1, M-V3, M-V4 (specs/006-motorcycles, issue #28).
test('V24: a motorcycle from the catalog, its spec sheet and categories', async ({ page }) => {
  await login(page)

  // The bundled catalog has motorcycles, separate from cars.
  const makes = await get<{ name: string }>(page.request, '/catalog/makes', { vehicle_type: 'motorcycle', q: 'ducati' })
  expect(makes.map((m) => m.name)).toContain('Ducati')
  expect(await get<{ name: string }>(page.request, '/catalog/makes', { q: 'ducati' })).toHaveLength(0)
  const versions = await get<{ displacement_cc: number | null; power_kw: number | null }>(page.request, '/catalog/engines', {
    vehicle_type: 'motorcycle', make: 'Ducati', model: 'Monster 821',
  })
  const version = versions.find((v) => v.displacement_cc !== null && v.power_kw !== null)
  if (!version) throw new Error('no Monster 821 version with power data')

  // M-V1: Make → Model → Version on the form.
  await page.goto('/vehicles/new')
  await page.getByLabel('Vehicle type').selectOption('motorcycle')
  await pick(page, 'Make', 'duc', 'Ducati')
  await pick(page, 'Model', 'monster 821', 'Monster 821')
  await page.getByRole('combobox', { name: 'Version' }).click()
  await page.getByRole('option', { name: new RegExp(`^${String(version.displacement_cc)}\\s*cm³\\s*·\\s*${String(version.power_kw)}\\s*kW`) }).first().click()
  await expect(page.getByRole('textbox', { name: /^Power/ })).toHaveValue(String(version.power_kw))
  await expect(page.getByRole('textbox', { name: /^Displacement/ })).toHaveValue(String(version.displacement_cc))
  await page.getByLabel('Current odometer').fill('12000')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'Ducati Monster 821' })).toBeVisible()
  const id = /\/vehicles\/(\d+)/.exec(page.url())?.[1]
  if (!id) throw new Error(`no vehicle id in ${page.url()}`)

  // M-V3: spec sheet with chain data; shaft drive shows final drive oil instead.
  await page.goto(`/vehicles/${id}/specs`)
  await page.getByRole('button', { name: 'Edit specs' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('textbox', { name: 'Wheel lug torque' })).toHaveCount(0)
  await dialog.getByRole('textbox', { name: 'Front sprocket teeth' }).fill('15')
  await dialog.getByRole('textbox', { name: 'Rear sprocket teeth' }).fill('46')
  await dialog.getByRole('textbox', { name: 'Chain size' }).fill('525')
  await dialog.getByRole('textbox', { name: 'Chain links' }).fill('108')
  await dialog.getByRole('textbox', { name: 'Intake', exact: true }).fill('0.10')
  await dialog.getByRole('textbox', { name: 'Intake, up to' }).fill('0.15')
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('15 / 46')).toBeVisible()
  await expect(page.getByText('525 · 108 links')).toBeVisible()
  await expect(page.getByText('Intake: 0.10–0.15 mm')).toBeVisible()

  await page.getByRole('button', { name: 'Edit specs' }).click()
  await dialog.getByLabel('Final drive').selectOption('shaft')
  await expect(dialog.getByRole('textbox', { name: 'Chain links' })).toHaveCount(0)
  await expect(dialog.getByText('Final drive oil')).toBeVisible()
  await dialog.getByRole('button', { name: 'Cancel' }).click()

  // M-V4: motorcycle categories on the motorcycle, none on a car.
  await page.goto(`/service/new?vehicle=${id}`)
  const category = page.getByLabel('Category').first()
  await expect(category.locator('option', { hasText: 'Chain: Clean, Lube, Adjust' })).toHaveCount(1)
  await expect(category.locator('option', { hasText: 'Cabin Filter' })).toHaveCount(0)
})

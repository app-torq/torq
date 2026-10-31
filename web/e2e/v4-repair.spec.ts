import { expect, test } from '@playwright/test'
import { createVehicle, login } from './helpers'

test('V4: repair project with BOM and completion', async ({ page }) => {
  await login(page)
  const vid = await createVehicle(page.request, 215200, 'E 220 CDI')

  await page.goto(`/vehicles/${vid}/repairs`)
  await page.getByRole('button', { name: 'New project' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('textbox', { name: 'Title' }).fill('Injector seal replacement')
  await dialog.getByLabel(/OBD2 fault codes/).fill('P0201, p0300,')
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByRole('heading', { name: 'Injector seal replacement' })).toBeVisible()
  await expect(page.getByText('P0300')).toBeVisible()

  await page.getByRole('button', { name: 'Add part' }).click()
  const part = page.getByRole('dialog')
  await part.getByRole('textbox', { name: 'Part' }).fill('Injector copper seal')
  await part.getByRole('textbox', { name: 'Qty' }).fill('4')
  await part.getByRole('textbox', { name: /^Unit cost/ }).fill('2.95')
  await page.getByRole('dialog').getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('cell', { name: '€11.80' }).first()).toBeVisible()

  await page.getByLabel('Move to').selectOption('completed')
  await page.getByLabel('Odometer at completion').fill('216000')
  await page.getByRole('button', { name: 'Mark completed' }).click()
  await expect(page.getByText('Completed').first()).toBeVisible()

  const res = await page.request.get(`/api/v1/vehicles/${vid}`)
  expect((await res.json()).vehicle.current_odometer_km).toBe(216000)

  const search = await page.request.get('/api/v1/repairs?q=P0201')
  expect((await search.json()).total).toBeGreaterThanOrEqual(1)
})

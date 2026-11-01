import { expect, test } from '@playwright/test'
import { ADMIN } from './helpers'

test('V1: first boot, garage and odometer rules', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Welcome to torq' })).toBeVisible()
  await page.getByLabel('Username').fill(ADMIN.username)
  await page.getByLabel('Display name').fill(ADMIN.display)
  await page.getByLabel('Password').fill(ADMIN.password)
  await page.getByRole('button', { name: 'Create administrator' }).click()
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()

  // Setup is not offered again.
  const status = await page.request.get('/api/v1/setup/status')
  expect(await status.json()).toEqual({ setup_required: false })

  await page.goto('/vehicles/new')
  await page.getByLabel('Make').fill('Mercedes-Benz')
  await page.getByLabel('Model', { exact: true }).fill('C 220 CDI')
  await page.getByLabel('Generation / chassis code').fill('S203')
  await page.getByLabel('Model year').fill('2006')
  await page.getByLabel('Engine code').fill('OM646')
  await page.getByLabel('Current odometer').fill('214500')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'Mercedes-Benz C 220 CDI' })).toBeVisible()

  await page.getByLabel('New odometer reading').fill('215200')
  await page.getByRole('button', { name: 'Update odometer' }).click()
  await expect(page.getByText('215,200 km').first()).toBeVisible()

  // Lowering asks for confirmation; cancelling keeps the value.
  await page.getByLabel('New odometer reading').fill('21520')
  await page.getByRole('button', { name: 'Update odometer' }).click()
  await expect(page.getByRole('heading', { name: 'Lower the odometer?' })).toBeVisible()
  await page.getByRole('button', { name: 'Cancel' }).click()
  await expect(page.getByText('215,200 km').first()).toBeVisible()
})

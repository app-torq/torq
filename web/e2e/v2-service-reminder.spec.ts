import { expect, test } from '@playwright/test'
import { apiPost, createVehicle, login } from './helpers'

test('V2: logging a service resets the linked reminder', async ({ page }) => {
  await login(page)
  const vid = await createVehicle(page.request, 214500, 'C 200 Kompressor')
  const start = new Date()
  start.setMonth(start.getMonth() - 3)
  await apiPost(page.request, '/reminders', {
    vehicle_id: vid, service_category_id: 1, interval_km: 10000, interval_months: 12,
    start_date: start.toISOString().slice(0, 10), start_odometer_km: 205500,
  })
  await apiPost(page.request, `/vehicles/${vid}/odometer`, { odometer_km: 214600 })

  await page.goto(`/vehicles/${vid}/reminders`)
  await expect(page.getByText('Due soon').first()).toBeVisible()
  await expect(page.getByText('900 km left')).toBeVisible()

  await page.goto(`/service/new?vehicle=${vid}`)
  await page.getByLabel('Odometer').fill('215600')
  await page.getByLabel('Category').selectOption({ label: 'Engine Oil & Filter' })
  await page.getByLabel('Brand').fill('Mobil 1')
  await page.getByLabel('Quantity').fill('6.5')
  await page.getByRole('textbox', { name: /^Cost/ }).fill('89.50')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'Service entry' })).toBeVisible()
  await expect(page.getByText('6.5 L')).toBeVisible()

  await page.goto(`/vehicles/${vid}/reminders`)
  await expect(page.getByText('Upcoming').first()).toBeVisible()
  await expect(page.getByText('10,000 km left')).toBeVisible()
})

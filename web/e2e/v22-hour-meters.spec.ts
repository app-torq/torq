import { expect, test } from '@playwright/test'
import { apiPost, login } from './helpers'

interface Vehicle { id: number; has_odometer: boolean; hour_meters: { id: number; name: string; current_hours: number }[] }
interface Category { id: number; builtin_key: string | null }

// Issue #24 (with #29): a tractor tracked by hours only, hours on a service
// entry, a reminder by hours, and hours in the printed history.
test('V22: hour meters on an hours-only vehicle, its service log, reminders and history', async ({ page }) => {
  await login(page)

  // Create the tractor in the form: hours only, no odometer field.
  await page.goto('/vehicles/new')
  await page.getByLabel('Make', { exact: true }).fill('Deutz')
  await page.getByLabel('Model', { exact: true }).fill('D 6206')
  await page.getByLabel('Track by').selectOption('hours')
  await expect(page.getByLabel('Current odometer')).toBeHidden()
  await page.getByLabel('Current hours').fill('1250.4')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'Deutz D 6206' })).toBeVisible()
  await expect(page.getByText('1,250.4 h').first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Odometer' })).toBeHidden()
  await expect(page.getByText(/\d km/).filter({ visible: true })).toHaveCount(0)

  const id = Number(/\/vehicles\/(\d+)/.exec(page.url())?.[1])
  const vehicle = ((await (await page.request.get(`/api/v1/vehicles/${String(id)}`)).json()) as { vehicle: Vehicle }).vehicle
  expect(vehicle.has_odometer).toBe(false)
  const meter = vehicle.hour_meters[0]
  if (!meter) throw new Error('no hour meter')

  // Quick update with the reading history; a lower value asks first.
  await page.getByLabel('New reading (Engine hours)').fill('1262')
  await page.getByRole('button', { name: 'Update hours' }).click()
  await expect(page.getByRole('cell', { name: '1,262 h' })).toBeVisible()
  await page.getByLabel('New reading (Engine hours)').fill('1126')
  await page.getByRole('button', { name: 'Update hours' }).click()
  await expect(page.getByRole('heading', { name: 'Lower the hours?' })).toBeVisible()
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()

  // The service form asks for hours instead of the odometer.
  await page.goto(`/service/new?vehicle=${String(id)}`)
  await expect(page.getByLabel('Engine hours')).toBeVisible()
  await expect(page.getByLabel('Odometer')).toHaveCount(0)

  // An oil reminder every 250 h, then an oil change at 1,262 h: due at 1,512 h.
  const cats = ((await (await page.request.get('/api/v1/service-categories')).json()) as { items: Category[] }).items
  const oil = cats.find((c) => c.builtin_key === 'engine_oil')?.id
  await apiPost(page.request, '/reminders', { vehicle_id: id, service_category_id: oil, hour_meter_id: meter.id, interval_hours: 250, interval_months: 12 })
  await apiPost(page.request, '/service-logs', {
    vehicle_id: id, service_date: new Date().toISOString().slice(0, 10), items: [{ category_id: oil, cost_cents: 8900 }],
    meter_readings: [{ hour_meter_id: meter.id, hours: 1262 }],
  })

  // 1,490 h: due soon by hours.
  await page.goto(`/vehicles/${String(id)}`)
  await page.getByLabel('New reading (Engine hours)').fill('1490')
  await page.getByRole('button', { name: 'Update hours' }).click()
  await page.goto(`/vehicles/${String(id)}/reminders`)
  await expect(page.getByText('Due soon').first()).toBeVisible()
  await expect(page.getByText('by hours')).toBeVisible()
  await expect(page.getByText('22 h left')).toBeVisible()
  await expect(page.getByText('at 1,512 h')).toBeVisible()

  // The printed history shows the hours, never a distance.
  await page.goto(`/vehicles/${String(id)}/history/print`)
  await expect(page.getByText('Engine hours').first()).toBeVisible()
  await expect(page.getByText('1,490 h')).toBeVisible()
  await expect(page.getByText(/· 1,262 h/)).toBeVisible()
  await expect(page.getByText(/\d km/).filter({ visible: true })).toHaveCount(0)
})

// Issue #39: the dashboard asks for what a vehicle is tracked by; with
// odometer and hours, both inputs are there.
test('V22b: the dashboard offers odometer and hours for a vehicle tracked by both', async ({ page }) => {
  await login(page)
  await apiPost(page.request, '/vehicles', {
    make: 'Hymer', model: 'B 544 Generator', current_odometer_km: 98000, hour_meters: [{ name: 'Generator', hours: 312 }],
  })
  await page.goto('/')
  const card = page.locator('div', { has: page.getByRole('link', { name: 'Hymer B 544 Generator' }) })
    .filter({ has: page.getByLabel('New odometer reading') }).last()
  await expect(card.getByLabel('New odometer reading')).toBeVisible()
  await card.getByLabel('New reading (Generator)').fill('320')
  await card.getByRole('button', { name: 'Update hours' }).click()
  await expect(card.getByText(/Generator: 320 h/)).toBeVisible()
})

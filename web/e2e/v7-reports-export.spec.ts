import { expect, test } from '@playwright/test'
import { apiPost, createVehicle, login } from './helpers'

test('V7: cost report, cost per km and exports', async ({ page }) => {
  await login(page)
  const vid = await createVehicle(page.request, 205000, 'CLK 270 CDI') // initial reading is dated today, inside 2026
  await apiPost(page.request, '/service-logs', { vehicle_id: vid, service_date: '2026-01-10', odometer_km: 205000, extra_cost_cents: 2000, items: [{ category_id: 1, cost_cents: 9000 }] })
  await apiPost(page.request, '/service-logs', { vehicle_id: vid, service_date: '2026-06-01', odometer_km: 215000, items: [{ category_id: 6, cost_cents: 31000 }] })
  const { repair } = await apiPost<{ repair: { id: number } }>(page.request, '/repairs', { vehicle_id: vid, title: 'Swirl flaps' })
  await apiPost(page.request, `/repairs/${repair.id}/parts`, { name: 'Intake manifold', quantity: 1, unit_cost_cents: 118000 })
  await apiPost(page.request, `/repairs/${repair.id}/status`, { status: 'completed', completed_on: '2026-10-20', completion_odometer_km: 225000 })

  const report = await (await page.request.get(`/api/v1/reports/costs?vehicle_id=${vid}&from=2026-01-01&to=2026-12-31`)).json()
  expect(report.total_cents).toBe(160000)
  expect(report.by_source).toEqual({ routine_service_cents: 42000, self_repair_cents: 118000 })

  await page.goto('/reports')
  const main = page.getByRole('main')
  await main.getByLabel('Vehicle').selectOption({ label: 'Mercedes-Benz CLK 270 CDI' })
  await main.getByLabel('Period').selectOption('custom')
  await main.getByLabel('From').fill('2026-01-01')
  await main.getByLabel('To').fill('2026-12-31')
  await expect(page.getByText('€1,600.00')).toBeVisible()
  await expect(page.getByText('€0.080/km')).toBeVisible()

  const csv = await (await page.request.get(`/api/v1/vehicles/${vid}/history?format=csv&include_costs=false`)).text()
  expect(csv.startsWith('﻿record_type,')).toBeTruthy()
  expect(csv).not.toContain('1180.00')

  await page.goto(`/vehicles/${vid}/history/print`)
  await expect(page.getByRole('heading', { name: 'Mercedes-Benz CLK 270 CDI' })).toBeVisible()
  await expect(page.getByText('Swirl flaps')).toBeVisible()
  await expect(page.getByRole('navigation')).toHaveCount(0)
})

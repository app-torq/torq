import { expect, test } from '@playwright/test'
import { apiPost, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

// Restore metric units and ISO dates for later runs against the same instance,
// also when the test fails halfway.
test.afterEach(async ({ page }) => {
  const back = await page.request.patch('/api/v1/me/preferences', {
    data: { date_format: 'YYYY-MM-DD', distance_unit: 'km', volume_unit: 'l', torque_unit: 'nm', pressure_unit: 'bar', displacement_unit: 'cc', power_unit: 'kw' },
    headers,
  })
  expect(back.ok()).toBeTruthy()
})

// Issues #10, #11 and #18: US units are display only (the API stays metric), a JDM
// chassis number is accepted with a warning, and a reminder whose last
// service is unknown is due right away.
test('V19: US units, a JDM chassis number and a never-done reminder', async ({ page }) => {
  await login(page)
  await page.goto('/settings/preferences')
  await page.getByRole('button', { name: 'US', exact: true }).click()
  await page.getByLabel('Date format').selectOption('MM/DD/YYYY')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Saved')).toBeVisible()

  // A new vehicle reads in miles; power and displacement are typed in hp and L.
  const { vehicle } = await apiPost<{ vehicle: { id: number } }>(page.request, '/vehicles', {
    make: 'Toyota', model: 'Supra', current_odometer_km: 80467,
  })
  const id = vehicle.id
  await page.goto(`/vehicles/${String(id)}/edit`)
  await expect(page.getByLabel('Odometer unit')).toHaveValue('mi')
  await page.getByRole('textbox', { name: /^Power/ }).fill('321')
  await page.getByRole('textbox', { name: /^Displacement/ }).fill('3.0')
  await page.getByLabel('VIN').fill('jza80-0012345')
  await expect(page.getByText('Not a standard 17-character VIN')).toBeVisible()
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('321 hp (239 kW)')).toBeVisible()
  await expect(page.getByText('50,000 mi').first()).toBeVisible()

  const res = await page.request.get(`/api/v1/vehicles/${String(id)}`)
  const saved = ((await res.json()) as { vehicle: Record<string, unknown> }).vehicle
  expect(saved).toMatchObject({ vin: 'JZA80-0012345', odometer_unit: 'mi', current_odometer_km: 80467, displacement_cc: 3000, power_w: 239370, power_kw: 239 })

  // Issue #18: this car shows PS and cm³ whatever the user prefers. Switching
  // the unit converts the typed values, and the stored values stay exact.
  await page.goto(`/vehicles/${String(id)}/edit`)
  await page.getByLabel('Power unit').selectOption('ps')
  await expect(page.getByRole('textbox', { name: /^Power/ })).toHaveValue('325')
  await page.getByLabel('Displacement unit').selectOption('cc')
  await expect(page.getByRole('textbox', { name: /^Displacement/ })).toHaveValue('3000')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('325 PS (239 kW)')).toBeVisible()
  await expect(page.getByText(/^3,000\scm³$/).first()).toBeVisible()
  const after = ((await (await page.request.get(`/api/v1/vehicles/${String(id)}`)).json()) as { vehicle: Record<string, unknown> }).vehicle
  expect(after).toMatchObject({ displacement_cc: 3000, power_w: 239370, displacement_unit: 'cc', power_unit: 'ps' })

  // Brake fluid with no known last service: overdue as soon as it exists.
  await page.goto(`/vehicles/${String(id)}/reminders`)
  await page.getByRole('button', { name: 'Add reminder' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Type').selectOption('standalone')
  await dialog.getByLabel('Title').fill('Brake fluid')
  await dialog.getByRole('textbox', { name: /^Every/ }).nth(1).fill('24')
  await dialog.getByLabel('Never done / unknown').check()
  await expect(dialog.getByLabel('Last done (date)')).toBeHidden()
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Due now: no record of when this was last done')).toBeVisible()
  await expect(page.getByText('Overdue').first()).toBeVisible()
})

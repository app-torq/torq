import { expect, test, type Browser } from '@playwright/test'
import { apiPost, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }
const password = 'workshop-manual-1'
const viewKeys = ['costs.view', 'service_logs.view', 'repairs.view', 'reminders.view', 'reports.view', 'attachments.view', 'notes.view', 'vehicle_ids.view']

async function signIn(browser: Browser, username: string) {
  const ctx = await browser.newContext({ locale: 'en-US' })
  const page = await ctx.newPage()
  expect((await page.request.post('/api/v1/auth/login', { data: { username, password }, headers })).ok()).toBeTruthy()
  return { ctx, page }
}

// Users share their own vehicles: an owner shares with a family member by
// username, refusals don't reveal accounts, the administrator sees who
// shared what, and taking it back hides the vehicle again.
test('V30: share own vehicles', async ({ page, browser }) => {
  await login(page)
  const { vehicle } = await apiPost<{ vehicle: { id: number } }>(page.request, '/vehicles', {
    make: 'Sharecar', model: 'Thirty', current_odometer_km: 3000,
  })
  const { role: owner } = await apiPost<{ role: { id: number } }>(page.request, '/roles', {
    name: 'Owner 30', all_vehicles: false, permissions: [...viewKeys, 'vehicles.edit', 'vehicle_access.share'],
  })
  const { role: family } = await apiPost<{ role: { id: number } }>(page.request, '/roles', {
    name: 'Family 30', all_vehicles: false, permissions: viewKeys,
  })
  const { user: olga } = await apiPost<{ user: { id: number } }>(page.request, '/users', { username: 'olga30', display_name: 'Olga Owner', password, role_id: owner.id })
  await apiPost(page.request, '/users', { username: 'jan30', display_name: 'Jan Family', password, role_id: family.id })
  const put = await page.request.put(`/api/v1/users/${String(olga.id)}/vehicle-access`, { data: { items: [{ vehicle_id: vehicle.id, level: 'edit' }] }, headers })
  expect(put.ok()).toBeTruthy()

  // Olga shares her car with Jan; asking for the administrator gets the neutral answer.
  const o = await signIn(browser, 'olga30')
  await o.page.goto(`/vehicles/${String(vehicle.id)}`)
  await expect(o.page.getByText('Not shared with anyone yet.')).toBeVisible()
  await o.page.getByLabel('Username').fill('admin')
  await o.page.getByRole('button', { name: 'Share', exact: true }).click()
  await expect(o.page.getByText('This vehicle can’t be shared with this user. Check the username.')).toBeVisible()
  await o.page.getByLabel('Username').fill('jan30')
  await o.page.getByRole('button', { name: 'Share', exact: true }).click()
  await expect(o.page.getByText('Jan Family')).toBeVisible()

  // Jan sees the car, read only, and who shared it.
  const j = await signIn(browser, 'jan30')
  await j.page.goto(`/vehicles/${String(vehicle.id)}`)
  await expect(j.page.getByRole('heading', { name: 'Sharecar Thirty' })).toBeVisible()
  await expect(j.page.getByText('Shared with you by Olga Owner')).toBeVisible()
  await expect(j.page.getByRole('heading', { name: 'Shared with' })).toHaveCount(0)

  // The administrator sees who shared it.
  await page.goto(`/vehicles/${String(vehicle.id)}`)
  await expect(page.getByText('Shared by Olga Owner')).toBeVisible()

  // Olga takes it back; Jan no longer finds the car.
  await o.page.getByRole('button', { name: 'Take back from Jan Family' }).click()
  await expect(o.page.getByText('Not shared with anyone yet.')).toBeVisible()
  await j.page.goto('/vehicles')
  await expect(j.page.getByText('Sharecar Thirty')).toHaveCount(0)
  await o.ctx.close()
  await j.ctx.close()
})

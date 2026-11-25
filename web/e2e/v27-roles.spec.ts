import { expect, test, type Browser } from '@playwright/test'
import { apiPost, login } from './helpers'

interface Category { id: number; builtin_key: string | null }
const headers = { 'X-Torq-Request': '1' }
const password = 'workshop-manual-1'

async function signedIn(browser: Browser, username: string) {
  const ctx = await browser.newContext({ locale: 'en-US' })
  const page = await ctx.newPage()
  const res = await page.request.post('/api/v1/auth/login', { data: { username, password }, headers })
  expect(res.ok()).toBeTruthy()
  return { ctx, page }
}

// Roles and permissions: a read-only viewer, and a customer who sees only the
// vehicles assigned to them.
test('V27: roles and vehicle access', async ({ page, browser }) => {
  await login(page)
  const vehicle = async (make: string) =>
    (await apiPost<{ vehicle: { id: number } }>(page.request, '/vehicles', { make, model: 'Roles', current_odometer_km: 1000 })).vehicle.id
  const own = await vehicle('Owncar')
  const family = await vehicle('Familycar')
  const other = await vehicle('Othercar')
  const cats = ((await (await page.request.get('/api/v1/service-categories')).json()) as { items: Category[] }).items
  const oil = cats.find((c) => c.builtin_key === 'engine_oil')?.id

  // The admin creates a role in Settings → Roles.
  await page.goto('/settings/roles')
  await page.getByRole('button', { name: 'Add role' }).click()
  const editor = page.getByRole('dialog')
  await editor.getByLabel('Name', { exact: true }).fill('Customer')
  await editor.getByLabel('Vehicles', { exact: true }).selectOption('assigned')
  await editor.getByRole('checkbox', { name: 'Add and edit service entries' }).click()
  await editor.getByRole('checkbox', { name: 'Add and edit projects' }).click()
  await editor.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('row', { name: /Customer/ })).toContainText('Assigned vehicles')

  // Users: a viewer and a customer, whose vehicles are set in the access dialog.
  const roles = ((await (await page.request.get('/api/v1/roles')).json()) as { items: { id: number; name: string }[] }).items
  const roleId = (name: string) => roles.find((r) => r.name === name)?.id
  await apiPost(page.request, '/users', { username: 'viewer27', display_name: 'Vera Viewer', password, role_id: roleId('Viewer') })
  await apiPost(page.request, '/users', { username: 'kunde27', display_name: 'Kai Kunde', password, role_id: roleId('Customer') })
  await page.goto('/settings/users')
  const row = page.getByRole('row', { name: /Kai Kunde/ })
  await expect(row.getByLabel('Role of Kai Kunde')).toHaveValue(String(roleId('Customer')))
  await row.getByRole('button', { name: '0 vehicles' }).click()
  const access = page.getByRole('dialog')
  await access.getByRole('radiogroup', { name: 'Owncar Roles' }).getByRole('radio', { name: 'Edit' }).click()
  await access.getByRole('radiogroup', { name: 'Familycar Roles' }).getByRole('radio', { name: 'View' }).click()
  await access.getByRole('button', { name: 'Save' }).click()
  await expect(row.getByRole('button', { name: '2 vehicles' })).toBeVisible()

  // The vehicle page tells the admin who can see it.
  await page.goto(`/vehicles/${String(family)}`)
  const card = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Access' }) })
  await expect(card.getByRole('listitem').filter({ hasText: 'Kai Kunde' })).toContainText('View')
  await expect(card.getByRole('listitem').filter({ hasText: 'Vera Viewer' })).toContainText('All vehicles (role)')

  // The viewer sees everything but can change nothing.
  const viewer = await signedIn(browser, 'viewer27')
  await viewer.page.goto(`/vehicles/${String(other)}`)
  await expect(viewer.page.getByRole('heading', { name: 'Othercar Roles' })).toBeVisible()
  await expect(viewer.page.getByRole('link', { name: 'Edit' })).toHaveCount(0)
  await expect(viewer.page.getByRole('button', { name: 'Delete vehicle' })).toHaveCount(0)
  await viewer.page.goto('/vehicles')
  await expect(viewer.page.getByRole('link', { name: 'Add vehicle' })).toHaveCount(0)
  const refused = await viewer.page.request.post('/api/v1/service-logs', {
    headers, data: { vehicle_id: other, service_date: '2026-10-01', odometer_km: 1100, items: [{ category_id: oil }] },
  })
  expect(refused.status()).toBe(403)
  await viewer.ctx.close()

  // The customer sees their two vehicles only, and edits only their own.
  const kunde = await signedIn(browser, 'kunde27')
  await kunde.page.goto('/vehicles')
  const main = kunde.page.getByRole('main')
  await expect(main.getByText('Owncar Roles')).toBeVisible()
  await expect(main.getByText('Familycar Roles')).toBeVisible()
  await expect(kunde.page.getByText('Othercar Roles')).toHaveCount(0)
  await kunde.page.goto(`/vehicles/${String(other)}`)
  await expect(kunde.page.getByText('This item no longer exists.')).toBeVisible()
  await kunde.page.goto(`/vehicles/${String(own)}/service`)
  await expect(kunde.page.getByRole('link', { name: 'Log service' })).toBeVisible()
  await kunde.page.goto(`/vehicles/${String(family)}/service`)
  await expect(kunde.page.getByRole('heading', { name: 'Familycar Roles' })).toBeVisible()
  await expect(kunde.page.getByRole('link', { name: 'Log service' })).toHaveCount(0)
  await apiPost(kunde.page.request, '/service-logs', { vehicle_id: own, service_date: '2026-10-01', odometer_km: 1100, items: [{ category_id: oil }] })
  const blocked = await kunde.page.request.post('/api/v1/service-logs', {
    headers, data: { vehicle_id: family, service_date: '2026-10-01', odometer_km: 1100, items: [{ category_id: oil }] },
  })
  expect(blocked.status()).toBe(403)
  await kunde.page.goto('/settings')
  await expect(kunde.page.getByRole('link', { name: 'Service categories' })).toHaveCount(0)
  await expect(kunde.page.getByRole('link', { name: 'Roles' })).toHaveCount(0)
  await kunde.ctx.close()
})

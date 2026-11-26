import { expect, test } from '@playwright/test'
import { apiPost, login } from './helpers'

interface Category { id: number; builtin_key: string | null }
interface ServiceLog { id: number; notes: string; items: { id: number; brand: string; cost_cents: number | null }[] }
const headers = { 'X-Torq-Request': '1' }
const password = 'workshop-manual-1'

// What a role sees: a customer who sees service entries without costs or
// notes and no self-repair projects, and whose edits keep the shop's prices.
test('V29: a role without costs', async ({ page, browser }) => {
  await login(page)
  const { vehicle } = await apiPost<{ vehicle: { id: number } }>(page.request, '/vehicles', {
    make: 'Seecar', model: 'Visibility', current_odometer_km: 1000, license_plate: 'SEE-29',
  })
  const cats = ((await (await page.request.get('/api/v1/service-categories')).json()) as { items: Category[] }).items
  const oil = cats.find((c) => c.builtin_key === 'engine_oil')?.id
  const { service_log: log } = await apiPost<{ service_log: ServiceLog }>(page.request, '/service-logs', {
    vehicle_id: vehicle.id, service_date: '2026-10-01', odometer_km: 1100, notes: 'Shop notes 29',
    items: [{ category_id: oil, brand: 'Oilbrand', cost_cents: 4321 }],
  })
  await apiPost(page.request, '/repairs', { vehicle_id: vehicle.id, title: 'Hidden project 29' })

  // The admin creates the role: it sees neither costs, notes, plate nor projects.
  await page.goto('/settings/roles')
  await page.getByRole('button', { name: 'Add role' }).click()
  const editor = page.getByRole('dialog')
  await editor.getByLabel('Name', { exact: true }).fill('Owner view')
  for (const name of ['See costs', 'See notes', 'See VIN and license plate', 'See self-repair projects']) {
    await editor.getByRole('checkbox', { name }).click()
  }
  await editor.getByRole('checkbox', { name: 'Add and edit service entries' }).click()
  await expect(editor.getByRole('checkbox', { name: 'Add and edit projects' })).not.toBeChecked()
  await editor.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('row', { name: /Owner view/ })).toBeVisible()
  const roles = ((await (await page.request.get('/api/v1/roles')).json()) as { items: { id: number; name: string }[] }).items
  await apiPost(page.request, '/users', { username: 'owner29', display_name: 'Olga Owner', password, role_id: roles.find((r) => r.name === 'Owner view')?.id })

  const ctx = await browser.newContext({ locale: 'en-US' })
  const owner = await ctx.newPage()
  expect((await owner.request.post('/api/v1/auth/login', { data: { username: 'owner29', password }, headers })).ok()).toBeTruthy()

  // The service entry shows its items, but no costs and no notes.
  await owner.goto(`/service/${String(log.id)}`)
  await expect(owner.getByRole('cell', { name: 'Oilbrand' })).toBeVisible()
  await expect(owner.getByText('Total')).toHaveCount(0)
  await expect(owner.getByText('Shop notes 29')).toHaveCount(0)
  await expect(owner.getByText('43.21')).toHaveCount(0)

  // No projects in the navigation, the vehicle tabs or by address; no plate.
  const nav = owner.getByRole('navigation', { name: 'Main navigation' })
  await expect(nav.getByRole('link', { name: 'Service Logbook' })).toBeVisible()
  await expect(nav.getByRole('link', { name: 'Self-Repair Projects' })).toHaveCount(0)
  await owner.goto(`/vehicles/${String(vehicle.id)}`)
  await expect(owner.getByRole('heading', { name: 'Seecar Visibility' })).toBeVisible()
  await expect(owner.getByRole('link', { name: 'Repairs' })).toHaveCount(0)
  await expect(owner.getByText('SEE-29')).toHaveCount(0)
  await owner.goto('/repairs')
  await expect(owner.getByText("You don't have permission to do this.")).toBeVisible()

  // Editing the entry changes the brand and keeps the price and the notes.
  await owner.goto(`/service/${String(log.id)}/edit`)
  await expect(owner.getByLabel('Cost')).toHaveCount(0)
  await owner.getByLabel('Brand').fill('Newbrand')
  await owner.getByRole('button', { name: 'Save' }).click()
  await expect(owner.getByRole('cell', { name: 'Newbrand' })).toBeVisible()
  await ctx.close()

  const saved = ((await (await page.request.get(`/api/v1/service-logs/${String(log.id)}`)).json()) as { service_log: ServiceLog }).service_log
  expect(saved.items[0]?.brand).toBe('Newbrand')
  expect(saved.items[0]?.cost_cents).toBe(4321)
  expect(saved.notes).toBe('Shop notes 29')
})

import { expect, test } from '@playwright/test'
import { apiPost, createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

interface Priority { id: number; builtin_key: string | null; custom_name: string | null }

// Show priorities with color and name again for later runs against the same instance.
test.afterEach(async ({ page }) => {
  const back = await page.request.patch('/api/v1/me/preferences', { data: { priority_display: 'both' }, headers })
  expect(back.ok()).toBeTruthy()
})

// Issue #15: per-garage priorities sort and filter projects.
test('V20: project priorities sort, filter and display', async ({ page }) => {
  await login(page)
  const vid = await createVehicle(page.request, 120000, 'Priorities')

  // Rename and recolor a built-in in the settings, then move it up.
  await page.goto('/settings/priorities')
  await page.getByRole('row', { name: /Low/ }).getByRole('button', { name: 'Edit' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Name').fill('Someday')
  await dialog.getByRole('radio', { name: 'Violet' }).click()
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('row', { name: /Someday/ })).toBeVisible()
  await page.getByRole('button', { name: 'Move Someday up' }).click()
  await expect(page.getByRole('row').nth(3)).toContainText('Someday')

  const res = await page.request.get('/api/v1/repair-priorities')
  const items = ((await res.json()) as { items: Priority[] }).items
  const id = (key: string) => String(items.find((p) => p.builtin_key === key)?.id)

  // A new project gets the default (Normal) preselected.
  await page.goto(`/vehicles/${String(vid)}/repairs`)
  await page.getByRole('button', { name: 'New project' }).click()
  await expect(page.getByRole('dialog').getByLabel('Priority')).toHaveValue(id('normal'))
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()

  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Rust repair' })
  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Wipers', priority_id: Number(id('low')) })
  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Brake line', priority_id: Number(id('urgent')) })
  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Paint chips', priority_id: null })

  // The list sorts by priority by default; projects without one come last.
  await page.reload()
  await page.getByRole('button', { name: 'List' }).click()
  const titles = page.getByRole('row').locator('td:first-child')
  await expect(titles).toHaveText(['Brake line', 'Wipers', 'Rust repair', 'Paint chips'])

  // Filter by two priorities; the choice is kept in the URL.
  await page.locator('summary', { hasText: 'Priority:' }).click()
  await page.getByRole('checkbox', { name: 'Urgent' }).check()
  await page.getByRole('checkbox', { name: 'No priority' }).check()
  await expect(titles).toHaveText(['Brake line', 'Paint chips'])
  expect(page.url()).toContain(`priority=${id('urgent')}`)

  // Color only: the name stays the accessible name.
  const pref = await page.request.patch('/api/v1/me/preferences', { data: { priority_display: 'color' }, headers })
  expect(pref.ok()).toBeTruthy()
  await page.reload()
  await expect(page.getByRole('img', { name: 'Priority: Urgent' }).first()).toBeVisible()
})

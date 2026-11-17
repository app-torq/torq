import { expect, test } from '@playwright/test'
import { apiPost, createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

// Back to the default card limit for later runs against the same instance.
test.afterEach(async ({ page }) => {
  const back = await page.request.patch('/api/v1/me/preferences', { data: { card_tag_limit: 3 }, headers })
  expect(back.ok()).toBeTruthy()
})

// Issue #16: free-form project tags, shortened on cards, filterable and
// searchable with the tags: prefix.
test('V21: project tags on cards, in the filter and in the search', async ({ page }) => {
  await login(page)
  const vid = await createVehicle(page.request, 98000, 'Tags')
  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Brake line', symptom: 'Weak braking', tags: ['brake-fluid', 'Brembo', 'brakes', 'rear axle', 'rust'] })
  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Bleed brakes', symptom: 'Spongy pedal', tags: ['brake-fluid'] })
  await apiPost(page.request, '/repairs', { vehicle_id: vid, title: 'Wipers' })

  // The card shows three tags (the default) and "+ 2 more".
  await page.goto(`/vehicles/${String(vid)}/repairs`)
  await page.getByRole('button', { name: 'Board' }).click()
  const card = page.locator('li', { hasText: 'Brake line' })
  await expect(card.getByRole('list', { name: 'Tags' }).getByRole('listitem')).toHaveText(['brake-fluid', 'brakes', 'Brembo', '+ 2 more'])

  // tags: in the search box, combined with free text.
  await page.getByRole('button', { name: 'List' }).click()
  const titles = page.getByRole('row').locator('td:first-child a')
  const search = page.getByRole('searchbox')
  await search.fill('tags:brake-fluid')
  await search.press('Enter')
  // Same priority and status: the most recently updated comes first.
  await expect(titles).toHaveText(['Bleed brakes', 'Brake line'])
  await search.fill('tags:brake-fluid spongy')
  await search.press('Enter')
  await expect(titles).toHaveText(['Bleed brakes'])
  await search.fill('')
  await search.press('Enter')

  // The tag filter keeps its choice in the URL.
  await page.locator('summary', { hasText: 'Tags:' }).click()
  await page.getByRole('checkbox', { name: 'Brembo' }).check()
  await expect(titles).toHaveText(['Brake line'])
  expect(page.url()).toContain('tag=Brembo')

  // A typed tag takes the spelling of the existing one.
  await page.goto(`/vehicles/${String(vid)}/repairs`)
  await page.getByRole('button', { name: 'New project' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill('Caliper rebuild')
  await dialog.getByLabel('Tags').fill('brembo')
  await dialog.getByLabel('Tags').press('Enter')
  await dialog.getByLabel('Tags').fill('front axle')
  await dialog.getByLabel('Tags').press('Enter')
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByRole('heading', { name: 'Caliper rebuild' })).toBeVisible()
  await expect(page.getByRole('list', { name: 'Tags' }).getByRole('listitem')).toHaveText(['Brembo', 'front axle'])

  // With "All" in the preferences, cards show every tag.
  const pref = await page.request.patch('/api/v1/me/preferences', { data: { card_tag_limit: 0 }, headers })
  expect(pref.ok()).toBeTruthy()
  await page.goto(`/vehicles/${String(vid)}/repairs`)
  await page.getByRole('button', { name: 'Board' }).click()
  await expect(card.getByRole('list', { name: 'Tags' }).getByRole('listitem')).toHaveCount(5)
})

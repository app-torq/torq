import { expect, test } from '@playwright/test'
import { createVehicle, login } from './helpers'

test.use({ viewport: { width: 360, height: 740 } })

test('pages never scroll horizontally at 360 px and focus is visible', async ({ page }) => {
  await login(page)
  const vid = await createVehicle(page.request, 120000, 'A 160')
  for (const path of ['/', '/vehicles', `/vehicles/${vid}`, `/vehicles/${vid}/specs`, '/service', `/service/new?vehicle=${vid}`, '/repairs', '/reminders', '/reports', '/settings/preferences']) {
    await page.goto(path)
    await page.waitForLoadState('networkidle')
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow, `${path} overflows by ${overflow}px`).toBeLessThanOrEqual(0)
  }

  // Keyboard users see where focus is (focus-ring utility draws a box-shadow).
  // Wait for the header to render first, otherwise Tab can land on <body>.
  await page.goto('/settings/preferences')
  await page.waitForLoadState('networkidle')
  await expect(page.getByRole('banner').getByRole('button').first()).toBeVisible()
  await page.keyboard.press('Tab')
  await page.keyboard.press('Tab')
  await expect.poll(() => page.evaluate(() => document.activeElement?.tagName)).not.toBe('BODY')
  const shadow = await page.evaluate(() => getComputedStyle(document.activeElement ?? document.body).boxShadow)
  expect(shadow).not.toBe('none')
})

test('vehicle form with quick-fill, variant pills and an open combobox fits 360 px and is labelled', async ({ page }) => {
  const { catalogKeys } = await import('./helpers')
  const keys = await catalogKeys()
  await login(page)
  for (const path of ['/vehicles/new', '/settings/about']) {
    await page.goto(path)
    await page.waitForLoadState('networkidle')
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow, `${path} overflows by ${overflow}px`).toBeLessThanOrEqual(0)
  }
  await page.goto('/vehicles/new')
  await page.getByLabel('HSN (2.1)').fill(keys.multi.key.slice(0, 4))
  await page.getByLabel('TSN (2.2)').fill(keys.multi.key.slice(5))
  await expect(page.getByRole('button', { name: 'None of these' })).toBeVisible()
  const make = page.getByRole('combobox', { name: 'Make' })
  await make.fill('a')
  await expect(page.getByRole('listbox')).toBeVisible()
  for (const name of ['Make', 'Model', 'Generation / chassis code', 'Engine / trim']) {
    await expect(page.getByRole('combobox', { name })).toBeVisible()
  }
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow, `form with pills and listbox overflows by ${overflow}px`).toBeLessThanOrEqual(0)
})

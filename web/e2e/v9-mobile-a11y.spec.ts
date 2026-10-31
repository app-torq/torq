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
  await page.goto('/settings/preferences')
  await page.keyboard.press('Tab')
  await page.keyboard.press('Tab')
  const shadow = await page.evaluate(() => getComputedStyle(document.activeElement ?? document.body).boxShadow)
  expect(shadow).not.toBe('none')
})

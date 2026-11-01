import { expect, test } from '@playwright/test'
import { login } from './helpers'

// The logo leads to the start page (dashboard) on desktop and in the mobile menu.
test('V16: the logo links to the dashboard; favicon and touch icon are served', async ({ page }) => {
  await login(page)
  const home = page.getByRole('link', { name: 'torq – go to dashboard' })

  await page.goto('/vehicles')
  await home.click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()

  await page.setViewportSize({ width: 375, height: 800 })
  await page.goto('/reminders')
  await page.getByRole('button', { name: 'Open menu' }).click()
  // Top bar logo plus the one in the open menu; the menu's is last.
  await expect(home).toHaveCount(2)
  await home.last().click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('button', { name: 'Close menu' })).toHaveCount(0)

  const favicon = await page.request.get('/favicon.svg')
  expect(favicon.ok()).toBeTruthy()
  expect(await favicon.text()).toContain('#f59e0b')
  const touch = await page.request.get('/apple-touch-icon.png')
  expect(touch.headers()['content-type']).toBe('image/png')
})

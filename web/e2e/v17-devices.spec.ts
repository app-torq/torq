import { expect, test } from '@playwright/test'
import { ADMIN, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

// Quickstart V5 (specs/003-mobile-client-api): an app sign-in shows up under
// Settings → Devices and can be signed out from the browser.
test('V17: devices list shows an app sign-in and signs it out', async ({ page, request }) => {
  const res = await request.post('/api/v1/auth/login', {
    data: { username: ADMIN.username, password: ADMIN.password, token: true, device_name: 'E2E Phone' }, headers,
  })
  expect(res.ok()).toBeTruthy()
  const { token } = (await res.json()) as { token: string }
  const asApp = { Authorization: `Bearer ${token}` }
  expect((await request.get('/api/v1/auth/me', { headers: asApp })).status()).toBe(200)

  await login(page)
  await page.goto('/settings/devices')
  await expect(page.getByRole('heading', { name: 'Devices' })).toBeVisible()
  await expect(page.getByRole('row', { name: /This device/ })).toBeVisible()
  const phone = page.getByRole('row', { name: /E2E Phone/ })
  await expect(phone).toBeVisible()

  await phone.getByRole('button', { name: 'Sign out' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Sign out' }).click()
  await expect(phone).toHaveCount(0)
  expect((await request.get('/api/v1/auth/me', { headers: asApp })).status()).toBe(401)

  // Browser sessions are named by browser and system, translated.
  await page.request.patch('/api/v1/me/preferences', { data: { locale: 'de' }, headers })
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Geräte' })).toBeVisible()
  await expect(page.getByRole('row', { name: /Dieses Gerät/ })).toContainText(/Chrome unter (macOS|Linux|Windows)/)
  await page.request.patch('/api/v1/me/preferences', { data: { locale: 'en' }, headers })
})

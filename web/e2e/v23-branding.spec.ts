import { expect, test } from '@playwright/test'
import { createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }
const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 10"><rect width="40" height="10" fill="#0ea5e9"/></svg>'

// #32: an admin sets a name, logo and contact details; they show in the
// header, the browser title, the printed history and on the sign-in page.
test('V23: instance name, logo and contact details', async ({ page }) => {
  await login(page)
  try {
    await page.goto('/settings/instance')
    await page.getByLabel(/^Name/).fill('Garage Nord')
    await page.getByLabel(/Phone/).fill('+49 40 123456')
    await page.getByRole('button', { name: 'Save' }).first().click()
    await expect(page.getByText('Saved').first()).toBeVisible()

    await page.locator('input[type="file"][accept*="svg"]').setInputFiles({ name: 'logo.svg', mimeType: 'image/svg+xml', buffer: Buffer.from(svg) })
    await expect(page.getByRole('img', { name: 'Logo' })).toBeVisible()

    const home = page.getByRole('link', { name: 'Garage Nord – go to dashboard' })
    await expect(home).toBeVisible()
    await expect(home.locator('img')).toHaveAttribute('src', /\/api\/v1\/instance\/logo\?v=/)
    await expect(page).toHaveTitle('Garage Nord · torq')

    const id = await createVehicle(page.request, 120000, 'Branding')
    await page.goto(`/vehicles/${String(id)}/history/print`)
    const header = page.locator('article header')
    await expect(header).toContainText('Garage Nord')
    await expect(header).toContainText('+49 40 123456')
    await expect(header.getByRole('img', { name: 'Garage Nord' })).toBeVisible()

    // Signed out: name and logo on the sign-in page, no contact details.
    await page.context().clearCookies()
    await page.goto('/')
    await expect(page.getByText('Garage Nord')).toBeVisible()
    await expect(page).toHaveTitle('Garage Nord · torq')
    const info = (await (await page.request.get('/api/v1/instance')).json()) as { branding: { contact?: unknown } }
    expect(info.branding.contact).toBeUndefined()
  } finally {
    await login(page)
    await page.request.patch('/api/v1/settings/instance', { data: { branding: {} }, headers })
    await page.request.delete('/api/v1/settings/instance/logo', { headers })
  }
})

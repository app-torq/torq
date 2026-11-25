import { expect, test } from '@playwright/test'
import { apiPost, login } from './helpers'

// Issue #38, part 1: a QR code per vehicle that opens it in torq, as PNG and
// as a printable label.
test('V25: vehicle QR code, PNG download and label', async ({ page }) => {
  await login(page)
  const { vehicle } = await apiPost<{ vehicle: { id: number } }>(page.request, '/vehicles', {
    make: 'Škoda', model: 'Octavia QR', license_plate: 'HH-QR 38', current_odometer_km: 120000,
  })
  const url = `/vehicles/${String(vehicle.id)}`

  await page.goto(url)
  const card = page.locator('section').filter({ has: page.getByRole('heading', { name: 'QR code' }) })
  await expect(page.getByRole('img', { name: 'QR code for Škoda Octavia QR' })).toBeVisible()
  // The code points at this vehicle under the address the browser uses.
  await expect(card.getByText(new URL(url, page.url()).href)).toBeVisible()
  // The test server runs on 127.0.0.1, which a phone can't reach.
  await expect(card.getByText(/only works on this computer/)).toBeVisible()

  const [download] = await Promise.all([page.waitForEvent('download'), card.getByRole('button', { name: 'Download PNG' }).click()])
  expect(download.suggestedFilename()).toBe('torq-skoda-octavia-qr-qr.png')
  const png = await download.createReadStream()
  const chunks: Buffer[] = []
  for await (const c of png) chunks.push(c as Buffer)
  expect(Buffer.concat(chunks).subarray(1, 4).toString()).toBe('PNG')

  await card.getByRole('link', { name: 'Print label' }).click()
  await expect(page).toHaveURL(`${url}/qr/print`)
  const label = page.locator('article')
  await expect(label.getByRole('heading', { name: 'Škoda Octavia QR' })).toBeVisible()
  await expect(label.getByText('HH-QR 38')).toBeVisible()
  await expect(label.getByRole('img', { name: 'QR code for Škoda Octavia QR' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Print' })).toBeVisible()
})

import { expect, test } from '@playwright/test'
import { apiPost, login } from './helpers'

interface Category { id: number; builtin_key: string | null }

// Issue #38, part 2: a public read-only link per vehicle, opened without an
// account, with renew/turn off on the vehicle and the admin's switch and list.
test('V26: public vehicle link', async ({ page, browser }) => {
  await login(page)
  const { vehicle } = await apiPost<{ vehicle: { id: number } }>(page.request, '/vehicles', {
    make: 'Volvo', model: 'V70 Share', model_year: 2008, license_plate: 'HH-SH 26', vin: 'YV1SW71K481234567', current_odometer_km: 201000,
  })
  const vid = vehicle.id
  const cats = ((await (await page.request.get('/api/v1/service-categories')).json()) as { items: Category[] }).items
  const oil = cats.find((c) => c.builtin_key === 'engine_oil')?.id
  await apiPost(page.request, '/service-logs', {
    vehicle_id: vid, service_date: '2026-10-01', odometer_km: 200500, notes: 'Secret note about the seller',
    items: [{ category_id: oil, brand: 'Motul', specification: '5W-30', cost_cents: 7450 }],
  })
  await apiPost(page.request, '/reminders', { vehicle_id: vid, service_category_id: oil, interval_km: 15000, interval_months: 12 })

  // Turn the link on from the vehicle's QR card.
  await page.goto(`/vehicles/${String(vid)}`)
  const card = page.locator('section').filter({ has: page.getByRole('heading', { name: 'QR code' }) })
  await card.getByRole('button', { name: 'Turn on' }).click()
  const address = (await card.locator('code').textContent()) ?? ''
  expect(address).toMatch(/\/s\/[\w-]{43}$/)

  // A visitor without an account sees the record, but nothing private.
  const visitor = await browser.newContext({ locale: 'en-US' })
  const guest = await visitor.newPage()
  await guest.goto(address)
  await expect(guest.getByRole('heading', { name: 'Volvo V70 Share' })).toBeVisible()
  await expect(guest.getByText(/Motul · 5W-30/)).toBeVisible()
  await expect(guest.getByRole('heading', { name: 'Upcoming maintenance' })).toBeVisible()
  await expect(guest.getByText('Engine oil').first()).toBeVisible()
  for (const hidden of ['HH-SH 26', 'YV1SW71K481234567', 'Secret note', '74.50', 'Marek']) {
    await expect(guest.getByText(hidden)).toHaveCount(0)
  }
  await expect(guest.getByRole('button')).toHaveCount(0)
  await expect(guest.getByRole('link', { name: 'Sign in to torq' })).toBeVisible()

  // Including costs and the plate shows them at the same address.
  for (const name of ['Costs', 'License plate']) {
    await card.getByRole('checkbox', { name }).click()
    await expect(card.getByRole('checkbox', { name })).toBeChecked()
  }
  await expect.poll(async () => {
    const res = await page.request.get(`/api/v1/vehicles/${String(vid)}/share`)
    return ((await res.json()) as { share: { include_costs: boolean; include_license_plate: boolean } }).share
  }).toMatchObject({ include_costs: true, include_license_plate: true })
  await guest.reload()
  await expect(guest.getByText('HH-SH 26')).toBeVisible()
  await expect(guest.getByText(/74\.50/).first()).toBeVisible()
  await expect(guest.getByText('YV1SW71K481234567')).toHaveCount(0)

  // Signed in, the page offers to open the vehicle in torq.
  await page.goto(address)
  await expect(page.getByRole('link', { name: 'Open in torq' })).toHaveAttribute('href', `/vehicles/${String(vid)}`)

  // The card and label switch to the public code.
  await page.goto(`/vehicles/${String(vid)}`)
  await card.getByRole('button', { name: 'Public link' }).click()
  await expect(card.getByText('Anyone who scans this code')).toBeVisible()
  await card.getByRole('link', { name: 'Print label' }).click()
  await expect(page).toHaveURL(`/vehicles/${String(vid)}/qr/print?code=public`)
  await expect(page.getByText('Scan to see the maintenance record')).toBeVisible()

  // Renewing retires the old address.
  await page.goto(`/vehicles/${String(vid)}`)
  await card.getByRole('button', { name: 'Renew link' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Renew link' }).click()
  await expect(card.locator('code')).not.toHaveText(address)
  const renewed = (await card.locator('code').textContent()) ?? ''
  await guest.goto(address)
  await expect(guest.getByRole('heading', { name: 'Link not found' })).toBeVisible()
  await guest.goto(renewed)
  await expect(guest.getByRole('heading', { name: 'Volvo V70 Share' })).toBeVisible()

  // The admin lists the link; the instance switch turns every link off.
  await page.goto('/settings/instance')
  const admin = page.locator('section').filter({ has: page.getByRole('heading', { name: 'Public links' }) })
  await expect(admin.getByRole('link', { name: 'Volvo V70 Share' })).toBeVisible()
  await admin.getByRole('checkbox', { name: 'Allow public links' }).click()
  await expect(admin.getByText(/All public links are turned off/)).toBeVisible()
  await expect(admin.getByRole('checkbox', { name: 'Allow public links' })).not.toBeChecked()
  await guest.reload()
  await expect(guest.getByRole('heading', { name: 'Link not found' })).toBeVisible()
  await admin.getByRole('checkbox', { name: 'Allow public links' }).click()
  await expect(admin.getByText(/All public links are turned off/)).toHaveCount(0)
  await expect(admin.getByRole('checkbox', { name: 'Allow public links' })).toBeChecked()
  await guest.reload()
  await expect(guest.getByRole('heading', { name: 'Volvo V70 Share' })).toBeVisible()

  // Turning it off from the list ends it.
  await admin.getByRole('row', { name: /Volvo V70 Share/ }).getByRole('button', { name: 'Turn off' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Turn off' }).click()
  await expect(admin.getByRole('link', { name: 'Volvo V70 Share' })).toHaveCount(0)
  await guest.reload()
  await expect(guest.getByRole('heading', { name: 'Link not found' })).toBeVisible()
  await visitor.close()
})

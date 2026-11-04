import { readFileSync } from 'node:fs'
import { expect, test, type Page } from '@playwright/test'
import { apiPost, createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

/** Loads torq until its service worker controls the page. */
async function withWorker(page: Page) {
  await page.goto('/')
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null)
}

// Quickstart (specs/004-installable-pwa): torq is installable as a web app.
test('V18a: manifest, icons and Apple meta tags make torq installable', async ({ page, request }) => {
  await page.goto('/')
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute('href', '/manifest.webmanifest')
  const tags: [string, string][] = [
    ['apple-mobile-web-app-capable', 'yes'],
    ['mobile-web-app-capable', 'yes'],
    ['apple-mobile-web-app-status-bar-style', 'black'],
    ['apple-mobile-web-app-title', 'torq'],
  ]
  for (const [name, content] of tags) {
    await expect(page.locator(`meta[name="${name}"]`)).toHaveAttribute('content', content)
  }

  const res = await request.get('/manifest.webmanifest')
  expect(res.headers()['content-type']).toBe('application/manifest+json')
  const manifest = (await res.json()) as { name: string; display: string; start_url: string; icons: { src: string; purpose: string }[] }
  expect(manifest).toMatchObject({ name: 'torq', display: 'standalone', start_url: '/' })
  expect(manifest.icons).toHaveLength(3)
  expect(manifest.icons.filter((i) => i.purpose === 'maskable')).toHaveLength(1)
  for (const icon of manifest.icons) {
    const r = await request.get(icon.src)
    expect(r.status(), icon.src).toBe(200)
    expect(r.headers()['content-type']).toBe('image/png')
  }
})

test('V18b–c: the worker keeps only the app shell, never data', async ({ page }) => {
  await login(page)
  await withWorker(page)

  // Real use: a vehicle, a service entry with a photo, the photo itself and an export.
  const vid = await createVehicle(page.request, 120000, 'PWA Test')
  const { service_log: log } = await apiPost<{ service_log: { id: number } }>(page.request, '/service-logs', {
    vehicle_id: vid, service_date: '2026-11-01', odometer_km: 120500, items: [{ category_id: 1, cost_cents: 4500 }],
  })
  const photo = readFileSync(new URL('../public/icons/icon-192.png', import.meta.url))
  const up = await page.request.post(`/api/v1/service-logs/${String(log.id)}/attachments`, {
    headers, multipart: { file: { name: 'photo.png', mimeType: 'image/png', buffer: photo } },
  })
  expect(up.ok(), await up.text()).toBeTruthy()
  const { attachment } = (await up.json()) as { attachment: { url: string } }
  await page.goto(`/vehicles/${String(vid)}`)
  await expect(page.getByRole('heading', { name: /PWA Test/ })).toBeVisible()
  const urls: [string, string] = [attachment.url, `/api/v1/vehicles/${String(vid)}/history?format=csv`]
  await page.evaluate(async ([photoUrl, exportUrl]) => {
    await fetch(photoUrl)
    await fetch(exportUrl)
  }, urls)
  await page.goto(attachment.url)

  await page.goto('/')
  const cached = await page.evaluate(async () => {
    const urls: string[] = []
    for (const name of await caches.keys()) {
      for (const req of await (await caches.open(name)).keys()) urls.push(new URL(req.url).pathname)
    }
    return urls
  })
  expect(cached.length).toBeGreaterThan(0)
  for (const path of cached) {
    expect(path, 'only shell files may be cached').toMatch(/^\/(index\.html|manifest\.webmanifest|favicon\.svg|apple-touch-icon\.png|icons\/[\w-]+\.png|assets\/[\w.-]+)$/)
  }
})

test('V18d: an unreachable server shows the offline screen, and Try again recovers', async ({ page, context }) => {
  await login(page)
  await withWorker(page)
  await context.setOffline(true)
  await page.reload()
  await expect(page.getByRole('heading', { name: "Can't reach your torq server" })).toBeVisible()
  await context.setOffline(false)
  await page.getByRole('button', { name: 'Try again' }).click()
  await expect(page.getByRole('link', { name: 'Dashboard' }).first()).toBeVisible()
})

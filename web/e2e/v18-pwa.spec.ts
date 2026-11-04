import { expect, test } from '@playwright/test'

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

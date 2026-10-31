import { expect, test } from '@playwright/test'
import { apiPost, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

// Quickstart V6 (specs/002-vehicle-catalog-lookup): attribution for members.
test('V13: every bundled dataset is attributed on the About page, visible to members', async ({ page, browser }) => {
  await login(page)
  await apiPost(page.request, '/users', { username: 'lena', display_name: 'Lena', password: 'workshop-manual-2', role: 'member' })
  const meta = (await (await page.request.get('/api/v1/catalog/meta')).json()) as {
    datasets: { name: string; license: string; license_url: string; attribution: string; uri: string }[]
  }
  expect(meta.datasets.length).toBeGreaterThanOrEqual(2)

  const ctx = await browser.newContext()
  const member = await ctx.newPage()
  const res = await member.request.post('/api/v1/auth/login', { data: { username: 'lena', password: 'workshop-manual-2' }, headers })
  expect(res.ok()).toBeTruthy()
  await member.goto('/settings/about')
  await expect(member.getByRole('heading', { name: 'About torq' })).toBeVisible()
  await expect(member.getByRole('link', { name: 'About & data sources' })).toBeVisible()
  for (const d of meta.datasets) {
    await expect(member.getByText(d.attribution, { exact: true })).toBeVisible()
    await expect(member.getByRole('link', { name: d.license }).first()).toHaveAttribute('href', d.license_url)
  }
  await expect(member.getByText(/Kraftfahrt-Bundesamt/).first()).toBeVisible()
  await expect(member.getByRole('link', { name: 'Datenlizenz Deutschland – Namensnennung – Version 2.0' })).toBeVisible()
  await ctx.close()
})

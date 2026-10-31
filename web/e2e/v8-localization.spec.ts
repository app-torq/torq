import { expect, test } from '@playwright/test'
import { login } from './helpers'

test('V8: German with DD.MM.YYYY and comma decimals', async ({ page }) => {
  await login(page)
  await page.goto('/settings/preferences')
  await page.getByLabel('Language').selectOption('de')
  await page.getByLabel('Date format').selectOption('DD.MM.YYYY')
  await page.getByLabel('Number format').selectOption('comma')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('link', { name: 'Übersicht' })).toBeVisible()
  await expect(page.getByText('15.10.2026 · 215.000 km · 6,5 L · 89,50 €')).toBeVisible()

  await page.goto('/service')
  await expect(page.getByRole('heading', { name: 'Wartungsheft' })).toBeVisible()
  // Restore English for later runs against the same instance.
  await page.request.patch('/api/v1/me/preferences', { data: { locale: 'en', date_format: 'YYYY-MM-DD', number_format: 'point' }, headers: { 'X-Torq-Request': '1' } })
})

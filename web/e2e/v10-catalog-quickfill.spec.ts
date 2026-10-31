import { expect, test } from '@playwright/test'
import { catalogKeys, login } from './helpers'

// Quickstart V1 (specs/002-vehicle-catalog-lookup): HSN/TSN quick-fill.
test('V10: HSN/TSN quick-fill fills, offers variants, undoes and stays non-blocking', async ({ page }) => {
  const keys = await catalogKeys()
  await login(page)
  await page.goto('/vehicles/new')

  const hsn = page.getByLabel('HSN (2.1)')
  const tsn = page.getByLabel('TSN (2.2)')
  const [sHsn, sTsn] = keys.single.key.split('/') as [string, string]

  // Typing the 4th HSN digit moves focus to TSN; the 3rd TSN character looks up.
  await hsn.click()
  await page.keyboard.type(sHsn)
  await expect(tsn).toBeFocused()
  await page.keyboard.type(sTsn.toLowerCase())
  await expect(tsn).toHaveValue(sTsn)
  await expect(page.getByLabel('Make')).toHaveValue(keys.single.variant.make)
  await expect(page.getByLabel('Model', { exact: true })).toHaveValue(keys.single.variant.model)
  await expect(page.getByLabel('Make')).toHaveClass(/ring-amber-500/)

  await page.getByRole('button', { name: 'Undo fill' }).click()
  await expect(page.getByLabel('Make')).toHaveValue('')

  // Several variants: nothing changes until one is picked.
  await hsn.fill(keys.multi.key.slice(0, 4))
  await tsn.fill(keys.multi.key.slice(5))
  const first = keys.multi.variants[0]
  if (!first) throw new Error('no variant')
  const pill = page.getByRole('button', { name: new RegExp(first.trade_name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) }).first()
  await expect(pill).toBeVisible()
  await expect(page.getByLabel('Make')).toHaveValue('')
  await pill.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByLabel('Make')).toHaveValue(first.make)

  // Unknown key: hint only, and the vehicle still saves.
  const [aHsn, aTsn] = keys.absent.split('/') as [string, string]
  await hsn.fill(aHsn)
  await tsn.fill(aTsn)
  await expect(page.getByText(/not found in local catalog/)).toBeVisible()
  await page.getByLabel('Model', { exact: true }).fill('Custom build')
  await page.getByLabel('Current odometer').fill('1000')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: `${first.make} Custom build` })).toBeVisible()

  // HSN/TSN are stored and shown when editing.
  await page.goto(page.url() + '/edit')
  await expect(page.getByLabel('HSN (2.1)')).toHaveValue(aHsn)
  await expect(page.getByLabel('TSN (2.2)')).toHaveValue(aTsn)
})

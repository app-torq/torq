import { expect, test, type Page } from '@playwright/test'
import { createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

/** Tops and heights of the visible controls in each form grid, grouped into visual rows. */
async function controlRows(page: Page) {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll('.form-grid')).flatMap((grid) => {
      const rows: { top: number; height: number }[][] = []
      const controls = Array.from(grid.querySelectorAll('input:not([type=checkbox]), select')) as HTMLElement[]
      for (const el of controls) {
        if (!el.offsetParent) continue
        const r = el.getBoundingClientRect()
        const row = rows.find((x) => Math.abs((x[0]?.top ?? 0) - r.top) < 30)
        if (row) row.push({ top: r.top, height: r.height })
        else rows.push([{ top: r.top, height: r.height }])
      }
      return rows
    }),
  )
}

async function expectAligned(page: Page) {
  const rows = await controlRows(page)
  expect(rows.length).toBeGreaterThan(0)
  for (const row of rows) {
    expect(new Set(row.map((c) => Math.round(c.top))).size, JSON.stringify(row)).toBe(1)
    expect(new Set(row.map((c) => Math.round(c.height))).size, JSON.stringify(row)).toBe(1)
  }
}

// Bug report 001: controls in a row must line up even when German labels wrap
// or a neighbour is a select, a date picker or a value + unit pair.
test('V14: form controls line up row by row', async ({ page }) => {
  await login(page)
  const id = await createVehicle(page.request, 150000)
  const de = await page.request.patch('/api/v1/me/preferences', { data: { locale: 'de' }, headers })
  expect(de.ok()).toBeTruthy()
  try {
    await page.goto(`/vehicles/${String(id)}/specs`)
    await page.getByRole('button', { name: 'Daten bearbeiten' }).click()
    await expectAligned(page)

    await page.goto(`/service/new?vehicle=${String(id)}`)
    await expect(page.getByText('Verwendete Spezifikation')).toBeVisible()
    await expectAligned(page)

    await page.goto(`/vehicles/${String(id)}/reminders`)
    await page.getByRole('button', { name: 'Erinnerung hinzufügen' }).first().click()
    await expectAligned(page)
  } finally {
    // The suite shares one admin; later specs expect English.
    await page.request.patch('/api/v1/me/preferences', { data: { locale: 'en' }, headers })
  }
})

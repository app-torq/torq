import { expect, test } from '@playwright/test'
import { createVehicle, login } from './helpers'

const headers = { 'X-Torq-Request': '1' }

// Spec auto-fill: sources are shown as links, manual changes are marked and
// everything persists only on Save. The web lookup itself needs
// GEMINI_API_KEY, so the e2e server runs without it and the button is off.
test('V15: auto-filled spec values link their source and turn manual when edited', async ({ page }) => {
  await login(page)
  const id = await createVehicle(page.request, 150000)
  const put = await page.request.put(`/api/v1/vehicles/${String(id)}/specs`, {
    headers,
    data: {
      engine_oil_viscosity: '5W-30', engine_oil_spec: 'MB 229.51', tire_pressure_front_bar: 2.3,
      meta: {
        engine_oil_viscosity: { is_auto: true, source_url: 'https://www.carwiki.de/w203', source_name: 'carwiki.de' },
        tire_pressure_front_bar: { is_auto: true, source_url: 'https://www.adac.de/reifendruck', source_name: 'adac.de' },
      },
    },
  })
  expect(put.ok(), await put.text()).toBeTruthy()

  await page.goto(`/vehicles/${String(id)}/specs`)
  await page.getByRole('button', { name: 'Edit specs' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('button', { name: 'Look up data automatically' })).toBeDisabled()
  const source = dialog.getByRole('link', { name: /source: carwiki.de/ })
  await expect(source).toHaveAttribute('href', 'https://www.carwiki.de/w203')
  await expect(source).toHaveAttribute('target', '_blank')
  await expect(source).toHaveText('Auto · carwiki.de')

  // Editing one value marks only that field; Cancel discards it.
  await dialog.getByRole('textbox', { name: 'Viscosity' }).fill('0W-30')
  await expect(dialog.getByText('Adjusted manually')).toBeVisible()
  await expect(dialog.getByRole('link', { name: /source: adac.de/ })).toBeVisible()
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await page.getByRole('button', { name: 'Edit specs' }).click()
  await expect(dialog.getByRole('link', { name: /source: carwiki.de/ })).toBeVisible()

  // Save persists the manual mark.
  await dialog.getByRole('textbox', { name: 'Viscosity' }).fill('0W-30')
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeHidden()
  const res = await page.request.get(`/api/v1/vehicles/${String(id)}/specs`)
  const { specs } = (await res.json()) as { specs: { engine_oil_viscosity: string; meta: Record<string, unknown> } }
  expect(specs.engine_oil_viscosity).toBe('0W-30')
  expect(specs.meta).toEqual({
    engine_oil_viscosity: { is_auto: false },
    tire_pressure_front_bar: { is_auto: true, source_url: 'https://www.adac.de/reifendruck', source_name: 'adac.de' },
  })
})

import { expect, test, type APIRequestContext } from '@playwright/test'
import { login } from './helpers'

interface Items<T> {
  items: T[]
}

async function get<T>(request: APIRequestContext, path: string, query: Record<string, string>): Promise<T[]> {
  const res = await request.get(`/api/v1${path}?${new URLSearchParams(query).toString()}`)
  expect(res.ok()).toBeTruthy()
  return ((await res.json()) as Items<T>).items
}

// Quickstart V2 (specs/002-vehicle-catalog-lookup): cascade + free text.
test('V11: keyboard-only cascade fills engine data; custom values save as typed', async ({ page }) => {
  await login(page)

  // Model names follow the language; generations are chassis codes.
  const en = await get<{ name: string }>(page.request, '/catalog/models', { make: 'Mercedes-Benz', q: 'c klasse', lang: 'en' })
  const de = await get<{ name: string }>(page.request, '/catalog/models', { make: 'Mercedes-Benz', q: 'c class', lang: 'de' })
  expect(en.map((m) => m.name)).toContain('C-Class')
  expect(de.map((m) => m.name)).toContain('C-Klasse')
  expect(de.map((m) => m.name)).not.toContain('C-Class')
  const codes = await get<{ name: string; body: string }>(page.request, '/catalog/generations', { make: 'Mercedes-Benz', model: 'C-Klasse' })
  expect(codes).toEqual(expect.arrayContaining([{ name: 'W203', body: '', year_from: 2000, year_to: 2007 }, expect.objectContaining({ name: 'S203', body: 'estate' })]))

  // Discover a make → model → generation → engine path with kW data.
  const make = 'Volkswagen'
  const models = await get<{ name: string }>(page.request, '/catalog/models', { make, q: 'golf' })
  const model = models.find((m) => m.name === 'Golf')?.name ?? models[0]?.name
  if (!model) throw new Error('catalog has no Golf')
  const gens = await get<{ name: string }>(page.request, '/catalog/generations', { make, model })
  let path: { generation: string; engine: { name: string; power_kw: number | null; fuel_type: string | null } } | undefined
  for (const g of gens) {
    const engines = await get<{ name: string; power_kw: number | null; fuel_type: string | null }>(page.request, '/catalog/engines', { make, model, generation: g.name })
    const e = engines.find((x) => x.power_kw != null && x.fuel_type != null)
    if (e) { path = { generation: g.name, engine: e }; break }
  }
  if (!path) throw new Error('no engine with power data')

  await page.goto('/vehicles/new')
  const pick = async (label: string, text: string, option: string) => {
    const box = page.getByRole('combobox', { name: label })
    await box.focus()
    await page.keyboard.type(text)
    const opt = page.getByRole('option', { name: option, exact: false }).first()
    await expect(opt).toBeVisible()
    // Focus first shows the unfiltered list; wait until the debounced search for the typed text replaced it.
    await expect
      .poll(async () => (await page.locator('[role="listbox"] [role="option"]').allTextContents()).every((o) => o.toLowerCase().includes(text.toLowerCase())))
      .toBe(true)
    // Move to the option with the arrow keys, then Enter.
    const count = await page.locator('[role="listbox"] [role="option"]').count()
    for (let i = 0; i < count; i++) {
      const selected = await page.locator('[role="option"][aria-selected="true"]').textContent()
      // The option text is the name, then details in parentheses ("Golf (1974–)"); "Golf Plus" must not match "Golf".
      if (selected === option || selected?.startsWith(`${option} (`)) break
      await page.keyboard.press('ArrowDown')
    }
    await page.keyboard.press('Enter')
    await expect(box).toHaveValue(option)
  }

  await pick('Make', 'volks', make)
  await pick('Model', model.slice(0, 3), model)
  await pick('Generation / chassis code', path.generation.slice(0, 4), path.generation)
  await pick('Engine / trim', path.engine.name.slice(0, 3), path.engine.name)
  await expect(page.getByLabel('Power')).toHaveValue(String(path.engine.power_kw))
  await expect(page.getByLabel('Power')).toHaveClass(/ring-amber-500/)

  // Changing the make clears the lower levels but keeps technical values.
  const makeBox = page.getByRole('combobox', { name: 'Make' })
  await makeBox.fill('BMW')
  await page.keyboard.press('Tab')
  await expect(page.getByRole('combobox', { name: 'Model' })).toHaveValue('')
  await expect(page.getByRole('combobox', { name: 'Engine / trim' })).toHaveValue('')
  await expect(page.getByLabel('Power')).toHaveValue(String(path.engine.power_kw))

  // All-custom vehicle: every level accepts free text.
  await makeBox.fill('Trabbi Custom')
  await expect(page.getByRole('option', { name: "Use custom: 'Trabbi Custom'" })).toBeVisible()
  await page.keyboard.press('Enter')
  await page.getByRole('combobox', { name: 'Model' }).fill('601 Kombi Swap')
  await page.getByRole('combobox', { name: 'Generation / chassis code' }).fill('P601K')
  await page.getByLabel('Current odometer').fill('42000')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'Trabbi Custom 601 Kombi Swap' })).toBeVisible()
})

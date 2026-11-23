import { describe, expect, it } from 'vitest'
import { createFormatter, type FormatPrefs } from '../../lib/format'
import type { CatalogEngine } from '../../lib/types'
import { versionLabel, yearRange } from './catalog'

const units = (u: string) => ({ cc: 'cm³', ci: 'ci', kw: 'kW', ps: 'PS', hp: 'hp' })[u] ?? u
const nbsp = (s: string) => s.replace(/[\u00a0\u202f]/g, ' ')
const prefs: FormatPrefs = { dateFormat: 'YYYY-MM-DD', numberFormat: 'point', currencyCode: 'EUR', currencySymbol: null }

const monster: CatalogEngine = {
  name: '821 cm³ 82 kW', engine_code: '', displacement_cc: 821, power_kw: 82, fuel_type: 'petrol', year_from: 2014, year_to: 2020,
}

describe('versionLabel', () => {
  it('shows displacement, power and years in the user units', () => {
    expect(nbsp(versionLabel(monster, createFormatter(prefs, units)))).toBe('821 cm³ · 82 kW · 2014–2020')
    expect(nbsp(versionLabel(monster, createFormatter({ ...prefs, powerUnit: 'hp', displacementUnit: 'ci' }, units))))
      .toBe('50 ci · 110 hp · 2014–2020')
  })

  it('leaves out what a version lacks', () => {
    const electric = { ...monster, displacement_cc: null, year_to: null }
    expect(nbsp(versionLabel(electric, createFormatter(prefs, units)))).toBe('82 kW · 2014–')
    const bare = { ...monster, displacement_cc: null, power_kw: null, year_from: null, year_to: null }
    expect(versionLabel(bare, createFormatter(prefs, units))).toBe('821 cm³ 82 kW')
  })
})

describe('yearRange', () => {
  it('formats open, closed and single years', () => {
    expect(yearRange(2014, 2020)).toBe('2014–2020')
    expect(yearRange(2014, null)).toBe('2014–')
    expect(yearRange(2019, 2019)).toBe('2019')
    expect(yearRange(null, 2020)).toBe('')
  })
})

import { describe, expect, it } from 'vitest'
import { createFormatter, kwToPs, parseMoneyToCents, parseNumber, toInputValue, type FormatPrefs } from './format'

const units = (u: string) => ({ km: 'km', l: 'L', ml: 'mL', nm: 'Nm', bar: 'bar', cc: 'cm³', kw: 'kW', ps: 'PS' })[u] ?? u
const nbsp = (s: string) => s.replace(/[\u00a0\u202f]/g, ' ')

const comma: FormatPrefs = { dateFormat: 'DD.MM.YYYY', numberFormat: 'comma', currencyCode: 'EUR', currencySymbol: null }
const point: FormatPrefs = { dateFormat: 'YYYY-MM-DD', numberFormat: 'point', currencyCode: 'USD', currencySymbol: null }

describe('createFormatter', () => {
  it('formats with comma decimals', () => {
    const f = createFormatter(comma, units)
    expect(nbsp(f.km(215000))).toBe('215.000 km')
    expect(nbsp(f.money(8950))).toBe('89,50 €')
    expect(f.date('2026-10-15')).toBe('15.10.2026')
    expect(nbsp(f.liters(6500))).toBe('6,5 L')
    expect(nbsp(f.bar(2.3))).toBe('2,3 bar')
  })

  it('formats with point decimals', () => {
    const f = createFormatter(point, units)
    expect(nbsp(f.km(215000))).toBe('215,000 km')
    expect(nbsp(f.money(8950))).toBe('$89.50')
    expect(f.date('2026-10-15')).toBe('2026-10-15')
    expect(nbsp(f.liters(6500))).toBe('6.5 L')
    expect(nbsp(f.nm(7.5))).toBe('7.5 Nm')
  })

  it('replaces the currency part with a custom symbol', () => {
    const f = createFormatter({ ...comma, currencyCode: 'CHF', currencySymbol: 'CHF' }, units)
    expect(nbsp(f.money(12345))).toBe('123,45 CHF')
  })

  it('formats power with derived PS and displacement in cm³', () => {
    const en = createFormatter(point, units)
    const de = createFormatter(comma, units)
    expect(nbsp(en.power(110))).toBe('110 kW (150 PS)')
    expect(nbsp(en.power(1))).toBe('1 kW (1 PS)')
    expect(nbsp(de.power(1500))).toBe('1.500 kW (2.039 PS)')
    expect(nbsp(de.displacement(2148))).toBe('2.148 cm³')
    expect(nbsp(en.displacement(2148))).toBe('2,148 cm³')
    expect(kwToPs(105)).toBe(143)
  })

  it('keeps the entered quantity unit', () => {
    const f = createFormatter(point, units)
    expect(nbsp(f.quantity(500, 'mL'))).toBe('500 mL')
    expect(nbsp(f.quantity(6500, 'L'))).toBe('6.5 L')
  })
})

describe('parseNumber', () => {
  it('parses comma format', () => {
    expect(parseNumber('1.234,5', 'comma')).toBe(1234.5)
    expect(parseNumber('6,5', 'comma')).toBe(6.5)
    expect(parseNumber('215.000', 'comma')).toBe(215000)
    expect(parseNumber('6.5', 'comma')).toBe(6.5)
  })
  it('parses point format', () => {
    expect(parseNumber('1,234.5', 'point')).toBe(1234.5)
    expect(parseNumber('', 'point')).toBeNull()
    expect(parseNumber('abc', 'point')).toBeNull()
  })
  it('converts money to cents without float drift', () => {
    expect(parseMoneyToCents('89,50', 'comma')).toBe(8950)
    expect(parseMoneyToCents('0.29', 'point')).toBe(29)
  })
  it('renders input values', () => {
    expect(toInputValue(6.5, 'comma')).toBe('6,5')
    expect(toInputValue(null, 'point')).toBe('')
  })
})

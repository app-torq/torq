import { describe, expect, it } from 'vitest'
import { createFormatter, kwToPs, parseMoneyToCents, parseNumber, toInputValue, type FormatPrefs } from './format'

const units = (u: string) =>
  ({ km: 'km', mi: 'mi', l: 'L', ml: 'mL', qt: 'qt', gal: 'gal', fl_oz: 'fl oz', nm: 'Nm', lbft: 'lb-ft', bar: 'bar', psi: 'psi', kpa: 'kPa', cc: 'cm³', ci: 'ci', kw: 'kW', ps: 'PS', hp: 'hp' })[u] ?? u
const nbsp = (s: string) => s.replace(/[\u00a0\u202f]/g, ' ')

const comma: FormatPrefs = { dateFormat: 'DD.MM.YYYY', numberFormat: 'comma', currencyCode: 'EUR', currencySymbol: null }
const point: FormatPrefs = { dateFormat: 'YYYY-MM-DD', numberFormat: 'point', currencyCode: 'USD', currencySymbol: null }

describe('createFormatter', () => {
  it('formats with comma decimals', () => {
    const f = createFormatter(comma, units)
    expect(nbsp(f.distance(215000, 'km'))).toBe('215.000 km')
    expect(nbsp(f.money(8950))).toBe('89,50 €')
    expect(f.date('2026-10-15')).toBe('15.10.2026')
    expect(nbsp(f.volume(6500))).toBe('6,5 L')
    expect(nbsp(f.pressure(2.3))).toBe('2,3 bar')
    expect(nbsp(f.pressureRange(2.2, 2.4))).toBe('2,2–2,4 bar')
    expect(nbsp(f.pressureRange(2.5, null))).toBe('2,5 bar')
  })

  it('formats with point decimals', () => {
    const f = createFormatter(point, units)
    expect(nbsp(f.distance(215000, 'km'))).toBe('215,000 km')
    expect(nbsp(f.money(8950))).toBe('$89.50')
    expect(f.date('2026-10-15')).toBe('2026-10-15')
    expect(nbsp(f.volume(6500))).toBe('6.5 L')
    expect(nbsp(f.torque(7.5))).toBe('7.5 Nm')
  })

  it('replaces the currency part with a custom symbol', () => {
    const f = createFormatter({ ...comma, currencyCode: 'CHF', currencySymbol: 'CHF' }, units)
    expect(nbsp(f.money(12345))).toBe('123,45 CHF')
  })

  it('formats power with derived PS and displacement in cm³', () => {
    const en = createFormatter(point, units)
    const de = createFormatter(comma, units)
    expect(nbsp(en.power(110000))).toBe('110 kW (150 PS)')
    expect(nbsp(en.power(1000))).toBe('1 kW (1 PS)')
    expect(nbsp(de.power(1500000))).toBe('1.500 kW (2.039 PS)')
    expect(nbsp(de.displacement(2148))).toBe('2.148 cm³')
    expect(nbsp(en.displacement(2148))).toBe('2,148 cm³')
    expect(kwToPs(105)).toBe(143)
    expect(nbsp(en.powerShort(110000))).toBe('110 kW')
  })

  it('shows PS first when preferred', () => {
    const en = createFormatter({ ...point, powerUnit: 'ps' }, units)
    const de = createFormatter({ ...comma, powerUnit: 'ps' }, units)
    expect(nbsp(en.power(110000))).toBe('150 PS (110 kW)')
    expect(nbsp(de.power(1500000))).toBe('2.039 PS (1.500 kW)')
    expect(nbsp(en.powerShort(105000))).toBe('143 PS')
  })

  it('keeps the entered quantity unit', () => {
    const f = createFormatter(point, units)
    expect(nbsp(f.quantity(500, 'mL'))).toBe('500 mL')
    expect(nbsp(f.quantity(6500, 'L'))).toBe('6.5 L')
    expect(nbsp(f.quantity(4732, 'qt'))).toBe('5 qt')
    expect(nbsp(f.quantity(946, 'fl_oz'))).toBe('32 fl oz')
  })

  it('converts to US units for display', () => {
    const us = createFormatter(
      { ...point, dateFormat: 'MM/DD/YYYY', timeFormat: '12h', powerUnit: 'hp', volumeUnit: 'qt', torqueUnit: 'lbft', pressureUnit: 'psi', displacementUnit: 'ci' },
      units,
    )
    expect(nbsp(us.distance(215000, 'mi'))).toBe('133,595 mi')
    expect(us.date('2026-10-15')).toBe('10/15/2026')
    expect(nbsp(us.dateTime('2026-10-31T21:05:00'))).toBe('10/31/2026 9:05 PM')
    expect(nbsp(us.volume(6500))).toBe('6.87 qt')
    expect(nbsp(us.torque(120))).toBe('88.5 lb-ft')
    expect(nbsp(us.pressureRange(2.2, 2.4))).toBe('31.9–34.8 psi')
    expect(nbsp(us.displacement(5735))).toBe('350 ci')
    expect(nbsp(us.power(224456))).toBe('301 hp (224 kW)')
    // A vehicle's own units win over the preference (#18).
    expect(nbsp(us.displacement(2998, 'cc'))).toBe('2,998 cm³')
    expect(nbsp(us.power(239370, 'ps'))).toBe('325 PS (239 kW)')
    expect(nbsp(us.power(224456, null))).toBe('301 hp (224 kW)')
    expect(nbsp(createFormatter({ ...point, displacementUnit: 'l', pressureUnit: 'kpa' }, units).displacement(1998))).toBe('2 L')
    expect(nbsp(createFormatter({ ...point, pressureUnit: 'kpa' }, units).pressure(2.3))).toBe('230 kPa')
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

// Locale-aware formatting driven by the user's preferences (Constitution V).
// Values arrive in metric units and are converted to the preferred display
// units here (Constitution 3.0.0, "Metric storage"); see units.ts.

import type { DateFormat, NumberFormat, TimeFormat } from './types'
import {
  DISPLACEMENT,
  DISTANCE,
  POWER,
  PRESSURE,
  QUANTITY,
  TORQUE,
  VOLUME,
  fromMetric,
  toMetric,
  type DisplacementUnit,
  type DistanceUnit,
  type PowerUnit,
  type PressureUnit,
  type QuantityUnit,
  type TorqueUnit,
  type UnitKey,
  type UnitSpec,
  type VolumeUnit,
} from './units'

export interface FormatPrefs {
  dateFormat: DateFormat
  numberFormat: NumberFormat
  currencyCode: string
  currencySymbol: string | null
  /** Display units; each defaults to the metric unit. */
  timeFormat?: TimeFormat
  /** Primary power unit; kW or PS is shown as secondary info. */
  powerUnit?: PowerUnit
  /** For distances not tied to a vehicle (vehicle distances use its odometer unit). */
  distanceUnit?: DistanceUnit
  volumeUnit?: VolumeUnit
  torqueUnit?: TorqueUnit
  pressureUnit?: PressureUnit
  displacementUnit?: DisplacementUnit
}

export type { UnitKey }
export type UnitLabel = (unit: UnitKey) => string

export const defaultPrefs: FormatPrefs = {
  dateFormat: 'YYYY-MM-DD',
  numberFormat: 'point',
  currencyCode: 'EUR',
  currencySymbol: null,
  powerUnit: 'kw',
}

export function numberLocale(numberFormat: NumberFormat): string {
  return numberFormat === 'comma' ? 'de-DE' : 'en-US'
}

export interface Formatter {
  number: (n: number, opts?: { decimals?: number; maxDecimals?: number }) => string
  money: (cents: number) => string
  /** Formats fractional cents per km, e.g. 8 → "€0.08/km" style money value without the /km suffix. */
  moneyPrecise: (cents: number, decimals: number) => string
  date: (iso: string) => string
  /** Date per preferences plus local time, e.g. "31.10.2026 21:05" or "10/31/2026 9:05 PM". */
  dateTime: (iso: string) => string
  /** A distance in km shown in the given unit (a vehicle's odometer unit), e.g. "133,595 mi". */
  distance: (km: number, unit: DistanceUnit) => string
  /** A capacity in mL in the preferred volume unit, e.g. "6.5 L" or "6.87 qt". */
  volume: (ml: number) => string
  /** Formats a fluid quantity the way the user entered it. */
  quantity: (ml: number, unit: QuantityUnit) => string
  torque: (nm: number) => string
  pressure: (bar: number) => string
  /** A pressure or a pressure range, e.g. "2,2–2,4 bar". */
  pressureRange: (lo: number, hi: number | null) => string
  /** Engine displacement, e.g. "2.148 cm³", "2.1 L" or "131 ci"; unit (a vehicle's) overrides the preference. */
  displacement: (cc: number, unit?: DisplacementUnit | null) => string
  /**
   * Power in watts in the preferred unit with kW or PS as secondary info, e.g. "110 kW (150 PS)" or
   * "301 hp (224 kW)"; unit (a vehicle's) overrides the preference.
   */
  power: (w: number, unit?: PowerUnit | null) => string
  /** Power in watts in the preferred unit only, e.g. "150 PS". */
  powerShort: (w: number) => string
  /** Hour meter hours, e.g. "1,250.4 h"; never converted. */
  hours: (h: number) => string
}

/** Metric horsepower (PS) from kW. */
export function kwToPs(kw: number): number {
  return Math.round(fromMetric(kw * 1000, POWER.ps))
}

/** A vehicle's power in watts, falling back to whole kW. */
export function powerW(v: { power_w?: number | null; power_kw: number | null }): number | null {
  if (v.power_w !== undefined && v.power_w !== null) return v.power_w
  return v.power_kw === null ? null : v.power_kw * 1000
}

export function createFormatter(prefs: FormatPrefs, unit: UnitLabel): Formatter {
  const locale = numberLocale(prefs.numberFormat)
  const numberFormats = new Map<string, Intl.NumberFormat>()
  const nf = (min: number, max: number) => {
    const key = `${min}:${max}`
    let f = numberFormats.get(key)
    if (!f) {
      f = new Intl.NumberFormat(locale, { minimumFractionDigits: min, maximumFractionDigits: max })
      numberFormats.set(key, f)
    }
    return f
  }

  const currency = (decimals: number) => {
    try {
      return new Intl.NumberFormat(locale, {
        style: 'currency',
        currency: prefs.currencyCode,
        currencyDisplay: 'narrowSymbol',
        minimumFractionDigits: decimals,
        maximumFractionDigits: decimals,
      })
    } catch {
      return new Intl.NumberFormat(locale, { style: 'currency', currency: 'EUR', currencyDisplay: 'narrowSymbol' })
    }
  }
  const money2 = currency(2)

  const formatMoney = (f: Intl.NumberFormat, value: number) => {
    const symbol = prefs.currencySymbol
    if (!symbol) return f.format(value)
    return f
      .formatToParts(value)
      .map((p) => (p.type === 'currency' ? symbol : p.value))
      .join('')
  }

  const dateParts = new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: 'UTC',
  })

  const timeFormat =
    prefs.timeFormat === '12h'
      ? new Intl.DateTimeFormat('en-US', { hour: 'numeric', minute: '2-digit', hourCycle: 'h12' })
      : new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })

  const number = (n: number, opts?: { decimals?: number; maxDecimals?: number }) => {
    const min = opts?.decimals ?? 0
    const max = opts?.maxDecimals ?? opts?.decimals ?? 0
    return nf(min, Math.max(min, max)).format(n)
  }

  const formatDate = (iso: string) => {
    const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso)
    if (!m) return iso
    const parts = dateParts.formatToParts(new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))))
    const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? ''
    switch (prefs.dateFormat) {
      case 'DD.MM.YYYY':
        return [get('day'), get('month'), get('year')].join('.')
      case 'MM/DD/YYYY':
        return [get('month'), get('day'), get('year')].join('/')
      default:
        return [get('year'), get('month'), get('day')].join('-')
    }
  }

  const inUnit = (metric: number, spec: UnitSpec) =>
    `${number(fromMetric(metric, spec), { maxDecimals: spec.decimals })} ${unit(spec.label)}`

  return {
    number,
    money: (cents) => formatMoney(money2, cents / 100),
    moneyPrecise: (cents, decimals) => formatMoney(currency(decimals), cents / 100),
    date: formatDate,
    dateTime: (iso) => {
      const d = new Date(iso)
      if (Number.isNaN(d.getTime())) return iso
      const local = `${String(d.getFullYear())}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
      return `${formatDate(local)} ${timeFormat.format(d)}`
    },
    distance: (km, u) => inUnit(km, DISTANCE[u]),
    volume: (ml) => inUnit(ml, VOLUME[prefs.volumeUnit ?? 'l']),
    quantity: (ml, u) => inUnit(ml, QUANTITY[u]),
    torque: (nm) => inUnit(nm, TORQUE[prefs.torqueUnit ?? 'nm']),
    pressure: (bar) => inUnit(bar, PRESSURE[prefs.pressureUnit ?? 'bar']),
    pressureRange: (lo, hi) => {
      const spec = PRESSURE[prefs.pressureUnit ?? 'bar']
      const v = (n: number) => number(fromMetric(n, spec), { maxDecimals: spec.decimals })
      return hi === null || hi === lo ? `${v(lo)} ${unit(spec.label)}` : `${v(lo)}–${v(hi)} ${unit(spec.label)}`
    },
    displacement: (cc, u) => inUnit(cc, DISPLACEMENT[u ?? prefs.displacementUnit ?? 'cc']),
    power: (w, u) => {
      const primary = u ?? prefs.powerUnit ?? 'kw'
      const secondary = primary === 'kw' ? 'ps' : 'kw'
      return `${inUnit(w, POWER[primary])} (${inUnit(w, POWER[secondary])})`
    },
    powerShort: (w) => inUnit(w, POWER[prefs.powerUnit ?? 'kw']),
    hours: (h) => `${number(h, { maxDecimals: 1 })} ${unit('h')}`,
  }
}

/** Parses user input like "1.234,5" (comma) or "1,234.5" (point). Returns null when invalid or empty. */
export function parseNumber(input: string, numberFormat: NumberFormat): number | null {
  const s = input.trim().replace(/[\s\u00a0\u202f']/g, '')
  if (s === '') return null
  const [group, decimal] = numberFormat === 'comma' ? ['.', ','] : [',', '.']
  let normalized = s.split(group).join('')
  // Be forgiving: in comma locales also accept a lone '.' as decimal separator when no comma is present
  // and the part after it is not exactly three digits (which would be a thousands group).
  if (numberFormat === 'comma' && !s.includes(',') && /^\d+\.\d{1,2}$|^\d+\.\d{4,}$/.test(s)) {
    normalized = s
  } else {
    normalized = normalized.replace(decimal, '.')
  }
  if (!/^-?\d+(\.\d+)?$/.test(normalized)) return null
  return Number(normalized)
}

/** Converts a money input string to integer cents. */
export function parseMoneyToCents(input: string, numberFormat: NumberFormat): number | null {
  const n = parseNumber(input, numberFormat)
  return n === null ? null : Math.round(n * 100)
}

/** Formats a number for an input field (no grouping), using the user's decimal separator. */
export function toInputValue(n: number | null | undefined, numberFormat: NumberFormat, maxDecimals = 3): string {
  if (n === null || n === undefined) return ''
  const s = String(Number(n.toFixed(maxDecimals)))
  return numberFormat === 'comma' ? s.replace('.', ',') : s
}

/** The largest hour meter value, 999,999.9 h. */
export const MAX_HOURS = 999999.9

/**
 * Parses typed hours ("1.250,4" or "1,250.4") rounded to a tenth: null for
 * empty input, undefined when it does not parse or is out of range.
 */
export function parseHours(raw: string, nf: NumberFormat): number | null | undefined {
  if (raw.trim() === '') return null
  const n = parseNumber(raw, nf)
  if (n === null || n < 0 || n > MAX_HOURS) return undefined
  return Math.round(n * 10) / 10
}

/** Input text for hours. */
export function hoursToInput(h: number | null | undefined, nf: NumberFormat): string {
  return toInputValue(h, nf, 1)
}

/** Input text for a stored metric value in the display unit. */
export function metricToInput(value: number | null | undefined, spec: UnitSpec, nf: NumberFormat): string {
  if (value === null || value === undefined) return ''
  return toInputValue(fromMetric(value, spec), nf, spec.decimals)
}

/**
 * Parses input in the display unit into the stored metric value: null for
 * empty input, undefined when it does not parse.
 */
export function inputToMetric(raw: string, spec: UnitSpec, nf: NumberFormat, integer: boolean): number | null | undefined {
  if (raw.trim() === '') return null
  const n = parseNumber(raw, nf)
  return n === null ? undefined : toMetric(n, spec, integer)
}

/**
 * Parses a whole, non-negative distance typed in unit (an odometer reading,
 * an interval) into km; null when it is empty or invalid.
 */
export function parseDistance(raw: string, unit: DistanceUnit, nf: NumberFormat): number | null {
  const n = parseNumber(raw, nf)
  if (n === null || n < 0 || !Number.isInteger(n)) return null
  return toMetric(n, DISTANCE[unit], true)
}

/** Input text for a distance in km, in unit. */
export function distanceInput(km: number | null | undefined, unit: DistanceUnit, nf: NumberFormat): string {
  return metricToInput(km, DISTANCE[unit], nf)
}

/**
 * A converted input field: its initial text and the stored value it shows.
 * Reading back an untouched field returns that stored value, so rounding to
 * the display unit (1998 cm³ shown as 2.0 L) never changes data that was not
 * edited.
 */
export interface MetricField {
  text: string
  value: number | null
}

export function metricField(value: number | null | undefined, spec: UnitSpec, nf: NumberFormat): MetricField {
  return { text: metricToInput(value, spec, nf), value: value ?? null }
}

/** The stored value for raw input: unchanged text keeps the initial value; undefined when it does not parse. */
export function readMetricField(raw: string, initial: MetricField, spec: UnitSpec, nf: NumberFormat, integer: boolean): number | null | undefined {
  if (raw.trim() === initial.text.trim()) return initial.value
  return inputToMetric(raw, spec, nf, integer)
}

/** Like parseDistance, but unchanged text keeps the initial km. */
export function readDistance(raw: string, initial: MetricField, unit: DistanceUnit, nf: NumberFormat): number | null {
  if (raw.trim() !== '' && raw.trim() === initial.text.trim()) return initial.value
  return parseDistance(raw, unit, nf)
}

export function distanceField(km: number | null | undefined, unit: DistanceUnit, nf: NumberFormat): MetricField {
  return metricField(km, DISTANCE[unit], nf)
}

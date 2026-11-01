// Locale-aware formatting driven by the user's preferences (Constitution V).
// Units are always metric. The only derived unit is PS from stored kW, for
// display (Hard Non-Goals, "Metric only").

import type { DateFormat, NumberFormat, PowerUnit } from './types'

export interface FormatPrefs {
  dateFormat: DateFormat
  numberFormat: NumberFormat
  currencyCode: string
  currencySymbol: string | null
  /** Primary power unit; the other one is shown as secondary info. Defaults to kW. */
  powerUnit?: PowerUnit
}

export type UnitKey = 'km' | 'l' | 'ml' | 'nm' | 'bar' | 'cc' | 'kw' | 'ps'
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
  /** Date per preferences plus local time, e.g. "31.10.2026 21:05". */
  dateTime: (iso: string) => string
  km: (n: number) => string
  liters: (ml: number) => string
  ml: (n: number) => string
  nm: (n: number) => string
  bar: (n: number) => string
  /** A pressure or a pressure range, e.g. "2,2–2,4 bar". */
  barRange: (lo: number, hi: number | null) => string
  /** Formats a fluid quantity the way the user entered it. */
  quantity: (ml: number, unit: 'L' | 'mL') => string
  /** Engine displacement, e.g. "2.148 cm³". */
  displacement: (cc: number) => string
  /** Power in the preferred unit with the other as secondary info, e.g. "110 kW (150 PS)" or "150 PS (110 kW)". */
  power: (kw: number) => string
  /** Power in the preferred unit only, e.g. "150 PS". */
  powerShort: (kw: number) => string
}

/** Metric horsepower (PS) for display only; PS is never stored or entered (FR-024). */
export function kwToPs(kw: number): number {
  return Math.round(kw * 1.35962)
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

  const timeFormat = new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })

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
    return prefs.dateFormat === 'DD.MM.YYYY'
      ? [get('day'), get('month'), get('year')].join('.')
      : [get('year'), get('month'), get('day')].join('-')
  }

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
    km: (n) => `${number(n)} ${unit('km')}`,
    liters: (ml) => `${number(ml / 1000, { maxDecimals: 2 })} ${unit('l')}`,
    ml: (n) => `${number(n)} ${unit('ml')}`,
    nm: (n) => `${number(n, { maxDecimals: 1 })} ${unit('nm')}`,
    bar: (n) => `${number(n, { maxDecimals: 2 })} ${unit('bar')}`,
    barRange: (lo, hi) =>
      hi === null || hi === lo
        ? `${number(lo, { maxDecimals: 2 })} ${unit('bar')}`
        : `${number(lo, { maxDecimals: 2 })}–${number(hi, { maxDecimals: 2 })} ${unit('bar')}`,
    quantity: (ml, u) => (u === 'L' ? `${number(ml / 1000, { maxDecimals: 3 })} ${unit('l')}` : `${number(ml)} ${unit('ml')}`),
    displacement: (cc) => `${number(cc)} ${unit('cc')}`,
    power: (kw) =>
      prefs.powerUnit === 'ps'
        ? `${number(kwToPs(kw))} ${unit('ps')} (${number(kw)} ${unit('kw')})`
        : `${number(kw)} ${unit('kw')} (${number(kwToPs(kw))} ${unit('ps')})`,
    powerShort: (kw) => (prefs.powerUnit === 'ps' ? `${number(kwToPs(kw))} ${unit('ps')}` : `${number(kw)} ${unit('kw')}`),
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

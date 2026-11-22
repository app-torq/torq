import type { TFunction } from 'i18next'
import type { Formatter } from '../../lib/format'
import type { DistanceUnit, MeterReading, Vehicle } from '../../lib/types'

/**
 * A vehicle's current reading for lists and headers: the odometer, the hours
 * of a vehicle tracked by hours only, or both ("215,000 km · Generator: 312 h").
 */
export function currentReading(v: Vehicle, fmt: Formatter, t: TFunction): string {
  const parts: string[] = []
  if (v.has_odometer) parts.push(fmt.distance(v.current_odometer_km, v.odometer_unit))
  for (const m of v.hour_meters) {
    const hours = fmt.hours(m.current_hours)
    parts.push(v.has_odometer ? t('vehicles:hours.named', { name: m.name, hours }) : hours)
  }
  return parts.join(' · ')
}

/**
 * The readings recorded on a service entry or repair: its distance when one
 * was recorded, and its hours ("215,000 km · 318.5 h" or "1,262 h").
 */
export function entryReading(fmt: Formatter, unit: DistanceUnit, km: number | null, kmRecorded: boolean, readings: MeterReading[]): string {
  const parts: string[] = []
  if (kmRecorded && km !== null) parts.push(fmt.distance(km, unit))
  for (const r of readings) parts.push(fmt.hours(r.hours))
  return parts.join(' · ')
}

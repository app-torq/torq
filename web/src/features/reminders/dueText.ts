import type { TFunction } from 'i18next'
import type { Formatter } from '../../lib/format'
import type { Reminder } from '../../lib/types'

/** "every 10,000 km or 12 months, whichever comes first" */
export function intervalText(r: Reminder, t: TFunction, fmt: Formatter): string {
  const parts: string[] = []
  if (r.interval_km !== null) parts.push(fmt.km(r.interval_km))
  if (r.interval_months !== null) parts.push(t('reminders:due.intervalMonths', { count: r.interval_months }))
  if (r.interval_days !== null) parts.push(t('reminders:due.intervalDays', { count: r.interval_days }))
  const joined = parts.join(` ${t('reminders:due.or')} `)
  const text = t('reminders:due.interval', { value: joined })
  return parts.length > 1 ? `${text}, ${t('reminders:due.whicheverFirst')}` : text
}

/** Remaining or exceeded distance/time, e.g. "900 km left · 350 days left". */
export function remainingText(r: Reminder, t: TFunction, fmt: Formatter): string {
  const parts: string[] = []
  if (r.remaining_km !== null) {
    parts.push(r.remaining_km >= 0 ? t('reminders:due.remainingKm', { value: fmt.km(r.remaining_km) }) : t('reminders:due.overKm', { value: fmt.km(-r.remaining_km) }))
  }
  if (r.remaining_days !== null) {
    if (r.remaining_days === 0) parts.push(t('reminders:due.dueToday'))
    else parts.push(r.remaining_days > 0 ? t('reminders:due.remainingDays', { count: r.remaining_days }) : t('reminders:due.overDays', { count: -r.remaining_days }))
  }
  return parts.join(' · ')
}

/** Next due point, e.g. "at 225,000 km · on 2027-10-15". */
export function dueText(r: Reminder, t: TFunction, fmt: Formatter): string {
  const parts: string[] = []
  if (r.next_due_km !== null) parts.push(t('reminders:due.km', { km: fmt.km(r.next_due_km) }))
  if (r.next_due_date !== null) parts.push(t('reminders:due.date', { date: fmt.date(r.next_due_date) }))
  return parts.join(' · ')
}

export const stateTone = { upcoming: 'zinc', due_soon: 'amber', overdue: 'red' } as const

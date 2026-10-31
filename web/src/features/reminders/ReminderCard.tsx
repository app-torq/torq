import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { useFormat } from '../../lib/useFormat'
import { vehicleName, type Reminder, type ServiceCategory, type Vehicle } from '../../lib/types'
import { Badge } from '../../components/ui'
import { categoryLabel } from '../service-log/hooks'
import { dueText, remainingText, stateTone } from './dueText'

export function reminderName(r: Reminder, cat: ServiceCategory | undefined, t: Parameters<typeof categoryLabel>[1]): string {
  if (r.title) return r.title
  return categoryLabel(cat, t)
}

export function ReminderSummary({ reminder: r, category, vehicle, actions }: { reminder: Reminder; category?: ServiceCategory | undefined; vehicle?: Vehicle | undefined; actions?: ReactNode }) {
  const { t } = useTranslation('reminders')
  const fmt = useFormat()
  return (
    <div className="flex flex-wrap items-start gap-3 py-3">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium text-zinc-100">{reminderName(r, category, t)}</span>
          <Badge tone={stateTone[r.state]}>{t(`states.${r.state}`)}</Badge>
          {r.paused && <Badge>{t('paused')}</Badge>}
          {r.state_trigger && r.state !== 'upcoming' && <span className="text-xs text-zinc-400">{t(`due.trigger.${r.state_trigger}`)}</span>}
        </div>
        {vehicle && (
          <Link to={`/vehicles/${vehicle.id}/reminders`} className="text-sm text-sky-300 hover:underline">
            {vehicleName(vehicle)}
          </Link>
        )}
        <div className="mt-0.5 text-sm tabular-nums text-zinc-300">{remainingText(r, t, fmt)}</div>
        <div className="text-xs text-zinc-400">{dueText(r, t, fmt)}</div>
      </div>
      {actions && <div className="flex flex-wrap gap-1">{actions}</div>}
    </div>
  )
}

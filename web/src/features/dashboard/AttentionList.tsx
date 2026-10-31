import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { CheckCircle2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useErrorText } from '../../lib/errors'
import type { Reminder, RepairSummary, Vehicle } from '../../lib/types'
import { Card, ErrorBanner, Loading } from '../../components/ui'
import { useCategories } from '../service-log/hooks'
import { ReminderSummary } from '../reminders/ReminderCard'

export interface DashboardData {
  vehicles: Vehicle[]
  attention: Reminder[]
  open_repairs: RepairSummary[]
}

export function useDashboard() {
  return useQuery({ queryKey: queryKeys.dashboard, queryFn: () => api.get<DashboardData>('/dashboard') })
}

/** Overdue first, then due soon, across the garage (FR-034). */
export function AttentionList() {
  const { t } = useTranslation('vehicles')
  const errorText = useErrorText()
  const dash = useDashboard()
  const categories = useCategories(true)
  const catById = new Map(categories.data?.items.map((c) => [c.id, c]))
  const vehicleById = new Map(dash.data?.vehicles.map((v) => [v.id, v]))
  return (
    <Card title={t('dashboard.attention')}>
      {dash.isPending && <Loading />}
      {dash.isError && <ErrorBanner message={errorText(dash.error)} onRetry={() => void dash.refetch()} />}
      {dash.data?.attention.length === 0 && (
        <p className="flex items-center gap-2 text-sm text-emerald-300">
          <CheckCircle2 size={16} />
          {t('dashboard.allGood')}
        </p>
      )}
      <ul className="divide-y divide-zinc-800">
        {dash.data?.attention.map((r) => {
          const cat = catById.get(r.service_category_id ?? 0)
          const veh = vehicleById.get(r.vehicle_id)
          return (
            <li key={r.id}>
              <ReminderSummary reminder={r} category={cat} vehicle={veh} />
            </li>
          )
        })}
      </ul>
    </Card>
  )
}

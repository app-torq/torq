import { useState } from 'react'
import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName } from '../../lib/types'
import { Badge, EmptyState, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { useVehicles, type ArchiveFilter } from './hooks'
import { VehicleAvatar } from './VehicleAvatar'

export function VehicleListPage() {
  const { t } = useTranslation('vehicles')
  const fmt = useFormat()
  const errorText = useErrorText()
  const [filter, setFilter] = useState<ArchiveFilter>('false')
  const vehicles = useVehicles(filter)

  const addLink = (
    <Link to="/vehicles/new" className="focus-ring inline-flex h-10 items-center gap-2 rounded-md bg-sky-500 px-4 text-sm font-medium text-zinc-950 hover:bg-sky-400">
      <Plus size={16} />
      {t('add')}
    </Link>
  )

  return (
    <div>
      <PageHeader title={t('title')} subtitle={t('subtitle')} actions={addLink} />
      <div className="mb-4 inline-flex rounded-md border border-zinc-800 bg-zinc-900 p-0.5" role="group">
        {(['false', 'true', 'all'] as const).map((f) => (
          <button
            key={f}
            type="button"
            aria-pressed={filter === f}
            onClick={() => { setFilter(f) }}
            className={`focus-ring rounded px-3 py-1.5 text-sm ${filter === f ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
          >
            {t(f === 'false' ? 'filter.active' : f === 'true' ? 'filter.archived' : 'filter.all')}
          </button>
        ))}
      </div>
      {vehicles.isPending && <Loading />}
      {vehicles.isError && <ErrorBanner message={errorText(vehicles.error)} onRetry={() => void vehicles.refetch()} />}
      {vehicles.data?.items.length === 0 && <EmptyState title={t('empty')} action={filter === 'false' ? addLink : undefined} />}
      <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {vehicles.data?.items.map((v) => (
          <li key={v.id}>
            <Link
              to={`/vehicles/${v.id}`}
              className="focus-ring flex gap-3 rounded-lg border border-zinc-800 bg-zinc-900 p-4 hover:border-zinc-700"
            >
              <VehicleAvatar vehicle={v} size={56} />
              <div className="min-w-0 flex-1">
                <div className="flex items-start justify-between gap-2">
                  <div className="truncate font-medium text-zinc-100">{vehicleName(v)}</div>
                  {v.archived && <Badge>{t('archivedBadge')}</Badge>}
                </div>
                <div className="truncate text-sm text-zinc-400">
                  {[v.chassis_code, v.engine_code, v.model_year].filter(Boolean).join(' · ')}
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-sm">
                  <span className="tabular-nums text-zinc-300">{fmt.km(v.current_odometer_km)}</span>
                  {v.license_plate && <span className="rounded border border-zinc-700 px-1.5 text-xs text-zinc-400">{v.license_plate}</span>}
                  {v.reminder_counts.overdue > 0 && <Badge tone="red">{v.reminder_counts.overdue}</Badge>}
                  {v.reminder_counts.due_soon > 0 && <Badge tone="amber">{v.reminder_counts.due_soon}</Badge>}
                </div>
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}

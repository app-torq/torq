import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName } from '../../lib/types'
import { Badge, Card, EmptyState, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { useVehicles } from '../vehicles/hooks'
import { VehicleAvatar } from '../vehicles/VehicleAvatar'
import { OdometerQuickUpdate } from '../vehicles/OdometerQuickUpdate'
import { dashboardPanels } from './panels'
import { InstallHint } from '../../app/pwa/InstallHint'

export function DashboardPage() {
  const { t } = useTranslation('vehicles')
  const fmt = useFormat()
  const errorText = useErrorText()
  const vehicles = useVehicles('false')

  return (
    <div className="space-y-6">
      <PageHeader title={t('dashboard.title')} />
      <InstallHint />
      {dashboardPanels.map((Panel, i) => <Panel key={i} />)}
      <section>
        <h2 className="mb-3 text-sm font-semibold text-zinc-300">{t('dashboard.garage')}</h2>
        {vehicles.isPending && <Loading />}
        {vehicles.isError && <ErrorBanner message={errorText(vehicles.error)} onRetry={() => void vehicles.refetch()} />}
        {vehicles.data?.items.length === 0 && (
          <EmptyState
            title={t('dashboard.noVehicles')}
            action={<Link to="/vehicles/new" className="text-sm text-sky-400 hover:underline">{t('add')}</Link>}
          />
        )}
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          {vehicles.data?.items.map((v) => (
            <Card key={v.id}>
              <div className="flex gap-3">
                <VehicleAvatar vehicle={v} size={48} />
                <div className="min-w-0 flex-1">
                  <Link to={`/vehicles/${v.id}`} className="focus-ring block truncate font-medium text-zinc-100 hover:text-sky-300">
                    {vehicleName(v)}
                  </Link>
                  <div className="flex flex-wrap items-center gap-2 text-sm text-zinc-400">
                    <span className="tabular-nums">{fmt.km(v.current_odometer_km)}</span>
                    {v.reminder_counts.overdue > 0 && <Badge tone="red">{v.reminder_counts.overdue}</Badge>}
                    {v.reminder_counts.due_soon > 0 && <Badge tone="amber">{v.reminder_counts.due_soon}</Badge>}
                  </div>
                </div>
              </div>
              <div className="mt-3">
                <OdometerQuickUpdate vehicle={v} compact />
              </div>
            </Card>
          ))}
        </div>
      </section>
    </div>
  )
}

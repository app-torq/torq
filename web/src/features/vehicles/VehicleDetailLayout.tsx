import { Link, NavLink, Outlet, useOutletContext, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Pencil } from 'lucide-react'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName, type Vehicle } from '../../lib/types'
import { Badge, ErrorBanner, Loading } from '../../components/ui'
import { useVehicle } from './hooks'
import { VehicleAvatar } from './VehicleAvatar'
import { vehicleTabs } from './tabs'

export function useVehicleContext(): Vehicle {
  return useOutletContext<Vehicle>()
}

export function VehicleDetailLayout() {
  const { id } = useParams()
  const vehicleId = Number(id)
  const { t } = useTranslation('vehicles')
  const fmt = useFormat()
  const errorText = useErrorText()
  const vehicle = useVehicle(vehicleId)

  if (vehicle.isPending) return <Loading />
  if (vehicle.isError) return <ErrorBanner message={errorText(vehicle.error)} onRetry={() => void vehicle.refetch()} />
  const v = vehicle.data

  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center gap-4">
        <VehicleAvatar vehicle={v} size={72} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{vehicleName(v)}</h1>
            {v.archived && <Badge>{t('archivedBadge')}</Badge>}
          </div>
          <p className="text-sm text-zinc-400">
            {[v.chassis_code, v.engine_code, v.model_year, fmt.km(v.current_odometer_km)].filter(Boolean).join(' · ')}
          </p>
        </div>
        <Link to={`/vehicles/${v.id}/edit`} className="focus-ring inline-flex h-10 items-center gap-2 rounded-md border border-zinc-700 bg-zinc-900 px-4 text-sm hover:bg-zinc-800">
          <Pencil size={15} />
          {t('common:actions.edit')}
        </Link>
      </div>
      <nav className="mb-6 flex gap-1 overflow-x-auto border-b border-zinc-800">
        {vehicleTabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to === '' ? `/vehicles/${v.id}` : `/vehicles/${v.id}/${tab.to}`}
            end={tab.end ?? false}
            className={({ isActive }) =>
              `focus-ring -mb-px whitespace-nowrap border-b-2 px-3 py-2 text-sm ${isActive ? 'border-sky-400 text-zinc-100' : 'border-transparent text-zinc-400 hover:text-zinc-200'}`
            }
          >
            {t(tab.labelKey)}
          </NavLink>
        ))}
      </nav>
      <Outlet context={v} />
    </div>
  )
}

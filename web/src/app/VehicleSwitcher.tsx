import { useMatch, useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { vehicleName } from '../lib/types'
import { useVehicles } from '../features/vehicles/hooks'

/** Jumps to a vehicle's detail page; reflects the vehicle currently shown. */
export function VehicleSwitcher({ onNavigate }: { onNavigate?: () => void }) {
  const { t } = useTranslation('vehicles')
  const navigate = useNavigate()
  const match = useMatch('/vehicles/:id/*')
  const vehicles = useVehicles('false')
  const current = match?.params.id && /^\d+$/.test(match.params.id) ? match.params.id : ''
  if (!vehicles.data || vehicles.data.items.length === 0) return null
  return (
    <select
      aria-label={t('switcher.label')}
      className="focus-ring w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-200"
      value={current}
      onChange={(e) => {
        void navigate(e.target.value ? `/vehicles/${e.target.value}` : '/vehicles')
        onNavigate?.()
      }}
    >
      <option value="">{t('switcher.all')}</option>
      {vehicles.data.items.map((v) => (
        <option key={v.id} value={v.id}>
          {vehicleName(v)}
        </option>
      ))}
    </select>
  )
}

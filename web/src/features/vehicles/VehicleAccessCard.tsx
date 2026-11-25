import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useErrorText } from '../../lib/errors'
import type { Vehicle, VehicleUser } from '../../lib/types'
import { Badge, Card, ErrorBanner, Loading } from '../../components/ui'

/** Who can see the vehicle and how; shown to administrators. */
export function VehicleAccessCard({ vehicle }: { vehicle: Vehicle }) {
  const { t } = useTranslation('vehicles')
  const errorText = useErrorText()
  const users = useQuery({
    queryKey: queryKeys.vehicleUsers(vehicle.id),
    queryFn: () => api.get<{ items: VehicleUser[] }>(`/vehicles/${String(vehicle.id)}/access`),
  })
  return (
    <Card title={t('access.title')}>
      <p className="mb-3 text-sm text-zinc-400">{t('access.intro')}</p>
      {users.isPending && <Loading />}
      {users.isError && <ErrorBanner message={errorText(users.error)} />}
      {users.data && (
        <ul className="divide-y divide-zinc-800">
          {users.data.items.map((u) => (
            <li key={u.user_id} className="flex flex-wrap items-center justify-between gap-2 py-2 text-sm">
              <span className="text-zinc-100">
                {u.display_name}
                <span className="ml-2 text-xs text-zinc-400">{u.role_name}</span>
              </span>
              <Badge tone={u.via === 'role' ? 'zinc' : u.level === 'edit' ? 'sky' : 'amber'}>
                {u.via === 'role' ? t('access.viaRole') : t(`access.level.${u.level}`)}
              </Badge>
            </li>
          ))}
        </ul>
      )}
      <Link to="/settings/users" className="focus-ring mt-3 inline-block text-sm text-sky-300 hover:underline">{t('access.manage')}</Link>
    </Card>
  )
}

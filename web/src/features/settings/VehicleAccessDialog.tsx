import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useErrorText } from '../../lib/errors'
import type { AccessLevel, ListResponse, User, Vehicle, VehicleAssignment } from '../../lib/types'
import { Button } from '../../components/Button'
import { Dialog } from '../../components/Dialog'
import { ErrorBanner, Loading, Notice } from '../../components/ui'

interface VehicleAccess {
  all_vehicles: boolean
  items: VehicleAssignment[]
}

type Choice = AccessLevel | 'none'

/** Sets, per vehicle, whether a user sees it and may change it (admins only). */
export function VehicleAccessDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const access = useQuery({
    queryKey: queryKeys.vehicleAccess(user.id),
    queryFn: () => api.get<VehicleAccess>(`/users/${user.id}/vehicle-access`),
  })
  const vehicles = useQuery({
    queryKey: [...queryKeys.vehicles, 'all'],
    queryFn: () => api.get<ListResponse<Vehicle>>('/vehicles', { archived: 'all' }),
  })
  return (
    <Dialog open wide onClose={onClose} title={t('users.vehicleAccessTitle', { name: user.display_name })}>
      {(access.isPending || vehicles.isPending) && <Loading />}
      {access.isError && <ErrorBanner message={errorText(access.error)} />}
      {vehicles.isError && <ErrorBanner message={errorText(vehicles.error)} />}
      {access.data && vehicles.data && (
        <AccessForm user={user} access={access.data} vehicles={vehicles.data.items} onClose={onClose} />
      )}
    </Dialog>
  )
}

function AccessForm({ user, access, vehicles, onClose }: { user: User; access: VehicleAccess; vehicles: Vehicle[]; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [choices, setChoices] = useState<Record<number, Choice>>(() =>
    Object.fromEntries(access.items.map((a) => [a.vehicle_id, a.level])),
  )
  const [query, setQuery] = useState('')
  const save = useMutation({
    mutationFn: () => api.put(`/users/${user.id}/vehicle-access`, {
      items: Object.entries(choices)
        .filter(([, c]) => c !== 'none')
        .map(([id, level]) => ({ vehicle_id: Number(id), level })),
    }),
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: queryKeys.users }),
        qc.invalidateQueries({ queryKey: queryKeys.vehicles }),
      ])
      onClose()
    },
  })
  const q = query.trim().toLowerCase()
  const shown = vehicles.filter((v) => !q || `${v.make} ${v.model} ${v.license_plate ?? ''}`.toLowerCase().includes(q))
  const options: { value: Choice; label: string }[] = [
    { value: 'none', label: t('users.access.none') },
    { value: 'view', label: t('users.access.view') },
    { value: 'edit', label: t('users.access.edit') },
  ]

  return (
    <div className="space-y-4">
      {access.all_vehicles && <Notice>{t('users.allVehiclesNote', { role: user.role_name })}</Notice>}
      <p className="text-sm text-zinc-400">{t('users.accessIntro')}</p>
      {save.isError && <ErrorBanner message={errorText(save.error)} />}
      <input
        type="search"
        aria-label={t('common:actions.search')}
        placeholder={t('common:actions.search')}
        value={query}
        onChange={(e) => { setQuery(e.target.value) }}
        className="focus-ring w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm"
      />
      <ul className="max-h-[50dvh] divide-y divide-zinc-800 overflow-y-auto rounded-md border border-zinc-800">
        {shown.map((v) => {
          const choice = choices[v.id] ?? 'none'
          const name = `${v.make} ${v.model}`
          return (
            <li key={v.id} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
              <span className="min-w-0 text-sm text-zinc-100">
                {name}
                {v.license_plate && <span className="ml-2 text-xs text-zinc-400">{v.license_plate}</span>}
              </span>
              <div role="radiogroup" aria-label={name} className="inline-flex rounded-md border border-zinc-700 p-0.5">
                {options.map((o) => (
                  <button
                    key={o.value}
                    type="button"
                    role="radio"
                    aria-checked={choice === o.value}
                    onClick={() => { setChoices((c) => ({ ...c, [v.id]: o.value })) }}
                    className={`focus-ring rounded px-2.5 py-1 text-xs ${choice === o.value ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-400 hover:text-zinc-100'}`}
                  >
                    {o.label}
                  </button>
                ))}
              </div>
            </li>
          )
        })}
      </ul>
      <div className="flex justify-end gap-2">
        <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
        <Button variant="primary" disabled={save.isPending} onClick={() => { save.mutate() }}>
          {t('common:actions.save')}
        </Button>
      </div>
    </div>
  )
}

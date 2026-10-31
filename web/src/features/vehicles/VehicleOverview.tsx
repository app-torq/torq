import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Archive, ArchiveRestore, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName, type ListResponse, type OdometerReading } from '../../lib/types'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { Card, DefinitionList, ErrorBanner, Table } from '../../components/ui'
import { invalidateOdometerDependents } from './hooks'
import { OdometerQuickUpdate } from './OdometerQuickUpdate'
import { useVehicleContext } from './VehicleDetailLayout'

export function VehicleOverview() {
  const v = useVehicleContext()
  const { t } = useTranslation('vehicles')
  const fmt = useFormat()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const errorText = useErrorText()
  const [deleting, setDeleting] = useState(false)

  const readings = useQuery({
    queryKey: queryKeys.odometerReadings(v.id),
    queryFn: () => api.get<ListResponse<OdometerReading>>(`/vehicles/${v.id}/odometer-readings`),
  })

  const archive = useMutation({
    mutationFn: () => api.post(`/vehicles/${v.id}/${v.archived ? 'unarchive' : 'archive'}`),
    onSuccess: () => invalidateOdometerDependents(qc),
  })
  const remove = useMutation({
    mutationFn: () => api.del(`/vehicles/${v.id}`, { confirm_name: vehicleName(v) }),
    onSuccess: async () => {
      qc.removeQueries({ queryKey: queryKeys.vehicle(v.id) })
      await invalidateOdometerDependents(qc)
      void navigate('/vehicles')
    },
  })

  const none = t('common:state.none')
  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <Card title={t('details')}>
        <DefinitionList
          items={[
            { label: t('fields.make'), value: v.make },
            { label: t('fields.model'), value: v.model },
            { label: t('fields.chassisCode'), value: v.chassis_code || none },
            { label: t('fields.engineCode'), value: v.engine_code || none },
            { label: t('fields.modelYear'), value: v.model_year ?? none },
            { label: t('fields.licensePlate'), value: v.license_plate ?? none },
            { label: t('fields.vin'), value: <span className="font-mono">{v.vin ?? none}</span> },
            { label: t('fields.odometer'), value: <span className="tabular-nums">{fmt.km(v.current_odometer_km)}</span> },
          ]}
        />
      </Card>
      <Card title={t('odometer.title')}>
        <div className="space-y-4">
          {!v.archived && <OdometerQuickUpdate vehicle={v} />}
          <div>
            <h3 className="mb-2 text-xs uppercase tracking-wide text-zinc-400">{t('odometer.history')}</h3>
            {readings.isError && <ErrorBanner message={errorText(readings.error)} />}
            {readings.data && (
              <div className="max-h-72 overflow-y-auto">
                <Table>
                  <tbody>
                    {readings.data.items.map((r) => (
                      <tr key={r.id}>
                        <td className="tabular-nums text-zinc-400">{fmt.date(r.reading_date)}</td>
                        <td className="tabular-nums text-zinc-100">{fmt.km(r.odometer_km)}</td>
                        <td className="text-zinc-400">{t(`odometer.sources.${r.source}`)}</td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              </div>
            )}
          </div>
        </div>
      </Card>
      <Card className="lg:col-span-2">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="max-w-xl text-sm text-zinc-400">{t('archiveHint')}</p>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => { archive.mutate() }} disabled={archive.isPending}>
              {v.archived ? <ArchiveRestore size={16} /> : <Archive size={16} />}
              {v.archived ? t('actions.unarchive') : t('actions.archive')}
            </Button>
            <Button variant="danger" onClick={() => { setDeleting(true) }}>
              <Trash2 size={16} />
              {t('actions.delete')}
            </Button>
          </div>
        </div>
        {archive.isError && <div className="mt-3"><ErrorBanner message={errorText(archive.error)} /></div>}
      </Card>
      <ConfirmDialog
        open={deleting}
        title={t('deleteTitle')}
        body={t('deleteBody', { name: vehicleName(v) })}
        confirmLabel={t('common:actions.delete')}
        danger
        typeToConfirm={vehicleName(v)}
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { remove.mutate() }}
        onClose={() => { setDeleting(false); remove.reset() }}
      />
    </div>
  )
}

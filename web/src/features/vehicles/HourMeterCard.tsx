import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { api, isApiError } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseHours } from '../../lib/format'
import { useFormat } from '../../lib/useFormat'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { HourMeter, HourMeterReading, ListResponse, Vehicle } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { canOn } from '../../lib/permissions'
import { TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Card, ErrorBanner, Table } from '../../components/ui'
import { invalidateOdometerDependents } from './hooks'
import { HoursQuickUpdate } from './HoursQuickUpdate'

/** An hour meter with quick update, reading history, rename and remove. */
export function HourMeterCard({ vehicle, meter }: { vehicle: Vehicle; meter: HourMeter }) {
  const { t } = useTranslation('vehicles')
  const fmt = useFormat()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [renaming, setRenaming] = useState(false)
  const [removing, setRemoving] = useState(false)
  const readings = useQuery({
    queryKey: queryKeys.hourReadings(meter.id),
    queryFn: () => api.get<ListResponse<HourMeterReading>>(`/hour-meters/${meter.id}/readings`),
  })
  const remove = useMutation({
    mutationFn: () => api.del(`/hour-meters/${meter.id}`),
    onSuccess: async () => {
      setRemoving(false)
      await invalidateOdometerDependents(qc)
    },
  })
  const inUse = remove.error && isApiError(remove.error, 'hour_meter.in_use')
  const canEdit = canOn(vehicle, 'vehicles.edit')
  const removeError = remove.isError ? (inUse ? t('hours.inUse') : errorText(remove.error)) : undefined
  return (
    <Card
      title={meter.name}
      actions={canEdit && (
        <div className="flex gap-0.5">
          <IconButton aria-label={t('hours.rename')} onClick={() => { setRenaming(true) }}>
            <Pencil size={15} />
          </IconButton>
          {vehicle.has_odometer && (
            <IconButton aria-label={t('hours.remove')} onClick={() => { setRemoving(true) }}>
              <Trash2 size={15} />
            </IconButton>
          )}
        </div>
      )}
    >
      <div className="space-y-4">
        <p className="text-2xl font-semibold tabular-nums text-zinc-100">{fmt.hours(meter.current_hours)}</p>
        {!vehicle.archived && canEdit && <HoursQuickUpdate meter={meter} />}
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
                      <td className="tabular-nums text-zinc-100">{fmt.hours(r.hours)}</td>
                      <td className="text-zinc-400">{t(`odometer.sources.${r.source}`)}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </div>
          )}
        </div>
      </div>
      {renaming && <RenameMeterDialog meter={meter} onClose={() => { setRenaming(false) }} />}
      <ConfirmDialog
        open={removing}
        title={t('hours.removeTitle', { name: meter.name })}
        body={t('hours.removeBody')}
        confirmLabel={t('hours.remove')}
        danger
        busy={remove.isPending}
        error={removeError}
        onConfirm={() => { remove.mutate() }}
        onClose={() => { setRemoving(false); remove.reset() }}
      />
    </Card>
  )
}

function RenameMeterDialog({ meter, onClose }: { meter: HourMeter; onClose: () => void }) {
  const { t } = useTranslation('vehicles')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [name, setName] = useState(meter.name)
  const save = useMutation({
    mutationFn: () => api.patch(`/hour-meters/${meter.id}`, { name }),
    onSuccess: async () => {
      await invalidateOdometerDependents(qc)
      onClose()
    },
  })
  const fe = fieldErrors(save.error)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('hours.rename')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={() => { save.mutate() }} disabled={save.isPending}>{t('common:actions.save')}</Button>
        </>
      }
    >
      {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
      <TextField label={t('hours.name')} value={name} onChange={setName} error={fe.name} maxLength={40} autoFocus />
    </Dialog>
  )
}

/** Adds an hour meter to a vehicle that has none, e.g. a motorhome's generator. */
export function AddHourMeter({ vehicle }: { vehicle: Vehicle }) {
  const { t } = useTranslation('vehicles')
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(() => t('hours.defaultName'))
  const [hours, setHours] = useState('')
  const [local, setLocal] = useState<string | undefined>()
  const create = useMutation({
    mutationFn: (h: number) => api.post(`/vehicles/${vehicle.id}/hour-meters`, { name, hours: h }),
    onSuccess: async () => {
      await invalidateOdometerDependents(qc)
      setOpen(false)
    },
  })
  const fe = fieldErrors(create.error)
  const submit = () => {
    const h = parseHours(hours, user.number_format)
    if (h === null || h === undefined) {
      setLocal('validation.number')
      return
    }
    setLocal(undefined)
    create.mutate(h)
  }
  return (
    <Card title={t('hours.title')}>
      <p className="mb-3 text-sm text-zinc-400">{t('hours.addHint')}</p>
      <Button onClick={() => { setOpen(true) }}>
        <Plus size={16} />
        {t('hours.add')}
      </Button>
      {open && (
        <Dialog
          open
          onClose={() => { setOpen(false) }}
          title={t('hours.add')}
          footer={
            <>
              <Button onClick={() => { setOpen(false) }}>{t('common:actions.cancel')}</Button>
              <Button variant="primary" onClick={submit} disabled={create.isPending}>{t('common:actions.save')}</Button>
            </>
          }
        >
          <div className="space-y-4">
            {create.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(create.error)} />}
            <TextField label={t('hours.name')} value={name} onChange={setName} error={fe.name} maxLength={40} hint={t('hours.nameHint')} />
            <NumberField label={t('hours.current')} value={hours} onChange={setHours} unit={t('common:units.h')} error={local ?? fe.hours} />
          </div>
        </Dialog>
      )}
    </Card>
  )
}

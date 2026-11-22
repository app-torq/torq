import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { distanceField, hoursToInput, parseHours, readDistance } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { RepairSummary } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { Dialog } from '../../components/Dialog'
import { ErrorBanner } from '../../components/ui'
import { invalidateOdometerDependents, useVehicle } from '../vehicles/hooks'
import { invalidateRepairs } from './hooks'

/**
 * Completing requires a date (default today) and the odometer (prefilled with
 * the current km), or the hours for a vehicle without an odometer.
 */
export function CompleteRepairDialog({ repair, onClose }: { repair: RepairSummary; onClose: () => void }) {
  const { t } = useTranslation('repairs')
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const vehicle = useVehicle(repair.vehicle_id)
  const [date, setDate] = useState(repair.completed_on ?? new Date().toISOString().slice(0, 10))
  const unit = vehicle.data?.odometer_unit ?? user.distance_unit
  const initial = distanceField(repair.completion_odometer_km ?? vehicle.data?.current_odometer_km, unit, user.number_format)
  const [km, setKm] = useState<string | null>(null)
  const [hoursText, setHoursText] = useState<string | null>(null)
  const [local, setLocal] = useState<Record<string, string>>({})
  const hasOdometer = vehicle.data?.has_odometer ?? true
  const meter = vehicle.data?.hour_meters[0]
  const complete = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.post(`/repairs/${repair.id}/status`, { status: 'completed', completed_on: date, ...body }),
    onSuccess: async () => {
      await Promise.all([invalidateRepairs(qc, repair.id), invalidateOdometerDependents(qc)])
      onClose()
    },
  })
  const fe = fieldErrors(complete.error)
  // Prefilled with the recorded or current odometer until edited.
  const value = km ?? initial.text
  // Hours are prefilled with the recorded or current reading too.
  const prevHours = repair.completion_meter_readings.find((r) => r.hour_meter_id === meter?.id)?.hours ?? meter?.current_hours
  const hoursValue = hoursText ?? hoursToInput(prevHours, user.number_format)
  const submit = () => {
    const errs: Record<string, string> = {}
    const body: Record<string, unknown> = {}
    if (hasOdometer) {
      const n = readDistance(value, initial, unit, user.number_format)
      if (n === null) errs.completion_odometer_km = 'validation.number'
      body.completion_odometer_km = n
    }
    if (meter) {
      const h = parseHours(hoursValue, user.number_format)
      if (h === undefined || (!hasOdometer && h === null)) errs.completion_meter_readings = 'validation.number'
      body.completion_meter_readings = h == null ? [] : [{ hour_meter_id: meter.id, hours: h }]
    }
    setLocal(errs)
    if (Object.keys(errs).length === 0) complete.mutate(body)
  }
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('complete.title')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={complete.isPending}>
            {t('complete.submit')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-zinc-400">{t('complete.intro')}</p>
        {complete.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(complete.error)} />}
        <TextField label={t('fields.completedOn')} type="date" value={date} onChange={setDate} error={fe.completed_on} />
        {hasOdometer && <NumberField label={t('fields.completionOdometer')} unit={t(`common:units.${unit}`)} decimal={false} value={value} onChange={setKm} error={local.completion_odometer_km ?? fe.completion_odometer_km} />}
        {meter && <NumberField label={meter.name} unit={t('common:units.h')} value={hoursValue} onChange={setHoursText} error={local.completion_meter_readings ?? fe.completion_meter_readings} optional={hasOdometer} />}
      </div>
    </Dialog>
  )
}

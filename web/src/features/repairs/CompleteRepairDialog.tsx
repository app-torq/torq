import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { parseNumber } from '../../lib/format'
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

/** Completing requires a date (default today) and the odometer (prefilled with the current km). */
export function CompleteRepairDialog({ repair, onClose }: { repair: RepairSummary; onClose: () => void }) {
  const { t } = useTranslation('repairs')
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const vehicle = useVehicle(repair.vehicle_id)
  const [date, setDate] = useState(repair.completed_on ?? new Date().toISOString().slice(0, 10))
  const [km, setKm] = useState(repair.completion_odometer_km !== null ? String(repair.completion_odometer_km) : '')
  const [local, setLocal] = useState<string | undefined>()
  const complete = useMutation({
    mutationFn: (odometer: number) => api.post(`/repairs/${repair.id}/status`, { status: 'completed', completed_on: date, completion_odometer_km: odometer }),
    onSuccess: async () => {
      await Promise.all([invalidateRepairs(qc, repair.id), invalidateOdometerDependents(qc)])
      onClose()
    },
  })
  const fe = fieldErrors(complete.error)
  const value = km === '' && vehicle.data ? String(vehicle.data.current_odometer_km) : km
  const submit = () => {
    const n = parseNumber(value, user.number_format)
    if (n === null || !Number.isInteger(n) || n < 0) {
      setLocal('validation.number')
      return
    }
    setLocal(undefined)
    complete.mutate(n)
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
        <NumberField label={t('fields.completionOdometer')} unit={t('common:units.km')} decimal={false} value={value} onChange={setKm} error={local ?? fe.completion_odometer_km} />
      </div>
    </Dialog>
  )
}

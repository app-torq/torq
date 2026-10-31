import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseNumber } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { vehicleName, type Reminder } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { Dialog } from '../../components/Dialog'
import { ErrorBanner } from '../../components/ui'
import { invalidateOdometerDependents, useVehicles } from '../vehicles/hooks'
import { categoryLabel, useCategories } from '../service-log/hooks'

interface Props {
  reminder: Reminder | null
  vehicleId?: number
  onClose: () => void
}

const str = (n: number | null | undefined) => (n === null || n === undefined ? '' : String(n))

export function ReminderForm({ reminder, vehicleId, onClose }: Props) {
  const { t } = useTranslation('reminders')
  const { user } = useSession()
  const nf = user.number_format
  const qc = useQueryClient()
  const errorText = useErrorText()
  const vehicles = useVehicles('false')
  const categories = useCategories(reminder !== null)

  const [vehicle, setVehicle] = useState(String(reminder?.vehicle_id ?? vehicleId ?? ''))
  const [kind, setKind] = useState<'linked' | 'standalone'>(reminder && reminder.service_category_id === null ? 'standalone' : 'linked')
  const [categoryId, setCategoryId] = useState(str(reminder?.service_category_id))
  const [title, setTitle] = useState(reminder?.title ?? '')
  const [intervalKm, setIntervalKm] = useState(str(reminder?.interval_km))
  const [timeUnit, setTimeUnit] = useState<'months' | 'days'>(reminder?.interval_days ? 'days' : 'months')
  const [intervalTime, setIntervalTime] = useState(str(reminder?.interval_months ?? reminder?.interval_days))
  const [leadKm, setLeadKm] = useState(str(reminder?.lead_km))
  const [leadDays, setLeadDays] = useState(str(reminder?.lead_days))
  const [startDate, setStartDate] = useState(reminder?.start_date ?? '')
  const [startKm, setStartKm] = useState(str(reminder?.start_odometer_km))
  const [local, setLocal] = useState<Record<string, string>>({})

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => (reminder ? api.patch(`/reminders/${reminder.id}`, body) : api.post('/reminders', body)),
    onSuccess: async () => {
      await invalidateOdometerDependents(qc)
      await qc.invalidateQueries({ queryKey: queryKeys.reminders })
      onClose()
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const onCategory = (id: string) => {
    setCategoryId(id)
    const c = categories.data?.items.find((x) => String(x.id) === id)
    if (c && !reminder && intervalKm === '' && intervalTime === '') {
      setIntervalKm(str(c.default_interval_km))
      setIntervalTime(str(c.default_interval_months))
      setTimeUnit('months')
    }
  }

  const submit = () => {
    const errs: Record<string, string> = {}
    const int = (field: string, raw: string): number | null => {
      if (raw.trim() === '') return null
      const n = parseNumber(raw, nf)
      if (n === null || !Number.isInteger(n) || n < 0) {
        errs[field] = 'validation.number'
        return null
      }
      return n
    }
    if (!vehicle) errs.vehicle_id = 'validation.required'
    const time = int(timeUnit === 'months' ? 'interval_months' : 'interval_days', intervalTime)
    const body: Record<string, unknown> = {
      vehicle_id: Number(vehicle),
      service_category_id: kind === 'linked' && categoryId ? Number(categoryId) : null,
      title: kind === 'standalone' ? title : '',
      interval_km: int('interval_km', intervalKm),
      interval_months: timeUnit === 'months' ? time : null,
      interval_days: timeUnit === 'days' ? time : null,
      start_date: startDate,
      start_odometer_km: int('start_odometer_km', startKm),
    }
    const lk = int('lead_km', leadKm)
    const ld = int('lead_days', leadDays)
    // On create, omitted leads use server defaults; on edit, empty means no lead.
    if (reminder || lk !== null) body.lead_km = lk
    if (reminder || ld !== null) body.lead_days = ld
    if (kind === 'linked' && !categoryId) errs.service_category_id = 'validation.required'
    setLocal(errs)
    if (Object.keys(errs).length === 0) save.mutate(body)
  }

  const km = t('common:units.km')
  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={reminder ? t('edit') : t('add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={save.isPending}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SelectField
            label={t('fields.vehicle')}
            value={vehicle}
            onChange={setVehicle}
            disabled={reminder !== null || vehicleId !== undefined}
            error={fe.vehicle_id}
            options={[{ value: '', label: '—' }, ...(vehicles.data?.items ?? []).map((v) => ({ value: String(v.id), label: vehicleName(v) }))]}
          />
          <SelectField
            label={t('fields.kind')}
            value={kind}
            onChange={(v) => { setKind(v as 'linked' | 'standalone') }}
            options={[{ value: 'linked', label: t('kind.linked') }, { value: 'standalone', label: t('kind.standalone') }]}
          />
          {kind === 'linked' ? (
            <SelectField
              label={t('fields.category')}
              value={categoryId}
              onChange={onCategory}
              error={fe.service_category_id}
              className="sm:col-span-2"
              options={[{ value: '', label: '—' }, ...(categories.data?.items ?? []).map((c) => ({ value: String(c.id), label: categoryLabel(c, t) }))]}
            />
          ) : (
            <TextField label={t('fields.title')} value={title} onChange={setTitle} hint={t('fields.titleHint')} error={fe.title} maxLength={120} className="sm:col-span-2" />
          )}
          <NumberField label={t('fields.intervalKm')} unit={km} decimal={false} value={intervalKm} onChange={setIntervalKm} error={fe.interval_km} optional />
          <div className="flex gap-2">
            <NumberField label={t('fields.intervalTime')} decimal={false} value={intervalTime} onChange={setIntervalTime} error={fe.interval_months ?? fe.interval_days} className="flex-1" optional />
            <SelectField
              label={t('fields.timeUnit')}
              value={timeUnit}
              onChange={(v) => { setTimeUnit(v as 'months' | 'days') }}
              options={[{ value: 'months', label: t('fields.months') }, { value: 'days', label: t('fields.days') }]}
              className="w-32"
            />
          </div>
          <NumberField label={t('fields.leadKm')} unit={km} decimal={false} value={leadKm} onChange={setLeadKm} error={fe.lead_km} hint={t('fields.leadHint')} optional />
          <NumberField label={t('fields.leadDays')} unit={t('fields.days')} decimal={false} value={leadDays} onChange={setLeadDays} error={fe.lead_days} optional />
          <TextField label={t('fields.startDate')} type="date" value={startDate} onChange={setStartDate} error={fe.start_date} hint={t('fields.startHint')} optional />
          <NumberField label={t('fields.startOdometer')} unit={km} decimal={false} value={startKm} onChange={setStartKm} error={fe.start_odometer_km} optional />
        </div>
      </div>
    </Dialog>
  )
}

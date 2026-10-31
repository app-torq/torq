import { useState, type SubmitEvent } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseNumber } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { Vehicle } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { Card, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { useVehicle } from './hooks'
import { AvatarUpload } from './AvatarUpload'

export function VehicleFormPage() {
  const { id } = useParams()
  if (id) return <EditVehicle id={Number(id)} />
  return <VehicleForm />
}

function EditVehicle({ id }: { id: number }) {
  const vehicle = useVehicle(id)
  const errorText = useErrorText()
  if (vehicle.isPending) return <Loading />
  if (vehicle.isError) return <ErrorBanner message={errorText(vehicle.error)} />
  return <VehicleForm vehicle={vehicle.data} />
}

function VehicleForm({ vehicle }: { vehicle?: Vehicle }) {
  const { t } = useTranslation('vehicles')
  const { user } = useSession()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [make, setMake] = useState(vehicle?.make ?? '')
  const [model, setModel] = useState(vehicle?.model ?? '')
  const [chassis, setChassis] = useState(vehicle?.chassis_code ?? '')
  const [year, setYear] = useState(vehicle?.model_year ? String(vehicle.model_year) : '')
  const [engine, setEngine] = useState(vehicle?.engine_code ?? '')
  const [vin, setVin] = useState(vehicle?.vin ?? '')
  const [plate, setPlate] = useState(vehicle?.license_plate ?? '')
  const [odometer, setOdometer] = useState('')
  const [local, setLocal] = useState<Record<string, string>>({})

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      vehicle
        ? api.patch<{ vehicle: Vehicle }>(`/vehicles/${vehicle.id}`, body)
        : api.post<{ vehicle: Vehicle }>('/vehicles', body),
    onSuccess: async ({ vehicle: saved }) => {
      await qc.invalidateQueries({ queryKey: queryKeys.vehicles })
      await qc.invalidateQueries({ queryKey: queryKeys.dashboard })
      void navigate(`/vehicles/${saved.id}`)
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    const errs: Record<string, string> = {}
    const yearNum = year.trim() === '' ? null : Number(year)
    if (yearNum !== null && !Number.isInteger(yearNum)) errs.model_year = 'validation.number'
    const body: Record<string, unknown> = {
      make, model, chassis_code: chassis, model_year: yearNum, engine_code: engine,
      vin: vin.trim() === '' ? null : vin, license_plate: plate.trim() === '' ? null : plate,
    }
    if (!vehicle) {
      const km = parseNumber(odometer, user.number_format)
      if (km === null || !Number.isInteger(km) || km < 0) errs.current_odometer_km = 'validation.number'
      body.current_odometer_km = km
    }
    setLocal(errs)
    if (Object.keys(errs).length === 0) save.mutate(body)
  }

  return (
    <div className="max-w-2xl">
      <PageHeader title={vehicle ? t('edit') : t('add')} />
      <Card>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <TextField label={t('fields.make')} value={make} onChange={setMake} error={fe.make} required />
            <TextField label={t('fields.model')} value={model} onChange={setModel} error={fe.model} required />
            <TextField label={t('fields.chassisCode')} value={chassis} onChange={setChassis} error={fe.chassis_code} hint={t('fields.chassisCodeHint')} optional />
            <TextField label={t('fields.engineCode')} value={engine} onChange={setEngine} error={fe.engine_code} hint={t('fields.engineCodeHint')} optional />
            <NumberField label={t('fields.modelYear')} value={year} onChange={setYear} error={fe.model_year} decimal={false} optional />
            {!vehicle && (
              <NumberField label={t('fields.odometer')} value={odometer} onChange={setOdometer} error={fe.current_odometer_km} unit={t('common:units.km')} decimal={false} />
            )}
            <TextField label={t('fields.vin')} value={vin} onChange={setVin} error={fe.vin} maxLength={17} className="sm:col-span-1" optional />
            <TextField label={t('fields.licensePlate')} value={plate} onChange={setPlate} error={fe.license_plate} maxLength={16} optional />
          </div>
          <div className="flex gap-2">
            <Button type="submit" variant="primary" disabled={save.isPending}>
              {save.isPending ? t('common:actions.saving') : t('common:actions.save')}
            </Button>
            <Button onClick={() => void navigate(-1)}>{t('common:actions.cancel')}</Button>
          </div>
        </form>
      </Card>
      {vehicle && <AvatarUpload vehicle={vehicle} />}
    </div>
  )
}

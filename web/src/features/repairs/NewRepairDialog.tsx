import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { vehicleName, type Repair } from '../../lib/types'
import { Button } from '../../components/Button'
import { SelectField, TextAreaField, TextField } from '../../components/Field'
import { Dialog } from '../../components/Dialog'
import { ErrorBanner } from '../../components/ui'
import { useVehicles } from '../vehicles/hooks'
import { FaultCodeInput } from './FaultCodeInput'
import { invalidateRepairs } from './hooks'

export function NewRepairDialog({ vehicleId, onClose }: { vehicleId?: number | undefined; onClose: () => void }) {
  const { t } = useTranslation('repairs')
  const navigate = useNavigate()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const vehicles = useVehicles('false')
  const [vehicle, setVehicle] = useState(vehicleId ? String(vehicleId) : '')
  const [title, setTitle] = useState('')
  const [symptom, setSymptom] = useState('')
  const [codes, setCodes] = useState<string[]>([])
  const create = useMutation({
    mutationFn: () => api.post<{ repair: Repair }>('/repairs', { vehicle_id: Number(vehicle), title, symptom, fault_codes: codes }),
    onSuccess: async ({ repair }) => {
      await invalidateRepairs(qc)
      onClose()
      void navigate(`/repairs/${repair.id}`)
    },
  })
  const fe = fieldErrors(create.error)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('addTitle')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" disabled={create.isPending || !vehicle} onClick={() => { create.mutate() }}>
            {t('common:actions.create')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {create.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(create.error)} />}
        <SelectField
          label={t('fields.vehicle')}
          value={vehicle}
          onChange={setVehicle}
          error={fe.vehicle_id}
          options={[{ value: '', label: '—' }, ...(vehicles.data?.items ?? []).map((v) => ({ value: String(v.id), label: vehicleName(v) }))]}
        />
        <TextField label={t('fields.title')} value={title} onChange={setTitle} error={fe.title} maxLength={120} autoFocus />
        <TextAreaField label={t('fields.symptom')} value={symptom} onChange={setSymptom} error={fe.symptom} rows={3} optional />
        <FaultCodeInput value={codes} onChange={setCodes} />
      </div>
    </Dialog>
  )
}

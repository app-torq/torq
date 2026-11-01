import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Gauge } from 'lucide-react'
import { api, isApiError } from '../../lib/api'
import { parseNumber } from '../../lib/format'
import { useFormat } from '../../lib/useFormat'
import { useErrorText, fieldErrors } from '../../lib/errors'
import type { Vehicle } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { NumberField } from '../../components/NumberField'
import { invalidateOdometerDependents } from './hooks'

interface Props {
  vehicle: Vehicle
  onDone?: () => void
  compact?: boolean
  autoFocus?: boolean
}

type Pending = { km: number; kind: 'decrease' | 'jump'; confirmDecrease: boolean; confirmJump: boolean }

/** Odometer quick-update with confirmation for decreases and large jumps (FR-013). */
export function OdometerQuickUpdate({ vehicle, onDone, compact, autoFocus }: Props) {
  const { t } = useTranslation('vehicles')
  const { user } = useSession()
  const fmt = useFormat()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [value, setValue] = useState('')
  const [localError, setLocalError] = useState<string | undefined>()
  const [pending, setPending] = useState<Pending | null>(null)

  const update = useMutation({
    mutationFn: (p: { km: number; confirmDecrease: boolean; confirmJump: boolean }) =>
      api.post<{ vehicle: Vehicle }>(`/vehicles/${vehicle.id}/odometer`, {
        odometer_km: p.km,
        confirm_decrease: p.confirmDecrease,
        confirm_large_jump: p.confirmJump,
      }),
    onSuccess: async () => {
      setValue('')
      setPending(null)
      await invalidateOdometerDependents(qc)
      onDone?.()
    },
    onError: (err, vars) => {
      if (isApiError(err, 'odometer.decrease_requires_confirmation')) {
        setPending({ km: vars.km, kind: 'decrease', confirmDecrease: true, confirmJump: vars.confirmJump })
      } else if (isApiError(err, 'odometer.large_jump_requires_confirmation')) {
        setPending({ km: vars.km, kind: 'jump', confirmDecrease: vars.confirmDecrease, confirmJump: true })
      }
    },
  })

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    const km = parseNumber(value, user.number_format)
    if (km === null || km < 0 || !Number.isInteger(km)) {
      setLocalError('validation.number')
      return
    }
    setLocalError(undefined)
    update.mutate({ km, confirmDecrease: false, confirmJump: false })
  }

  const needsConfirm = update.error && (isApiError(update.error, 'odometer.decrease_requires_confirmation') || isApiError(update.error, 'odometer.large_jump_requires_confirmation'))
  const fe = fieldErrors(update.error)
  const serverError = update.isError && !needsConfirm ? (fe.odometer_km ?? errorText(update.error)) : undefined
  const vars = { current: fmt.km(vehicle.current_odometer_km), next: pending ? fmt.km(pending.km) : '' }

  return (
    <form onSubmit={submit} className={`flex items-end gap-2 ${compact ? '' : 'flex-wrap'}`} noValidate>
      <NumberField
        label={t('odometer.newValue')}
        value={value}
        onChange={setValue}
        unit={t('common:units.km')}
        decimal={false}
        placeholder={fmt.number(vehicle.current_odometer_km)}
        error={localError ?? serverError}
        className="min-w-0 flex-1"
        autoFocus={autoFocus}
      />
      <Button type="submit" variant="primary" disabled={update.isPending || value.trim() === ''} aria-label={t('odometer.update')} title={t('odometer.update')}>
        <Gauge size={16} />
        {!compact && t('odometer.update')}
      </Button>
      <ConfirmDialog
        open={pending !== null}
        title={pending?.kind === 'decrease' ? t('odometer.decreaseTitle') : t('odometer.jumpTitle')}
        body={pending?.kind === 'decrease' ? t('odometer.decreaseBody', vars) : t('odometer.jumpBody', vars)}
        confirmLabel={t('common:actions.confirm')}
        danger={pending?.kind === 'decrease'}
        busy={update.isPending}
        onConfirm={() => { if (pending) update.mutate({ km: pending.km, confirmDecrease: pending.confirmDecrease, confirmJump: pending.confirmJump }) }}
        onClose={() => { setPending(null); update.reset() }}
      />
    </form>
  )
}

import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Timer } from 'lucide-react'
import { api, isApiError } from '../../lib/api'
import { parseHours } from '../../lib/format'
import { useFormat } from '../../lib/useFormat'
import { useErrorText, fieldErrors } from '../../lib/errors'
import type { HourMeter, Vehicle } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { NumberField } from '../../components/NumberField'
import { invalidateOdometerDependents } from './hooks'

interface Props {
  meter: HourMeter
  onDone?: () => void
  compact?: boolean
  autoFocus?: boolean
}

type Pending = { hours: number; kind: 'decrease' | 'jump'; confirmDecrease: boolean; confirmJump: boolean }

/** Hour meter quick update with the same confirmations as the odometer. */
export function HoursQuickUpdate({ meter, onDone, compact, autoFocus }: Props) {
  const { t } = useTranslation('vehicles')
  const { user } = useSession()
  const fmt = useFormat()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [value, setValue] = useState('')
  const [localError, setLocalError] = useState<string | undefined>()
  const [pending, setPending] = useState<Pending | null>(null)

  const update = useMutation({
    mutationFn: (p: { hours: number; confirmDecrease: boolean; confirmJump: boolean }) =>
      api.post<{ vehicle: Vehicle }>(`/hour-meters/${meter.id}/readings`, {
        hours: p.hours,
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
      if (isApiError(err, 'hour_meter.decrease_requires_confirmation')) {
        setPending({ hours: vars.hours, kind: 'decrease', confirmDecrease: true, confirmJump: vars.confirmJump })
      } else if (isApiError(err, 'hour_meter.large_jump_requires_confirmation')) {
        setPending({ hours: vars.hours, kind: 'jump', confirmDecrease: vars.confirmDecrease, confirmJump: true })
      }
    },
  })

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    const hours = parseHours(value, user.number_format)
    if (hours === null || hours === undefined) {
      setLocalError('validation.number')
      return
    }
    setLocalError(undefined)
    update.mutate({ hours, confirmDecrease: false, confirmJump: false })
  }

  const needsConfirm = update.error && (isApiError(update.error, 'hour_meter.decrease_requires_confirmation') || isApiError(update.error, 'hour_meter.large_jump_requires_confirmation'))
  const fe = fieldErrors(update.error)
  const serverError = update.isError && !needsConfirm ? (fe.hours ?? errorText(update.error)) : undefined
  const vars = { current: fmt.hours(meter.current_hours), next: pending ? fmt.hours(pending.hours) : '' }

  return (
    <form onSubmit={submit} className={`flex items-end gap-2 ${compact ? '' : 'flex-wrap'}`} noValidate>
      <NumberField
        label={t('hours.newValue', { name: meter.name })}
        value={value}
        onChange={setValue}
        unit={t('common:units.h')}
        placeholder={fmt.number(meter.current_hours, { maxDecimals: 1 })}
        error={localError ?? serverError}
        className="min-w-0 flex-1"
        autoFocus={autoFocus}
      />
      <Button type="submit" variant="primary" disabled={update.isPending || value.trim() === ''} aria-label={t('hours.update')} title={t('hours.update')}>
        <Timer size={16} />
        {!compact && t('hours.update')}
      </Button>
      <ConfirmDialog
        open={pending !== null}
        title={pending?.kind === 'decrease' ? t('hours.decreaseTitle') : t('hours.jumpTitle')}
        body={pending?.kind === 'decrease' ? t('hours.decreaseBody', vars) : t('hours.jumpBody', vars)}
        confirmLabel={t('common:actions.confirm')}
        danger={pending?.kind === 'decrease'}
        busy={update.isPending}
        onConfirm={() => { if (pending) update.mutate({ hours: pending.hours, confirmDecrease: pending.confirmDecrease, confirmJump: pending.confirmJump }) }}
        onClose={() => { setPending(null); update.reset() }}
      />
    </form>
  )
}

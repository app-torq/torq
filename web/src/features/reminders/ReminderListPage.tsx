import { useState } from 'react'
import { Link } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { CheckCircle2, NotebookPen, Pause, Pencil, Play, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseNumber } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { Reminder, ReminderState } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { Checkbox, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Card, EmptyState, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { invalidateOdometerDependents, useVehicles } from '../vehicles/hooks'
import { useCategories } from '../service-log/hooks'
import { useReminders } from './hooks'
import { ReminderForm } from './ReminderForm'
import { ReminderSummary, reminderName } from './ReminderCard'

const order: ReminderState[] = ['overdue', 'due_soon', 'upcoming']

export function ReminderListPage({ vehicleId }: { vehicleId?: number }) {
  const { t } = useTranslation('reminders')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [showPaused, setShowPaused] = useState(false)
  const reminders = useReminders({ ...(vehicleId !== undefined ? { vehicle_id: vehicleId } : {}), include_paused: showPaused })
  const vehicles = useVehicles('all')
  const categories = useCategories(true)
  const [editing, setEditing] = useState<Reminder | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Reminder | null>(null)
  const [marking, setMarking] = useState<Reminder | null>(null)

  const refresh = async () => {
    await invalidateOdometerDependents(qc)
    await qc.invalidateQueries({ queryKey: queryKeys.reminders })
  }
  const pause = useMutation({
    mutationFn: (r: Reminder) => api.post(`/reminders/${r.id}/${r.paused ? 'resume' : 'pause'}`),
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/reminders/${id}`),
    onSuccess: async () => { setDeleting(null); await refresh() },
  })

  const vehicleById = new Map(vehicles.data?.items.map((v) => [v.id, v]))
  const catById = new Map(categories.data?.items.map((c) => [c.id, c]))
  const items = reminders.data?.items ?? []

  const addButton = (
    <Button variant="primary" onClick={() => { setEditing('new') }}>
      <Plus size={16} />
      {t('add')}
    </Button>
  )

  return (
    <div>
      {vehicleId === undefined ? (
        <PageHeader title={t('title')} subtitle={t('subtitle')} actions={addButton} />
      ) : (
        <div className="mb-4 flex justify-end">{addButton}</div>
      )}
      <div className="mb-4">
        <Checkbox label={t('showPaused')} checked={showPaused} onChange={setShowPaused} />
      </div>
      {pause.isError && <div className="mb-3"><ErrorBanner message={errorText(pause.error)} /></div>}
      {reminders.isPending && <Loading />}
      {reminders.isError && <ErrorBanner message={errorText(reminders.error)} onRetry={() => void reminders.refetch()} />}
      {reminders.isSuccess && items.length === 0 && <EmptyState title={t('empty')} />}
      <div className="space-y-4">
        {order.map((state) => {
          const group = items.filter((r) => r.state === state)
          if (group.length === 0) return null
          return (
            <Card key={state} title={t(`states.${state}`)}>
              <ul className="divide-y divide-zinc-800">
                {group.map((r) => (
                  <li key={r.id}>
                    <ReminderSummary
                      reminder={r}
                      category={catById.get(r.service_category_id ?? 0)}
                      vehicle={vehicleId === undefined ? vehicleById.get(r.vehicle_id) : undefined}
                      actions={
                        <>
                          {r.service_category_id === null ? (
                            <Button size="sm" onClick={() => { setMarking(r) }}>
                              <CheckCircle2 size={15} />
                              {t('actions.markDone')}
                            </Button>
                          ) : (
                            <Link to={`/service/new?vehicle=${r.vehicle_id}`} className="focus-ring inline-flex h-8 items-center gap-2 rounded-md border border-zinc-700 bg-zinc-900 px-2.5 text-sm hover:bg-zinc-800">
                              <NotebookPen size={15} />
                              {t('actions.logService')}
                            </Link>
                          )}
                          <IconButton aria-label={r.paused ? t('actions.resume') : t('actions.pause')} title={r.paused ? t('actions.resume') : t('actions.pause')} onClick={() => { pause.mutate(r) }}>
                            {r.paused ? <Play size={15} /> : <Pause size={15} />}
                          </IconButton>
                          <IconButton aria-label={t('common:actions.edit')} onClick={() => { setEditing(r) }}>
                            <Pencil size={15} />
                          </IconButton>
                          <IconButton aria-label={t('common:actions.delete')} onClick={() => { setDeleting(r) }}>
                            <Trash2 size={15} />
                          </IconButton>
                        </>
                      }
                    />
                  </li>
                ))}
              </ul>
            </Card>
          )
        })}
      </div>
      {editing !== null && <ReminderForm reminder={editing === 'new' ? null : editing} {...(vehicleId !== undefined ? { vehicleId } : {})} onClose={() => { setEditing(null) }} />}
      {marking && <MarkDoneDialog reminder={marking} onClose={() => { setMarking(null) }} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('deleteTitle')}
        body={deleting ? `${t('deleteBody')} (${reminderName(deleting, catById.get(deleting.service_category_id ?? 0), t)})` : ''}
        confirmLabel={t('common:actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { if (deleting) remove.mutate(deleting.id) }}
        onClose={() => { setDeleting(null); remove.reset() }}
      />
    </div>
  )
}

function MarkDoneDialog({ reminder, onClose }: { reminder: Reminder; onClose: () => void }) {
  const { t } = useTranslation('reminders')
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const vehicles = useVehicles('all')
  const current = vehicles.data?.items.find((v) => v.id === reminder.vehicle_id)?.current_odometer_km
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10))
  const [km, setKm] = useState('')
  const [local, setLocal] = useState<string | undefined>()
  const done = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.post(`/reminders/${reminder.id}/done`, body),
    onSuccess: async () => {
      await invalidateOdometerDependents(qc)
      onClose()
    },
  })
  const fe = fieldErrors(done.error)
  const submit = () => {
    const body: Record<string, unknown> = { done_on: date }
    if (km.trim() !== '') {
      const n = parseNumber(km, user.number_format)
      if (n === null || !Number.isInteger(n) || n < 0) {
        setLocal('validation.number')
        return
      }
      body.odometer_km = n
    }
    setLocal(undefined)
    done.mutate(body)
  }
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('markDone.title')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={done.isPending}>
            {t('actions.markDone')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-zinc-300">{reminder.title}</p>
        {done.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(done.error)} />}
        <TextField label={t('markDone.date')} type="date" value={date} onChange={setDate} error={fe.done_on} />
        <NumberField
          label={t('markDone.odometer')}
          unit={t('common:units.km')}
          decimal={false}
          value={km}
          onChange={setKm}
          placeholder={current !== undefined ? String(current) : ''}
          error={local ?? fe.odometer_km}
          optional
        />
      </div>
    </Dialog>
  )
}

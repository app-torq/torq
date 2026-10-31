import { useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowDown, ArrowUp, Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseNumber, toInputValue } from '../../lib/format'
import { useFormat } from '../../lib/useFormat'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { SpecEntry, VehicleSpecs } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { TextAreaField, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Card, ErrorBanner, Loading } from '../../components/ui'
import { useVehicleContext } from './VehicleDetailLayout'
import { SpecEntryImages } from './SpecEntryImages'

interface SpecsResponse {
  specs: VehicleSpecs
  entries: SpecEntry[]
}

export function useSpecs(vehicleId: number) {
  return useQuery({
    queryKey: queryKeys.specs(vehicleId),
    queryFn: () => api.get<SpecsResponse>(`/vehicles/${vehicleId}/specs`),
  })
}

type FluidKey = 'transmission_fluid' | 'differential_fluid' | 'brake_fluid' | 'coolant'
const fluidGroups: { key: FluidKey; label: string }[] = [
  { key: 'transmission_fluid', label: 'groups.transmission' },
  { key: 'differential_fluid', label: 'groups.differential' },
  { key: 'brake_fluid', label: 'groups.brake' },
  { key: 'coolant', label: 'groups.coolant' },
]

function fluidSpec(s: VehicleSpecs, k: FluidKey): string {
  return k === 'coolant' ? s.coolant_spec : s[`${k}_spec`]
}
function fluidCapacity(s: VehicleSpecs, k: FluidKey): number | null {
  return k === 'coolant' ? s.coolant_capacity_ml : s[`${k}_capacity_ml`]
}

export function SpecSheetPage() {
  const v = useVehicleContext()
  const { t } = useTranslation('specs')
  const fmt = useFormat()
  const errorText = useErrorText()
  const specs = useSpecs(v.id)
  const [editing, setEditing] = useState(false)

  if (specs.isPending) return <Loading />
  if (specs.isError) return <ErrorBanner message={errorText(specs.error)} onRetry={() => void specs.refetch()} />
  const s = specs.data.specs
  const none = t('common:state.none')
  const join = (...parts: (string | null)[]) => parts.filter((p): p is string => !!p).join(' · ') || none

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-zinc-400">{t('intro')}</p>
        <Button onClick={() => { setEditing(true) }}>
          <Pencil size={15} />
          {t('edit')}
        </Button>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <Pin title={t('groups.engineOil')}>
          {join(s.engine_oil_viscosity, s.engine_oil_spec, s.engine_oil_capacity_ml ? fmt.liters(s.engine_oil_capacity_ml) : null)}
        </Pin>
        {fluidGroups.map((g) => {
          const cap = fluidCapacity(s, g.key)
          return (
            <Pin key={g.key} title={t(g.label)}>
              {join(fluidSpec(s, g.key), cap ? fmt.liters(cap) : null)}
            </Pin>
          )
        })}
        <Pin title={t('groups.wheels')}>
          <div className="space-y-0.5">
            <div>{t('fields.lugTorque')}: {s.wheel_lug_torque_nm ? fmt.nm(s.wheel_lug_torque_nm) : none}</div>
            <div>
              {t('fields.pressureFront')}: {s.tire_pressure_front_bar ? fmt.bar(s.tire_pressure_front_bar) : none}
            </div>
            <div>
              {t('fields.pressureRear')}: {s.tire_pressure_rear_bar ? fmt.bar(s.tire_pressure_rear_bar) : none}
            </div>
          </div>
        </Pin>
      </div>
      <CustomEntries vehicleId={v.id} entries={specs.data.entries} />
      {editing && <SpecsEditDialog vehicleId={v.id} specs={s} onClose={() => { setEditing(false) }} />}
    </div>
  )
}

function Pin({ title, children }: { title: ReactNode; children: ReactNode }) {
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
      <div className="mb-1 text-xs uppercase tracking-wide text-zinc-400">{title}</div>
      <div className="text-sm tabular-nums text-zinc-100">{children}</div>
    </div>
  )
}

const capacityKeys = ['engine_oil', 'transmission_fluid', 'differential_fluid', 'brake_fluid', 'coolant'] as const
type CapacityKey = (typeof capacityKeys)[number]

function SpecsEditDialog({ vehicleId, specs, onClose }: { vehicleId: number; specs: VehicleSpecs; onClose: () => void }) {
  const { t } = useTranslation('specs')
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const nf = user.number_format
  const [text, setText] = useState({
    engine_oil_viscosity: specs.engine_oil_viscosity,
    engine_oil_spec: specs.engine_oil_spec,
    transmission_fluid_spec: specs.transmission_fluid_spec,
    differential_fluid_spec: specs.differential_fluid_spec,
    brake_fluid_spec: specs.brake_fluid_spec,
    coolant_spec: specs.coolant_spec,
  })
  const [caps, setCaps] = useState<Record<CapacityKey, string>>({
    engine_oil: toInputValue(specs.engine_oil_capacity_ml === null ? null : specs.engine_oil_capacity_ml / 1000, nf),
    transmission_fluid: toInputValue(specs.transmission_fluid_capacity_ml === null ? null : specs.transmission_fluid_capacity_ml / 1000, nf),
    differential_fluid: toInputValue(specs.differential_fluid_capacity_ml === null ? null : specs.differential_fluid_capacity_ml / 1000, nf),
    brake_fluid: toInputValue(specs.brake_fluid_capacity_ml === null ? null : specs.brake_fluid_capacity_ml / 1000, nf),
    coolant: toInputValue(specs.coolant_capacity_ml === null ? null : specs.coolant_capacity_ml / 1000, nf),
  })
  const [torque, setTorque] = useState(toInputValue(specs.wheel_lug_torque_nm, nf, 1))
  const [front, setFront] = useState(toInputValue(specs.tire_pressure_front_bar, nf, 2))
  const [rear, setRear] = useState(toInputValue(specs.tire_pressure_rear_bar, nf, 2))
  const [local, setLocal] = useState<Record<string, string>>({})

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.put(`/vehicles/${vehicleId}/specs`, body),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.specs(vehicleId) })
      onClose()
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const submit = () => {
    const errs: Record<string, string> = {}
    const num = (field: string, raw: string): number | null => {
      if (raw.trim() === '') return null
      const n = parseNumber(raw, nf)
      if (n === null) errs[field] = 'validation.number'
      return n
    }
    const body: Record<string, unknown> = { ...text }
    for (const k of capacityKeys) {
      const liters = num(`${k}_capacity_ml`, caps[k])
      body[`${k === 'coolant' ? 'coolant' : k}_capacity_ml`] = liters === null ? null : Math.round(liters * 1000)
    }
    body.wheel_lug_torque_nm = num('wheel_lug_torque_nm', torque)
    body.tire_pressure_front_bar = num('tire_pressure_front_bar', front)
    body.tire_pressure_rear_bar = num('tire_pressure_rear_bar', rear)
    setLocal(errs)
    if (Object.keys(errs).length === 0) save.mutate(body)
  }

  const L = t('common:units.l')
  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={t('edit')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={save.isPending}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <fieldset className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.engineOil')}</legend>
          <TextField label={t('fields.viscosity')} value={text.engine_oil_viscosity} onChange={(x) => { setText({ ...text, engine_oil_viscosity: x }) }} placeholder="5W-30" error={fe.engine_oil_viscosity} />
          <TextField label={t('fields.spec')} value={text.engine_oil_spec} onChange={(x) => { setText({ ...text, engine_oil_spec: x }) }} placeholder="MB 229.51" error={fe.engine_oil_spec} />
          <NumberField label={t('fields.capacity')} unit={L} value={caps.engine_oil} onChange={(x) => { setCaps({ ...caps, engine_oil: x }) }} error={fe.engine_oil_capacity_ml} />
        </fieldset>
        {fluidGroups.map((g) => (
          <fieldset key={g.key} className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <legend className="mb-2 text-sm font-semibold text-zinc-200">{t(g.label)}</legend>
            <TextField
              label={t('fields.spec')}
              className="sm:col-span-2"
              value={text[`${g.key}_spec` as keyof typeof text]}
              onChange={(x) => { setText({ ...text, [`${g.key}_spec`]: x }) }}
              error={fe[`${g.key}_spec`]}
            />
            <NumberField label={t('fields.capacity')} unit={L} value={caps[g.key]} onChange={(x) => { setCaps({ ...caps, [g.key]: x }) }} error={fe[`${g.key}_capacity_ml`]} />
          </fieldset>
        ))}
        <fieldset className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.wheels')}</legend>
          <NumberField label={t('fields.lugTorque')} unit={t('common:units.nm')} value={torque} onChange={setTorque} error={fe.wheel_lug_torque_nm} />
          <NumberField label={t('fields.pressureFront')} unit={t('common:units.bar')} value={front} onChange={setFront} error={fe.tire_pressure_front_bar} />
          <NumberField label={t('fields.pressureRear')} unit={t('common:units.bar')} value={rear} onChange={setRear} error={fe.tire_pressure_rear_bar} />
        </fieldset>
      </div>
    </Dialog>
  )
}

function CustomEntries({ vehicleId, entries }: { vehicleId: number; entries: SpecEntry[] }) {
  const { t } = useTranslation('specs')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [editing, setEditing] = useState<SpecEntry | 'new' | null>(null)
  const [deleting, setDeleting] = useState<SpecEntry | null>(null)
  const invalidate = () => qc.invalidateQueries({ queryKey: queryKeys.specs(vehicleId) })

  const reorder = useMutation({
    mutationFn: (ids: number[]) => api.put(`/vehicles/${vehicleId}/spec-entries/order`, { ids }),
    onSettled: invalidate,
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/spec-entries/${id}`),
    onSuccess: async () => { setDeleting(null); await invalidate() },
  })

  const move = (index: number, delta: number) => {
    const ids = entries.map((e) => e.id)
    const target = index + delta
    const a = ids[index]
    const b = ids[target]
    if (a === undefined || b === undefined) return
    ids[index] = b
    ids[target] = a
    reorder.mutate(ids)
  }

  return (
    <Card
      title={t('groups.custom')}
      actions={
        <Button size="sm" onClick={() => { setEditing('new') }}>
          <Plus size={15} />
          {t('entries.add')}
        </Button>
      }
    >
      {reorder.isError && <ErrorBanner message={errorText(reorder.error)} />}
      {entries.length === 0 && <p className="text-sm text-zinc-400">{t('empty')}</p>}
      <ul className="divide-y divide-zinc-800">
        {entries.map((e, i) => (
          <li key={e.id} className="flex gap-3 py-3">
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-zinc-100">{e.label}</div>
              {e.value && <div className="whitespace-pre-wrap text-sm text-zinc-300">{e.value}</div>}
              <SpecEntryImages entry={e} vehicleId={vehicleId} />
            </div>
            <div className="flex shrink-0 items-start gap-0.5">
              <IconButton aria-label={t('common:actions.moveUp')} disabled={i === 0 || reorder.isPending} onClick={() => { move(i, -1) }}>
                <ArrowUp size={15} />
              </IconButton>
              <IconButton aria-label={t('common:actions.moveDown')} disabled={i === entries.length - 1 || reorder.isPending} onClick={() => { move(i, 1) }}>
                <ArrowDown size={15} />
              </IconButton>
              <IconButton aria-label={t('common:actions.edit')} onClick={() => { setEditing(e) }}>
                <Pencil size={15} />
              </IconButton>
              <IconButton aria-label={t('common:actions.delete')} onClick={() => { setDeleting(e) }}>
                <Trash2 size={15} />
              </IconButton>
            </div>
          </li>
        ))}
      </ul>
      {editing !== null && <EntryDialog vehicleId={vehicleId} entry={editing === 'new' ? null : editing} onClose={() => { setEditing(null) }} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('entries.deleteTitle')}
        body={t('entries.deleteBody', { label: deleting?.label ?? '' })}
        confirmLabel={t('common:actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { if (deleting) remove.mutate(deleting.id) }}
        onClose={() => { setDeleting(null); remove.reset() }}
      />
    </Card>
  )
}

function EntryDialog({ vehicleId, entry, onClose }: { vehicleId: number; entry: SpecEntry | null; onClose: () => void }) {
  const { t } = useTranslation('specs')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [label, setLabel] = useState(entry?.label ?? '')
  const [value, setValue] = useState(entry?.value ?? '')
  const save = useMutation({
    mutationFn: () => (entry ? api.patch(`/spec-entries/${entry.id}`, { label, value }) : api.post(`/vehicles/${vehicleId}/spec-entries`, { label, value })),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.specs(vehicleId) })
      onClose()
    },
  })
  const fe = fieldErrors(save.error)
  return (
    <Dialog
      open
      onClose={onClose}
      title={entry ? t('entries.editTitle') : t('entries.addTitle')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={() => { save.mutate() }} disabled={save.isPending}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <TextField label={t('fields.label')} value={label} onChange={setLabel} error={fe.label} maxLength={80} autoFocus />
        <TextAreaField label={t('fields.value')} value={value} onChange={setValue} error={fe.value} maxLength={2000} hint={t('entries.valueOrImage')} />
      </div>
    </Dialog>
  )
}

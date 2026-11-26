import { useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowDown, ArrowUp, ExternalLink, Loader2, Pencil, PencilLine, Plus, Sparkles, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { powerW } from '../../lib/format'
import { PRESSURE, TORQUE, VOLUME } from '../../lib/units'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { FINAL_DRIVES, type AutofillValue, type MotoFieldKey, type SpecEntry, type SpecFieldKey, type SpecMeta, type VehicleSpecs } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { canOn, useCan } from '../../lib/permissions'
import { Checkbox, SelectField, TextAreaField, TextField } from '../../components/Field'
import { NumberField, NumberRangeField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Card, ErrorBanner, Loading, Notice } from '../../components/ui'
import { useVehicleContext } from './VehicleDetailLayout'
import { SpecEntryAttachments } from './SpecEntryAttachments'
import { applyAutofill, editField, editMoto, initialDraft, rangeMeta, toRequestBody, type AutofillSummary, type SpecUnits } from './specsDraft'

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

/**
 * The fluid groups a vehicle shows. On a motorcycle the differential fields
 * hold the final drive oil, shown for shaft drive only (FR-014).
 */
function fluidGroupsFor(motorcycle: boolean, finalDrive: string | null) {
  if (!motorcycle) return fluidGroups
  return fluidGroups
    .filter((g) => g.key !== 'differential_fluid' || finalDrive === 'shaft')
    .map((g) => (g.key === 'differential_fluid' ? { ...g, label: 'groups.finalDriveOil' } : g))
}

/** Chain and sprocket data apply to chain drive, or while the drive is not set. */
const hasChain = (finalDrive: string | null) => !finalDrive || finalDrive === 'chain'

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
  const m = s.motorcycle
  const motorcycle = v.vehicle_type === 'motorcycle'
  const none = t('common:state.none')
  const join = (...parts: (string | null)[]) => parts.filter((p): p is string => !!p).join(' · ') || none
  const mm = (lo: number | null, hi: number | null) =>
    lo == null ? none : `${[lo, hi].filter((x): x is number => x != null).map((x) => fmt.number(x, { decimals: 2 })).join('–')} mm`
  const teeth = (n: number | null) => (n == null ? '—' : String(n))

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-zinc-400">{t('intro')}</p>
        {canOn(v, 'specs.edit') && (
          <Button onClick={() => { setEditing(true) }}>
            <Pencil size={15} />
            {t('edit')}
          </Button>
        )}
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <Pin title={t('groups.powertrain')}>
          <div className="space-y-0.5">
            <div>{t('vehicles:fields.engineCode')}: {v.engine_code || none}</div>
            <div>{t('vehicles:fields.power')}: {v.power_kw != null ? fmt.power(powerW(v) ?? 0, v.power_unit) : none}</div>
            <div>{t('vehicles:fields.displacement')}: {v.displacement_cc != null ? fmt.displacement(v.displacement_cc, v.displacement_unit) : none}</div>
            <div>{t('vehicles:fields.fuelType')}: {v.fuel_type ? t(`vehicles:fuel.${v.fuel_type}`) : none}</div>
            <div>{t('vehicles:fields.hsnTsn')}: {v.hsn || v.tsn ? <span className="font-mono">{`${v.hsn ?? '—'} / ${v.tsn ?? '—'}`}</span> : none}</div>
          </div>
        </Pin>
        <Pin title={t('groups.engineOil')}>
          {join(s.engine_oil_viscosity, s.engine_oil_spec, s.engine_oil_capacity_ml ? fmt.volume(s.engine_oil_capacity_ml) : null)}
        </Pin>
        {fluidGroupsFor(motorcycle, m.final_drive).map((g) => {
          const cap = fluidCapacity(s, g.key)
          return (
            <Pin key={g.key} title={t(g.label)}>
              {join(fluidSpec(s, g.key), cap ? fmt.volume(cap) : null)}
            </Pin>
          )
        })}
        {motorcycle && (
          <>
            <Pin title={t('groups.drivetrain')}>
              <div className="space-y-0.5">
                <div>{t('fields.finalDrive')}: {m.final_drive ? t(`finalDrive.${m.final_drive}`) : none}</div>
                {hasChain(m.final_drive) && (
                  <>
                    <div>
                      {t('fields.sprockets')}:{' '}
                      {m.sprocket_front_teeth != null || m.sprocket_rear_teeth != null ? `${teeth(m.sprocket_front_teeth)} / ${teeth(m.sprocket_rear_teeth)}` : none}
                    </div>
                    <div>
                      {t('fields.chain')}: {join(m.chain_size || null, m.chain_links != null ? t('fields.chainLinksValue', { count: m.chain_links }) : null)}
                    </div>
                  </>
                )}
              </div>
            </Pin>
            <Pin title={t('groups.forkOil')}>
              {join(m.fork_oil_spec || null, m.fork_oil_capacity_ml ? t('fields.perLeg', { volume: fmt.volume(m.fork_oil_capacity_ml) }) : null)}
            </Pin>
            <Pin title={t('groups.valveClearance')}>
              <div className="space-y-0.5">
                <div>{t('fields.intake')}: {mm(m.valve_clearance_intake_min_mm, m.valve_clearance_intake_max_mm)}</div>
                <div>{t('fields.exhaust')}: {mm(m.valve_clearance_exhaust_min_mm, m.valve_clearance_exhaust_max_mm)}</div>
              </div>
            </Pin>
          </>
        )}
        <Pin title={t('groups.wheels')}>
          <div className="space-y-0.5">
            {!motorcycle && <div>{t('fields.lugTorque')}: {s.wheel_lug_torque_nm ? fmt.torque(s.wheel_lug_torque_nm) : none}</div>}
            <div>
              {t('fields.pressureFront')}: {s.tire_pressure_front_bar ? fmt.pressureRange(s.tire_pressure_front_bar, s.tire_pressure_front_max_bar) : none}
            </div>
            <div>
              {t('fields.pressureRear')}: {s.tire_pressure_rear_bar ? fmt.pressureRange(s.tire_pressure_rear_bar, s.tire_pressure_rear_max_bar) : none}
            </div>
          </div>
        </Pin>
      </div>
      <CustomEntries vehicleId={v.id} entries={specs.data.entries} canEdit={canOn(v, 'specs.edit')} />
      {editing && <SpecsEditDialog vehicleId={v.id} specs={s} motorcycle={motorcycle} onClose={() => { setEditing(false) }} />}
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

export function SpecsEditDialog({ vehicleId, specs, motorcycle = false, onClose }: {
  vehicleId: number
  specs: VehicleSpecs
  /** Shows the motorcycle fields and hides the car-only ones. */
  motorcycle?: boolean
  onClose: () => void
}) {
  const { t } = useTranslation('specs')
  const { user, instance } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const nf = user.number_format
  const units: SpecUnits = { volume: user.volume_unit, torque: user.torque_unit, pressure: user.pressure_unit }
  const [draft, setDraft] = useState(() => initialDraft(specs, nf, units))
  const [local, setLocal] = useState<Record<string, string>>({})
  const [overwrite, setOverwrite] = useState(false)
  const [summary, setSummary] = useState<AutofillSummary | null>(null)

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.put<{ specs: VehicleSpecs }>(`/vehicles/${vehicleId}/specs`, body),
    onSuccess: async ({ specs: saved }) => {
      // Show the saved values right away; the refetch then confirms them.
      qc.setQueryData<SpecsResponse>(queryKeys.specs(vehicleId), (prev) => (prev ? { ...prev, specs: saved } : prev))
      await qc.invalidateQueries({ queryKey: queryKeys.specs(vehicleId) })
      onClose()
    },
  })
  const autofill = useMutation({
    mutationFn: () => api.post<{ fields: Record<string, AutofillValue> }>(`/vehicles/${vehicleId}/autofill-specs`),
    onSuccess: ({ fields }) => {
      // Applied to the latest draft, so edits made while the lookup ran survive.
      setDraft((prev) => {
        const res = applyAutofill(prev, fields, overwrite, nf, units)
        setSummary(res.summary)
        return res.draft
      })
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const submit = () => {
    const { body, errors } = toRequestBody(draft, nf, units)
    setLocal(errors)
    if (Object.keys(errors).length === 0) save.mutate(body)
  }

  /** Props shared by every spec input: value, change handler and the source badge. */
  const bind = (k: SpecFieldKey) => ({
    value: draft.values[k],
    onChange: (x: string) => { setDraft((prev) => editField(prev, k, x)) },
    labelAside: <SourceBadge meta={draft.meta[k]} />,
    error: fe[k],
  })

  /** Props of a motorcycle input. */
  const bindMoto = (k: MotoFieldKey) => ({
    value: draft.moto[k],
    onChange: (x: string) => { setDraft((prev) => editMoto(prev, k, x)) },
    error: fe[`motorcycle.${k}`],
  })
  const finalDrive = draft.moto.final_drive

  const L = t(`common:units.${VOLUME[units.volume].label}`)
  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={t('edit')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={save.isPending || autofill.isPending}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <div className="space-y-2 rounded-lg border border-zinc-800 bg-zinc-950/60 p-3">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <Button size="sm" onClick={() => { autofill.mutate() }} disabled={!instance.spec_autofill || autofill.isPending}>
              {autofill.isPending ? <Loader2 aria-hidden size={14} className="animate-spin" /> : <Sparkles aria-hidden size={14} />}
              {autofill.isPending ? t('autofill.running') : t('autofill.button')}
            </Button>
            <Checkbox label={t('autofill.overwrite')} checked={overwrite} onChange={setOverwrite} disabled={!instance.spec_autofill || autofill.isPending} />
          </div>
          <p className="text-xs text-zinc-400">{instance.spec_autofill ? t('autofill.hint') : t('autofill.unavailable')}</p>
          {autofill.isError && <ErrorBanner message={errorText(autofill.error)} />}
          {summary && !autofill.isPending && (
            <Notice tone={summary.filled.length > 0 ? 'emerald' : 'amber'}>
              {summary.filled.length > 0
                ? t('autofill.summary', { filled: summary.filled.length, kept: summary.kept.length, missing: summary.missing.length })
                : t('autofill.nothing', { kept: summary.kept.length })}
            </Notice>
          )}
        </div>
        {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <fieldset className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-3">
          <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.engineOil')}</legend>
          <TextField label={t('fields.viscosity')} placeholder="5W-30" {...bind('engine_oil_viscosity')} />
          <TextField label={t('fields.spec')} placeholder="MB 229.51" {...bind('engine_oil_spec')} />
          <NumberField label={t('fields.capacity')} unit={L} {...bind('engine_oil_capacity_ml')} />
        </fieldset>
        {fluidGroupsFor(motorcycle, finalDrive).map((g) => (
          <fieldset key={g.key} className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-3">
            <legend className="mb-2 text-sm font-semibold text-zinc-200">{t(g.label)}</legend>
            <TextField label={t('fields.spec')} className="sm:col-span-2" {...bind(`${g.key}_spec`)} />
            <NumberField label={t('fields.capacity')} unit={L} {...bind(`${g.key}_capacity_ml`)} />
          </fieldset>
        ))}
        {motorcycle && (
          <>
            <fieldset className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-4">
              <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.drivetrain')}</legend>
              <SelectField
                label={t('fields.finalDrive')}
                className="sm:col-span-4"
                options={[{ value: '', label: t('common:state.none') }, ...FINAL_DRIVES.map((d) => ({ value: d, label: t(`finalDrive.${d}`) }))]}
                {...bindMoto('final_drive')}
              />
              {hasChain(finalDrive) && (
                <>
                  <NumberField label={t('fields.sprocketFront')} decimal={false} {...bindMoto('sprocket_front_teeth')} />
                  <NumberField label={t('fields.sprocketRear')} decimal={false} {...bindMoto('sprocket_rear_teeth')} />
                  <TextField label={t('fields.chainSize')} placeholder="525" maxLength={40} {...bindMoto('chain_size')} />
                  <NumberField label={t('fields.chainLinks')} decimal={false} {...bindMoto('chain_links')} />
                </>
              )}
            </fieldset>
            <fieldset className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-3">
              <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.forkOil')}</legend>
              <TextField label={t('fields.spec')} placeholder="SAE 7.5W" className="sm:col-span-2" maxLength={120} {...bindMoto('fork_oil_spec')} />
              <NumberField label={t('fields.capacity')} unit={L} hint={t('fields.perLegHint')} {...bindMoto('fork_oil_capacity_ml')} />
            </fieldset>
            <fieldset className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-2">
              <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.valveClearance')}</legend>
              {(['intake', 'exhaust'] as const).map((side) => {
                const lo = `valve_clearance_${side}_min_mm` as const
                const hi = `valve_clearance_${side}_max_mm` as const
                const label = t(side === 'intake' ? 'fields.intake' : 'fields.exhaust')
                return (
                  <NumberRangeField
                    key={side}
                    label={label}
                    from={draft.moto[lo]}
                    to={draft.moto[hi]}
                    onFromChange={(x) => { setDraft((prev) => editMoto(prev, lo, x)) }}
                    onToChange={(x) => { setDraft((prev) => editMoto(prev, hi, x)) }}
                    toLabel={t('fields.clearanceUpTo', { field: label })}
                    toPlaceholder={t('fields.pressureMax')}
                    unit="mm"
                    error={fe[`motorcycle.${lo}`] ?? fe[`motorcycle.${hi}`]}
                  />
                )
              })}
            </fieldset>
          </>
        )}
        <fieldset className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-3">
          <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.wheels')}</legend>
          {!motorcycle && <NumberField label={t('fields.lugTorque')} unit={t(`common:units.${TORQUE[units.torque].label}`)} {...bind('wheel_lug_torque_nm')} />}
          {(['front', 'rear'] as const).map((axle) => {
            const lo = `tire_pressure_${axle}_bar` as const
            const hi = `tire_pressure_${axle}_max_bar` as const
            const label = t(axle === 'front' ? 'fields.pressureFront' : 'fields.pressureRear')
            return (
              <NumberRangeField
                key={axle}
                label={label}
                labelAside={<SourceBadge meta={rangeMeta(draft, lo, hi)} />}
                from={draft.values[lo]}
                to={draft.values[hi]}
                onFromChange={(x) => { setDraft((prev) => editField(prev, lo, x)) }}
                onToChange={(x) => { setDraft((prev) => editField(prev, hi, x)) }}
                toLabel={t('fields.pressureUpTo', { field: label })}
                toPlaceholder={t('fields.pressureMax')}
                unit={t(`common:units.${PRESSURE[units.pressure].label}`)}
                error={fe[lo] ?? fe[hi]}
                hint={t('fields.pressureRangeHint')}
              />
            )
          })}
        </fieldset>
      </div>
    </Dialog>
  )
}

/**
 * Marks an auto-filled value with a link to its source, or a value the user
 * changed after the auto-fill. Manually entered values get no badge.
 */
function SourceBadge({ meta }: { meta: SpecMeta | undefined }) {
  const { t } = useTranslation('specs')
  if (!meta) return null
  if (!meta.is_auto || !meta.source_url) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium text-zinc-400 ring-1 ring-zinc-700 ring-inset">
        <PencilLine aria-hidden size={11} />
        {t('autofill.manual')}
      </span>
    )
  }
  const name = meta.source_name || new URL(meta.source_url).hostname.replace(/^www\./, '')
  return (
    <a
      href={meta.source_url}
      target="_blank"
      rel="noopener noreferrer"
      title={t('autofill.openSource', { url: meta.source_url })}
      aria-label={t('autofill.sourceLabel', { name })}
      className="focus-ring inline-flex max-w-full items-center gap-1 rounded-full bg-sky-500/10 px-2 py-0.5 text-[11px] font-medium text-sky-300 ring-1 ring-sky-500/30 ring-inset transition-colors hover:bg-sky-500/20 hover:text-sky-200"
    >
      <Sparkles aria-hidden size={11} className="shrink-0" />
      <span className="truncate">{t('autofill.auto')} · {name}</span>
      <ExternalLink aria-hidden size={10} className="shrink-0" />
    </a>
  )
}

function CustomEntries({ vehicleId, entries, canEdit }: { vehicleId: number; entries: SpecEntry[]; canEdit: boolean }) {
  const { t } = useTranslation('specs')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const seesAttachments = useCan('attachments.view')
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
      actions={canEdit && (
        <Button size="sm" onClick={() => { setEditing('new') }}>
          <Plus size={15} />
          {t('entries.add')}
        </Button>
      )}
    >
      {reorder.isError && <ErrorBanner message={errorText(reorder.error)} />}
      {entries.length === 0 && <p className="text-sm text-zinc-400">{t('empty')}</p>}
      <ul className="divide-y divide-zinc-800">
        {entries.map((e, i) => (
          <li key={e.id} className="flex gap-3 py-3">
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium text-zinc-100">{e.label}</div>
              {e.value && <div className="whitespace-pre-wrap text-sm text-zinc-300">{e.value}</div>}
              {seesAttachments && <SpecEntryAttachments entry={e} vehicleId={vehicleId} canEdit={canEdit} />}
            </div>
            {canEdit && <div className="flex shrink-0 items-start gap-0.5">
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
            </div>}
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

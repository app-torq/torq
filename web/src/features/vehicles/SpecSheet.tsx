import { useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowDown, ArrowUp, ExternalLink, Loader2, Pencil, PencilLine, Plus, Sparkles, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { AutofillValue, SpecEntry, SpecFieldKey, SpecMeta, VehicleSpecs } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { Checkbox, TextAreaField, TextField } from '../../components/Field'
import { NumberField, NumberRangeField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Card, ErrorBanner, Loading, Notice } from '../../components/ui'
import { useVehicleContext } from './VehicleDetailLayout'
import { SpecEntryImages } from './SpecEntryImages'
import { applyAutofill, editField, initialDraft, rangeMeta, toRequestBody, type AutofillSummary } from './specsDraft'

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
        <Pin title={t('groups.powertrain')}>
          <div className="space-y-0.5">
            <div>{t('vehicles:fields.engineCode')}: {v.engine_code || none}</div>
            <div>{t('vehicles:fields.power')}: {v.power_kw != null ? fmt.power(v.power_kw) : none}</div>
            <div>{t('vehicles:fields.displacement')}: {v.displacement_cc != null ? fmt.displacement(v.displacement_cc) : none}</div>
            <div>{t('vehicles:fields.fuelType')}: {v.fuel_type ? t(`vehicles:fuel.${v.fuel_type}`) : none}</div>
            <div>{t('vehicles:fields.hsnTsn')}: {v.hsn || v.tsn ? <span className="font-mono">{`${v.hsn ?? '—'} / ${v.tsn ?? '—'}`}</span> : none}</div>
          </div>
        </Pin>
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
              {t('fields.pressureFront')}: {s.tire_pressure_front_bar ? fmt.barRange(s.tire_pressure_front_bar, s.tire_pressure_front_max_bar) : none}
            </div>
            <div>
              {t('fields.pressureRear')}: {s.tire_pressure_rear_bar ? fmt.barRange(s.tire_pressure_rear_bar, s.tire_pressure_rear_max_bar) : none}
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

export function SpecsEditDialog({ vehicleId, specs, onClose }: { vehicleId: number; specs: VehicleSpecs; onClose: () => void }) {
  const { t } = useTranslation('specs')
  const { user, instance } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const nf = user.number_format
  const [draft, setDraft] = useState(() => initialDraft(specs, nf))
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
        const res = applyAutofill(prev, fields, overwrite, nf)
        setSummary(res.summary)
        return res.draft
      })
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const submit = () => {
    const { body, errors } = toRequestBody(draft, nf)
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
        {fluidGroups.map((g) => (
          <fieldset key={g.key} className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-3">
            <legend className="mb-2 text-sm font-semibold text-zinc-200">{t(g.label)}</legend>
            <TextField label={t('fields.spec')} className="sm:col-span-2" {...bind(`${g.key}_spec`)} />
            <NumberField label={t('fields.capacity')} unit={L} {...bind(`${g.key}_capacity_ml`)} />
          </fieldset>
        ))}
        <fieldset className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-3">
          <legend className="mb-2 text-sm font-semibold text-zinc-200">{t('groups.wheels')}</legend>
          <NumberField label={t('fields.lugTorque')} unit={t('common:units.nm')} {...bind('wheel_lug_torque_nm')} />
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
                unit={t('common:units.bar')}
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

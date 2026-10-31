import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Pencil, Plus, Trash2, Wrench } from 'lucide-react'
import { api } from '../../lib/api'
import { parseNumber, toInputValue } from '../../lib/format'
import { useFormat } from '../../lib/useFormat'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { vehicleName, type Repair, type TorqueSpec } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { TextAreaField, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { AttachmentGallery } from '../../components/AttachmentGallery'
import { FileDrop } from '../../components/FileDrop'
import { Badge, Card, DefinitionList, ErrorBanner, Loading, Notice, PageHeader, Table } from '../../components/ui'
import { invalidateOdometerDependents, useVehicle } from '../vehicles/hooks'
import { invalidateRepairs, statusTone, useRepair } from './hooks'
import { StatusMenu } from './RepairBoardPage'
import { CompleteRepairDialog } from './CompleteRepairDialog'
import { FaultCodeInput } from './FaultCodeInput'
import { PartsTable } from './PartsTable'

export function RepairDetailPage() {
  const { id } = useParams()
  const repairId = Number(id)
  const { t } = useTranslation('repairs')
  const fmt = useFormat()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const errorText = useErrorText()
  const repair = useRepair(repairId)
  const vehicle = useVehicle(repair.data?.vehicle_id ?? 0)
  const [editing, setEditing] = useState(false)
  const [completing, setCompleting] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const remove = useMutation({
    mutationFn: () => api.del(`/repairs/${repairId}`),
    onSuccess: async () => {
      qc.removeQueries({ queryKey: ['repairs', 'detail', repairId] })
      await Promise.all([invalidateRepairs(qc), invalidateOdometerDependents(qc)])
      void navigate('/repairs')
    },
  })

  if (repair.isPending) return <Loading />
  if (repair.isError) return <ErrorBanner message={errorText(repair.error)} />
  const r = repair.data
  const tools = r.special_tools.split('\n').map((s) => s.trim()).filter(Boolean)
  const none = t('common:state.none')
  const refresh = () => void invalidateRepairs(qc, r.id)

  return (
    <div className="space-y-6">
      <PageHeader
        title={r.title}
        subtitle={vehicle.data ? <Link className="text-sky-300 hover:underline" to={`/vehicles/${vehicle.data.id}/repairs`}>{vehicleName(vehicle.data)}</Link> : undefined}
        actions={
          <>
            <Button onClick={() => { setEditing(true) }}>
              <Pencil size={15} />
              {t('common:actions.edit')}
            </Button>
            <Button variant="danger" onClick={() => { setDeleting(true) }}>
              <Trash2 size={15} />
              {t('common:actions.delete')}
            </Button>
          </>
        }
      />
      <Card>
        <div className="flex flex-wrap items-start gap-6">
          <div className="space-y-2">
            <Badge tone={statusTone[r.status]}>{t(`statuses.${r.status}`)}</Badge>
            <div className="w-48"><StatusMenu repair={r} onComplete={() => { setCompleting(true) }} /></div>
          </div>
          <div className="min-w-0 flex-1">
            <DefinitionList
              items={[
                { label: t('fields.startedOn'), value: r.started_on ? fmt.date(r.started_on) : none },
                { label: t('fields.completedOn'), value: r.status === 'completed' && r.completed_on ? fmt.date(r.completed_on) : none },
                { label: t('fields.completionOdometer'), value: r.status === 'completed' && r.completion_odometer_km !== null ? fmt.km(r.completion_odometer_km) : none },
                { label: t('parts.total'), value: <span className="tabular-nums">{fmt.money(r.parts_total_cents)}</span> },
              ]}
            />
          </div>
        </div>
        {r.status !== 'completed' && r.completed_on && <div className="mt-3"><Notice tone="amber">{t('reopenHint')}</Notice></div>}
      </Card>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card title={t('sections.diagnostics')}>
          <div className="space-y-3 text-sm">
            <Section label={t('fields.symptom')} text={r.symptom} />
            <div>
              <div className="text-xs uppercase tracking-wide text-zinc-400">{t('fields.faultCodes')}</div>
              <div className="mt-1 flex flex-wrap gap-1">
                {r.fault_codes.length === 0 && <span className="text-zinc-400">{none}</span>}
                {r.fault_codes.map((c) => <span key={c} className="rounded bg-amber-500/15 px-1.5 py-0.5 font-mono text-xs text-amber-200">{c}</span>)}
              </div>
            </div>
            <Section label={t('fields.rootCause')} text={r.root_cause} />
          </div>
        </Card>
        <Card title={t('sections.reference')}>
          <div className="space-y-4 text-sm">
            {r.torque_specs.length > 0 ? (
              <Table>
                <thead>
                  <tr>
                    <th>{t('fields.torqueLabel')}</th>
                    <th className="text-right">{t('fields.torqueValue')}</th>
                    <th>{t('fields.torqueNote')}</th>
                  </tr>
                </thead>
                <tbody>
                  {r.torque_specs.map((ts, i) => (
                    <tr key={i}>
                      <td className="text-zinc-100">{ts.label}</td>
                      <td className="text-right tabular-nums text-sky-200">{fmt.nm(ts.value_nm)}</td>
                      <td className="text-zinc-400">{ts.note}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            ) : (
              <p className="text-zinc-400">{t('fields.torqueSpecs')}: {none}</p>
            )}
            <div>
              <div className="text-xs uppercase tracking-wide text-zinc-400">{t('fields.specialTools')}</div>
              <div className="mt-1 flex flex-wrap gap-1">
                {tools.length === 0 && <span className="text-zinc-400">{none}</span>}
                {tools.map((tool) => (
                  <span key={tool} className="inline-flex items-center gap-1 rounded border border-zinc-700 px-1.5 py-0.5 text-xs text-zinc-200">
                    <Wrench size={11} />
                    {tool}
                  </span>
                ))}
              </div>
            </div>
          </div>
        </Card>
      </div>

      <Card title={t('sections.procedure')}>
        {r.procedure_notes ? <p className="whitespace-pre-wrap text-sm text-zinc-200">{r.procedure_notes}</p> : <p className="text-sm text-zinc-400">{none}</p>}
      </Card>

      <PartsTable repair={r} />

      <Card title={t('sections.attachments')}>
        <div className="space-y-4">
          <AttachmentGallery items={r.attachments} onChanged={refresh} />
          <FileDrop path={`/repairs/${r.id}/attachments`} onUploaded={refresh} />
        </div>
      </Card>

      {editing && <EditRepairDialog repair={r} onClose={() => { setEditing(false) }} />}
      {completing && <CompleteRepairDialog repair={r} onClose={() => { setCompleting(false) }} />}
      <ConfirmDialog
        open={deleting}
        title={t('deleteTitle')}
        body={t('deleteBody', { title: r.title })}
        confirmLabel={t('common:actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { remove.mutate() }}
        onClose={() => { setDeleting(false); remove.reset() }}
      />
    </div>
  )
}

function Section({ label, text }: { label: string; text: string }) {
  const { t } = useTranslation()
  return (
    <div>
      <div className="text-xs uppercase tracking-wide text-zinc-400">{label}</div>
      <p className="mt-1 whitespace-pre-wrap text-zinc-200">{text || <span className="text-zinc-400">{t('state.none')}</span>}</p>
    </div>
  )
}

interface TorqueDraft { key: number; label: string; value: string; note: string }
let torqueKey = 1

function EditRepairDialog({ repair, onClose }: { repair: Repair; onClose: () => void }) {
  const { t } = useTranslation('repairs')
  const { user } = useSession()
  const nf = user.number_format
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [title, setTitle] = useState(repair.title)
  const [startedOn, setStartedOn] = useState(repair.started_on ?? '')
  const [symptom, setSymptom] = useState(repair.symptom)
  const [codes, setCodes] = useState(repair.fault_codes)
  const [rootCause, setRootCause] = useState(repair.root_cause)
  const [procedure, setProcedure] = useState(repair.procedure_notes)
  const [tools, setTools] = useState(repair.special_tools)
  const [completedOn, setCompletedOn] = useState(repair.completed_on ?? '')
  const [completionKm, setCompletionKm] = useState(repair.completion_odometer_km !== null ? String(repair.completion_odometer_km) : '')
  const [torques, setTorques] = useState<TorqueDraft[]>(repair.torque_specs.map((ts) => ({ key: torqueKey++, label: ts.label, value: toInputValue(ts.value_nm, nf, 1), note: ts.note })))
  const [local, setLocal] = useState<Record<string, string>>({})
  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.patch(`/repairs/${repair.id}`, body),
    onSuccess: async () => {
      await Promise.all([invalidateRepairs(qc, repair.id), invalidateOdometerDependents(qc)])
      onClose()
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }
  const submit = () => {
    const errs: Record<string, string> = {}
    const specs: TorqueSpec[] = []
    for (const d of torques) {
      if (!d.label.trim() && !d.value.trim()) continue
      const v = parseNumber(d.value, nf)
      if (!d.label.trim() || v === null || v <= 0) {
        errs.torque_specs = 'repair.torque_spec_invalid'
        continue
      }
      specs.push({ label: d.label, value_nm: v, note: d.note })
    }
    const body: Record<string, unknown> = {
      title, started_on: startedOn || null, symptom, fault_codes: codes, root_cause: rootCause,
      procedure_notes: procedure, special_tools: tools, torque_specs: specs,
    }
    if (repair.status === 'completed') {
      body.completed_on = completedOn
      const km = parseNumber(completionKm, nf)
      if (km === null || !Number.isInteger(km) || km < 0) errs.completion_odometer_km = 'validation.number'
      body.completion_odometer_km = km
    }
    setLocal(errs)
    if (Object.keys(errs).length === 0) save.mutate(body)
  }
  const updateTorque = (key: number, patch: Partial<TorqueDraft>) => {
    setTorques((prev) => prev.map((d) => (d.key === key ? { ...d, ...patch } : d)))
  }
  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={t('common:actions.edit')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={save.isPending}>{t('common:actions.save')}</Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <TextField label={t('fields.title')} value={title} onChange={setTitle} error={fe.title} maxLength={120} />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <TextField label={t('fields.startedOn')} type="date" value={startedOn} onChange={setStartedOn} error={fe.started_on} optional />
          {repair.status === 'completed' && (
            <>
              <TextField label={t('fields.completedOn')} type="date" value={completedOn} onChange={setCompletedOn} error={fe.completed_on} />
              <NumberField label={t('fields.completionOdometer')} unit={t('common:units.km')} decimal={false} value={completionKm} onChange={setCompletionKm} error={fe.completion_odometer_km} />
            </>
          )}
        </div>
        <TextAreaField label={t('fields.symptom')} value={symptom} onChange={setSymptom} error={fe.symptom} rows={3} optional />
        <FaultCodeInput value={codes} onChange={setCodes} />
        <TextAreaField label={t('fields.rootCause')} value={rootCause} onChange={setRootCause} error={fe.root_cause} rows={2} optional />
        <TextAreaField label={t('fields.procedure')} value={procedure} onChange={setProcedure} error={fe.procedure_notes} rows={8} optional />
        <TextAreaField label={t('fields.specialTools')} value={tools} onChange={setTools} hint={t('fields.specialToolsHint')} error={fe.special_tools} rows={3} optional />
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-zinc-300">{t('fields.torqueSpecs')}</legend>
          {fe.torque_specs && <p className="text-xs text-red-400">{t(`errors:${fe.torque_specs}`)}</p>}
          {torques.map((d) => (
            <div key={d.key} className="grid grid-cols-[1fr_7rem_1fr_auto] items-end gap-2">
              <TextField label={t('fields.torqueLabel')} value={d.label} onChange={(v) => { updateTorque(d.key, { label: v }) }} />
              <NumberField label={t('fields.torqueValue')} unit={t('common:units.nm')} value={d.value} onChange={(v) => { updateTorque(d.key, { value: v }) }} />
              <TextField label={t('fields.torqueNote')} value={d.note} onChange={(v) => { updateTorque(d.key, { note: v }) }} placeholder="+ 90°" />
              <IconButton aria-label={t('common:actions.remove')} className="mb-1" onClick={() => { setTorques((prev) => prev.filter((x) => x.key !== d.key)) }}>
                <Trash2 size={15} />
              </IconButton>
            </div>
          ))}
          <Button size="sm" onClick={() => { setTorques((prev) => [...prev, { key: torqueKey++, label: '', value: '', note: '' }]) }}>
            <Plus size={15} />
            {t('fields.addTorque')}
          </Button>
        </fieldset>
      </div>
    </Dialog>
  )
}

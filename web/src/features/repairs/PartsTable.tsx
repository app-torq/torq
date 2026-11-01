import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowDown, ArrowUp, ExternalLink, Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { parseMoneyToCents, parseNumber, toInputValue } from '../../lib/format'
import { useFormat } from '../../lib/useFormat'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { CostType, PartStatus, Repair, RepairPart } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Badge, Card, ErrorBanner, Table } from '../../components/ui'
import { invalidateRepairs } from './hooks'

const partStatuses: PartStatus[] = ['needed', 'ordered', 'in_stock', 'installed']
const partTone = { needed: 'red', ordered: 'amber', in_stock: 'sky', installed: 'emerald' } as const

/** Parts bill of materials with totals and per-status counts (FR-024, FR-025). */
export function PartsTable({ repair }: { repair: Repair }) {
  const { t } = useTranslation('repairs')
  const fmt = useFormat()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [editing, setEditing] = useState<RepairPart | 'new' | null>(null)
  const [deleting, setDeleting] = useState<RepairPart | null>(null)
  const refresh = () => invalidateRepairs(qc, repair.id)
  const setStatus = useMutation({
    mutationFn: ({ id, status }: { id: number; status: PartStatus }) => api.patch(`/repair-parts/${id}`, { status }),
    onSuccess: refresh,
  })
  const reorder = useMutation({
    mutationFn: (ids: number[]) => api.put(`/repairs/${repair.id}/parts/order`, { ids }),
    onSettled: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/repair-parts/${id}`),
    onSuccess: async () => { setDeleting(null); await refresh() },
  })
  const move = (i: number, d: number) => {
    const ids = repair.parts.map((p) => p.id)
    const a = ids[i]
    const b = ids[i + d]
    if (a === undefined || b === undefined) return
    ids[i] = b
    ids[i + d] = a
    reorder.mutate(ids)
  }
  const none = t('common:state.none')

  return (
    <Card
      title={t('sections.parts')}
      actions={
        <Button size="sm" onClick={() => { setEditing('new') }}>
          <Plus size={15} />
          {t('parts.add')}
        </Button>
      }
    >
      {(setStatus.isError || reorder.isError) && <div className="mb-3"><ErrorBanner message={errorText(setStatus.error ?? reorder.error)} /></div>}
      {repair.parts.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t('parts.name')}</th>
              <th>{t('parts.oem')}</th>
              <th>{t('parts.aftermarketBrand')}</th>
              <th>{t('parts.aftermarketNumber')}</th>
              <th className="text-right">{t('parts.quantity')}</th>
              <th className="text-right">{t('parts.unitCost')}</th>
              <th className="text-right">{t('parts.lineTotal')}</th>
              <th>{t('parts.status')}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {repair.parts.map((p, i) => (
              <tr key={p.id}>
                <td className="text-zinc-100">
                  <div className="flex items-center gap-1.5">
                    {p.name}
                    {p.supplier_url && (
                      <a href={p.supplier_url} target="_blank" rel="noopener noreferrer" className="text-sky-400 hover:text-sky-300" aria-label={t('parts.openShop')} title={t('parts.openShop')}>
                        <ExternalLink size={13} />
                      </a>
                    )}
                  </div>
                  {p.cost_type === 'fluids' && <span className="text-xs text-zinc-400">{t('service:costTypes.fluids')}</span>}
                </td>
                <td className="font-mono text-xs">{p.oem_part_number || none}</td>
                <td>{p.aftermarket_brand || none}</td>
                <td className="font-mono text-xs">{p.aftermarket_part_number || none}</td>
                <td className="text-right tabular-nums">{fmt.number(p.quantity)}</td>
                <td className="text-right tabular-nums">{p.unit_cost_cents !== null ? fmt.money(p.unit_cost_cents) : none}</td>
                <td className="text-right tabular-nums text-zinc-100">{fmt.money(p.line_total_cents)}</td>
                <td>
                  <select
                    aria-label={t('parts.status')}
                    value={p.status}
                    onChange={(e) => { setStatus.mutate({ id: p.id, status: e.target.value as PartStatus }) }}
                    className="focus-ring rounded border border-zinc-700 bg-zinc-950 px-1.5 py-1 text-xs"
                  >
                    {partStatuses.map((s) => <option key={s} value={s}>{t(`parts.statuses.${s}`)}</option>)}
                  </select>
                </td>
                <td>
                  <div className="flex justify-end gap-0.5">
                    <IconButton aria-label={t('common:actions.moveUp')} disabled={i === 0} onClick={() => { move(i, -1) }}><ArrowUp size={14} /></IconButton>
                    <IconButton aria-label={t('common:actions.moveDown')} disabled={i === repair.parts.length - 1} onClick={() => { move(i, 1) }}><ArrowDown size={14} /></IconButton>
                    <IconButton aria-label={t('common:actions.edit')} onClick={() => { setEditing(p) }}><Pencil size={14} /></IconButton>
                    <IconButton aria-label={t('common:actions.delete')} onClick={() => { setDeleting(p) }}><Trash2 size={14} /></IconButton>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <td colSpan={6} className="border-t border-zinc-800 px-3 py-2">
                <div className="flex flex-wrap gap-1.5">
                  {partStatuses.map((s) => <Badge key={s} tone={partTone[s]}>{`${t(`parts.statuses.${s}`)}: ${fmt.number(repair.part_status_counts[s])}`}</Badge>)}
                </div>
              </td>
              <td className="border-t border-zinc-800 px-3 py-2 text-right font-semibold tabular-nums text-zinc-100">{fmt.money(repair.parts_total_cents)}</td>
              <td colSpan={2} className="border-t border-zinc-800" />
            </tr>
          </tfoot>
        </Table>
      )}
      {repair.parts.length === 0 && <p className="text-sm text-zinc-400">{t('common:state.empty')}</p>}
      {editing !== null && <PartDialog repairId={repair.id} part={editing === 'new' ? null : editing} onClose={() => { setEditing(null) }} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('parts.deleteTitle')}
        body={t('parts.deleteBody', { name: deleting?.name ?? '' })}
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

function PartDialog({ repairId, part, onClose }: { repairId: number; part: RepairPart | null; onClose: () => void }) {
  const { t } = useTranslation('repairs')
  const { user } = useSession()
  const nf = user.number_format
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [name, setName] = useState(part?.name ?? '')
  const [oem, setOem] = useState(part?.oem_part_number ?? '')
  const [brand, setBrand] = useState(part?.aftermarket_brand ?? '')
  const [number, setNumber] = useState(part?.aftermarket_part_number ?? '')
  const [url, setUrl] = useState(part?.supplier_url ?? '')
  const [qty, setQty] = useState(String(part?.quantity ?? 1))
  const [cost, setCost] = useState(part?.unit_cost_cents != null ? toInputValue(part.unit_cost_cents / 100, nf, 2) : '')
  const [costType, setCostType] = useState<CostType>(part?.cost_type ?? 'parts')
  const [status, setStatus] = useState<PartStatus>(part?.status ?? 'needed')
  const [local, setLocal] = useState<Record<string, string>>({})
  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => (part ? api.patch(`/repair-parts/${part.id}`, body) : api.post(`/repairs/${repairId}/parts`, body)),
    onSuccess: async () => {
      await invalidateRepairs(qc, repairId)
      onClose()
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }
  const submit = () => {
    const errs: Record<string, string> = {}
    const q = parseNumber(qty, nf)
    if (q === null || !Number.isInteger(q) || q < 1) errs.quantity = 'validation.positive'
    let cents: number | null = null
    if (cost.trim() !== '') {
      cents = parseMoneyToCents(cost, nf)
      if (cents === null || cents < 0) errs.unit_cost_cents = 'validation.number'
    }
    setLocal(errs)
    if (Object.keys(errs).length > 0) return
    save.mutate({
      name, oem_part_number: oem, aftermarket_brand: brand, aftermarket_part_number: number, supplier_url: url,
      quantity: q, unit_cost_cents: cents, cost_type: costType, status,
    })
  }
  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={part ? t('parts.editTitle') : t('parts.add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={save.isPending}>{t('common:actions.save')}</Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField label={t('parts.name')} value={name} onChange={setName} error={fe.name} maxLength={120} autoFocus className="sm:col-span-2" />
          <TextField label={t('parts.oem')} value={oem} onChange={setOem} error={fe.oem_part_number} optional />
          <TextField label={t('parts.aftermarketBrand')} value={brand} onChange={setBrand} error={fe.aftermarket_brand} optional />
          <TextField label={t('parts.aftermarketNumber')} value={number} onChange={setNumber} error={fe.aftermarket_part_number} optional />
          <TextField label={t('parts.supplier')} type="url" value={url} onChange={setUrl} error={fe.supplier_url} placeholder="https://" optional />
          <NumberField label={t('parts.quantity')} decimal={false} value={qty} onChange={setQty} error={fe.quantity} />
          <NumberField label={t('parts.unitCost')} value={cost} onChange={setCost} error={fe.unit_cost_cents} optional />
          <SelectField
            label={t('parts.costType')}
            value={costType}
            onChange={(v) => { setCostType(v as CostType) }}
            options={[{ value: 'parts', label: t('service:costTypes.parts') }, { value: 'fluids', label: t('service:costTypes.fluids') }]}
          />
          <SelectField
            label={t('parts.status')}
            value={status}
            onChange={(v) => { setStatus(v as PartStatus) }}
            options={partStatuses.map((s) => ({ value: s, label: t(`parts.statuses.${s}`) }))}
          />
        </div>
      </div>
    </Dialog>
  )
}

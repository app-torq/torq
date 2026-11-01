import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Columns3, List, Plus } from 'lucide-react'
import { api } from '../../lib/api'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName, type RepairStatus, type RepairSummary, type Vehicle } from '../../lib/types'
import { Button } from '../../components/Button'
import { inputClass } from '../../components/Field'
import { Badge, EmptyState, ErrorBanner, Loading, PageHeader, Table } from '../../components/ui'
import { useVehicles } from '../vehicles/hooks'
import { invalidateRepairs, repairStatuses, statusTone, useRepairs } from './hooks'
import { NewRepairDialog } from './NewRepairDialog'
import { CompleteRepairDialog } from './CompleteRepairDialog'

const VIEW_KEY = 'torq.repairs.view'

function readView(): 'board' | 'list' {
  try {
    return localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'board'
  } catch {
    return 'board'
  }
}

export function RepairBoardPage({ vehicleId }: { vehicleId?: number }) {
  const { t } = useTranslation('repairs')
  const errorText = useErrorText()
  const [view, setView] = useState(readView)
  const [vehicleFilter, setVehicleFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [q, setQ] = useState('')
  const [search, setSearch] = useState('')
  const [params, setParams] = useSearchParams()
  const [creating, setCreating] = useState(params.get('new') === '1')
  const vehicles = useVehicles('all')
  const repairs = useRepairs({
    vehicle_id: vehicleId ?? (Number(vehicleFilter) || undefined),
    ...(statusFilter ? { status: [statusFilter] } : {}),
    q: search,
  })
  const vehicleById = new Map(vehicles.data?.items.map((v) => [v.id, v]))

  const changeView = (v: 'board' | 'list') => {
    setView(v)
    try {
      localStorage.setItem(VIEW_KEY, v)
    } catch {
      // Storage unavailable (private mode); the choice just isn't remembered.
    }
  }

  const items = repairs.data?.items ?? []
  return (
    <div>
      {vehicleId === undefined ? (
        <PageHeader
          title={t('title')}
          subtitle={t('subtitle')}
          actions={
            <Button variant="primary" onClick={() => { setCreating(true) }}>
              <Plus size={16} />
              {t('add')}
            </Button>
          }
        />
      ) : (
        <div className="mb-4 flex justify-end">
          <Button variant="primary" onClick={() => { setCreating(true) }}>
            <Plus size={16} />
            {t('add')}
          </Button>
        </div>
      )}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="inline-flex rounded-md border border-zinc-800 bg-zinc-900 p-0.5" role="group">
          {(['board', 'list'] as const).map((v) => (
            <button
              key={v}
              type="button"
              aria-pressed={view === v}
              onClick={() => { changeView(v) }}
              className={`focus-ring inline-flex items-center gap-1.5 rounded px-3 py-1.5 text-sm ${view === v ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
            >
              {v === 'board' ? <Columns3 size={15} /> : <List size={15} />}
              {t(`view.${v}`)}
            </button>
          ))}
        </div>
        {vehicleId === undefined && (
          <select aria-label={t('fields.vehicle')} className={`${inputClass} w-auto`} value={vehicleFilter} onChange={(e) => { setVehicleFilter(e.target.value) }}>
            <option value="">{t('service:filters.allVehicles')}</option>
            {vehicles.data?.items.map((v) => <option key={v.id} value={v.id}>{vehicleName(v)}</option>)}
          </select>
        )}
        {view === 'list' && (
          <select aria-label={t('fields.status')} className={`${inputClass} w-auto`} value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value) }}>
            <option value="">{t('filters.allStatuses')}</option>
            {repairStatuses.map((s) => <option key={s} value={s}>{t(`statuses.${s}`)}</option>)}
          </select>
        )}
        <form className="min-w-[12rem] flex-1" onSubmit={(e) => { e.preventDefault(); setSearch(q.trim()) }}>
          <input type="search" aria-label={t('filters.search')} placeholder={t('filters.search')} className={inputClass} value={q} onChange={(e) => { setQ(e.target.value) }} onBlur={() => { setSearch(q.trim()) }} />
        </form>
      </div>
      {repairs.isPending && <Loading />}
      {repairs.isError && <ErrorBanner message={errorText(repairs.error)} onRetry={() => void repairs.refetch()} />}
      {repairs.isSuccess && items.length === 0 && <EmptyState title={t('empty')} />}
      {items.length > 0 && view === 'board' && <Board items={items} vehicleById={vehicleById} showVehicle={vehicleId === undefined} />}
      {items.length > 0 && view === 'list' && <RepairTable items={items} vehicleById={vehicleById} showVehicle={vehicleId === undefined} />}
      {creating && <NewRepairDialog vehicleId={vehicleId ?? (Number(vehicleFilter) || undefined)} onClose={() => { setCreating(false); if (params.has('new')) setParams({}, { replace: true }) }} />}
    </div>
  )
}

function Board({ items, vehicleById, showVehicle }: { items: RepairSummary[]; vehicleById: Map<number, Vehicle>; showVehicle: boolean }) {
  const { t } = useTranslation('repairs')
  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
      {repairStatuses.map((status) => {
        const column = items.filter((r) => r.status === status)
        return (
          <section key={status} className="rounded-lg border border-zinc-800 bg-zinc-900/50">
            <h2 className="flex items-center justify-between border-b border-zinc-800 px-3 py-2 text-sm font-semibold text-zinc-300">
              {t(`statuses.${status}`)}
              <Badge tone={statusTone[status]}>{column.length}</Badge>
            </h2>
            <ul className="space-y-2 p-2">
              {column.map((r) => (
                <li key={r.id}>
                  <RepairCard repair={r} vehicle={showVehicle ? vehicleById.get(r.vehicle_id) : undefined} />
                </li>
              ))}
            </ul>
          </section>
        )
      })}
    </div>
  )
}

function RepairCard({ repair: r, vehicle }: { repair: RepairSummary; vehicle: Vehicle | undefined }) {
  const { t } = useTranslation('repairs')
  const fmt = useFormat()
  const [completing, setCompleting] = useState(false)
  return (
    <div className="rounded-md border border-zinc-800 bg-zinc-900 p-3">
      <Link to={`/repairs/${r.id}`} className="focus-ring block font-medium text-zinc-100 hover:text-sky-300">{r.title}</Link>
      {vehicle && <div className="text-xs text-zinc-400">{vehicleName(vehicle)}</div>}
      {r.fault_codes.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1">
          {r.fault_codes.map((c) => <span key={c} className="rounded bg-amber-500/15 px-1 font-mono text-[11px] text-amber-200">{c}</span>)}
        </div>
      )}
      <div className="mt-2 flex items-center justify-between gap-2 text-xs text-zinc-400">
        <span>{r.part_status_counts.needed + r.part_status_counts.ordered > 0 ? t('parts.countSummary', r.part_status_counts) : ''}</span>
        <span className="tabular-nums">{r.parts_total_cents > 0 ? fmt.money(r.parts_total_cents) : ''}</span>
      </div>
      <StatusMenu repair={r} onComplete={() => { setCompleting(true) }} />
      {completing && <CompleteRepairDialog repair={r} onClose={() => { setCompleting(false) }} />}
    </div>
  )
}

/** Moves a project between workflow columns; completing opens the completion dialog. */
export function StatusMenu({ repair, onComplete }: { repair: RepairSummary; onComplete: () => void }) {
  const { t } = useTranslation('repairs')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const move = useMutation({
    mutationFn: (status: RepairStatus) => api.post(`/repairs/${repair.id}/status`, { status }),
    onSuccess: () => invalidateRepairs(qc, repair.id),
  })
  return (
    <div className="mt-2">
      <select
        aria-label={t('moveTo')}
        className="focus-ring w-full rounded border border-zinc-700 bg-zinc-950 px-2 py-1 text-xs text-zinc-300"
        value={repair.status}
        disabled={move.isPending}
        onChange={(e) => {
          const next = e.target.value as RepairStatus
          if (next === 'completed') onComplete()
          else move.mutate(next)
        }}
      >
        {repairStatuses.map((s) => <option key={s} value={s}>{t(`statuses.${s}`)}</option>)}
      </select>
      {move.isError && <p className="mt-1 text-xs text-red-400">{errorText(move.error)}</p>}
    </div>
  )
}

function RepairTable({ items, vehicleById, showVehicle }: { items: RepairSummary[]; vehicleById: Map<number, Vehicle>; showVehicle: boolean }) {
  const { t } = useTranslation('repairs')
  const fmt = useFormat()
  return (
    <Table>
      <thead>
        <tr>
          <th>{t('fields.title')}</th>
          {showVehicle && <th>{t('fields.vehicle')}</th>}
          <th>{t('fields.status')}</th>
          <th>{t('fields.faultCodes')}</th>
          <th>{t('fields.completedOn')}</th>
          <th className="text-right">{t('parts.total')}</th>
        </tr>
      </thead>
      <tbody>
        {items.map((r) => {
          const v = vehicleById.get(r.vehicle_id)
          return (
            <tr key={r.id}>
              <td><Link to={`/repairs/${r.id}`} className="text-sky-300 hover:underline">{r.title}</Link></td>
              {showVehicle && <td className="text-zinc-300">{v ? vehicleName(v) : ''}</td>}
              <td><Badge tone={statusTone[r.status]}>{t(`statuses.${r.status}`)}</Badge></td>
              <td className="font-mono text-xs text-amber-200">{r.fault_codes.join(', ')}</td>
              <td className="tabular-nums text-zinc-400">{r.completed_on && r.status === 'completed' ? fmt.date(r.completed_on) : ''}</td>
              <td className="text-right tabular-nums">{fmt.money(r.parts_total_cents)}</td>
            </tr>
          )
        })}
      </tbody>
    </Table>
  )
}

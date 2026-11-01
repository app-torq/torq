import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName, type CostReport } from '../../lib/types'
import { inputClass } from '../../components/Field'
import { Card, ErrorBanner, Loading, PageHeader, Table } from '../../components/ui'
import { useVehicles } from '../vehicles/hooks'

type Preset = 'thisYear' | 'lastYear' | 'last12' | 'all' | 'custom'

const iso = (d: Date) => d.toISOString().slice(0, 10)

function presetRange(p: Preset): { from: string; to: string } {
  const now = new Date()
  const y = now.getFullYear()
  switch (p) {
    case 'thisYear':
      return { from: `${y}-01-01`, to: `${y}-12-31` }
    case 'lastYear':
      return { from: `${y - 1}-01-01`, to: `${y - 1}-12-31` }
    case 'last12': {
      const from = new Date(now)
      from.setFullYear(y - 1)
      from.setDate(from.getDate() + 1)
      return { from: iso(from), to: iso(now) }
    }
    default:
      return { from: '', to: '' }
  }
}

export function ReportsPage() {
  const { t } = useTranslation('reports')
  const fmt = useFormat()
  const errorText = useErrorText()
  const vehicles = useVehicles('all')
  const [vehicle, setVehicle] = useState('')
  const [preset, setPreset] = useState<Preset>('thisYear')
  const [custom, setCustom] = useState(presetRange('thisYear'))
  const range = preset === 'custom' ? custom : presetRange(preset)
  const params = { vehicle_id: Number(vehicle) || undefined, from: range.from || undefined, to: range.to || undefined }
  const report = useQuery({ queryKey: queryKeys.costReport(params), queryFn: () => api.get<CostReport>('/reports/costs', params) })
  const vehicleById = new Map(vehicles.data?.items.map((v) => [v.id, v]))
  const r = report.data

  return (
    <div className="space-y-6">
      <PageHeader title={t('title')} subtitle={t('subtitle')} />
      <div className="flex flex-wrap items-end gap-2">
        <select aria-label={t('filters.vehicle')} className={`${inputClass} w-auto`} value={vehicle} onChange={(e) => { setVehicle(e.target.value) }}>
          <option value="">{t('filters.allVehicles')}</option>
          {vehicles.data?.items.map((v) => <option key={v.id} value={v.id}>{vehicleName(v)}</option>)}
        </select>
        <select aria-label={t('filters.range')} className={`${inputClass} w-auto`} value={preset} onChange={(e) => { setPreset(e.target.value as Preset) }}>
          {(['thisYear', 'lastYear', 'last12', 'all', 'custom'] as const).map((p) => <option key={p} value={p}>{t(`filters.presets.${p}`)}</option>)}
        </select>
        {preset === 'custom' && (
          <>
            <input type="date" aria-label={t('filters.from')} className={`${inputClass} w-auto`} value={custom.from} onChange={(e) => { setCustom({ ...custom, from: e.target.value }) }} />
            <input type="date" aria-label={t('filters.to')} className={`${inputClass} w-auto`} value={custom.to} onChange={(e) => { setCustom({ ...custom, to: e.target.value }) }} />
          </>
        )}
      </div>
      {report.isPending && <Loading />}
      {report.isError && <ErrorBanner message={errorText(report.error)} onRetry={() => void report.refetch()} />}
      {r && (
        <>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <Tile label={t('tiles.total')} value={fmt.money(r.total_cents)} />
            <Tile label={t('tiles.costPerKm')} value={r.cost_per_km_cents === null ? t('tiles.notEnoughData') : t('tiles.perKm', { value: fmt.moneyPrecise(r.cost_per_km_cents, r.cost_per_km_cents < 10 ? 3 : 2) })} muted={r.cost_per_km_cents === null} />
            <Tile label={t('tiles.distance')} value={r.distance_km === null ? t('tiles.notEnoughData') : fmt.km(r.distance_km)} muted={r.distance_km === null} />
          </div>
          {r.total_cents === 0 ? (
            <p className="text-sm text-zinc-400">{t('empty')}</p>
          ) : (
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
              <Card title={t('breakdown.bySource')}>
                <Meter
                  total={r.total_cents}
                  rows={[
                    { label: t('breakdown.service'), value: r.by_source.routine_service_cents, color: 'bg-sky-400' },
                    { label: t('breakdown.repairs'), value: r.by_source.self_repair_cents, color: 'bg-emerald-400' },
                  ]}
                />
              </Card>
              <Card title={t('breakdown.byType')}>
                <Meter
                  total={r.total_cents}
                  rows={[
                    { label: t('breakdown.parts'), value: r.by_cost_type.parts_cents, color: 'bg-sky-400' },
                    { label: t('breakdown.fluids'), value: r.by_cost_type.fluids_cents, color: 'bg-amber-400' },
                    { label: t('breakdown.other'), value: r.by_cost_type.other_cents, color: 'bg-zinc-400' },
                  ]}
                />
              </Card>
            </div>
          )}
          {r.by_vehicle && r.by_vehicle.length > 0 && (
            <Card title={t('byVehicle')}>
              <Table>
                <thead>
                  <tr>
                    <th>{t('filters.vehicle')}</th>
                    <th className="text-right">{t('breakdown.service')}</th>
                    <th className="text-right">{t('breakdown.repairs')}</th>
                    <th className="text-right">{t('tiles.total')}</th>
                    <th className="text-right">{t('tiles.costPerKm')}</th>
                  </tr>
                </thead>
                <tbody>
                  {r.by_vehicle.map((v) => {
                    const veh = vehicleById.get(v.vehicle_id)
                    return (
                      <tr key={v.vehicle_id}>
                        <td className="text-zinc-100">{veh ? vehicleName(veh) : ''}</td>
                        <td className="text-right tabular-nums">{fmt.money(v.by_source.routine_service_cents)}</td>
                        <td className="text-right tabular-nums">{fmt.money(v.by_source.self_repair_cents)}</td>
                        <td className="text-right tabular-nums text-zinc-100">{fmt.money(v.total_cents)}</td>
                        <td className="text-right tabular-nums text-zinc-400">{v.cost_per_km_cents === null ? t('tiles.notEnoughData') : t('tiles.perKm', { value: fmt.moneyPrecise(v.cost_per_km_cents, 3) })}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </Table>
            </Card>
          )}
        </>
      )}
    </div>
  )
}

function Tile({ label, value, muted }: { label: string; value: string; muted?: boolean }) {
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
      <div className="text-xs uppercase tracking-wide text-zinc-400">{label}</div>
      <div className={`mt-1 text-2xl font-semibold tabular-nums ${muted ? 'text-zinc-400' : 'text-zinc-100'}`}>{value}</div>
    </div>
  )
}

/** CSS bar meter; no chart library (research R14). */
function Meter({ total, rows }: { total: number; rows: { label: string; value: number; color: string }[] }) {
  const fmt = useFormat()
  return (
    <div className="space-y-3">
      <div className="flex h-3 overflow-hidden rounded bg-zinc-800">
        {rows.map((r) => (r.value > 0 ? <div key={r.label} className={r.color} style={{ width: `${(r.value / total) * 100}%` }} /> : null))}
      </div>
      <ul className="space-y-1.5 text-sm">
        {rows.map((r) => (
          <li key={r.label} className="flex items-center gap-2">
            <span className={`h-2.5 w-2.5 rounded-sm ${r.color}`} />
            <span className="flex-1 text-zinc-300">{r.label}</span>
            <span className="tabular-nums text-zinc-100">{fmt.money(r.value)}</span>
            <span className="w-14 text-right tabular-nums text-zinc-400">{fmt.number(total ? (r.value / total) * 100 : 0)} %</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

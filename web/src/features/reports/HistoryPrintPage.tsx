import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Download, Printer } from 'lucide-react'
import { API_BASE, api, withQuery } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import type { VehicleHistory } from '../../lib/types'
import { Button } from '../../components/Button'
import { Checkbox } from '../../components/Field'
import { ErrorBanner, Loading } from '../../components/ui'
import { BRAND } from '../../lib/brand'

/**
 * Printable Vehicle History Report (FR-047). Rendered outside the app layout.
 * On screen it is dark like every page; print: variants switch to black on
 * white for paper only (research R15).
 */
export function HistoryPrintPage() {
  const { id } = useParams()
  const vehicleId = Number(id)
  const { t } = useTranslation(['reports', 'vehicles', 'specs', 'service', 'repairs'])
  const fmt = useFormat()
  const errorText = useErrorText()
  const [costs, setCosts] = useState(true)
  const [notes, setNotes] = useState(true)
  const params = { include_costs: costs, include_notes: notes }
  const history = useQuery({
    queryKey: queryKeys.history(vehicleId, params),
    queryFn: () => api.get<VehicleHistory>(`/vehicles/${vehicleId}/history`, { ...params, format: 'json', download: '0' }),
  })
  const exportUrl = (format: 'json' | 'csv') => API_BASE + withQuery(`/vehicles/${vehicleId}/history`, { ...params, format })
  const h = history.data
  const none = t('common:state.none')

  return (
    <div className="mx-auto max-w-4xl px-4 py-6 print:max-w-none print:bg-white print:p-0 print:text-black">
      <div className="mb-6 flex flex-wrap items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900 p-3 print:hidden">
        <Link to={`/vehicles/${vehicleId}/history`} className="focus-ring inline-flex items-center gap-1 text-sm text-sky-300 hover:underline">
          <ArrowLeft size={15} />
          {t('history.back')}
        </Link>
        <span className="flex-1" />
        <Checkbox label={t('history.includeCosts')} checked={costs} onChange={setCosts} />
        <Checkbox label={t('history.includeNotes')} checked={notes} onChange={setNotes} />
        <Button variant="primary" onClick={() => { window.print() }}>
          <Printer size={15} />
          {t('history.print')}
        </Button>
        <a href={exportUrl('json')} className="focus-ring inline-flex h-10 items-center gap-2 rounded-md border border-zinc-700 px-3 text-sm hover:bg-zinc-800">
          <Download size={15} />
          {t('history.downloadJson')}
        </a>
        <a href={exportUrl('csv')} className="focus-ring inline-flex h-10 items-center gap-2 rounded-md border border-zinc-700 px-3 text-sm hover:bg-zinc-800">
          <Download size={15} />
          {t('history.downloadCsv')}
        </a>
      </div>
      {history.isPending && <Loading />}
      {history.isError && <ErrorBanner message={errorText(history.error)} />}
      {h && (
        <article className="space-y-8 text-sm print:text-[11pt]">
          <header className="border-b border-zinc-800 pb-4 print:border-black">
            <div className="text-xs uppercase tracking-wide text-zinc-400 print:text-black">{BRAND} · {t('history.title')}</div>
            <h1 className="mt-1 text-2xl font-semibold print:text-black">{h.vehicle.make} {h.vehicle.model}</h1>
            <p className="text-zinc-400 print:text-black">{t('history.generated', { date: fmt.date(new Date().toISOString().slice(0, 10)) })}</p>
          </header>

          <section>
            <h2 className="mb-2 text-base font-semibold">{t('history.profile')}</h2>
            <dl className="grid grid-cols-2 gap-x-6 gap-y-1 sm:grid-cols-3">
              {[
                [t('vehicles:fields.chassisCode'), h.vehicle.chassis_code || none],
                [t('vehicles:fields.engineCode'), h.vehicle.engine_code || none],
                [t('vehicles:fields.displacement'), h.vehicle.displacement_cc != null ? fmt.displacement(h.vehicle.displacement_cc) : none],
                [t('vehicles:fields.power'), h.vehicle.power_kw != null ? fmt.power(h.vehicle.power_kw) : none],
                [t('vehicles:fields.fuelType'), h.vehicle.fuel_type ? t(`vehicles:fuel.${h.vehicle.fuel_type}`) : none],
                [t('vehicles:fields.hsnTsn'), h.vehicle.hsn || h.vehicle.tsn ? `${h.vehicle.hsn ?? '—'} / ${h.vehicle.tsn ?? '—'}` : none],
                [t('vehicles:fields.modelYear'), h.vehicle.model_year ?? none],
                [t('vehicles:fields.vin'), h.vehicle.vin ?? none],
                [t('vehicles:fields.licensePlate'), h.vehicle.license_plate ?? none],
                [t('vehicles:fields.odometer'), fmt.km(h.vehicle.current_odometer_km)],
              ].map(([label, value]) => (
                <div key={String(label)}>
                  <dt className="text-xs text-zinc-400 print:text-black">{label}</dt>
                  <dd>{value}</dd>
                </div>
              ))}
            </dl>
          </section>

          <section>
            <h2 className="mb-2 text-base font-semibold">{t('history.specs')}</h2>
            <ul className="grid grid-cols-1 gap-1 sm:grid-cols-2">
              {h.specs.engine_oil_viscosity || h.specs.engine_oil_spec ? (
                <li>{t('specs:groups.engineOil')}: {[h.specs.engine_oil_viscosity, h.specs.engine_oil_spec, typeof h.specs.engine_oil_capacity_ml === 'number' ? fmt.liters(h.specs.engine_oil_capacity_ml) : null].filter(Boolean).join(' · ')}</li>
              ) : null}
              {typeof h.specs.wheel_lug_torque_nm === 'number' && <li>{t('specs:fields.lugTorque')}: {fmt.nm(h.specs.wheel_lug_torque_nm)}</li>}
              {h.spec_entries.map((e) => <li key={e.label}>{e.label}: {e.value}</li>)}
            </ul>
          </section>

          <section>
            <h2 className="mb-2 text-base font-semibold">{t('history.services')}</h2>
            {h.service_logs.length === 0 && <p className="text-zinc-400">{t('history.noServices')}</p>}
            <div className="space-y-3">
              {h.service_logs.map((l) => (
                <div key={l.id} className="break-inside-avoid border-b border-zinc-800 pb-2 print:border-gray-400">
                  <div className="flex justify-between font-medium">
                    <span>{fmt.date(l.service_date)} · {fmt.km(l.odometer_km)}</span>
                    {l.total_cents !== undefined && <span className="tabular-nums">{fmt.money(l.total_cents)}</span>}
                  </div>
                  <ul className="ml-4 list-disc">
                    {l.items.map((it, i) => (
                      <li key={i}>
                        {[it.category, it.brand, it.specification, it.quantity_ml !== null && it.quantity_unit ? fmt.quantity(it.quantity_ml, it.quantity_unit) : null, it.oem_part_number, it.aftermarket_part_number].filter(Boolean).join(' · ')}
                        {it.cost_cents !== undefined && it.cost_cents !== null && <span className="tabular-nums"> — {fmt.money(it.cost_cents)}</span>}
                      </li>
                    ))}
                  </ul>
                  {l.notes && <p className="mt-1 whitespace-pre-wrap text-zinc-400 print:text-black">{l.notes}</p>}
                </div>
              ))}
            </div>
          </section>

          <section>
            <h2 className="mb-2 text-base font-semibold">{t('history.repairs')}</h2>
            {h.repairs.length === 0 && <p className="text-zinc-400">{t('history.noRepairs')}</p>}
            <div className="space-y-4">
              {h.repairs.map((r) => (
                <div key={r.id} className="break-inside-avoid border-b border-zinc-800 pb-3 print:border-gray-400">
                  <div className="flex justify-between font-medium">
                    <span>{fmt.date(r.completed_on)} · {fmt.km(r.completion_odometer_km)} · {r.title}</span>
                    {r.parts_total_cents !== undefined && <span className="tabular-nums">{fmt.money(r.parts_total_cents)}</span>}
                  </div>
                  {r.fault_codes.length > 0 && <p className="font-mono text-xs">{r.fault_codes.join(', ')}</p>}
                  {r.symptom && <p><span className="text-zinc-400 print:text-black">{t('repairs:fields.symptom')}:</span> {r.symptom}</p>}
                  {r.root_cause && <p><span className="text-zinc-400 print:text-black">{t('repairs:fields.rootCause')}:</span> {r.root_cause}</p>}
                  {r.torque_specs.length > 0 && <p>{r.torque_specs.map((ts) => `${ts.label} ${fmt.nm(ts.value_nm)}${ts.note ? ` (${ts.note})` : ''}`).join('; ')}</p>}
                  {r.parts.length > 0 && (
                    <ul className="ml-4 list-disc">
                      {r.parts.map((p, i) => (
                        <li key={i}>
                          {`${fmt.number(p.quantity)} × ${p.name}`} {[p.oem_part_number, p.aftermarket_brand, p.aftermarket_part_number].filter(Boolean).join(' · ')}
                          {p.unit_cost_cents !== undefined && p.unit_cost_cents !== null && <span className="tabular-nums"> — {fmt.money(p.unit_cost_cents * p.quantity)}</span>}
                        </li>
                      ))}
                    </ul>
                  )}
                  {r.procedure_notes && <p className="mt-1 whitespace-pre-wrap text-zinc-400 print:text-black">{r.procedure_notes}</p>}
                </div>
              ))}
            </div>
          </section>
        </article>
      )}
    </div>
  )
}

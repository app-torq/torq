import { useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ExternalLink, LogIn } from 'lucide-react'
import i18n from '../../i18n'
import { api, isApiError } from '../../lib/api'
import { createFormatter, powerW, type UnitKey } from '../../lib/format'
import { queryKeys } from '../../lib/queryKeys'
import type { PublicVehicleRecord } from '../../lib/types'
import { Badge, Loading } from '../../components/ui'
import { TorqLogo } from '../../components/Logo'
import { useErrorText } from '../../lib/errors'
import { dueText, remainingText, stateTone } from '../reminders/dueText'
import { entryReading } from '../vehicles/readings'
import { specLines } from '../vehicles/specLines'

/** The share token from a /s/<token> path. */
export function shareToken(pathname: string): string {
  return decodeURIComponent(pathname.slice('/s/'.length).split('/')[0] ?? '')
}

/**
 * The public, read-only record of a shared vehicle (#38). Rendered outside
 * the sign-in gate: visitors have no account, so the language follows their
 * browser and formats the instance defaults. Nothing here can change data.
 */
export function SharePage({ token }: { token: string }) {
  const lang = navigator.languages[0]?.toLowerCase().startsWith('de') ? 'de' : 'en'
  useEffect(() => {
    document.documentElement.lang = lang
    if (i18n.language !== lang) void i18n.changeLanguage(lang)
  }, [lang])
  const { t } = useTranslation(['share', 'common'])
  const errorText = useErrorText()
  const record = useQuery({
    queryKey: queryKeys.sharedRecord(token),
    queryFn: () => api.get<PublicVehicleRecord>(`/shared/${encodeURIComponent(token)}`),
    retry: false,
  })
  const notFound = isApiError(record.error) && record.error.status === 404

  return (
    <div lang={lang} className="mx-auto max-w-3xl px-4 py-6 print:max-w-none print:bg-white print:p-0 print:text-black">
      {record.isPending && <Loading />}
      {record.isError && (
        <div className="flex min-h-[60dvh] flex-col items-center justify-center gap-4 text-center">
          <TorqLogo className="h-8" />
          <h1 className="text-lg font-semibold text-zinc-100">{notFound ? t('notFound.title') : t('error')}</h1>
          <p className="max-w-sm text-sm text-zinc-400">{notFound ? t('notFound.body') : errorText(record.error)}</p>
        </div>
      )}
      {record.data && <Record record={record.data} />}
    </div>
  )
}

function Record({ record }: { record: PublicVehicleRecord }) {
  const { t } = useTranslation(['share', 'common', 'vehicles', 'specs'])
  const f = record.formats
  const fmt = useMemo(
    () =>
      createFormatter(
        {
          dateFormat: f.date_format, numberFormat: f.number_format, currencyCode: f.currency_code, currencySymbol: null,
          timeFormat: f.time_format, powerUnit: f.power_unit, distanceUnit: f.distance_unit, volumeUnit: f.volume_unit,
          torqueUnit: f.torque_unit, pressureUnit: f.pressure_unit, displacementUnit: f.displacement_unit,
        },
        (u: UnitKey) => t(`common:units.${u}`),
      ),
    [f, t],
  )
  const v = record.vehicle
  const unit = v.odometer_unit
  const motorcycle = v.vehicle_type === 'motorcycle'
  const none = t('common:state.none')
  const reading = [
    v.has_odometer ? fmt.distance(v.current_odometer_km, unit) : null,
    ...v.hour_meters.map((m) => `${m.name}: ${fmt.hours(m.current_hours)}`),
  ].filter(Boolean).join(' · ')
  const lines = specLines(record.specs, motorcycle, fmt, t)
  const profile = [
    [t('vehicles:fields.vehicleType'), t(`vehicles:vehicleType.${motorcycle ? 'motorcycle' : 'car'}`)],
    [t('vehicles:fields.modelYear'), v.model_year ?? none],
    [t('vehicles:fields.chassisCode'), v.chassis_code || none],
    [t('vehicles:fields.engineCode'), v.engine_code || none],
    [t('vehicles:fields.displacement'), v.displacement_cc !== null ? fmt.displacement(v.displacement_cc, v.displacement_unit) : none],
    [t('vehicles:fields.power'), v.power_kw !== null ? fmt.power(powerW(v) ?? 0, v.power_unit) : none],
    [t('vehicles:fields.fuelType'), v.fuel_type ? t(`vehicles:fuel.${v.fuel_type}`) : none],
    ...(v.hsn || v.tsn ? [[t('vehicles:fields.hsnTsn'), `${v.hsn ?? '—'} / ${v.tsn ?? '—'}`]] : []),
    ...(v.license_plate ? [[t('vehicles:fields.licensePlate'), v.license_plate]] : []),
    ...(v.vin ? [[t('vehicles:fields.vin'), v.vin]] : []),
  ]

  return (
    <article className="space-y-8 text-sm print:text-[11pt]">
      <header className="space-y-3 border-b border-zinc-800 pb-4 print:border-black">
        <div className="flex flex-wrap items-center justify-between gap-3">
          {record.branding.name || record.branding.logo_url ? (
            <div className="flex items-center gap-2 text-sm font-semibold text-zinc-200 print:text-black">
              {record.branding.logo_url && <img src={record.branding.logo_url} alt="" className="max-h-8 max-w-32 object-contain" />}
              {record.branding.name}
            </div>
          ) : <TorqLogo className="h-7" />}
          <div className="flex flex-wrap gap-2 print:hidden">
            {record.vehicle_id !== undefined ? (
              <a href={`/vehicles/${String(record.vehicle_id)}`} className="focus-ring inline-flex h-8 items-center gap-2 rounded-md border border-zinc-700 px-2.5 text-sm hover:bg-zinc-800">
                <ExternalLink size={15} />
                {t('openInTorq')}
              </a>
            ) : (
              <a href="/" className="focus-ring inline-flex h-8 items-center gap-2 rounded-md border border-zinc-700 px-2.5 text-sm hover:bg-zinc-800">
                <LogIn size={15} />
                {t('signIn')}
              </a>
            )}
          </div>
        </div>
        <div>
          <div className="text-xs uppercase tracking-wide text-zinc-400 print:text-black">{t('title')}</div>
          <h1 className="mt-1 text-2xl font-semibold text-zinc-100 print:text-black">{v.make} {v.model}</h1>
          {reading && <p className="tabular-nums text-zinc-300 print:text-black">{reading}</p>}
          <p className="mt-1 text-xs text-zinc-500 print:text-black">{t('readOnly')}</p>
        </div>
      </header>

      <section aria-labelledby="share-profile">
        <h2 id="share-profile" className="mb-2 text-base font-semibold text-zinc-100 print:text-black">{t('profile')}</h2>
        <dl className="grid grid-cols-2 gap-x-6 gap-y-1 sm:grid-cols-3">
          {profile.map(([label, value]) => (
            <div key={String(label)}>
              <dt className="text-xs text-zinc-400 print:text-black">{label}</dt>
              <dd className="break-words text-zinc-100 print:text-black">{value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section aria-labelledby="share-reminders">
        <h2 id="share-reminders" className="mb-2 text-base font-semibold text-zinc-100 print:text-black">{t('reminders')}</h2>
        {record.reminders.length === 0 && <p className="text-zinc-400">{t('noReminders')}</p>}
        <ul className="space-y-2">
          {record.reminders.map((r, i) => (
            <li key={i} className="break-inside-avoid rounded-md border border-zinc-800 p-3 print:border-gray-400">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium text-zinc-100 print:text-black">{r.name}</span>
                <Badge tone={stateTone[r.state]}>{t(`reminders:states.${r.state}`)}</Badge>
              </div>
              <p className="tabular-nums text-zinc-400 print:text-black">
                {[dueText(r, t, fmt, unit), remainingText(r, t, fmt, unit)].filter(Boolean).join(' · ')}
              </p>
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="share-services">
        <h2 id="share-services" className="mb-2 text-base font-semibold text-zinc-100 print:text-black">{t('services')}</h2>
        {record.service_logs.length === 0 && <p className="text-zinc-400">{t('noServices')}</p>}
        <div className="space-y-3">
          {record.service_logs.map((l, i) => (
            <div key={i} className="break-inside-avoid border-b border-zinc-800 pb-2 print:border-gray-400">
              <div className="flex justify-between gap-3 font-medium text-zinc-100 print:text-black">
                <span>{[fmt.date(l.service_date), entryReading(fmt, unit, l.odometer_km, l.odometer_recorded, l.meter_readings)].filter(Boolean).join(' · ')}</span>
                {l.total_cents !== null && <span className="tabular-nums">{fmt.money(l.total_cents)}</span>}
              </div>
              <ul className="ml-4 list-disc text-zinc-300 print:text-black">
                {l.items.map((it, j) => (
                  <li key={j}>
                    {[it.category, it.brand, it.specification, it.quantity_ml !== null && it.quantity_unit ? fmt.quantity(it.quantity_ml, it.quantity_unit) : null,
                      it.oem_part_number || it.aftermarket_part_number ? [it.oem_part_number, it.aftermarket_part_number].filter(Boolean).join(' / ') : null,
                      it.cost_cents !== null ? fmt.money(it.cost_cents) : null].filter(Boolean).join(' · ')}
                  </li>
                ))}
                {l.extra_cost_cents !== null && l.extra_cost_cents > 0 && (
                  <li>{[l.extra_cost_label, fmt.money(l.extra_cost_cents)].filter(Boolean).join(' · ')}</li>
                )}
              </ul>
            </div>
          ))}
        </div>
      </section>

      <section aria-labelledby="share-specs">
        <h2 id="share-specs" className="mb-2 text-base font-semibold text-zinc-100 print:text-black">{t('specs')}</h2>
        {lines.length === 0 && record.spec_entries.length === 0 && <p className="text-zinc-400">{t('noSpecs')}</p>}
        <ul className="grid grid-cols-1 gap-1 text-zinc-300 sm:grid-cols-2 print:text-black">
          {lines.map((line) => <li key={line}>{line}</li>)}
          {record.spec_entries.map((e, i) => <li key={`e${String(i)}`} className="whitespace-pre-wrap">{e.label}: {e.value}</li>)}
        </ul>
      </section>
    </article>
  )
}

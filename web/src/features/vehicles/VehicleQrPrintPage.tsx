import { Link, useParams } from 'react-router'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Printer } from 'lucide-react'
import { useErrorText } from '../../lib/errors'
import { vehicleName } from '../../lib/types'
import { Button } from '../../components/Button'
import { QrCode } from '../../components/QrCode'
import { ErrorBanner, Loading } from '../../components/ui'
import { useBranding } from '../../app/branding'
import { useVehicle } from './hooks'
import { vehicleQrUrl } from './VehicleQrCard'

/**
 * A printable label with a vehicle's QR code (#38), about 7 cm wide, to stick
 * on the vehicle. Rendered outside the app layout like the printed history;
 * the browser's print dialog also saves it as PDF.
 */
export function VehicleQrPrintPage() {
  const { id } = useParams()
  const { t } = useTranslation('vehicles')
  const errorText = useErrorText()
  const vehicle = useVehicle(Number(id))
  const branding = useBranding()
  const v = vehicle.data

  return (
    <div className="mx-auto max-w-3xl px-4 py-6 print:max-w-none print:p-0">
      <div className="mb-6 flex flex-wrap items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900 p-3 print:hidden">
        <Link to={`/vehicles/${String(id)}`} className="focus-ring inline-flex items-center gap-1 text-sm text-sky-300 hover:underline">
          <ArrowLeft size={15} />
          {t('qr.back')}
        </Link>
        <span className="flex-1" />
        <Button variant="primary" onClick={() => { window.print() }}>
          <Printer size={15} />
          {t('qr.printNow')}
        </Button>
      </div>
      {vehicle.isPending && <Loading />}
      {vehicle.isError && <ErrorBanner message={errorText(vehicle.error)} />}
      {v && (
        <article className="mx-auto w-[7cm] space-y-2 rounded-lg border border-zinc-300 bg-white p-4 text-center text-black print:mx-0 print:rounded-none">
          {(branding?.logo_url || branding?.name) && (
            <div className="flex items-center justify-center gap-2 text-xs font-semibold">
              {branding.logo_url && <img src={branding.logo_url} alt="" className="max-h-6 max-w-24 object-contain" />}
              {branding.name && <span>{branding.name}</span>}
            </div>
          )}
          <h1 className="text-base font-semibold leading-tight">{vehicleName(v)}</h1>
          {v.license_plate && <p className="font-mono text-sm">{v.license_plate}</p>}
          <QrCode value={vehicleQrUrl(v)} label={t('qr.label', { name: vehicleName(v) })} className="mx-auto w-full" />
          <p className="text-[10px] leading-tight">{t('qr.scanHint')}</p>
        </article>
      )}
    </div>
  )
}

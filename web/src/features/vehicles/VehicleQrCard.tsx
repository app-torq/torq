import { useState } from 'react'
import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Download, Printer } from 'lucide-react'
import { qrPng, isLocalAddress } from '../../lib/qr'
import { vehicleName, type Vehicle } from '../../lib/types'
import { Button } from '../../components/Button'
import { QrCode } from '../../components/QrCode'
import { Card, ErrorBanner, Notice } from '../../components/ui'

/**
 * The address a vehicle's QR code opens: its page, at the address this
 * browser uses for torq. Scanning it without being signed in shows the
 * sign-in page, which then opens the vehicle.
 */
export function vehicleQrUrl(v: Pick<Vehicle, 'id'>): string {
  return `${window.location.origin}/vehicles/${String(v.id)}`
}

/** File name for a downloaded code: "torq-mercedes-benz-c-220-cdi-qr.png". */
function fileName(v: Vehicle): string {
  const slug = vehicleName(v).toLowerCase().normalize('NFKD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
  return `torq-${slug || String(v.id)}-qr.png`
}

/** A vehicle's QR code to print and stick on it (#38). */
export function VehicleQrCard({ vehicle }: { vehicle: Vehicle }) {
  const { t } = useTranslation('vehicles')
  const url = vehicleQrUrl(vehicle)
  const [error, setError] = useState(false)

  const download = async () => {
    setError(false)
    try {
      const href = URL.createObjectURL(await qrPng(url))
      const a = document.createElement('a')
      a.href = href
      a.download = fileName(vehicle)
      a.click()
      setTimeout(() => { URL.revokeObjectURL(href) }, 1000)
    } catch {
      setError(true)
    }
  }

  return (
    <Card title={t('qr.title')}>
      <div className="flex flex-wrap items-start gap-4">
        <QrCode value={url} label={t('qr.label', { name: vehicleName(vehicle) })} className="w-36 rounded-md p-1" />
        <div className="min-w-0 flex-1 space-y-3">
          <p className="text-sm text-zinc-400">{t('qr.intro')}</p>
          <p className="break-all font-mono text-xs text-zinc-400">{url}</p>
          {isLocalAddress(url) && <Notice tone="amber">{t('qr.localAddress')}</Notice>}
          {error && <ErrorBanner message={t('qr.downloadFailed')} />}
          <div className="flex flex-wrap gap-2">
            <Button size="sm" onClick={() => { void download() }}>
              <Download size={15} />
              {t('qr.download')}
            </Button>
            <Link
              to={`/vehicles/${String(vehicle.id)}/qr/print`}
              className="focus-ring inline-flex h-8 items-center gap-2 rounded-md border border-zinc-700 px-2.5 text-sm hover:bg-zinc-800"
            >
              <Printer size={15} />
              {t('qr.print')}
            </Link>
          </div>
        </div>
      </div>
    </Card>
  )
}

import { useState } from 'react'
import { Link } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Copy, Download, Printer, RefreshCw } from 'lucide-react'
import { api } from '../../lib/api'
import { useErrorText } from '../../lib/errors'
import { useFormat } from '../../lib/useFormat'
import { queryKeys } from '../../lib/queryKeys'
import { qrPng, isLocalAddress } from '../../lib/qr'
import { vehicleName, type Share, type Vehicle } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { Checkbox } from '../../components/Field'
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

/** The public address of a share link (#38), at the address this browser uses for torq. */
export function shareUrl(s: Pick<Share, 'path'>): string {
  return window.location.origin + s.path
}

/** File name for a downloaded code: "torq-mercedes-benz-c-220-cdi-qr.png", "…-public-qr.png" for the public link. */
function fileName(v: Vehicle, publicCode: boolean): string {
  const slug = vehicleName(v).toLowerCase().normalize('NFKD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
  return `torq-${slug || String(v.id)}${publicCode ? '-public' : ''}-qr.png`
}

/** A vehicle's QR code to print and stick on it, and its public link (#38). */
export function VehicleQrCard({ vehicle }: { vehicle: Vehicle }) {
  const { t } = useTranslation('vehicles')
  const { instance } = useSession()
  const share = useQuery({
    queryKey: queryKeys.share(vehicle.id),
    queryFn: () => api.get<{ share: Share | null }>(`/vehicles/${String(vehicle.id)}/share`),
    enabled: instance.public_links,
  })
  const link = instance.public_links ? share.data?.share ?? null : null
  const [code, setCode] = useState<'app' | 'public'>('app')
  const publicCode = code === 'public' && link !== null
  const url = publicCode ? shareUrl(link) : vehicleQrUrl(vehicle)
  const [error, setError] = useState(false)

  const download = async () => {
    setError(false)
    try {
      const href = URL.createObjectURL(await qrPng(url))
      const a = document.createElement('a')
      a.href = href
      a.download = fileName(vehicle, publicCode)
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
          {link && (
            <div className="inline-flex rounded-md border border-zinc-800 bg-zinc-900 p-0.5" role="group" aria-label={t('qr.codeFor')}>
              {(['app', 'public'] as const).map((c) => (
                <button
                  key={c}
                  type="button"
                  aria-pressed={code === c}
                  onClick={() => { setCode(c) }}
                  className={`focus-ring rounded px-3 py-1 text-sm ${code === c ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:text-zinc-200'}`}
                >
                  {t(c === 'app' ? 'qr.codeApp' : 'qr.codePublic')}
                </button>
              ))}
            </div>
          )}
          <p className="text-sm text-zinc-400">{t(publicCode ? 'qr.introPublic' : 'qr.intro')}</p>
          <p className="break-all font-mono text-xs text-zinc-400">{url}</p>
          {isLocalAddress(url) && <Notice tone="amber">{t('qr.localAddress')}</Notice>}
          {error && <ErrorBanner message={t('qr.downloadFailed')} />}
          <div className="flex flex-wrap gap-2">
            <Button size="sm" onClick={() => { void download() }}>
              <Download size={15} />
              {t('qr.download')}
            </Button>
            <Link
              to={`/vehicles/${String(vehicle.id)}/qr/print${publicCode ? '?code=public' : ''}`}
              className="focus-ring inline-flex h-8 items-center gap-2 rounded-md border border-zinc-700 px-2.5 text-sm hover:bg-zinc-800"
            >
              <Printer size={15} />
              {t('qr.print')}
            </Link>
          </div>
        </div>
      </div>
      {instance.public_links && share.data && (
        <PublicLink vehicle={vehicle} link={link} demo={instance.demo_mode} onOff={() => { setCode('app') }} />
      )}
      {share.isError && <ErrorBanner message={t('share.loadFailed')} />}
    </Card>
  )
}

type Options = Pick<Share, 'include_costs' | 'include_license_plate' | 'include_vin'>

/** Turns a vehicle's public read-only link on and off, and sets what it shows. */
function PublicLink({ vehicle, link, demo, onOff }: { vehicle: Vehicle; link: Share | null; demo: boolean; onOff: () => void }) {
  const { t } = useTranslation(['vehicles', 'errors'])
  const errorText = useErrorText()
  const fmt = useFormat()
  const qc = useQueryClient()
  const path = `/vehicles/${String(vehicle.id)}/share`
  const [confirm, setConfirm] = useState<'renew' | 'off' | null>(null)
  const [copied, setCopied] = useState(false)
  const saved = (data: { share: Share | null }) => {
    qc.setQueryData(queryKeys.share(vehicle.id), data)
    void qc.invalidateQueries({ queryKey: queryKeys.shares })
  }
  // Options show their new state at once and roll back if saving fails. While
  // another change is on its way, its response is the one to keep.
  const optionsKey = ['share-options', vehicle.id]
  const put = useMutation({
    mutationKey: optionsKey,
    mutationFn: (opts: Partial<Options>) => api.put<{ share: Share }>(path, opts),
    onMutate: (opts) => {
      const key = queryKeys.share(vehicle.id)
      const prev = qc.getQueryData<{ share: Share | null }>(key)
      if (prev?.share) qc.setQueryData(key, { share: { ...prev.share, ...opts } })
      return prev
    },
    onError: (_, __, prev) => { if (prev) qc.setQueryData(queryKeys.share(vehicle.id), prev) },
    onSuccess: (data) => { if (qc.isMutating({ mutationKey: optionsKey }) <= 1) saved(data) },
  })
  const renew = useMutation({
    mutationFn: () => api.post<{ share: Share }>(`${path}/renew`),
    onSuccess: (data) => { saved(data); setConfirm(null) },
  })
  const off = useMutation({
    mutationFn: () => api.del(path),
    onSuccess: () => { saved({ share: null }); setConfirm(null); onOff() },
  })
  const busy = put.isPending || renew.isPending || off.isPending

  const copy = async () => {
    if (!link) return
    try {
      await navigator.clipboard.writeText(shareUrl(link))
      setCopied(true)
      setTimeout(() => { setCopied(false) }, 2000)
    } catch {
      // No clipboard access (e.g. plain http); the address is shown to copy by hand.
    }
  }

  return (
    <section aria-labelledby="public-link" className="mt-4 space-y-3 border-t border-zinc-800 pt-4">
      <h3 id="public-link" className="text-sm font-semibold text-zinc-100">{t('share.title')}</h3>
      {!link ? (
        <>
          <p className="text-sm text-zinc-400">{t('share.intro')}</p>
          {demo && <Notice tone="amber">{t('errors:demo.shares_locked')}</Notice>}
          <Button size="sm" variant="primary" disabled={demo || busy} onClick={() => { put.mutate({}) }}>
            {t('share.turnOn')}
          </Button>
        </>
      ) : (
        <>
          <p className="text-sm text-zinc-400">{t('share.on')}</p>
          {vehicle.archived && <Notice tone="amber">{t('share.archived')}</Notice>}
          <div className="flex flex-wrap items-center gap-2">
            <code className="min-w-0 flex-1 break-all rounded bg-zinc-950 px-2 py-1 text-xs text-zinc-300">{shareUrl(link)}</code>
            <Button size="sm" onClick={() => { void copy() }}>
              <Copy size={15} />
              {copied ? t('share.copied') : t('share.copy')}
            </Button>
          </div>
          <fieldset className="space-y-1.5">
            <legend className="mb-1 text-sm text-zinc-300">{t('share.alsoShow')}</legend>
            {(['include_costs', 'include_license_plate', 'include_vin'] as const).map((k) => (
              <div key={k}>
                <Checkbox label={t(`share.options.${k}`)} checked={link[k]} disabled={demo || renew.isPending || off.isPending} onChange={(v) => { put.mutate({ [k]: v }) }} />
              </div>
            ))}
          </fieldset>
          <p className="text-xs text-zinc-500">
            {link.last_opened_at ? t('share.lastOpened', { date: fmt.dateTime(link.last_opened_at) }) : t('share.neverOpened')}
          </p>
          {demo && <Notice tone="amber">{t('errors:demo.shares_locked')}</Notice>}
          <div className="flex flex-wrap gap-2">
            <Button size="sm" disabled={demo || busy} onClick={() => { setConfirm('renew') }}>
              <RefreshCw size={15} />
              {t('share.renew')}
            </Button>
            <Button size="sm" variant="danger" disabled={busy} onClick={() => { setConfirm('off') }}>
              {t('share.turnOff')}
            </Button>
          </div>
        </>
      )}
      {put.isError && <ErrorBanner message={errorText(put.error)} />}
      <ConfirmDialog
        open={confirm === 'renew'}
        title={t('share.renewTitle')}
        body={t('share.renewBody')}
        confirmLabel={t('share.renew')}
        busy={renew.isPending}
        error={renew.isError ? errorText(renew.error) : undefined}
        onConfirm={() => { renew.mutate() }}
        onClose={() => { setConfirm(null); renew.reset() }}
      />
      <ConfirmDialog
        open={confirm === 'off'}
        title={t('share.offTitle')}
        body={t('share.offBody')}
        confirmLabel={t('share.turnOff')}
        danger
        busy={off.isPending}
        error={off.isError ? errorText(off.error) : undefined}
        onConfirm={() => { off.mutate() }}
        onClose={() => { setConfirm(null); off.reset() }}
      />
    </section>
  )
}

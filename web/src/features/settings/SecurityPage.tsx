import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Copy, KeyRound, Pencil, Printer, Trash2 } from 'lucide-react'
import { api, isApiError } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { useFormat } from '../../lib/useFormat'
import type { MfaStatus, Passkey } from '../../lib/types'
import { createPasskey, passkeysSupported } from '../../lib/webauthn'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { TextField } from '../../components/Field'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { QrCode } from '../../components/QrCode'
import { Badge, Card, ErrorBanner, Loading, Notice, PageHeader } from '../../components/ui'
import { PasswordCard } from '../auth/ChangePasswordPage'

export function useMfaStatus() {
  return useQuery({ queryKey: queryKeys.mfa, queryFn: () => api.get<MfaStatus>('/me/mfa') })
}

/** Settings → Sign-in & security: password, authenticator app, recovery codes and passkeys. */
export function SecurityPage() {
  const { t } = useTranslation('settings')
  const errorText = useErrorText()
  const { user, instance } = useSession()
  const status = useMfaStatus()
  // New recovery codes are shown once, right after they were made.
  const [codes, setCodes] = useState<string[] | null>(null)
  const showCodes = (c: string[]) => { if (c.length > 0) setCodes(c) }

  return (
    <div className="max-w-2xl space-y-6">
      <PageHeader title={t('security.title')} subtitle={t('security.intro')} />
      {user.mfa_setup_required && <Notice tone="amber">{t('security.setupRequired')}</Notice>}
      {instance.demo_mode && <Notice tone="amber">{t('errors:demo.accounts_locked')}</Notice>}
      {status.isPending && <Loading />}
      {status.isError && <ErrorBanner message={errorText(status.error)} onRetry={() => void status.refetch()} />}
      {codes && <RecoveryCodes codes={codes} onClose={() => { setCodes(null) }} />}
      {status.data && (
        <>
          <TotpCard status={status.data} onCodes={showCodes} />
          <PasskeysCard status={status.data} onCodes={showCodes} />
          <RecoveryCard status={status.data} onCodes={showCodes} />
        </>
      )}
      {!user.mfa_setup_required && <PasswordCard />}
    </div>
  )
}

function useInvalidateMfa() {
  const qc = useQueryClient()
  // The session's mfa_setup_required and mfa_enabled change with the factors.
  return () => Promise.all([qc.invalidateQueries({ queryKey: queryKeys.mfa }), qc.invalidateQueries({ queryKey: queryKeys.me })])
}

function TotpCard({ status, onCodes }: { status: MfaStatus; onCodes: (c: string[]) => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const invalidate = useInvalidateMfa()
  const [setup, setSetup] = useState<{ secret: string; uri: string } | null>(null)
  const [code, setCode] = useState('')
  const [turningOff, setTurningOff] = useState(false)

  const begin = useMutation({
    mutationFn: () => api.post<{ secret: string; uri: string }>('/me/totp'),
    onSuccess: (s) => { setSetup(s); setCode('') },
  })
  const confirm = useMutation({
    mutationFn: () => api.post<{ recovery_codes: string[] }>('/me/totp/confirm', { code }),
    onSuccess: async (r) => { setSetup(null); onCodes(r.recovery_codes); await invalidate() },
  })
  const off = useMutation({
    mutationFn: () => api.del('/me/totp'),
    onSuccess: async () => { setTurningOff(false); await invalidate() },
  })
  const fe = fieldErrors(confirm.error)

  return (
    <Card title={t('security.totp.title')} actions={status.totp ? <Badge tone="emerald">{t('security.on')}</Badge> : undefined}>
      <p className="mb-4 text-sm text-zinc-400">{t('security.totp.intro')}</p>
      {begin.isError && <ErrorBanner message={errorText(begin.error)} />}
      {status.totp ? (
        <Button size="sm" variant="danger" onClick={() => { setTurningOff(true) }}>{t('security.totp.turnOff')}</Button>
      ) : setup ? (
        <form
          className="space-y-4"
          onSubmit={(e) => { e.preventDefault(); confirm.mutate() }}
          noValidate
        >
          <ol className="list-decimal space-y-1 pl-5 text-sm text-zinc-300">
            <li>{t('security.totp.step1')}</li>
            <li>{t('security.totp.step2')}</li>
          </ol>
          <div className="flex flex-wrap items-center gap-4">
            <QrCode value={setup.uri} label={t('security.totp.qr')} className="h-40 w-40 rounded bg-white p-2" />
            <div className="min-w-0 space-y-1 text-sm">
              <div className="text-zinc-400">{t('security.totp.key')}</div>
              <code className="break-all rounded bg-zinc-950 px-2 py-1 font-mono text-zinc-100">{setup.secret.replace(/(.{4})/g, '$1 ').trim()}</code>
            </div>
          </div>
          {confirm.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(confirm.error)} />}
          <TextField label={t('security.totp.code')} value={code} onChange={setCode} error={fe.code} inputMode="numeric" autoComplete="one-time-code" maxLength={7} />
          <div className="flex gap-2">
            <Button type="submit" variant="primary" disabled={confirm.isPending || !code.trim()}>{t('security.totp.confirm')}</Button>
            <Button onClick={() => { setSetup(null) }}>{t('common:actions.cancel')}</Button>
          </div>
        </form>
      ) : (
        <Button size="sm" variant="primary" disabled={begin.isPending} onClick={() => { begin.mutate() }}>{t('security.totp.setUp')}</Button>
      )}
      <ConfirmDialog
        open={turningOff}
        title={t('security.totp.turnOff')}
        body={t('security.totp.turnOffBody')}
        confirmLabel={t('security.totp.turnOff')}
        danger
        busy={off.isPending}
        error={off.isError ? errorText(off.error) : undefined}
        onConfirm={() => { off.mutate() }}
        onClose={() => { setTurningOff(false); off.reset() }}
      />
    </Card>
  )
}

function PasskeysCard({ status, onCodes }: { status: MfaStatus; onCodes: (c: string[]) => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const fmt = useFormat()
  const invalidate = useInvalidateMfa()
  const [naming, setNaming] = useState(false)
  const [renaming, setRenaming] = useState<Passkey | null>(null)
  const [removing, setRemoving] = useState<Passkey | null>(null)
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/me/passkeys/${String(id)}`),
    onSuccess: async () => { setRemoving(null); await invalidate() },
  })

  return (
    <Card
      title={t('security.passkeys.title')}
      actions={status.passkeys_available && passkeysSupported() && (
        <Button size="sm" onClick={() => { setNaming(true) }}>
          <KeyRound size={15} />
          {t('security.passkeys.add')}
        </Button>
      )}
    >
      <p className="mb-4 text-sm text-zinc-400">{t('security.passkeys.intro')}</p>
      {!status.passkeys_available && <Notice>{t('security.passkeys.unavailable')}</Notice>}
      {status.passkeys_available && !passkeysSupported() && <Notice>{t('security.passkeys.unsupported')}</Notice>}
      {status.passkeys.length > 0 && (
        <ul className="divide-y divide-zinc-800">
          {status.passkeys.map((p) => (
            <li key={p.id} className="flex flex-wrap items-center justify-between gap-2 py-2">
              <div className="min-w-0">
                <div className="text-sm text-zinc-100">{p.name}</div>
                <div className="text-xs text-zinc-400">
                  {[t('security.passkeys.added', { date: fmt.date(p.created_at.slice(0, 10)) }),
                    p.last_used_at ? t('security.passkeys.lastUsed', { date: fmt.dateTime(p.last_used_at) }) : t('security.passkeys.neverUsed')].join(' · ')}
                </div>
              </div>
              <div className="flex gap-0.5">
                <IconButton aria-label={t('security.passkeys.rename', { name: p.name })} onClick={() => { setRenaming(p) }}><Pencil size={15} /></IconButton>
                <IconButton aria-label={t('security.passkeys.remove', { name: p.name })} onClick={() => { setRemoving(p) }}><Trash2 size={15} /></IconButton>
              </div>
            </li>
          ))}
        </ul>
      )}
      {naming && <AddPasskeyDialog onClose={() => { setNaming(false) }} onCodes={onCodes} />}
      {renaming && <RenamePasskeyDialog passkey={renaming} onClose={() => { setRenaming(null) }} />}
      <ConfirmDialog
        open={removing !== null}
        title={t('security.passkeys.removeTitle')}
        body={t('security.passkeys.removeBody', { name: removing?.name ?? '' })}
        confirmLabel={t('common:actions.remove')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { if (removing) remove.mutate(removing.id) }}
        onClose={() => { setRemoving(null); remove.reset() }}
      />
    </Card>
  )
}

function AddPasskeyDialog({ onClose, onCodes }: { onClose: () => void; onCodes: (c: string[]) => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const invalidate = useInvalidateMfa()
  const [name, setName] = useState('')
  const add = useMutation({
    mutationFn: async () => {
      const { session, options } = await api.post<{ session: string; options: { publicKey: Record<string, unknown> } }>('/me/passkeys/options')
      return api.post<{ recovery_codes: string[] }>('/me/passkeys', { session, name, credential: await createPasskey(options) })
    },
    onSuccess: async (r) => { onCodes(r.recovery_codes); await invalidate(); onClose() },
  })
  const fe = fieldErrors(add.error)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('security.passkeys.add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" disabled={add.isPending || !name.trim()} onClick={() => { add.mutate() }}>{t('security.passkeys.create')}</Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-zinc-400">{t('security.passkeys.addIntro')}</p>
        {add.isError && Object.keys(fe).length === 0 && <ErrorBanner message={isApiError(add.error) ? errorText(add.error) : t('security.passkeys.cancelled')} />}
        <TextField label={t('security.passkeys.name')} value={name} onChange={setName} error={fe.name} placeholder={t('security.passkeys.namePlaceholder')} maxLength={64} autoFocus />
      </div>
    </Dialog>
  )
}

function RenamePasskeyDialog({ passkey, onClose }: { passkey: Passkey; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const invalidate = useInvalidateMfa()
  const [name, setName] = useState(passkey.name)
  const rename = useMutation({
    mutationFn: () => api.patch(`/me/passkeys/${String(passkey.id)}`, { name }),
    onSuccess: async () => { await invalidate(); onClose() },
  })
  const fe = fieldErrors(rename.error)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('security.passkeys.rename', { name: passkey.name })}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" disabled={rename.isPending || !name.trim()} onClick={() => { rename.mutate() }}>{t('common:actions.save')}</Button>
        </>
      }
    >
      <TextField label={t('security.passkeys.name')} value={name} onChange={setName} error={fe.name} maxLength={64} autoFocus />
    </Dialog>
  )
}

function RecoveryCard({ status, onCodes }: { status: MfaStatus; onCodes: (c: string[]) => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const invalidate = useInvalidateMfa()
  const [confirming, setConfirming] = useState(false)
  const renew = useMutation({
    mutationFn: () => api.post<{ recovery_codes: string[] }>('/me/recovery-codes'),
    onSuccess: async (r) => { setConfirming(false); onCodes(r.recovery_codes); await invalidate() },
  })
  if (!status.totp && status.passkeys.length === 0) return null
  return (
    <Card title={t('security.recovery.title')}>
      <p className="mb-3 text-sm text-zinc-400">{t('security.recovery.intro')}</p>
      <p className="mb-4 text-sm text-zinc-200">{t('security.recovery.left', { count: status.recovery_codes_left })}</p>
      {status.recovery_codes_left <= 2 && <div className="mb-4"><Notice tone="amber">{t('security.recovery.low')}</Notice></div>}
      <Button size="sm" onClick={() => { setConfirming(true) }}>{t('security.recovery.renew')}</Button>
      <ConfirmDialog
        open={confirming}
        title={t('security.recovery.renew')}
        body={t('security.recovery.renewBody')}
        confirmLabel={t('security.recovery.renew')}
        busy={renew.isPending}
        error={renew.isError ? errorText(renew.error) : undefined}
        onConfirm={() => { renew.mutate() }}
        onClose={() => { setConfirming(false); renew.reset() }}
      />
    </Card>
  )
}

/** New recovery codes, shown once with copy and print. */
function RecoveryCodes({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(codes.join('\n'))
      setCopied(true)
    } catch {
      // No clipboard (plain http): the codes are on screen to write down.
    }
  }
  return (
    <section aria-labelledby="new-recovery-codes" className="space-y-3 rounded-lg border border-amber-500/40 bg-amber-500/5 p-4 print:border-black print:text-black">
      <h2 id="new-recovery-codes" className="text-base font-semibold text-zinc-100 print:text-black">{t('security.recovery.newTitle')}</h2>
      <p className="text-sm text-zinc-300 print:text-black">{t('security.recovery.newIntro')}</p>
      <ul className="grid grid-cols-2 gap-x-6 gap-y-1 font-mono text-sm text-zinc-100 print:text-black">
        {codes.map((c) => <li key={c}>{c}</li>)}
      </ul>
      <div className="flex flex-wrap gap-2 print:hidden">
        <Button size="sm" onClick={() => { void copy() }}><Copy size={15} />{copied ? t('security.recovery.copied') : t('security.recovery.copy')}</Button>
        <Button size="sm" onClick={() => { window.print() }}><Printer size={15} />{t('security.recovery.print')}</Button>
        <Button size="sm" variant="primary" onClick={onClose}>{t('security.recovery.saved')}</Button>
      </div>
    </section>
  )
}

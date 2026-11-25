import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { KeyRound } from 'lucide-react'
import { api, isApiError } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { MeResponse, MfaMethod } from '../../lib/types'
import { useErrorText } from '../../lib/errors'
import { getPasskey, passkeysSupported } from '../../lib/webauthn'
import { useServerInfo } from '../../app/branding'
import { Button } from '../../components/Button'
import { TextField } from '../../components/Field'
import { ErrorBanner } from '../../components/ui'
import { AuthCard } from './AuthCard'

interface Challenge {
  id: string
  methods: MfaMethod[]
}

export function LoginPage() {
  const { t } = useTranslation('auth')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const info = useServerInfo()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [challenge, setChallenge] = useState<Challenge | null>(null)

  const signedIn = (me: MeResponse) => { qc.setQueryData(queryKeys.me, me) }
  const login = useMutation({
    mutationFn: () => api.post<MeResponse>('/auth/login', { username, password }),
    onSuccess: signedIn,
    onError: (err) => {
      // A second factor is needed: no session yet, but a challenge.
      if (isApiError(err, 'auth.mfa_required')) {
        setChallenge({ id: err.params.challenge as string, methods: err.params.methods as MfaMethod[] })
      }
    },
  })
  const passkey = useMutation({
    mutationFn: async () => {
      const { session, options } = await api.post<{ session: string; options: { publicKey: Record<string, unknown> } }>('/auth/passkey/options')
      return api.post<MeResponse>('/auth/passkey', { session, credential: await getPasskey(options) })
    },
    onSuccess: signedIn,
  })

  if (challenge) {
    return <SecondFactor challenge={challenge} onDone={signedIn} onBack={() => { setChallenge(null); setPassword(''); login.reset() }} />
  }

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    login.mutate()
  }
  const loginError = login.isError && !isApiError(login.error, 'auth.mfa_required')
  // A cancelled or timed-out browser prompt isn't worth an error message.
  const passkeyError = passkey.isError && isApiError(passkey.error)

  return (
    <AuthCard title={t('login.title')} onSubmit={submit}>
      {loginError && <ErrorBanner message={errorText(login.error)} />}
      {passkeyError && <ErrorBanner message={errorText(passkey.error)} />}
      <TextField label={t('fields.username')} value={username} onChange={setUsername} autoComplete="username webauthn" autoFocus required />
      <TextField label={t('fields.password')} type="password" value={password} onChange={setPassword} autoComplete="current-password" required />
      <Button type="submit" variant="primary" className="w-full" disabled={login.isPending || !username || !password}>
        {t('login.submit')}
      </Button>
      {info.data?.passkeys && passkeysSupported() && (
        <Button className="w-full" disabled={passkey.isPending} onClick={() => { passkey.mutate() }}>
          <KeyRound size={16} />
          {t('login.passkey')}
        </Button>
      )}
    </AuthCard>
  )
}

/** The second step: a code from the authenticator app, a recovery code or a passkey. */
function SecondFactor({ challenge, onDone, onBack }: { challenge: Challenge; onDone: (me: MeResponse) => void; onBack: () => void }) {
  const { t } = useTranslation('auth')
  const errorText = useErrorText()
  const has = (m: MfaMethod) => challenge.methods.includes(m)
  const [mode, setMode] = useState<'totp' | 'recovery_code'>(has('totp') ? 'totp' : 'recovery_code')
  const [code, setCode] = useState('')

  const verify = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.post<MeResponse>('/auth/mfa', { challenge: challenge.id, ...body }),
    onSuccess: onDone,
    onError: () => { setCode('') },
  })
  const passkey = useMutation({
    mutationFn: async () => {
      const { options } = await api.post<{ options: { publicKey: Record<string, unknown> } }>('/auth/mfa/passkey-options', { challenge: challenge.id })
      return api.post<MeResponse>('/auth/mfa', { challenge: challenge.id, passkey: await getPasskey(options) })
    },
    onSuccess: onDone,
  })
  const expired = [verify.error, passkey.error].some((e) => isApiError(e, 'auth.mfa_challenge_expired'))

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    verify.mutate(mode === 'totp' ? { code } : { recovery_code: code })
  }

  return (
    <AuthCard title={t('mfa.title')} intro={mode === 'totp' ? t('mfa.totpIntro') : t('mfa.recoveryIntro')} onSubmit={submit}>
      {verify.isError && <ErrorBanner message={errorText(verify.error)} />}
      {passkey.isError && isApiError(passkey.error) && <ErrorBanner message={errorText(passkey.error)} />}
      {expired ? (
        <Button variant="primary" className="w-full" onClick={onBack}>{t('mfa.startOver')}</Button>
      ) : (
        <>
          {mode === 'totp' ? (
            <TextField
              label={t('mfa.code')}
              value={code}
              onChange={setCode}
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={7}
              autoFocus
              required
            />
          ) : (
            <TextField label={t('mfa.recoveryCode')} value={code} onChange={setCode} autoComplete="off" autoFocus required />
          )}
          <Button type="submit" variant="primary" className="w-full" disabled={verify.isPending || !code.trim()}>
            {t('mfa.verify')}
          </Button>
          {has('passkey') && passkeysSupported() && (
            <Button className="w-full" disabled={passkey.isPending} onClick={() => { passkey.mutate() }}>
              <KeyRound size={16} />
              {t('mfa.usePasskey')}
            </Button>
          )}
          <div className="flex flex-wrap justify-between gap-2 text-sm">
            {has('totp') && has('recovery_code') && (
              <button type="button" className="focus-ring text-sky-300 hover:underline" onClick={() => { setMode(mode === 'totp' ? 'recovery_code' : 'totp'); setCode(''); verify.reset() }}>
                {mode === 'totp' ? t('mfa.useRecovery') : t('mfa.useTotp')}
              </button>
            )}
            <button type="button" className="focus-ring text-zinc-400 hover:underline" onClick={onBack}>{t('mfa.back')}</button>
          </div>
        </>
      )}
    </AuthCard>
  )
}

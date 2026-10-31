import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { MeResponse } from '../../lib/types'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { Button } from '../../components/Button'
import { TextField } from '../../components/Field'
import { ErrorBanner } from '../../components/ui'
import { AuthCard } from './AuthCard'

export function SetupPage() {
  const { t, i18n } = useTranslation('auth')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')

  const setup = useMutation({
    mutationFn: () =>
      api.post<MeResponse>('/setup', { username, display_name: displayName, password, locale: i18n.language.startsWith('de') ? 'de' : 'en' }),
    onSuccess: (me) => {
      qc.setQueryData(queryKeys.setupStatus, { setup_required: false })
      qc.setQueryData(queryKeys.me, me)
    },
  })
  const fe = fieldErrors(setup.error)

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    setup.mutate()
  }

  return (
    <AuthCard title={t('setup.title')} intro={t('setup.intro')} onSubmit={submit}>
      {setup.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(setup.error)} />}
      <TextField label={t('fields.username')} value={username} onChange={setUsername} error={fe.username} autoComplete="username" required />
      <TextField label={t('fields.displayName')} value={displayName} onChange={setDisplayName} error={fe.display_name} autoComplete="name" required />
      <TextField
        label={t('fields.password')}
        type="password"
        value={password}
        onChange={setPassword}
        error={fe.password}
        hint={t('fields.passwordHint')}
        autoComplete="new-password"
        required
      />
      <Button type="submit" variant="primary" className="w-full" disabled={setup.isPending}>
        {t('setup.submit')}
      </Button>
    </AuthCard>
  )
}

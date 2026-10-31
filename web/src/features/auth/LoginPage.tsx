import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { MeResponse } from '../../lib/types'
import { useErrorText } from '../../lib/errors'
import { Button } from '../../components/Button'
import { TextField } from '../../components/Field'
import { ErrorBanner } from '../../components/ui'
import { AuthCard } from './AuthCard'

export function LoginPage() {
  const { t } = useTranslation('auth')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const login = useMutation({
    mutationFn: () => api.post<MeResponse>('/auth/login', { username, password }),
    onSuccess: (me) => { qc.setQueryData(queryKeys.me, me) },
  })

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    login.mutate()
  }

  return (
    <AuthCard title={t('login.title')} onSubmit={submit}>
      {login.isError && <ErrorBanner message={errorText(login.error)} />}
      <TextField label={t('fields.username')} value={username} onChange={setUsername} autoComplete="username" autoFocus required />
      <TextField label={t('fields.password')} type="password" value={password} onChange={setPassword} autoComplete="current-password" required />
      <Button type="submit" variant="primary" className="w-full" disabled={login.isPending || !username || !password}>
        {t('login.submit')}
      </Button>
    </AuthCard>
  )
}

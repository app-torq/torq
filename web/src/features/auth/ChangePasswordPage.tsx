import { useState, type SubmitEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { Button } from '../../components/Button'
import { TextField } from '../../components/Field'
import { Card, ErrorBanner, Notice, PageHeader } from '../../components/ui'

export function ChangePasswordPage() {
  const { t } = useTranslation('auth')
  const errorText = useErrorText()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [repeat, setRepeat] = useState('')
  const [mismatch, setMismatch] = useState(false)

  const change = useMutation({
    mutationFn: () => api.post('/auth/password', { current_password: current, new_password: next }),
    onSuccess: () => { setCurrent(''); setNext(''); setRepeat('') },
  })
  const fe = fieldErrors(change.error)

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    if (next !== repeat) {
      setMismatch(true)
      return
    }
    setMismatch(false)
    change.mutate()
  }

  return (
    <div className="max-w-lg">
      <PageHeader title={t('password.title')} subtitle={t('password.intro')} />
      <Card>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {change.isSuccess && <Notice tone="emerald">{t('password.changed')}</Notice>}
          {change.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(change.error)} />}
          <TextField label={t('fields.currentPassword')} type="password" value={current} onChange={setCurrent} error={fe.current_password} autoComplete="current-password" />
          <TextField label={t('fields.newPassword')} type="password" value={next} onChange={setNext} error={fe.new_password} hint={t('fields.passwordHint')} autoComplete="new-password" />
          <TextField
            label={t('fields.confirmPassword')}
            type="password"
            value={repeat}
            onChange={setRepeat}
            error={mismatch ? t('password.mismatch') : undefined}
            autoComplete="new-password"
          />
          <Button type="submit" variant="primary" disabled={change.isPending || !current || !next}>
            {t('common:actions.save')}
          </Button>
        </form>
      </Card>
    </div>
  )
}

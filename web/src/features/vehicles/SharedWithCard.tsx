import { useState, type SubmitEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { canOn } from '../../lib/permissions'
import type { AccessLevel, SharedWith, Vehicle } from '../../lib/types'
import { Button } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { Card, ErrorBanner, Notice } from '../../components/ui'

/**
 * The users the signed-in user shared this vehicle with (#50). Shown when
 * they may share it, or still have shares on it after losing the permission:
 * those can be taken back but not added or raised. Usernames are typed, never
 * suggested, so nobody learns which accounts exist.
 */
export function SharedWithCard({ vehicle }: { vehicle: Vehicle }) {
  const { t } = useTranslation(['vehicles', 'settings', 'errors'])
  const qc = useQueryClient()
  const errorText = useErrorText()
  const canShare = canOn(vehicle, 'vehicle_access.share')
  const path = `/vehicles/${String(vehicle.id)}/shared-with`
  const shares = useQuery({
    queryKey: queryKeys.sharedWith(vehicle.id),
    queryFn: () => api.get<{ items: SharedWith[] }>(path),
  })
  const [username, setUsername] = useState('')
  const [level, setLevel] = useState<AccessLevel>('view')
  const invalidate = () => qc.invalidateQueries({ queryKey: queryKeys.sharedWith(vehicle.id) })
  const share = useMutation({
    mutationFn: (body: { username: string; level: AccessLevel }) => api.put(path, body),
    onSuccess: async (_, body) => {
      if (body.username === username) setUsername('')
      await invalidate()
    },
  })
  const unshare = useMutation({
    mutationFn: (userId: number) => api.del(`${path}/${String(userId)}`),
    onSuccess: invalidate,
  })

  const items = shares.data?.items ?? []
  if (!canShare && items.length === 0) return null

  const levels = [
    { value: 'view', label: t('settings:users.access.view') },
    { value: 'edit', label: t('settings:users.access.edit') },
  ]
  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    share.mutate({ username: username.trim(), level })
  }
  const fields = fieldErrors(share.error)

  return (
    <Card title={t('sharedWith.title')}>
      <p className="mb-3 text-sm text-zinc-400">{t('sharedWith.intro')}</p>
      {!canShare && <Notice tone="amber">{t('sharedWith.noPermission')}</Notice>}
      {shares.isError && <ErrorBanner message={errorText(shares.error)} />}
      {(share.isError && !fields.username) && <ErrorBanner message={errorText(share.error)} />}
      {unshare.isError && <ErrorBanner message={errorText(unshare.error)} />}
      {shares.isSuccess && items.length === 0 && <p className="text-sm text-zinc-500">{t('sharedWith.empty')}</p>}
      {items.length > 0 && (
        <ul className="divide-y divide-zinc-800">
          {items.map((s) => (
            <li key={s.user_id} className="flex flex-wrap items-center justify-between gap-2 py-2 text-sm">
              <span className="text-zinc-100">
                {s.display_name}
                <span className="ml-2 text-xs text-zinc-400">{s.username}</span>
              </span>
              <span className="flex items-center gap-2">
                <select
                  aria-label={t('sharedWith.levelLabel', { name: s.display_name })}
                  value={s.level}
                  onChange={(e) => { share.mutate({ username: s.username, level: e.target.value as AccessLevel }) }}
                  disabled={!canShare || share.isPending}
                  className="focus-ring rounded-md border border-zinc-700 bg-zinc-950 px-2 py-1 text-xs"
                >
                  {levels.map((l) => <option key={l.value} value={l.value}>{l.label}</option>)}
                </select>
                <Button
                  size="sm"
                  aria-label={t('sharedWith.removeLabel', { name: s.display_name })}
                  disabled={unshare.isPending}
                  onClick={() => { unshare.mutate(s.user_id) }}
                >
                  {t('sharedWith.remove')}
                </Button>
              </span>
            </li>
          ))}
        </ul>
      )}
      {canShare && (
        <form onSubmit={submit} className="mt-3 flex flex-wrap items-end gap-2">
          <TextField
            label={t('sharedWith.username')}
            hint={t('sharedWith.usernameHint')}
            value={username}
            onChange={setUsername}
            error={fields.username ? t(`errors:${fields.username}`) : undefined}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            className="min-w-48 flex-1"
          />
          <SelectField
            label={t('sharedWith.level')}
            value={level}
            onChange={(v) => { setLevel(v as AccessLevel) }}
            options={levels}
            className="w-32"
          />
          <Button type="submit" variant="primary" disabled={share.isPending || username.trim() === ''}>
            {t('sharedWith.add')}
          </Button>
        </form>
      )}
    </Card>
  )
}

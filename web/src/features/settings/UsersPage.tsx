import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { Role, User } from '../../lib/types'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Badge, ErrorBanner, Loading, Notice, PageHeader, Table } from '../../components/ui'

export function UsersPage() {
  const { t } = useTranslation(['settings', 'common'])
  const { user: me } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const users = useQuery({ queryKey: queryKeys.users, queryFn: () => api.get<{ items: User[] }>('/users') })
  const [creating, setCreating] = useState(false)
  const [resetting, setResetting] = useState<User | null>(null)
  const [deleting, setDeleting] = useState<User | null>(null)
  const [actionError, setActionError] = useState<unknown>(null)

  const invalidate = () => qc.invalidateQueries({ queryKey: queryKeys.users })
  const update = useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) => api.patch(`/users/${id}`, body),
    onSuccess: () => { setActionError(null); void invalidate() },
    onError: setActionError,
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/users/${id}`),
    onSuccess: () => { setDeleting(null); void invalidate() },
  })

  return (
    <div>
      <PageHeader
        title={t('users.title')}
        subtitle={t('users.intro')}
        actions={
          <Button variant="primary" onClick={() => { setCreating(true) }}>
            <Plus size={16} />
            {t('users.add')}
          </Button>
        }
      />
      {actionError !== null && <div className="mb-4"><ErrorBanner message={errorText(actionError)} /></div>}
      {users.isPending && <Loading />}
      {users.isError && <ErrorBanner message={errorText(users.error)} onRetry={() => void users.refetch()} />}
      {users.data && (
        <Table>
          <thead>
            <tr>
              <th>{t('auth:fields.displayName')}</th>
              <th>{t('auth:fields.username')}</th>
              <th>{t('users.role')}</th>
              <th>{t('users.status')}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.data.items.map((u) => (
              <tr key={u.id}>
                <td className="text-zinc-100">
                  {u.display_name}
                  {u.id === me.id && <span className="ml-2 text-xs text-zinc-400">({t('users.you')})</span>}
                </td>
                <td className="text-zinc-400">{u.username}</td>
                <td>
                  <select
                    aria-label={t('users.role')}
                    className="focus-ring rounded border border-zinc-700 bg-zinc-950 px-2 py-1 text-sm"
                    value={u.role}
                    onChange={(e) => { update.mutate({ id: u.id, body: { role: e.target.value } }) }}
                  >
                    <option value="admin">{t('users.roles.admin')}</option>
                    <option value="member">{t('users.roles.member')}</option>
                  </select>
                </td>
                <td>{u.disabled ? <Badge tone="red">{t('users.disabled')}</Badge> : <Badge tone="emerald">{t('users.active')}</Badge>}</td>
                <td>
                  <div className="flex justify-end gap-1">
                    {u.id !== me.id && (
                      <Button size="sm" variant="ghost" onClick={() => { update.mutate({ id: u.id, body: { disabled: !u.disabled } }) }}>
                        {u.disabled ? t('users.enable') : t('users.disable')}
                      </Button>
                    )}
                    <Button size="sm" variant="ghost" onClick={() => { setResetting(u) }}>
                      {t('users.resetPassword')}
                    </Button>
                    {u.id !== me.id && (
                      <Button size="sm" variant="ghost" className="text-red-400" onClick={() => { setDeleting(u) }}>
                        {t('common:actions.delete')}
                      </Button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}

      <CreateUserDialog open={creating} onClose={() => { setCreating(false) }} onCreated={() => { setCreating(false); void invalidate() }} />
      <ResetPasswordDialog user={resetting} onClose={() => { setResetting(null) }} />
      <ConfirmDialog
        open={deleting !== null}
        title={t('users.deleteTitle')}
        body={t('users.deleteBody', { name: deleting?.display_name ?? '' })}
        confirmLabel={t('common:actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { if (deleting) remove.mutate(deleting.id) }}
        onClose={() => { setDeleting(null); remove.reset() }}
      />
    </div>
  )
}

function CreateUserDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('member')
  const create = useMutation({
    mutationFn: () => api.post('/users', { username, display_name: displayName, password, role }),
    onSuccess: () => {
      setUsername(''); setDisplayName(''); setPassword(''); setRole('member')
      onCreated()
    },
  })
  const fe = fieldErrors(create.error)
  return (
    <Dialog
      open={open}
      onClose={() => { create.reset(); onClose() }}
      title={t('users.add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" disabled={create.isPending} onClick={() => { create.mutate() }}>
            {t('common:actions.create')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {create.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(create.error)} />}
        <TextField label={t('auth:fields.username')} value={username} onChange={setUsername} error={fe.username} autoComplete="off" />
        <TextField label={t('auth:fields.displayName')} value={displayName} onChange={setDisplayName} error={fe.display_name} />
        <TextField label={t('auth:fields.password')} type="password" value={password} onChange={setPassword} error={fe.password} hint={t('auth:fields.passwordHint')} autoComplete="new-password" />
        <SelectField
          label={t('users.role')}
          value={role}
          onChange={(v) => { setRole(v as Role) }}
          options={[
            { value: 'member', label: t('users.roles.member') },
            { value: 'admin', label: t('users.roles.admin') },
          ]}
        />
      </div>
    </Dialog>
  )
}

function ResetPasswordDialog({ user, onClose }: { user: User | null; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const [password, setPassword] = useState('')
  const reset = useMutation({
    mutationFn: (id: number) => api.post(`/users/${id}/password`, { new_password: password }),
    onSuccess: () => { setPassword('') },
  })
  const fe = fieldErrors(reset.error)
  const close = () => { reset.reset(); setPassword(''); onClose() }
  return (
    <Dialog
      open={user !== null}
      onClose={close}
      title={t('users.resetPassword')}
      footer={
        <>
          <Button onClick={close}>{t('common:actions.close')}</Button>
          <Button variant="primary" disabled={reset.isPending || reset.isSuccess} onClick={() => { if (user) reset.mutate(user.id) }}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-zinc-400">{t('users.resetPasswordIntro', { name: user?.display_name ?? '' })}</p>
        {reset.isSuccess && <Notice tone="emerald">{t('users.passwordReset')}</Notice>}
        {reset.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(reset.error)} />}
        <TextField label={t('auth:fields.newPassword')} type="password" value={password} onChange={setPassword} error={fe.new_password} hint={t('auth:fields.passwordHint')} autoComplete="new-password" />
      </div>
    </Dialog>
  )
}

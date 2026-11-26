import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { Permission, PermissionInfo, Role } from '../../lib/types'
import { Button, IconButton } from '../../components/Button'
import { Checkbox, SelectField, TextAreaField, TextField } from '../../components/Field'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Badge, ErrorBanner, Loading, Notice, PageHeader, Table } from '../../components/ui'
import { useSession } from '../../app/session'
import { needsView, viewPermissions } from '../../lib/permissions'

export interface RolesResponse {
  items: Role[]
  permissions: PermissionInfo[]
}

/** The roles list, shared by the roles page and the user management. */
export function useRoles() {
  return useQuery({ queryKey: queryKeys.roles, queryFn: () => api.get<RolesResponse>('/roles') })
}

/** Settings → Roles: what each role may do and which vehicles its users see. */
export function RolesPage() {
  const { t } = useTranslation(['settings', 'common'])
  const errorText = useErrorText()
  const roles = useRoles()
  const [editing, setEditing] = useState<Role | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Role | null>(null)

  return (
    <div>
      <PageHeader
        title={t('roles.title')}
        subtitle={t('roles.intro')}
        actions={
          <Button variant="primary" onClick={() => { setEditing('new') }}>
            <Plus size={16} />
            {t('roles.add')}
          </Button>
        }
      />
      {roles.isPending && <Loading />}
      {roles.isError && <ErrorBanner message={errorText(roles.error)} onRetry={() => void roles.refetch()} />}
      {roles.data && (
        <Table>
          <thead>
            <tr>
              <th>{t('roles.name')}</th>
              <th>{t('roles.vehicles')}</th>
              <th className="text-right">{t('roles.users')}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {roles.data.items.map((r) => (
              <tr key={r.id}>
                <td>
                  <div className="flex flex-wrap items-center gap-2 text-zinc-100">
                    {r.name}
                    {r.builtin_key && <Badge>{t('roles.builtin')}</Badge>}
                    {r.require_mfa && <Badge tone="sky">{t('roles.mfaBadge')}</Badge>}
                  </div>
                  {r.description && <div className="text-xs text-zinc-400">{r.description}</div>}
                </td>
                <td className="text-zinc-300">{r.all_vehicles ? t('roles.allVehicles') : t('roles.assignedVehicles')}</td>
                <td className="text-right tabular-nums text-zinc-300">{r.user_count}</td>
                <td>
                  <div className="flex justify-end gap-0.5">
                    <IconButton aria-label={t('roles.editNamed', { name: r.name })} onClick={() => { setEditing(r) }}>
                      <Pencil size={15} />
                    </IconButton>
                    {r.builtin_key === null && (
                      <IconButton aria-label={t('roles.deleteNamed', { name: r.name })} className="text-red-400" onClick={() => { setDeleting(r) }}>
                        <Trash2 size={15} />
                      </IconButton>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {roles.data && editing !== null && (
        <RoleDialog role={editing === 'new' ? null : editing} permissions={roles.data.permissions} onClose={() => { setEditing(null) }} />
      )}
      {roles.data && deleting !== null && (
        <DeleteRoleDialog role={deleting} roles={roles.data.items} onClose={() => { setDeleting(null) }} />
      )}
    </div>
  )
}

function RoleDialog({ role, permissions, onClose }: { role: Role | null; permissions: PermissionInfo[]; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [name, setName] = useState(role?.custom_name ?? (role ? role.name : ''))
  const [description, setDescription] = useState(role?.description ?? '')
  const [allVehicles, setAllVehicles] = useState(role?.all_vehicles ?? true)
  // A new role sees everything until the administrator hides something.
  const [granted, setGranted] = useState<Permission[]>(role?.permissions ?? viewPermissions)
  const [requireMfa, setRequireMfa] = useState(role?.require_mfa ?? false)
  const builtin = role?.builtin_key != null
  // The Administrator role can only require a second factor.
  const adminRole = role?.builtin_key === 'admin'
  const { user } = useSession()
  const lockingMyself = requireMfa && !role?.require_mfa && role?.id === user.role_id && !user.mfa_enabled

  const save = useMutation({
    mutationFn: () => {
      if (adminRole) return api.patch(`/roles/${role.id}`, { require_mfa: requireMfa })
      const body = {
        require_mfa: requireMfa,
        // An unchanged built-in name stays translated.
        name: builtin && name.trim() === role.name && role.custom_name === null ? undefined : name,
        description, all_vehicles: allVehicles, permissions: granted,
      }
      return role ? api.patch(`/roles/${role.id}`, body) : api.post('/roles', body)
    },
    onSuccess: async () => {
      await Promise.all([qc.invalidateQueries({ queryKey: queryKeys.roles }), qc.invalidateQueries({ queryKey: queryKeys.users })])
      onClose()
    },
  })
  const resetName = useMutation({
    mutationFn: () => api.patch(`/roles/${role?.id ?? 0}`, { name: null }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.roles })
      onClose()
    },
  })
  const fe = fieldErrors(save.error)
  // Changing an area needs seeing it: ticking a change ticks its "see"
  // permission, unticking a "see" permission unticks its changes.
  const toggle = (p: Permission, on: boolean) => {
    setGranted((g) => {
      if (!on) return g.filter((x) => x !== p && needsView[x] !== p)
      const view = needsView[p]
      return [...g, p, ...(view && !g.includes(view) ? [view] : [])]
    })
  }
  const group = (scope: PermissionInfo['scope']) => (
    <fieldset className="space-y-2">
      <legend className="mb-1 text-sm font-medium text-zinc-200">{t(`roles.scope.${scope}`)}</legend>
      <p className="text-xs text-zinc-400">{t(`roles.scopeHint.${scope}`)}</p>
      {permissions.filter((p) => p.scope === scope).map((p) => {
        const view = needsView[p.key]
        return (
          <div key={p.key}>
            <Checkbox label={t(`roles.permission.${p.key}`, { defaultValue: p.key })} checked={granted.includes(p.key)} onChange={(on) => { toggle(p.key, on) }} />
            {view && <p className="ml-6 text-xs text-zinc-500">{t('roles.needsView', { permission: t(`roles.permission.${view}`) })}</p>}
          </div>
        )
      })}
    </fieldset>
  )
  const losingVehicles = role !== null && role.all_vehicles && !allVehicles && role.user_count > 0

  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={role ? t('roles.editNamed', { name: role.name }) : t('roles.add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" disabled={save.isPending} onClick={() => { save.mutate() }}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        {adminRole && <p className="text-sm text-zinc-400">{t('roles.adminRole')}</p>}
        {!adminRole && (<>
        <TextField
          label={t('roles.name')}
          value={name}
          onChange={setName}
          error={fe.name}
          maxLength={40}
          labelAside={builtin && role.custom_name !== null ? (
            <button type="button" className="focus-ring text-xs text-sky-300 hover:underline" onClick={() => { resetName.mutate() }}>
              {t('roles.resetName')}
            </button>
          ) : undefined}
        />
        <TextAreaField label={t('roles.description')} value={description} onChange={setDescription} error={fe.description} rows={2} optional maxLength={200} />
        <SelectField
          label={t('roles.vehicles')}
          value={allVehicles ? 'all' : 'assigned'}
          onChange={(v) => { setAllVehicles(v === 'all') }}
          hint={allVehicles ? t('roles.allVehiclesHint') : t('roles.assignedVehiclesHint')}
          options={[
            { value: 'all', label: t('roles.allVehicles') },
            { value: 'assigned', label: t('roles.assignedVehicles') },
          ]}
        />
        {losingVehicles && <Notice tone="amber">{t('roles.losingVehicles', { count: role.user_count })}</Notice>}
        {fe.permissions && <ErrorBanner message={errorText(save.error)} />}
        {group('view')}
        <div className="grid gap-6 sm:grid-cols-2">
          {group('vehicle')}
          {group('garage')}
        </div>
        <p className="text-xs text-zinc-400">{t('roles.adminOnly')}</p>
        </>)}
        <div className="space-y-1 border-t border-zinc-800 pt-4">
          <Checkbox label={t('roles.requireMfa')} checked={requireMfa} onChange={setRequireMfa} />
          <p className="text-xs text-zinc-400">{t('roles.requireMfaHint')}</p>
        </div>
        {lockingMyself && <Notice tone="amber">{t('roles.requireMfaSelf')}</Notice>}
      </div>
    </Dialog>
  )
}

function DeleteRoleDialog({ role, roles, onClose }: { role: Role; roles: Role[]; onClose: () => void }) {
  const { t } = useTranslation(['settings', 'common'])
  const qc = useQueryClient()
  const errorText = useErrorText()
  const others = roles.filter((r) => r.id !== role.id)
  const fallback = others.find((r) => r.builtin_key === 'member') ?? others[0]
  const [moveTo, setMoveTo] = useState(String(fallback?.id ?? ''))
  const remove = useMutation({
    mutationFn: () => api.del(`/roles/${role.id}${role.user_count > 0 ? `?move_to=${moveTo}` : ''}`),
    onSuccess: async () => {
      await Promise.all([qc.invalidateQueries({ queryKey: queryKeys.roles }), qc.invalidateQueries({ queryKey: queryKeys.users })])
      onClose()
    },
  })
  return (
    <ConfirmDialog
      open
      title={t('roles.deleteNamed', { name: role.name })}
      body={
        role.user_count > 0 ? (
          <div className="space-y-3">
            <p>{t('roles.deleteMove', { count: role.user_count })}</p>
            <SelectField label={t('roles.moveTo')} value={moveTo} onChange={setMoveTo} options={others.map((r) => ({ value: String(r.id), label: r.name }))} />
          </div>
        ) : t('roles.deleteBody')
      }
      confirmLabel={t('common:actions.delete')}
      danger
      busy={remove.isPending}
      error={remove.isError ? errorText(remove.error) : undefined}
      onConfirm={() => { remove.mutate() }}
      onClose={onClose}
    />
  )
}

import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api, isApiError } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { Session } from '../../lib/types'
import { useErrorText } from '../../lib/errors'
import { useFormat } from '../../lib/useFormat'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { Badge, ErrorBanner, Loading, Notice, PageHeader, Table } from '../../components/ui'
import { deviceLabel } from './deviceLabel'

/**
 * The signed-in user's sessions, browsers and apps alike, with sign-out per
 * device (US3). Unavailable in the demo, where everyone shares one account.
 */
export function DevicesPage() {
  const { t } = useTranslation(['settings', 'common'])
  const { instance } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const fmt = useFormat()
  const sessions = useQuery({
    queryKey: queryKeys.sessions,
    queryFn: () => api.get<{ items: Session[] }>('/me/sessions'),
    enabled: !instance.demo_mode,
  })
  const [revoking, setRevoking] = useState<Session | null>(null)
  const [revokingOthers, setRevokingOthers] = useState(false)

  const signedOut = () => {
    // Same as the logout button: the auth gate shows the login page.
    qc.clear()
    qc.setQueryData(queryKeys.setupStatus, { setup_required: false })
    qc.setQueryData(queryKeys.me, null)
  }
  const revoke = useMutation({
    mutationFn: (s: Session) => api.del(`/me/sessions/${s.id}`),
    onSuccess: (_, s) => {
      setRevoking(null)
      if (s.current) signedOut()
      else void qc.invalidateQueries({ queryKey: queryKeys.sessions })
    },
  })
  const revokeOthers = useMutation({
    mutationFn: () => api.del('/me/sessions'),
    onSuccess: () => { setRevokingOthers(false); void qc.invalidateQueries({ queryKey: queryKeys.sessions }) },
  })

  const locked = instance.demo_mode || isApiError(sessions.error, 'demo.accounts_locked')
  const others = sessions.data?.items.filter((s) => !s.current).length ?? 0

  return (
    <div>
      <PageHeader
        title={t('devices.title')}
        subtitle={t('devices.intro')}
        actions={!locked && others > 0 && (
          <Button onClick={() => { setRevokingOthers(true) }}>{t('devices.revokeOthers')}</Button>
        )}
      />
      {locked && <Notice>{t('devices.demo')}</Notice>}
      {!locked && sessions.isPending && <Loading />}
      {!locked && sessions.isError && <ErrorBanner message={errorText(sessions.error)} onRetry={() => void sessions.refetch()} />}
      {!locked && sessions.data && (
        <>
          <Table>
            <thead>
              <tr>
                <th>{t('devices.device')}</th>
                <th>{t('devices.signedIn')}</th>
                <th>{t('devices.lastActive')}</th>
                <th>{t('devices.ip')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {sessions.data.items.map((s) => (
                <tr key={s.id}>
                  <td className="text-zinc-100">
                    {deviceLabel(s, t)}
                    {s.current && <span className="ml-2"><Badge tone="emerald">{t('devices.thisDevice')}</Badge></span>}
                  </td>
                  <td className="text-zinc-400">{fmt.dateTime(s.created_at)}</td>
                  <td className="text-zinc-400">{fmt.dateTime(s.last_seen_at)}</td>
                  <td className="font-mono text-xs text-zinc-400">{s.ip}</td>
                  <td>
                    <div className="flex justify-end">
                      <Button size="sm" variant="ghost" className="text-red-400" onClick={() => { setRevoking(s) }}>
                        {t('devices.revoke')}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
          {others === 0 && <p className="mt-3 text-sm text-zinc-400">{t('devices.none')}</p>}
        </>
      )}

      <ConfirmDialog
        open={revoking !== null}
        title={t('devices.revokeTitle')}
        body={revoking?.current ? t('devices.revokeCurrentBody') : t('devices.revokeBody', { name: revoking ? deviceLabel(revoking, t) : '' })}
        confirmLabel={t('devices.revoke')}
        danger
        busy={revoke.isPending}
        error={revoke.isError ? errorText(revoke.error) : undefined}
        onConfirm={() => { if (revoking) revoke.mutate(revoking) }}
        onClose={() => { setRevoking(null); revoke.reset() }}
      />
      <ConfirmDialog
        open={revokingOthers}
        title={t('devices.revokeOthersTitle')}
        body={t('devices.revokeOthersBody')}
        confirmLabel={t('devices.revokeOthers')}
        danger
        busy={revokeOthers.isPending}
        error={revokeOthers.isError ? errorText(revokeOthers.error) : undefined}
        onConfirm={() => { revokeOthers.mutate() }}
        onClose={() => { setRevokingOthers(false); revokeOthers.reset() }}
      />
    </div>
  )
}

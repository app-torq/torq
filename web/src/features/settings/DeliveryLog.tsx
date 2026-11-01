import { Fragment, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import type { Delivery, ListResponse } from '../../lib/types'
import { Button } from '../../components/Button'
import { Badge, ErrorBanner, Loading, Table } from '../../components/ui'

const PAGE = 20
const tone = { pending: 'amber', succeeded: 'emerald', failed: 'red' } as const

export function DeliveryLog({ webhookId }: { webhookId: number }) {
  const { t } = useTranslation('webhooks')
  const fmt = useFormat()
  const errorText = useErrorText()
  const [page, setPage] = useState(0)
  const [open, setOpen] = useState<number | null>(null)
  const log = useQuery({
    queryKey: queryKeys.deliveries(webhookId, page),
    queryFn: () => api.get<ListResponse<Delivery>>(`/webhooks/${webhookId}/deliveries`, { limit: PAGE, offset: page * PAGE }),
  })
  const time = (iso: string) => `${fmt.date(iso.slice(0, 10))} ${new Date(iso).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}`
  if (log.isPending) return <Loading />
  if (log.isError) return <ErrorBanner message={errorText(log.error)} />
  if (log.data.items.length === 0) return <p className="text-sm text-zinc-400">{t('log.empty')}</p>
  const pages = Math.ceil(log.data.total / PAGE)
  return (
    <div className="space-y-2">
      <Table>
        <thead>
          <tr>
            <th>{t('log.time')}</th>
            <th>{t('log.event')}</th>
            <th>{t('log.status')}</th>
            <th className="text-right">{t('log.attempts')}</th>
            <th className="text-right">{t('log.code')}</th>
          </tr>
        </thead>
        <tbody>
          {log.data.items.map((d) => (
            <Fragment key={d.id}>
              <tr className="cursor-pointer hover:bg-zinc-900/60" onClick={() => { setOpen(open === d.id ? null : d.id) }}>
                <td className="tabular-nums text-zinc-400">{time(d.created_at)}</td>
                <td>{t(`eventNames.${d.event}`)}</td>
                <td>
                  <Badge tone={tone[d.status]}>{t(`deliveryStatus.${d.status}`)}</Badge>
                  {d.status === 'pending' && d.next_attempt_at && <span className="ml-2 text-xs text-zinc-400">{t('log.nextAttempt', { time: time(d.next_attempt_at) })}</span>}
                </td>
                <td className="text-right tabular-nums">{d.attempt_count}</td>
                <td className="text-right tabular-nums">{d.last_status_code ?? (d.last_error ? t(`errors.${d.last_error}`) : '')}</td>
              </tr>
              {open === d.id && (
                <tr>
                  <td colSpan={5}>
                    <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-all font-mono text-xs text-zinc-400">{d.response_excerpt || t('test.noResponse')}</pre>
                  </td>
                </tr>
              )}
            </Fragment>
          ))}
        </tbody>
      </Table>
      {pages > 1 && (
        <div className="flex items-center justify-end gap-2">
          <Button size="sm" disabled={page === 0} onClick={() => { setPage(page - 1) }}>{t('common:actions.back')}</Button>
          <span className="text-xs text-zinc-400">{page + 1} / {pages}</span>
          <Button size="sm" disabled={page + 1 >= pages} onClick={() => { setPage(page + 1) }}>{t('common:actions.loadMore')}</Button>
        </div>
      )}
    </div>
  )
}

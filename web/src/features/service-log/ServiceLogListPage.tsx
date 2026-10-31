import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { useInfiniteQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName, type ListResponse, type ServiceLog } from '../../lib/types'
import { Button } from '../../components/Button'
import { inputClass } from '../../components/Field'
import { Badge, EmptyState, ErrorBanner, Loading, PageHeader, Table } from '../../components/ui'
import { useVehicles } from '../vehicles/hooks'
import { categoryLabel, useCategories } from './hooks'

const PAGE = 50

/** The Service Logbook list; with vehicleId it is embedded in the vehicle detail tab. */
export function ServiceLogListPage({ vehicleId }: { vehicleId?: number }) {
  const { t } = useTranslation('service')
  const fmt = useFormat()
  const errorText = useErrorText()
  const [params, setParams] = useSearchParams()
  const vehicles = useVehicles('all')
  const categories = useCategories(true)
  const [q, setQ] = useState(params.get('q') ?? '')

  const filters = {
    vehicle_id: vehicleId ?? (Number(params.get('vehicle')) || undefined),
    category_id: Number(params.get('category')) || undefined,
    from: params.get('from') ?? undefined,
    to: params.get('to') ?? undefined,
    q: params.get('q') ?? undefined,
  }
  const logs = useInfiniteQuery({
    queryKey: queryKeys.serviceLogList(filters),
    queryFn: ({ pageParam }) => api.get<ListResponse<ServiceLog>>('/service-logs', { ...filters, limit: PAGE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (last, all) => {
      const loaded = all.reduce((n, p) => n + p.items.length, 0)
      return loaded < last.total ? loaded : undefined
    },
  })

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }
  const vehicleById = new Map(vehicles.data?.items.map((v) => [v.id, v]))
  const catById = new Map(categories.data?.items.map((c) => [c.id, c]))
  const items = logs.data?.pages.flatMap((p) => p.items) ?? []
  const total = logs.data?.pages[0]?.total ?? 0
  const newLink = `/service/new${filters.vehicle_id ? `?vehicle=${filters.vehicle_id}` : ''}`

  return (
    <div>
      {vehicleId === undefined && (
        <PageHeader
          title={t('title')}
          subtitle={t('subtitle')}
          actions={
            <Link to={newLink} className="focus-ring inline-flex h-10 items-center gap-2 rounded-md bg-sky-500 px-4 text-sm font-medium text-zinc-950 hover:bg-sky-400">
              <Plus size={16} />
              {t('add')}
            </Link>
          }
        />
      )}
      {vehicleId !== undefined && (
        <div className="mb-4 flex justify-end">
          <Link to={newLink} className="focus-ring inline-flex h-9 items-center gap-2 rounded-md bg-sky-500 px-3 text-sm font-medium text-zinc-950 hover:bg-sky-400">
            <Plus size={16} />
            {t('add')}
          </Link>
        </div>
      )}
      <form
        className="mb-4 grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5"
        onSubmit={(e) => { e.preventDefault(); setParam('q', q.trim()) }}
      >
        {vehicleId === undefined && (
          <select aria-label={t('fields.vehicle')} className={inputClass} value={params.get('vehicle') ?? ''} onChange={(e) => { setParam('vehicle', e.target.value) }}>
            <option value="">{t('filters.allVehicles')}</option>
            {vehicles.data?.items.map((v) => <option key={v.id} value={v.id}>{vehicleName(v)}</option>)}
          </select>
        )}
        <select aria-label={t('fields.category')} className={inputClass} value={params.get('category') ?? ''} onChange={(e) => { setParam('category', e.target.value) }}>
          <option value="">{t('filters.allCategories')}</option>
          {categories.data?.items.map((c) => <option key={c.id} value={c.id}>{categoryLabel(c, t)}</option>)}
        </select>
        <input type="date" aria-label={t('filters.from')} className={inputClass} value={params.get('from') ?? ''} onChange={(e) => { setParam('from', e.target.value) }} />
        <input type="date" aria-label={t('filters.to')} className={inputClass} value={params.get('to') ?? ''} onChange={(e) => { setParam('to', e.target.value) }} />
        <input type="search" aria-label={t('filters.search')} placeholder={t('filters.search')} className={inputClass} value={q} onChange={(e) => { setQ(e.target.value) }} onBlur={() => { setParam('q', q.trim()) }} />
      </form>
      {logs.isPending && <Loading />}
      {logs.isError && <ErrorBanner message={errorText(logs.error)} onRetry={() => void logs.refetch()} />}
      {logs.isSuccess && items.length === 0 && <EmptyState title={t('empty')} />}
      {items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t('fields.date')}</th>
              {vehicleId === undefined && <th>{t('fields.vehicle')}</th>}
              <th>{t('fields.odometer')}</th>
              <th>{t('items.title')}</th>
              <th className="text-right">{t('fields.total')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((l) => {
              const v = vehicleById.get(l.vehicle_id)
              return (
                <tr key={l.id} className="hover:bg-zinc-900/60">
                  <td className="tabular-nums">
                    <Link to={`/service/${l.id}`} className="focus-ring text-sky-300 hover:underline">{fmt.date(l.service_date)}</Link>
                  </td>
                  {vehicleId === undefined && <td className="text-zinc-300">{v ? vehicleName(v) : ''}</td>}
                  <td className="tabular-nums text-zinc-300">{fmt.km(l.odometer_km)}</td>
                  <td>
                    <div className="flex max-w-md flex-wrap gap-1">
                      {l.items.map((it) => <Badge key={it.id}>{categoryLabel(catById.get(it.category_id), t)}</Badge>)}
                    </div>
                  </td>
                  <td className="text-right tabular-nums text-zinc-100">{fmt.money(l.total_cents)}</td>
                </tr>
              )
            })}
          </tbody>
        </Table>
      )}
      {items.length > 0 && (
        <div className="mt-3 flex items-center justify-between text-sm text-zinc-400">
          <span>{t('common:pagination.showing', { count: items.length, total })}</span>
          {logs.hasNextPage && (
            <Button size="sm" onClick={() => void logs.fetchNextPage()} disabled={logs.isFetchingNextPage}>
              {t('common:actions.loadMore')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

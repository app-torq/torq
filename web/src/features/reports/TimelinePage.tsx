import { Link } from 'react-router'
import { useInfiniteQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { FileText, Hammer, Wrench } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import type { ListResponse, TimelineItem } from '../../lib/types'
import { Button } from '../../components/Button'
import { EmptyState, ErrorBanner, Loading } from '../../components/ui'
import { useVehicleContext } from '../vehicles/VehicleDetailLayout'
import { categoryLabel, useCategories } from '../service-log/hooks'

const PAGE = 50

/** Read-only chronological history combining services and completed repairs (FR-046). */
export function TimelinePage() {
  const v = useVehicleContext()
  const { t } = useTranslation('reports')
  const fmt = useFormat()
  const errorText = useErrorText()
  const categories = useCategories(true)
  const catById = new Map(categories.data?.items.map((c) => [c.id, c]))
  const timeline = useInfiniteQuery({
    queryKey: queryKeys.timeline(v.id),
    queryFn: ({ pageParam }) => api.get<ListResponse<TimelineItem>>(`/vehicles/${v.id}/timeline`, { limit: PAGE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (last, all) => {
      const n = all.reduce((s, p) => s + p.items.length, 0)
      return n < last.total ? n : undefined
    },
  })
  const items = timeline.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Link to={`/vehicles/${v.id}/history/print`} className="focus-ring inline-flex h-9 items-center gap-2 rounded-md border border-zinc-700 bg-zinc-900 px-3 text-sm hover:bg-zinc-800">
          <FileText size={15} />
          {t('timeline.export')}
        </Link>
      </div>
      {timeline.isPending && <Loading />}
      {timeline.isError && <ErrorBanner message={errorText(timeline.error)} />}
      {timeline.isSuccess && items.length === 0 && <EmptyState title={t('timeline.empty')} />}
      <ol className="relative space-y-3 border-l border-zinc-800 pl-6">
        {items.map((it) => {
          const Icon = it.kind === 'service' ? Wrench : Hammer
          const label = it.kind === 'service' ? (it.category_ids ?? []).map((id) => categoryLabel(catById.get(id), t)).join(', ') : it.title ?? ''
          return (
            <li key={`${it.kind}-${it.id}`} className="relative">
              <span className={`absolute -left-[35px] top-1 flex h-6 w-6 items-center justify-center rounded-full border ${it.kind === 'service' ? 'border-sky-500/40 bg-sky-500/15 text-sky-300' : 'border-emerald-500/40 bg-emerald-500/15 text-emerald-300'}`}>
                <Icon size={13} />
              </span>
              <Link to={it.kind === 'service' ? `/service/${it.id}` : `/repairs/${it.id}`} className="focus-ring block rounded-lg border border-zinc-800 bg-zinc-900 p-3 hover:border-zinc-700">
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-zinc-400">
                  <span className="tabular-nums">{fmt.date(it.date)} · {fmt.km(it.odometer_km)}</span>
                  <span>{t(`timeline.${it.kind}`)}</span>
                </div>
                <div className="mt-1 flex items-center justify-between gap-2">
                  <span className="text-sm text-zinc-100">{label}</span>
                  <span className="text-sm tabular-nums text-zinc-300">{fmt.money(it.total_cents)}</span>
                </div>
              </Link>
            </li>
          )
        })}
      </ol>
      {timeline.hasNextPage && (
        <Button onClick={() => void timeline.fetchNextPage()} disabled={timeline.isFetchingNextPage}>{t('common:actions.loadMore')}</Button>
      )}
    </div>
  )
}

import { Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import { useFormat } from '../../lib/useFormat'
import { Badge, Card } from '../../components/ui'
import { statusTone } from '../repairs/hooks'
import { useDashboard } from './AttentionList'

export function OpenRepairs() {
  const { t } = useTranslation(['vehicles', 'repairs'])
  const fmt = useFormat()
  const dash = useDashboard()
  const items = dash.data?.open_repairs ?? []
  return (
    <Card title={t('dashboard.openRepairs')}>
      {dash.data && items.length === 0 && <p className="text-sm text-zinc-400">{t('dashboard.noOpenRepairs')}</p>}
      <ul className="divide-y divide-zinc-800">
        {items.map((r) => (
          <li key={r.id} className="flex flex-wrap items-center gap-2 py-2">
            <Link to={`/repairs/${r.id}`} className="focus-ring min-w-0 flex-1 truncate text-sm text-zinc-100 hover:text-sky-300">{r.title}</Link>
            <Badge tone={statusTone[r.status]}>{t(`repairs:statuses.${r.status}`)}</Badge>
            {r.part_status_counts.needed + r.part_status_counts.ordered > 0 && (
              <span className="text-xs text-zinc-400">{t('repairs:parts.countSummary', r.part_status_counts)}</span>
            )}
            {r.parts_total_cents > 0 && <span className="text-xs tabular-nums text-zinc-400">{fmt.money(r.parts_total_cents)}</span>}
          </li>
        ))}
      </ul>
    </Card>
  )
}

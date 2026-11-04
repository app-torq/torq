import { useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Pencil, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useFormat } from '../../lib/useFormat'
import { useErrorText } from '../../lib/errors'
import { vehicleName, type ServiceLog } from '../../lib/types'
import { Button } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { Card, DefinitionList, ErrorBanner, Loading, Notice, PageHeader, Table } from '../../components/ui'
import { invalidateOdometerDependents, useVehicle } from '../vehicles/hooks'
import { categoryLabel, useCategories } from './hooks'
import { ServiceLogAttachments } from './ServiceLogAttachments'
import { BackLink } from '../../components/BackLink'

export function ServiceLogDetailPage() {
  const { id } = useParams()
  const logId = Number(id)
  const { t } = useTranslation('service')
  const fmt = useFormat()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const errorText = useErrorText()
  const categories = useCategories(true)
  const [deleting, setDeleting] = useState(false)
  const warnings = (location.state as { warnings?: string[] } | null)?.warnings ?? []

  const log = useQuery({
    queryKey: queryKeys.serviceLog(logId),
    queryFn: () => api.get<{ service_log: ServiceLog }>(`/service-logs/${logId}`).then((r) => r.service_log),
  })
  const vehicle = useVehicle(log.data?.vehicle_id ?? 0)
  const remove = useMutation({
    mutationFn: () => api.del(`/service-logs/${logId}`),
    onSuccess: async () => {
      qc.removeQueries({ queryKey: queryKeys.serviceLog(logId) })
      await Promise.all([invalidateOdometerDependents(qc), qc.invalidateQueries({ queryKey: queryKeys.serviceLogs })])
      void navigate('/service')
    },
  })

  if (log.isPending) return <Loading />
  if (log.isError) return <ErrorBanner message={errorText(log.error)} />
  const l = log.data
  const catById = new Map(categories.data?.items.map((c) => [c.id, c]))
  const none = t('common:state.none')

  return (
    <div className="space-y-6">
      <BackLink fallback="/service" />
      <PageHeader
        title={t('detail')}
        subtitle={vehicle.data ? <Link className="text-sky-300 hover:underline" to={`/vehicles/${vehicle.data.id}`}>{vehicleName(vehicle.data)}</Link> : undefined}
        actions={
          <>
            <Link to={`/service/${l.id}/edit`} className="focus-ring inline-flex h-10 items-center gap-2 rounded-md border border-zinc-700 bg-zinc-900 px-4 text-sm hover:bg-zinc-800">
              <Pencil size={15} />
              {t('common:actions.edit')}
            </Link>
            <Button variant="danger" onClick={() => { setDeleting(true) }}>
              <Trash2 size={15} />
              {t('common:actions.delete')}
            </Button>
          </>
        }
      />
      {warnings.map((w) => <Notice key={w} tone="amber">{t(`warnings.${w}`)}</Notice>)}
      <Card>
        <DefinitionList
          items={[
            { label: t('fields.date'), value: fmt.date(l.service_date) },
            { label: t('fields.odometer'), value: <span className="tabular-nums">{fmt.km(l.odometer_km)}</span> },
            { label: t('fields.total'), value: <span className="tabular-nums">{fmt.money(l.total_cents)}</span> },
            ...(l.extra_cost_cents > 0 ? [{ label: l.extra_cost_label || t('fields.extraCost'), value: <span className="tabular-nums">{fmt.money(l.extra_cost_cents)}</span> }] : []),
          ]}
        />
        {l.notes && <p className="mt-4 whitespace-pre-wrap border-t border-zinc-800 pt-4 text-sm text-zinc-300">{l.notes}</p>}
      </Card>
      <Card title={t('items.title')}>
        <Table>
          <thead>
            <tr>
              <th>{t('fields.category')}</th>
              <th>{t('fields.brand')}</th>
              <th>{t('fields.specification')}</th>
              <th>{t('fields.quantity')}</th>
              <th>{t('fields.oemPart')}</th>
              <th>{t('fields.aftermarketPart')}</th>
              <th>{t('fields.costType')}</th>
              <th className="text-right">{t('fields.cost')}</th>
            </tr>
          </thead>
          <tbody>
            {l.items.map((it) => (
              <tr key={it.id}>
                <td className="text-zinc-100">{categoryLabel(catById.get(it.category_id), t)}</td>
                <td>{it.brand || none}</td>
                <td>{it.specification || none}</td>
                <td className="tabular-nums">{it.quantity_ml !== null && it.quantity_unit ? fmt.quantity(it.quantity_ml, it.quantity_unit) : none}</td>
                <td className="font-mono text-xs">{it.oem_part_number || none}</td>
                <td className="font-mono text-xs">{it.aftermarket_part_number || none}</td>
                <td className="text-zinc-400">{t(`costTypes.${it.cost_type}`)}</td>
                <td className="text-right tabular-nums">{it.cost_cents !== null ? fmt.money(it.cost_cents) : none}</td>
              </tr>
            ))}
          </tbody>
        </Table>
      </Card>
      <ServiceLogAttachments log={l} />
      <ConfirmDialog
        open={deleting}
        title={t('deleteTitle')}
        body={t('deleteBody')}
        confirmLabel={t('common:actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { remove.mutate() }}
        onClose={() => { setDeleting(false); remove.reset() }}
      />
    </div>
  )
}

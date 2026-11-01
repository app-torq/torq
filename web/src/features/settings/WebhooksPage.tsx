import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useErrorText } from '../../lib/errors'
import type { ListResponse, Webhook } from '../../lib/types'
import { Button, IconButton } from '../../components/Button'
import { ConfirmDialog } from '../../components/Dialog'
import { Badge, Card, EmptyState, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { WebhookForm } from './WebhookForm'
import { WebhookTestPanel } from './WebhookTestPanel'
import { DeliveryLog } from './DeliveryLog'

export function WebhooksPage() {
  const { t } = useTranslation('webhooks')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const hooks = useQuery({ queryKey: queryKeys.webhooks, queryFn: () => api.get<ListResponse<Webhook>>('/webhooks') })
  const [editing, setEditing] = useState<Webhook | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Webhook | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const refresh = () => qc.invalidateQueries({ queryKey: queryKeys.webhooks })
  const toggle = useMutation({
    mutationFn: (w: Webhook) => api.patch(`/webhooks/${w.id}`, { enabled: !w.enabled }),
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/webhooks/${id}`),
    onSuccess: async () => { setDeleting(null); setSelected(null); await refresh() },
  })

  return (
    <div>
      <PageHeader
        title={t('title')}
        subtitle={t('intro')}
        actions={
          <Button variant="primary" onClick={() => { setEditing('new') }}>
            <Plus size={16} />
            {t('add')}
          </Button>
        }
      />
      {toggle.isError && <div className="mb-3"><ErrorBanner message={errorText(toggle.error)} /></div>}
      {hooks.isPending && <Loading />}
      {hooks.isError && <ErrorBanner message={errorText(hooks.error)} onRetry={() => void hooks.refetch()} />}
      {hooks.data?.items.length === 0 && <EmptyState title={t('empty')} />}
      <div className="space-y-4">
        {hooks.data?.items.map((w) => {
          const events = (Object.keys(w.events) as (keyof Webhook['events'])[]).filter((k) => w.events[k])
          return (
            <Card
              key={w.id}
              title={
                <span className="flex flex-wrap items-center gap-2">
                  {w.name}
                  <Badge tone="sky">{t(`presets.${w.preset}`)}</Badge>
                  {w.enabled ? <Badge tone="emerald">{t('enabled')}</Badge> : <Badge>{t('disabled')}</Badge>}
                </span>
              }
              actions={
                <>
                  <Button size="sm" variant="ghost" onClick={() => { toggle.mutate(w) }}>
                    {w.enabled ? t('disabled') : t('enabled')}
                  </Button>
                  <IconButton aria-label={t('common:actions.edit')} onClick={() => { setEditing(w) }}><Pencil size={15} /></IconButton>
                  <IconButton aria-label={t('common:actions.delete')} onClick={() => { setDeleting(w) }}><Trash2 size={15} /></IconButton>
                </>
              }
            >
              <div className="space-y-3">
                <p className="break-all font-mono text-xs text-zinc-400">{w.url}</p>
                <div className="flex flex-wrap gap-1">
                  {events.length === 0 && <span className="text-xs text-amber-300">{t('noEvents')}</span>}
                  {events.map((e) => <Badge key={e}>{t(`events.${e}`)}</Badge>)}
                </div>
                <WebhookTestPanel webhook={w} onSent={() => { setSelected(w.id); void qc.invalidateQueries({ queryKey: ['webhooks', 'deliveries', w.id] }) }} />
                <details open={selected === w.id} onToggle={(e) => { if ((e.target as HTMLDetailsElement).open) setSelected(w.id) }}>
                  <summary className="cursor-pointer text-sm text-zinc-400 hover:text-zinc-200">{t('log.title')}</summary>
                  {selected === w.id && <div className="mt-3"><DeliveryLog webhookId={w.id} /></div>}
                </details>
              </div>
            </Card>
          )
        })}
      </div>
      {editing !== null && <WebhookForm webhook={editing === 'new' ? null : editing} onClose={() => { setEditing(null) }} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('deleteTitle')}
        body={t('deleteBody', { name: deleting?.name ?? '' })}
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
